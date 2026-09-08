// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/masterzen/winrm"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// Windows 主机支持：WinRM 客户端封装（执行/指标/轮换共用）

// IsWindows 主机是否 Windows
func IsWindows(h *model.Host) bool {
	return h.OSType == "windows"
}

// WinRMEndpoints WinRM 端口解析（默认 5985）
func WinRMPortOf(h *model.Host) int {
	if h.WinRMPort > 0 {
		return h.WinRMPort
	}
	return 5985
}

// WinRMClientFor 构造目标主机的 WinRM 客户端（HTTP；HTTPS 5986 时跳过自签证书校验，与 LDAP TLS 同策略）
func WinRMClientFor(h *model.Host, username, password string) (*winrm.Client, error) {
	if !IsWindows(h) {
		return nil, fmt.Errorf("仅 Windows 主机支持 WinRM")
	}
	port := WinRMPortOf(h)
	user := username
	if user == "" {
		user = h.Username
	}
	var pass string
	if password != "" {
		pass = password
	} else if h.Password != "" {
		p, err := pkg.Decrypt(h.Password)
		if err != nil {
			return nil, err
		}
		pass = p
	}
	endpoint := winrm.NewEndpoint(h.IP, port, false, true, nil, nil, nil, 0)
	c, err := winrm.NewClient(endpoint, user, pass)
	if err != nil {
		return nil, fmt.Errorf("WinRM 连接失败: %w", err)
	}
	return c, nil
}

// WinRMRun 在 Windows 主机执行 PowerShell 命令（固定 InvariantCulture 输出，避免本地化解析差异）
func WinRMRun(h *model.Host, username, password, command string, timeoutSec int) (string, int, error) {
	c, err := WinRMClientFor(h, username, password)
	if err != nil {
		return "", -1, err
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	// 强制文化不变性 + UTF8 输出
	ps := "[Console]::OutputEncoding=[Text.Encoding]::UTF8; " + command
	encoded := winrm.Powershell(ps)
	type result struct {
		out  string
		code int
		err  error
	}
	done := make(chan result, 1)
	go func() {
		stdout, stderr, code, rerr := c.RunWithString(encoded, "")
		r := result{out: stdout, code: code, err: rerr}
		if rerr != nil {
			// 非零退出码在库中会带 error 返回；输出仍有值时按业务错误处理
			r.err = nil
			r.out += "\n[winrm] " + rerr.Error()
			r.code = 1
		}
		if stderr != "" {
			r.out += "\n[stderr] " + strings.TrimSpace(stderr)
		}
		done <- r
	}()
	select {
	case r := <-done:
		return strings.TrimSpace(r.out), r.code, r.err
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		return "", -1, fmt.Errorf("WinRM 执行超时（%ds）", timeoutSec)
	}
}

// defaultCredential 取主机默认 OS 账号（无则 nil）
func defaultCredential(h *model.Host) *model.HostCredential {
	var c model.HostCredential
	if err := model.DB.Where("host_id = ?", h.ID).Order("is_default DESC, id ASC").First(&c).Error; err != nil {
		return nil
	}
	return &c
}
