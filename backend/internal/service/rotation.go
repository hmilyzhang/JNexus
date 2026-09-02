// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"autoops/internal/model"
	"autoops/internal/pkg"
)

// chpasswd 安全字符集：不含单引号/反斜杠/$ 等 shell 敏感字符
const pwdCharset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789#@%+_=."

// GetRotationEnabled 全局密码轮换开关
func GetRotationEnabled() bool {
	return SystemConfigMap()["rotation_enabled"] == "true"
}

// GenerateStrongPassword 生成 20 位随机强密码（避开 shell 敏感字符）
func GenerateStrongPassword() (string, error) {
	out := make([]byte, 20)
	max := big.NewInt(int64(len(pwdCharset)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = pwdCharset[n.Int64()]
	}
	return string(out), nil
}

// runCapture 执行命令并捕获输出，返回 (输出, 退出码, 错误)
func runCapture(cli *gossh.Client, cmd string) (string, int, error) {
	sess, err := cli.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	code := 0
	if err != nil {
		// 兼容不发送退出码的服务端（ExitMissingError / EOF）：有输出即视为正常结束
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

// RotateCredentialPassword 轮换单个 OS 账号密码：
// 本地账号检测 → 生成随机密码 → chpasswd → 新密码加密落库。
// LDAP/域账号（非 /etc/passwd 本地账号）自动跳过并返回 errLDAPSkip。
func RotateCredentialPassword(host *model.Host, cred *model.HostCredential) (string, error) {
	if cred.AuthType != "password" || cred.Password == "" {
		return "", fmt.Errorf("仅密码认证的账号支持轮换")
	}
	curPwd, err := pkg.Decrypt(cred.Password)
	if err != nil {
		return "", fmt.Errorf("解密当前密码失败: %w", err)
	}

	cli, err := dialWithPassword(*host, cred.Username, curPwd)
	if err != nil {
		return "", fmt.Errorf("密码登录失败: %w", err)
	}
	defer cli.Close()

	isLocal, code, err := runCapture(cli, fmt.Sprintf("grep -q '^%s:' /etc/passwd && echo LOCAL || echo REMOTE", cred.Username))
	if err != nil {
		return "", fmt.Errorf("账号类型检测失败: %w", err)
	}
	if strings.Contains(isLocal, "REMOTE") {
		return "", errLDAPSkip
	}

	newPwd, err := GenerateStrongPassword()
	if err != nil {
		return "", err
	}
	// chpasswd 从 stdin 读取，密码不出现在命令行参数/进程列表中
	out, code, err := runCapture(cli, fmt.Sprintf("printf '%%s\\n' '%s:%s' | chpasswd", cred.Username, newPwd))
	if err != nil {
		return "", fmt.Errorf("执行 chpasswd 失败: %w", err)
	}
	if code != 0 {
		return "", fmt.Errorf("chpasswd 退出码 %d: %s（可能被密码策略拒绝）", code, out)
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

var errLDAPSkip = fmt.Errorf("LDAP/域账号，不执行本地密码轮换")

// ScanDueRotations 到期轮换扫描（调度循环调用；全局开关关闭时不执行）
func ScanDueRotations() {
	defer func() { recover() }()
	if !GetRotationEnabled() {
		return
	}
	var creds []model.HostCredential
	if err := model.DB.Where("rotate_enabled = ? AND auth_type = ?", true, "password").Find(&creds).Error; err != nil {
		return
	}
	now := time.Now()
	for i := range creds {
		c := creds[i]
		due := c.LastRotatedAt == nil || now.Sub(*c.LastRotatedAt) > time.Duration(c.RotateDays)*24*time.Hour
		if !due {
			continue
		}
		go rotateOne(c.ID)
		time.Sleep(500 * time.Millisecond) // 轻微错峰，避免同时连爆目标机
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
		_, err := RotateCredentialPassword(&host, &cred)
		if err != nil {
			result = "轮换失败: " + err.Error()
		} else {
			result = "轮换成功"
		}
	}
	model.DB.Model(&cred).Updates(map[string]any{
		"last_rotated_at":      time.Now(),
		"last_rotation_result": result,
	})
	notifyRotation(cred, result)
}

// NotifyRotationResult 对外暴露的轮换结果通知（手动轮换也走这里）
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
	subject := fmt.Sprintf("[AutoOps] 密码轮换 %s — %s (%s)", result, cred.Username, cred.Label)
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
