// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"encoding/base64"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/sshpool"
)

// chpasswd safe character set: excludes shell-sensitive characters such as single quotes, backslashes, and $
const (
	pwdUpper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	pwdLower = "abcdefghijkmnopqrstuvwxyz"
	pwdDigit = "23456789"
	pwdSpec  = "#@%+_=."
)

// RotationPolicy is the password rotation policy (system config)
type RotationPolicy struct {
	Length     int
	Complexity string // high: upper+lower+digits+special; medium: letters+digits; low: lowercase+digits
	Days       int    // default rotation period in days
}

// GetRotationEnabled returns the global password rotation switch
func GetRotationEnabled() bool {
	return SystemConfigMap()["rotation_enabled"] == "true"
}

// GetRotationPolicy reads the rotation policy (with defaults and boundary corrections)
func GetRotationPolicy() RotationPolicy {
	m := SystemConfigMap()
	p := RotationPolicy{Complexity: "high", Days: 90, Length: 20}
	fmt.Sscanf(m["rotation_length"], "%d", &p.Length)
	if p.Length < 8 {
		p.Length = 8
	}
	if p.Length > 64 {
		p.Length = 64
	}
	switch m["rotation_complexity"] {
	case "medium", "low":
		p.Complexity = m["rotation_complexity"]
	}
	fmt.Sscanf(m["rotation_days"], "%d", &p.Days)
	if p.Days < 1 {
		p.Days = 90
	}
	return p
}

// GenerateStrongPassword generates a random password per the policy (guarantees at least one char per class, avoids shell-sensitive characters)
func GenerateStrongPassword(p RotationPolicy) (string, error) {
	classes := []string{pwdUpper, pwdLower, pwdDigit}
	switch p.Complexity {
	case "high":
		classes = append(classes, pwdSpec)
	case "low":
		classes = []string{pwdLower, pwdDigit}
	}
	all := strings.Join(classes, "")
	out := make([]byte, 0, p.Length)
	// at least one char per class
	for _, cls := range classes {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(cls))))
		if err != nil {
			return "", err
		}
		out = append(out, cls[n.Int64()])
	}
	for len(out) < p.Length {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(all))))
		if err != nil {
			return "", err
		}
		out = append(out, all[n.Int64()])
	}
	// Fisher-Yates shuffle
	for i := len(out) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := n.Int64()
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

// runCapture runs a command and captures output, returning (output, exit code, error)
func runCapture(cli *gossh.Client, cmd string) (string, int, error) {
	sess, err := cli.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	code := 0
	if err != nil {
		// tolerate servers that never send an exit code (ExitMissingError / EOF): output present means normal completion
		var eme *gossh.ExitMissingError
		if ee, ok := err.(*gossh.ExitError); ok {
			code = ee.ExitStatus()
		} else if errors.As(err, &eme) || strings.Contains(err.Error(), "EOF") {
			err = nil
		} else {
			return strings.TrimSpace(string(out)), -1, err
		}
	}
	return strings.TrimSpace(string(out)), code, nil
}

// RotateCredentialPassword rotates a single OS account password: a random password
// is generated, applied on the host, and stored back encrypted. LDAP/domain accounts
// (not in /etc/passwd) are skipped automatically with errLDAPSkip.
// Linux privilege chain (vault pattern): chpasswd is root-only, so first the account
// itself is tried (key or password login), then the host's other stored credentials
// (root-like first). uid 0 runs chpasswd directly, everyone else via `sudo -n` —
// which only works for NOPASSWD sudoers.
func RotateCredentialPassword(host *model.Host, cred *model.HostCredential) (string, error) {
	if cred.Password == "" {
		return "", fmt.Errorf("账号未保存密码，无法轮换")
	}
	if IsWindows(host) {
		return rotateWindowsPassword(host, cred)
	}

	policy := GetRotationPolicy()
	newPwd, err := GenerateStrongPassword(policy)
	if err != nil {
		return "", err
	}

	// privilege chain: the account itself first, then the host's other credentials
	attempts := []model.HostCredential{*cred}
	var siblings []model.HostCredential
	model.DB.Where("host_id = ? AND id <> ? AND ((auth_type = 'password' AND password <> '') OR (auth_type = 'key' AND ssh_key_id IS NOT NULL))",
		host.ID, cred.ID).Order("is_default DESC, id ASC").Find(&siblings)
	sort.Slice(siblings, func(i, j int) bool {
		if (siblings[i].Username == "root") != (siblings[j].Username == "root") {
			return siblings[i].Username == "root" // root first
		}
		return siblings[i].ID < siblings[j].ID
	})
	attempts = append(attempts, siblings...)

	var lastErr error
	for _, actor := range attempts {
		cli, derr := sshpool.ClientForCredential(host, &actor)
		if derr != nil {
			lastErr = derr
			continue
		}
		// the target must be a local account; domain accounts are never rotated
		isLocal, _, lerr := runCapture(cli, fmt.Sprintf("grep -q '^%s:' /etc/passwd && echo LOCAL || echo REMOTE", cred.Username))
		if lerr != nil {
			cli.Close()
			lastErr = fmt.Errorf("账号类型检测失败: %w", lerr)
			continue
		}
		if strings.Contains(isLocal, "REMOTE") {
			cli.Close()
			return "", errLDAPSkip
		}
		// chpasswd is root-only: uid 0 runs it directly, everyone else via NOPASSWD sudo
		uidOut, _, uerr := runCapture(cli, "id -u")
		var cmd string
		if uerr == nil && strings.TrimSpace(uidOut) == "0" {
			cmd = fmt.Sprintf("printf '%%s\\n' '%s:%s' | chpasswd", cred.Username, newPwd)
		} else {
			cmd = fmt.Sprintf("printf '%%s\\n' '%s:%s' | sudo -n chpasswd", cred.Username, newPwd)
		}
		// chpasswd reads from stdin so the password never appears in command-line args / process lists
		out, code, cerr := runCapture(cli, cmd)
		cli.Close()
		if cerr == nil && code == 0 {
			enc, eerr := pkg.Encrypt(newPwd)
			if eerr != nil {
				return "", eerr
			}
			model.DB.Model(cred).Updates(map[string]any{
				"password":             enc,
				"last_rotated_at":      time.Now(),
				"last_rotation_result": "轮换成功",
			})
			return newPwd, nil
		}
		if cerr != nil {
			lastErr = fmt.Errorf("执行 chpasswd 失败: %w", cerr)
		} else {
			lastErr = fmt.Errorf("chpasswd 退出码 %d: %s", code, strings.TrimSpace(out))
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无可用登录凭据")
	}
	if len(siblings) == 0 {
		return "", fmt.Errorf("普通账号无 chpasswd 权限（root 专属命令），且主机上没有其它可协助改密的凭据: %w", lastErr)
	}
	return "", fmt.Errorf("自身及主机上 %d 个其它凭据均无法完成改密（需要 root 或 NOPASSWD sudo）: %w", len(siblings), lastErr)
}

// rotateWindowsPassword rotates a local Windows account password via WinRM
// (Set-LocalUser). The new password is base64-wrapped so nothing can break out
// of the PowerShell string; domain accounts are skipped like LDAP on Linux.
func rotateWindowsPassword(host *model.Host, cred *model.HostCredential) (string, error) {
	curPwd, err := pkg.Decrypt(cred.Password)
	if err != nil {
		return "", fmt.Errorf("解密当前密码失败: %w", err)
	}

	user := cred.Username
	if strings.Contains(user, "@") {
		return "", errLDAPSkip // UPN form = domain account
	}
	if i := strings.IndexByte(user, '\\'); i >= 0 {
		prefix := user[:i]
		if !strings.EqualFold(prefix, host.Name) && prefix != "." && prefix != strings.Split(host.IP, ".")[0] {
			return "", errLDAPSkip // another machine's domain prefix
		}
		user = user[i+1:]
	}

	// local vs domain detection via the local SAM database
	detect := fmt.Sprintf("if (Get-LocalUser -Name '%s' -ErrorAction SilentlyContinue) { 'LOCAL' } else { 'REMOTE' }", psQuote(user))
	out, _, err := WinRMRun(host, cred.Username, curPwd, detect, 30)
	if err != nil {
		return "", fmt.Errorf("WinRM 连接失败: %w", err)
	}
	if strings.Contains(out, "REMOTE") {
		return "", errLDAPSkip
	}

	policy := GetRotationPolicy()
	newPwd, err := GenerateStrongPassword(policy)
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(newPwd))
	cmd := fmt.Sprintf("$ErrorActionPreference='Stop'; $p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); Set-LocalUser -Name '%s' -Password (ConvertTo-SecureString $p -AsPlainText -Force); 'PWD-OK'",
		b64, psQuote(user))
	out, code, err := WinRMRun(host, cred.Username, curPwd, cmd, 45)
	if err != nil || code != 0 || !strings.Contains(out, "PWD-OK") {
		detail := strings.TrimSpace(out)
		if err != nil {
			return "", fmt.Errorf("Set-LocalUser 执行失败: %w", err)
		}
		return "", fmt.Errorf("Set-LocalUser 失败（可能被密码策略拒绝）: %s", detail)
	}

	enc, err := pkg.Encrypt(newPwd)
	if err != nil {
		return "", err
	}
	model.DB.Model(cred).Updates(map[string]any{
		"password":             enc,
		"last_rotated_at":      time.Now(),
		"last_rotation_result": "轮换成功",
	})
	return newPwd, nil
}

// psQuote escapes a value for single-quoted PowerShell string literals
func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

var errLDAPSkip = fmt.Errorf("LDAP/域账号，不执行本地密码轮换")

// ScanDueRotations scans for due rotations (called by the scheduler loop; no-op when the global switch is off)
func ScanDueRotations() {
	defer func() { recover() }()
	if !GetRotationEnabled() {
		return
	}
	var creds []model.HostCredential
	// rotatable = a stored (encrypted) password exists: password-auth accounts and
	// paired key accounts whose password was kept; LDAP/domain accounts are excluded
	if err := model.DB.Where("rotate_enabled = ? AND is_ldap = ? AND password <> ''", true, false).Find(&creds).Error; err != nil {
		return
	}
	policy := GetRotationPolicy()
	now := time.Now()
	for i := range creds {
		c := creds[i]
		days := c.RotateDays
		if days <= 0 {
			days = policy.Days
		}
		due := c.LastRotatedAt == nil || now.Sub(*c.LastRotatedAt) > time.Duration(days)*24*time.Hour
		if !due {
			continue
		}
		go rotateOne(c.ID)
		time.Sleep(500 * time.Millisecond) // slight staggering to avoid hammering target hosts simultaneously
	}
}

// RecordPasswordHistory archives a credential's new password (encrypted) into the
// history table, pruned to the most recent 24 entries per credential. Best-effort:
// failures are swallowed so password changes never break on bookkeeping.
func RecordPasswordHistory(credID uint, plainPwd, source, operator string) {
	defer func() { recover() }()
	if plainPwd == "" {
		return
	}
	enc, err := pkg.Encrypt(plainPwd)
	if err != nil {
		return
	}
	if err := model.DB.Create(&model.CredentialPasswordHistory{
		CredentialID: credID, Password: enc, Source: source, Operator: operator, ChangedAt: time.Now(),
	}).Error; err != nil {
		return
	}
	var old []model.CredentialPasswordHistory
	if err := model.DB.Where("credential_id = ?", credID).Order("id DESC").Offset(24).Find(&old).Error; err == nil && len(old) > 0 {
		ids := make([]uint, 0, len(old))
		for _, o := range old {
			ids = append(ids, o.ID)
		}
		model.DB.Where("id IN ?", ids).Delete(&model.CredentialPasswordHistory{})
	}
}

func rotateOne(credID uint) {
	defer func() { recover() }()
	var cred model.HostCredential
	if err := model.DB.First(&cred, credID).Error; err != nil || !cred.RotateEnabled {
		return
	}
	var host model.Host
	if err := model.DB.First(&host, cred.HostID).Error; err != nil {
		return
	}
	var result string
	if cred.IsLDAP {
		result = "LDAP/域账号，跳过轮换"
	} else {
		if newPwd, err := RotateCredentialPassword(&host, &cred); err != nil {
			result = "轮换失败: " + err.Error()
		} else {
			result = "轮换成功"
			RecordPasswordHistory(cred.ID, newPwd, "scheduled", "system")
		}
	}
	model.DB.Model(&cred).Updates(map[string]any{
		"last_rotated_at":      time.Now(),
		"last_rotation_result": result,
	})
	notifyRotation(cred, result)
}

// NotifyRotationResult is the exported rotation result notification (manual rotation also goes through here)
func NotifyRotationResult(cred model.HostCredential, result string, err error) {
	if err != nil && result == "" {
		result = "轮换失败: " + err.Error()
	}
	notifyRotation(cred, result)
}

func notifyRotation(cred model.HostCredential, result string) {
	defer func() { recover() }()
	smtp := LoadSMTPSettings()
	if !smtp.Enabled || !smtp.Notify || len(smtp.Recipients) == 0 {
		return
	}
	subject := fmt.Sprintf("[JNexus] 密码轮换 %s — %s (%s)", result, cred.Username, cred.Label)
	body := fmt.Sprintf(`<p>OS 账号密码轮换结果：<b>%s</b></p>
<p>主机 ID: %d &nbsp; 账号: %s &nbsp; 用途: %s<br/>时间: %s</p>
<p style="color:#909399;font-size:12px">新密码已加密保存，不会通过邮件发送。By JJ Zhang Version 1.0</p>`,
		htmlEscape(result), cred.HostID, htmlEscape(cred.Username), htmlEscape(cred.Label),
		time.Now().Format("2006-01-02 15:04:05"))
	_ = SendMail(smtp, smtp.Recipients, subject, body)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;")
	return r.Replace(s)
}
