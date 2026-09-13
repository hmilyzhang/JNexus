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

	// Kerberos (domain environments): preferred over NTLM for CIS-hardened domains —
	// no Basic auth, no NTLM, works over TLS so targets keep AllowUnencrypted=false.
	// Requires: WinRM HTTPS listener on 5986 (AD CS auto-enrolled cert via GPO), a krb5.conf
	// reachable by the server, and a resolvable SPN (WSMAN/<fqdn>).
	if h.WinRMKerberos {
		return winRMKerberosClient(h, user, pass)
	}

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

// winRMKerberosClient builds the Kerberos transport for a domain-joined Windows host.
// Realm comes from the account UPN suffix (user@GLBANK.COM) or the winrm_krb5_realm setting.
func winRMKerberosClient(h *model.Host, user, pass string) (*winrm.Client, error) {
	if !tcpOpen(h.IP, 5986) {
		return nil, fmt.Errorf("Kerberos 认证需要 WinRM HTTPS（5986）：请在目标机配置 HTTPS 监听与企业证书")
	}
	realm := ""
	if i := strings.IndexByte(user, '@'); i >= 0 {
		realm = strings.ToUpper(user[i+1:])
		user = user[:i]
	}
	if realm == "" {
		realm = strings.ToUpper(strings.TrimSpace(SystemConfigMap()["winrm_krb5_realm"]))
	}
	if realm == "" {
		return nil, fmt.Errorf("Kerberos 缺少域（Realm）：账号需为 user@REALM 格式，或在系统配置 winrm_krb5_realm 中指定")
	}
	spn := strings.TrimSpace(h.WinRMSPN)
	if spn == "" {
		spn = "WSMAN/" + h.Name
	}
	port := 5986
	if h.WinRMPort == 5986 {
		port = h.WinRMPort
	}
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter {
		return winrm.NewClientKerberos(&winrm.Settings{
			WinRMUsername: user,
			WinRMPassword: pass,
			KrbRealm:      realm,
			KrbSpn:        spn,
			KrbConfig:     SystemConfigMap()["winrm_krb5_config"], // e.g. /etc/krb5.conf; empty = OS default path
			WinRMProto:    "https",
			WinRMPort:     port,
			WinRMHost:     h.IP,
			WinRMInsecure: true,
		})
	}
	return winrm.NewClientWithParameters(
		winrm.NewEndpoint(h.IP, port, true, true, nil, nil, nil, 0), user, pass, params)
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
