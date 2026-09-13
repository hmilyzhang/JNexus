// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/masterzen/winrm"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// Windows host support: WinRM client wrapper (shared by execution/metrics/rotation)

// IsWindows reports whether the host is Windows
func IsWindows(h *model.Host) bool {
	return h.OSType == "windows"
}

// WinRMPortOf resolves the WinRM port (default 5985)
func WinRMPortOf(h *model.Host) int {
	if h.WinRMPort > 0 {
		return h.WinRMPort
	}
	return 5985
}

// WinRMClientFor builds a WinRM client for the target host: HTTP/HTTPS is negotiated automatically.
// When port 5986 (HTTPS) is reachable on the target, the encrypted connection is preferred with the
// default Basic transport (immune to NTLM blocking policies; Basic is only enabled over TLS).
// Otherwise it falls back to HTTP with NTLM negotiation (works for local and DOMAIN/user accounts).
func WinRMClientFor(h *model.Host, username, password string) (*winrm.Client, error) {
	if !IsWindows(h) {
		return nil, fmt.Errorf("仅 Windows 主机支持 WinRM")
	}
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

	params := winrm.DefaultParameters

	// auto-negotiation: prefer encryption when 5986 (HTTPS) is reachable; otherwise fall back to HTTP
	if tcpOpen(h.IP, 5986) {
		return winrm.NewClientWithParameters(
			winrm.NewEndpoint(h.IP, 5986, true, true, nil, nil, nil, 0), user, pass, params)
	}
	// Windows enables only Negotiate auth by default: attach the NTLM transport to complete the handshake automatically (domain accounts DOMAIN/user also work)
	params.TransportDecorator = func() winrm.Transporter { return winrm.NewClientNTLMWithDial(params.Dial) }
	return winrm.NewClientWithParameters(
		winrm.NewEndpoint(h.IP, WinRMPortOf(h), false, true, nil, nil, nil, 0), user, pass, params)
}

// tcpOpen quickly probes TCP port reachability (intranet refusal returns immediately; firewall drops take up to 1.5s)
func tcpOpen(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 1500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// WinRMRun executes a PowerShell command on a Windows host (pins InvariantCulture output to avoid locale-dependent parsing)
func WinRMRun(h *model.Host, username, password, command string, timeoutSec int) (string, int, error) {
	c, err := WinRMClientFor(h, username, password)
	if err != nil {
		return "", -1, err
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	// force culture invariance + UTF8 output
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
			// non-zero exit codes come back wrapped in an error by the library; when output is still present, treat it as a business error
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

// defaultCredential returns the host's default OS account (nil when absent)
func defaultCredential(h *model.Host) *model.HostCredential {
	var c model.HostCredential
	if err := model.DB.Where("host_id = ?", h.ID).Order("is_default DESC, id ASC").First(&c).Error; err != nil {
		return nil
	}
	return &c
}
