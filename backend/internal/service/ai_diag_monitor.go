// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// AI diagnosis extensions:
//  1. App monitors (http/tcp/ping): on a DOWN alert the platform runs fixed
//     read-only network probes from the server side (DNS, TCP connect, HTTP
//     re-probe with TLS details, ICMP ping). When the target IP matches a
//     managed Linux host, extra fixed read-only state is gathered over SSH to
//     correlate service vs. system causes. The AI analysis is pushed through
//     the channels bound to the monitor.
//  2. Windows hosts: host metric alerts (disk/mem/cpu) gather fixed read-only
//     state via WinRM instead of SSH (no cleanup on Windows — the cleanup
//     catalog is POSIX-only).
// In both cases the AI only analyzes gathered facts; commands are fixed in code.

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/sshpool"
)

// ---- App monitor diagnosis ----

// per-monitor cooldown map (shares the ai_diag_cooldown_min setting)
var aiMonDiagMu sync.Mutex
var aiMonDiagLast = map[uint]time.Time{}

func aiMonDiagCoolingDown(monID uint, min int) bool {
	aiMonDiagMu.Lock()
	cooling := false
	if last, ok := aiMonDiagLast[monID]; ok && time.Since(last) < time.Duration(min)*time.Minute {
		cooling = true
	} else {
		aiMonDiagLast[monID] = time.Now()
	}
	aiMonDiagMu.Unlock()
	return cooling
}

// AutoDiagnoseMonitor is the gated entry called (async) after an app monitor
// fires DOWN; every gate failure is a silent no-op.
func AutoDiagnoseMonitor(m *model.Monitor, errMsg string, respMs int) {
	defer func() { recover() }()
	cfg := LoadAiDiagConfig()
	ai := LoadAISettings()
	if !cfg.Enabled || !ai.Enabled || ai.BaseURL == "" {
		return
	}
	if aiMonDiagCoolingDown(m.ID, cfg.CooldownMin) {
		return
	}
	RunMonitorDiagnosis(m, errMsg, respMs)
}

// RunMonitorDiagnosis executes the ungated monitor diagnosis core
func RunMonitorDiagnosis(m *model.Monitor, errMsg string, respMs int) {
	defer func() { recover() }()
	ai := LoadAISettings()
	if ai.BaseURL == "" {
		return
	}

	probe, ips := netDiagnose(m)
	hostState, hostName := monitorHostCorrelate(m, ips)

	analysis, aerr := aiDiagMonitorAnalyze(ai, m, errMsg, respMs, probe, hostState, hostName)
	if aerr != nil {
		analysis = "AI 分析失败: " + aerr.Error()
	}

	sendMonitorDiagReport(m, analysis)

	summary := "应用监控诊断: " + analysis
	if len(summary) > 400 {
		summary = summary[:400]
	}
	LogAlertEvent("ai_diag", "warn", m.Name, summary)
}

// safeTarget validates a hostname/IP for exec use (ping): letters, digits, dot,
// dash, underscore only — no shell metacharacters.
var safeTarget = regexp.MustCompile(`^[0-9A-Za-z._-]{1,253}$`)

// netDiagnose runs the fixed server-side read-only probes and returns a
// sectioned text report plus the resolved IPs
func netDiagnose(m *model.Monitor) (report string, ips []string) {
	var b strings.Builder
	target := strings.TrimSpace(m.Target)
	if target == "" {
		return "", nil
	}
	host, schemePort := target, 0
	if m.Type == "http" {
		u, err := parseHTTPURL(target)
		if err != nil {
			return "---TARGET---\nURL 无法解析: " + err.Error(), nil
		}
		host = u.Hostname()
		if p, perr := strconv.Atoi(u.Port()); perr == nil {
			schemePort = p
		}
	}
	if !safeTarget.MatchString(host) {
		return "---TARGET---\n目标包含非法字符，跳过网络探测\n", nil
	}

	// DNS resolution (3s budget)
	dctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, derr := net.DefaultResolver.LookupHost(dctx, host)
	if derr != nil || len(ips) == 0 {
		b.WriteString("---DNS---\n解析失败: " + fmt.Sprint(derr) + "\n")
	} else {
		b.WriteString("---DNS---\n" + host + " → " + strings.Join(ips, ", ") + "\n")
	}

	// TCP connect (3s budget) for http/tcp monitors
	port := m.Port
	if port == 0 {
		port = schemePort
	}
	if port > 0 && m.Type != "ping" {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		start := time.Now()
		conn, terr := net.DialTimeout("tcp", addr, 3*time.Second)
		lat := time.Since(start).Milliseconds()
		if terr != nil {
			b.WriteString("---TCP---\n" + addr + " 连接失败: " + terr.Error() + "\n")
		} else {
			conn.Close()
			b.WriteString(fmt.Sprintf("---TCP---\n%s 连接成功 (%dms)\n", addr, lat))
		}
	}

	// HTTP re-probe (5s budget): status, latency, TLS validity, body snippet
	if m.Type == "http" {
		b.WriteString("---HTTP---\n" + httpProbe(target) + "\n")
	}

	// ICMP ping via system tool (target already validated by safeTarget)
	b.WriteString("---PING---\n" + pingProbe(host) + "\n")

	return b.String(), ips
}

func parseHTTPURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("仅支持 http/https")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("缺少主机名")
	}
	return u, nil
}

// httpProbe re-requests the monitored URL once and reports status/latency/TLS.
// TLS verification errors are tolerated for diagnosis (the certificate state is
// reported instead of failing the probe).
func httpProbe(raw string) string {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return "URL 无法解析: " + err.Error()
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // diagnosis only; cert validity is reported below
		},
	}
	start := time.Now()
	resp, err := client.Get(u.String())
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return u.String() + " 请求失败: " + err.Error()
	}
	defer resp.Body.Close()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s → HTTP %d (%dms)\n", u.String(), resp.StatusCode, lat))
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		b.WriteString(fmt.Sprintf("TLS: 颁发者 %s，有效期至 %s（剩余 %d 天）\n",
			cert.Issuer.CommonName, cert.NotAfter.Format("2006-01-02"),
			int(time.Until(cert.NotAfter).Hours()/24)))
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		b.WriteString("重定向: " + loc + "\n")
	}
	sniff, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if s := strings.TrimSpace(string(sniff)); s != "" {
		b.WriteString("响应片段: " + s + "\n")
	}
	return b.String()
}

// pingProbe runs one system ping (Windows: ping -n 1 -w 3000, POSIX: ping -c 1 -W 3)
func pingProbe(host string) string {
	if !safeTarget.MatchString(host) {
		return "跳过（目标未通过安全校验）"
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "3000", host)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "3", host)
	}
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if len(s) > 600 {
		s = s[:600]
	}
	if err != nil && s == "" {
		return "ping 执行失败: " + err.Error()
	}
	return s
}

// monitorHostCorrelate: when the monitored target resolves to a managed Linux
// host, gather fixed read-only state over SSH to correlate service vs system
// causes. Best-effort: any failure returns an empty string.
func monitorHostCorrelate(m *model.Monitor, ips []string) (state, hostName string) {
	defer func() { recover() }()
	if m.Type == "ping" {
		return "", "" // ping targets correlate weakly; skip
	}
	if len(ips) == 0 {
		return "", ""
	}
	var h model.Host
	found := false
	for _, ip := range ips {
		if err := model.DB.Where("ip = ?", ip).First(&h).Error; err == nil {
			found = true
			break
		}
	}
	if !found || IsWindows(&h) {
		return "", ""
	}
	cli, err := sshpool.ClientFor(&h)
	if err != nil {
		return "", ""
	}
	defer cli.Close()

	port := m.Port
	if port == 0 && m.Type == "http" {
		if u, err := parseHTTPURL(m.Target); err == nil {
			if p, perr := strconv.Atoi(u.Port()); perr == nil {
				port = p
			}
		}
	}
	script := aiDiagBaseScript // DF / MEM / UPTIME / TOPCPU / TOPMEM
	if port > 0 {
		// listening check is inserted in front (fixed read-only commands only)
		script = fmt.Sprintf(`echo '---LISTEN---'; ss -ltn 2>/dev/null | grep -E '[:.]%d\b' || echo 'port %d NOT listening'; %s`, port, port, script)
	}
	diag, _, err := runCapture(cli, script)
	if err != nil || strings.TrimSpace(diag) == "" {
		return "", ""
	}
	if len(diag) > 4000 {
		diag = diag[:4000]
	}
	return "---关联主机 " + h.Name + "（" + h.IP + "）SSH 采集---\n" + diag, h.Name
}

// aiDiagMonitorAnalyze sends the monitor alert context + probe results to the AI
func aiDiagMonitorAnalyze(ai AISettings, m *model.Monitor, errMsg string, respMs int, probe, hostState, hostName string) (string, error) {
	sys := strings.TrimSpace(SystemConfigMap()["ai_diag_prompt"])
	if sys == "" {
		sys = aiDiagDefaultPrompt
	}
	guard := AIInjectionGuardEnabled()
	if guard {
		sys += AISecurityGuard
	}
	wrap := func(s string) string {
		if guard {
			return "<<<UNTRUSTED_MACHINE_OUTPUT 开始>>>\n" + s + "\n<<<UNTRUSTED_MACHINE_OUTPUT 结束>>>"
		}
		return s
	}
	user := fmt.Sprintf("应用监控故障告警: 监控项=%s 类型=%s 目标=%s 最近错误=%s 响应=%dms\n\n网络探测结果:\n%s",
		m.Name, m.Type, monitorTargetText(m), errMsg, respMs, wrap(probe))
	if hostState != "" {
		user += "\n\n" + wrap(hostState)
	}
	return AIChat(ai, sys, user)
}

// sendMonitorDiagReport pushes the analysis through the channels bound to the
// monitor (same bindings as its alert notifications)
func sendMonitorDiagReport(m *model.Monitor, analysis string) {
	var bindings []model.MonitorChannel
	model.DB.Where("monitor_id = ?", m.ID).Find(&bindings)
	if len(bindings) == 0 {
		return
	}
	var channelIDs []uint
	for _, b := range bindings {
		channelIDs = append(channelIDs, b.ChannelID)
	}
	var channels []model.AlertChannel
	model.DB.Where("id IN ? AND enabled = ?", channelIDs, true).Find(&channels)
	vars := map[string]string{
		"monitor": m.Name, "type": m.Type, "target": monitorTargetText(m),
		"status": "DOWN", "error": "AI 诊断报告",
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	subject := fmt.Sprintf("AI 诊断 [应用监控] %s（%s）", m.Name, monitorTargetText(m))
	for i := range channels {
		ch := channels[i]
		go func() {
			defer func() { recover() }()
			_ = SendViaChannel(&ch, vars, subject, analysis)
		}()
	}
}

// ---- Windows host diagnosis (WinRM gather) ----

// winDiagScript: fixed read-only PowerShell probes (CPU / memory / disks / top processes)
const winDiagScript = "Write-Host '---CPU---'; (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average; " +
	"Write-Host '---MEM---'; Get-CimInstance Win32_OperatingSystem | ForEach-Object { 'Total {0:N0} MB, Free {1:N0} MB' -f ($_.TotalVisibleMemorySize/1KB), ($_.FreePhysicalMemory/1KB) }; " +
	"Write-Host '---DISK---'; Get-CimInstance Win32_LogicalDisk -Filter \"DriveType=3\" | ForEach-Object { '{0} used {1:P1}, free {2:N1} GB' -f $_.DeviceID, (1-$_.FreeSpace/$_.Size), ($_.FreeSpace/1GB) }; " +
	"Write-Host '---TOPPROC---'; Get-Process | Sort-Object CPU -Descending | Select-Object -First 8 Name,CPU,@{n='MemMB';e={[math]::Round($_.WS/1MB)}} | Format-Table -AutoSize | Out-String"

// RunWindowsDiagnosis gathers Windows state via WinRM and reuses the host
// diagnosis analysis/delivery path (no cleanup on Windows)
func RunWindowsDiagnosis(h *model.Host, level, metric string, value, threshold float64) {
	defer func() { recover() }()
	ai := LoadAISettings()
	if ai.BaseURL == "" {
		return
	}
	var user, pass string
	if cred := defaultCredential(h); cred != nil && cred.Password != "" {
		p, err := pkgDec(cred.Password)
		if err != nil {
			return
		}
		user, pass = cred.Username, p
	} else if h.Password != "" {
		p, err := pkgDec(h.Password)
		if err != nil {
			return
		}
		pass = p
	} else {
		return // WinRM needs a password credential
	}
	out, _, err := WinRMRun(h, user, pass, winDiagScript, 60)
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}
	analysis, aerr := aiDiagAnalyze(ai, h, level, metric, value, threshold, out)
	if aerr != nil {
		analysis = "AI 分析失败: " + aerr.Error()
	}
	SendAiDiagReport(h, level, metric, value, threshold, analysis, "")
	summary := analysis
	if len(summary) > 400 {
		summary = summary[:400]
	}
	LogAlertEvent("ai_diag", level, h.Name, summary)
}
