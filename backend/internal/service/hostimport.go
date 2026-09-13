// JNexus Ops Platform — By JJ Zhang, Version 1.0

package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// GenerateKeyPairRaw generates a key pair, stores it, and returns (installable public key line, key record)
func GenerateKeyPairRaw(comment string) (string, *model.SSHKey, error) {
	k, err := GenerateAndStoreKeyPair(fmt.Sprintf("%s-%s", comment, time.Now().Format("20060102150405")), comment)
	if err != nil {
		return "", nil, err
	}
	return KeyPairPublicLine(k), k, nil
}

// GenerateAndStoreKeyPair generates an ed25519 key pair, encrypts and stores it, and returns the SSHKey record
func GenerateAndStoreKeyPair(name, comment string) (*model.SSHKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return nil, err
	}
	pubLine := string(gossh.MarshalAuthorizedKey(sshPub))
	block, err := gossh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, err
	}
	privPEM := string(pem.EncodeToMemory(block))
	enc, err := pkg.Encrypt(privPEM)
	if err != nil {
		return nil, err
	}
	key := &model.SSHKey{Name: name, PublicKey: fmt.Sprintf("%s %s", comment, pubLine), PrivateKey: enc}
	if err := model.DB.Create(key).Error; err != nil {
		return nil, err
	}
	return key, nil
}

// KeyPairPublicLine returns the installable public key line of the key record (without comment)
func KeyPairPublicLine(key *model.SSHKey) string {
	// PublicKey stores "comment ssh-xxx base64..."; take the last type + data parts
	pub := key.PublicKey
	fields := splitFields(pub)
	if len(fields) >= 2 {
		return fields[len(fields)-2] + " " + fields[len(fields)-1]
	}
	return pub
}

func splitFields(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// InstallPubKeyWithPassword logs in to the target machine with username/password and writes the platform public key
// to authorized_keys, after which the target machine allows passwordless login with this key. The target is Linux.
func InstallPubKeyWithPassword(ip string, port int, username, password, pubLine string) error {
	if port == 0 {
		port = 22
	}
	cfg := &gossh.ClientConfig{
		User:            username,
		Auth:            []gossh.AuthMethod{gossh.Password(password)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	cli, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), cfg)
	if err != nil {
		return fmt.Errorf("密码登录失败: %w", err)
	}
	defer cli.Close()

	cmd := fmt.Sprintf(
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && "+
			"grep -qxF '%s' ~/.ssh/authorized_keys || echo '%s' >> ~/.ssh/authorized_keys; "+
			"chmod 600 ~/.ssh/authorized_keys", pubLine, pubLine)
	code, err := RunCommandBackground(cli, cmd)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("写入 authorized_keys 失败，退出码 %d", code)
	}
	return nil
}

// RunCommandBackground runs the command silently (output not captured) and returns the exit code
func RunCommandBackground(cli *gossh.Client, cmd string) (int, error) {
	sess, err := cli.NewSession()
	if err != nil {
		return -1, err
	}
	defer sess.Close()
	code := 0
	if err := sess.Run(cmd); err != nil {
		if ee, ok := err.(*gossh.ExitError); ok {
			code = ee.ExitStatus()
		} else {
			return -1, err
		}
	}
	return code, nil
}
