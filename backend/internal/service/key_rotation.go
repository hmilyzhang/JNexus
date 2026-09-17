// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// SSH platform key rotation: periodically generates a new ed25519 key pair,
// pushes it to every paired host (append new + remove old), then replaces the
// stored key material — so all credentials transparently use the new key.

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"

	gossh "golang.org/x/crypto/ssh"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// ---- Config ----

type KeyRotationConfig struct {
	Enabled bool
	Days    int
	LastRun *time.Time `json:"-"`
}

func LoadKeyRotationConfig() KeyRotationConfig {
	m := SystemConfigMap()
	cfg := KeyRotationConfig{Enabled: m["ssh_key_rotation_enabled"] == "true"}
	if n, e := strconv.Atoi(m["ssh_key_rotation_days"]); e == nil && n >= 7 {
		cfg.Days = n
	}
	if cfg.Days == 0 {
		cfg.Days = 30
	}
	if raw := strings.TrimSpace(m["ssh_key_rotation_last"]); raw != "" {
		if t, e := time.Parse(time.RFC3339, raw); e == nil {
			cfg.LastRun = &t
		}
	}
	return cfg
}

func SaveKeyRotationLast(t time.Time) {
	_ = SetSystemConfigs(map[string]string{"ssh_key_rotation_last": t.Format(time.RFC3339)})
}

// ---- Key material helpers ----

// generateKeyPairBytes returns (pubLine, privPEM) for a fresh ed25519 pair
func generateKeyPairBytes(comment string) (string, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return "", "", err
	}
	pubLine := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(sshPub)))
	block, err := gossh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return "", "", err
	}
	privPEM := string(pem.EncodeToMemory(block))
	return fmt.Sprintf("%s %s", comment, pubLine), privPEM, nil
}

// RotatePlatformSSHKey generates a new key pair, pushes it to every host paired
// with the platform key (via the OLD key), removes the OLD pub key, then
// replaces the stored key material. Returns (hosts_ok, hosts_fail, error).
func RotatePlatformSSHKey() (int, int, error) {
	old, err := EnsurePlatformKey()
	if err != nil {
		return 0, 0, err
	}
	oldPrivPEM, err := pkg.Decrypt(old.PrivateKey)
	if err != nil {
		return 0, 0, fmt.Errorf("旧私钥解密失败: %w", err)
	}
	signer, err := gossh.ParsePrivateKey([]byte(oldPrivPEM))
	if err != nil {
		return 0, 0, fmt.Errorf("旧私钥解析失败: %w", err)
	}

	// generate the new key pair
	pubLine, privPEM, err := generateKeyPairBytes("jnexus-platform-rotated")
	if err != nil {
		return 0, 0, err
	}

	// old pub line's unique base64 body for removal
	oldPubBody := old.PublicKey
	if i := strings.IndexByte(oldPubBody, ' '); i > 0 {
		oldPubBody = strings.Fields(oldPubBody)[1] // base64 body
	}

	// find all credentials that reference the platform key
	var creds []model.HostCredential
	model.DB.Where("ssh_key_id = ? AND auth_type = ?", old.ID, "key").Find(&creds)

	// collect unique host IDs
	hostIDs := map[uint]bool{}
	for _, c := range creds {
		hostIDs[c.HostID] = true
	}

	ok, fail := 0, 0
	for hid := range hostIDs {
		var h model.Host
		if err := model.DB.First(&h, hid).Error; err != nil {
			fail++
			continue
		}
		// install new pub + remove old pub (single SSH round-trip via old key)
		newB64 := pubLine
		if i := strings.IndexByte(newB64, ' '); i > 0 {
			newB64 = strings.Fields(newB64)[1]
		}
		cmd := fmt.Sprintf(
			"grep -qxF '%s' ~/.ssh/authorized_keys || echo '%s' >> ~/.ssh/authorized_keys; "+
				"sed -i '/%s/d' ~/.ssh/authorized_keys; "+
				"chmod 600 ~/.ssh/authorized_keys",
			newB64, newB64, oldPubBody)
		rcli, derr := dialWithSigner(&h, signer)
		if derr != nil {
			fail++
			continue
		}
		_, err = RunCommandOnClient(rcli, cmd)
		rcli.Close()
		if err != nil {
			fail++
			continue
		}
		ok++
	}

	// replace the stored key material (same SSHKey ID → credentials auto-update)
	old.PublicKey = pubLine
	enc, err := pkg.Encrypt(privPEM)
	if err != nil {
		return ok, fail, err
	}
	old.PrivateKey = enc
	if err := model.DB.Model(old).Updates(map[string]interface{}{
		"public_key": old.PublicKey, "private_key": old.PrivateKey,
	}).Error; err != nil {
		return ok, fail, err
	}

	SaveKeyRotationLast(time.Now())
	return ok, fail, nil
}

// ---- Scheduler hook ----

var keyRotMu sync.Mutex
var keyRotLastCheck time.Time

// CheckKeyRotation called from the monitor loop; runs at most once per hour
func CheckKeyRotation() {
	keyRotMu.Lock()
	if time.Since(keyRotLastCheck) < time.Hour {
		keyRotMu.Unlock()
		return
	}
	keyRotLastCheck = time.Now()
	keyRotMu.Unlock()

	cfg := LoadKeyRotationConfig()
	if !cfg.Enabled {
		return
	}
	if cfg.LastRun != nil && time.Since(*cfg.LastRun) < time.Duration(cfg.Days)*24*time.Hour {
		return
	}
	ok, fail, err := RotatePlatformSSHKey()
	if err != nil {
		fmt.Println("[key-rotation] error:", err.Error())
		return
	}
	fmt.Println("[key-rotation] done: ok=", ok, "fail=", fail)
	LogAlertEvent("ssh_key_rotation", "info", "platform", fmt.Sprintf("SSH 密钥已轮换: %d 台成功, %d 台失败", ok, fail))
}

// ---- SSH connect helper (using the old key's signer, not the pool) ----

func dialWithSigner(h *model.Host, signer gossh.Signer) (*gossh.Client, error) {
	port := h.Port
	if port == 0 {
		port = 22
	}
	cfg := &gossh.ClientConfig{
		User:            h.Username,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	cli, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", h.IP, port), cfg)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func RunCommandOnClient(cli *gossh.Client, cmd string) (string, error) {
	sess, err := cli.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}
