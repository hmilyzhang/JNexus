// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
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

// Monitoring components: app monitors (HTTP/TCP/Ping, Uptime Kuma style) + host base resources (CPU/memory/disk)

func MonitorEnabled() bool { return SystemConfigMap()["monitor_enabled"] != "false" }
func MonitorInterval() int {
	n := 0
	fmt.Sscanf(SystemConfigMap()["monitor_interval_sec"], "%d", &n)
	if n < 15 {
		n = 60
	}
	return n
}

// ---------- App monitor checks ----------

// statusAccepted checks whether an HTTP status code is within the accepted range (e.g. "200-299,301")
func statusAccepted(code int, accepted string) bool {
	spec := strings.TrimSpace(accepted)
	if spec == "" {
		spec = "200-299"
	}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			loN, e1 := strconv.Atoi(strings.TrimSpace(lo))
			hiN, e2 := strconv.Atoi(strings.TrimSpace(hi))
			if e1 == nil && e2 == nil && code >= loN && code <= hiN {
				return true
			}
		} else if n, e := strconv.Atoi(part); e == nil && code == n {
			return true
		}
	}
	return false
}

// CheckMonitor runs one monitor check: returns (ok, response ms, error message)
func CheckMonitor(m *model.Monitor) (bool, int, string) {
	timeout := time.Duration(m.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	start := time.Now()
	switch m.Type {
	case "http":
		return checkHTTP(m, timeout, start)
	case "tcp":
		return checkTCP(m, timeout, start)
	case "ping":
		return checkPing(m, timeout, start)
	default:
		return false, 0, "未知监控类型: " + m.Type
	}
}

func checkHTTP(m *model.Monitor, timeout time.Duration, start time.Time) (bool, int, string) {
	url := strings.TrimSpace(m.Target)
	if url == "" {
		return false, 0, "URL 不能为空"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}
	method := m.Method
	if method == "" {
		method = "GET"
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return false, 0, "URL 非法: " + err.Error()
	}
	req.Header.Set("User-Agent", "JNexus-Monitor/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0, err.Error()
	}
	defer resp.Body.Close()
	ms := int(time.Since(start).Milliseconds())
	if !statusAccepted(resp.StatusCode, m.AcceptedStatus) {
		return false, ms, fmt.Sprintf("HTTP %d 不在允许范围", resp.StatusCode)
	}
	if m.Keyword != "" {
		buf := make([]byte, 0, 1<<20)
		tmp := make([]byte, 32*1024)
		for len(buf) < 1<<20 {
			n, e := resp.Body.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if e != nil {
				break
			}
		}
		contain := strings.Contains(string(buf), m.Keyword)
		if m.KeywordType == "absent" && contain {
			return false, ms, "响应包含不应出现的关键字: " + m.Keyword
		}
		if m.KeywordType != "absent" && !contain {
			return false, ms, "响应未包含关键字: " + m.Keyword
		}
	}
	return true, ms, ""
}

func checkTCP(m *model.Monitor, timeout time.Duration, start time.Time) (bool, int, string) {
	host := strings.TrimSpace(m.Target)
	if host == "" || m.Port <= 0 {
		return false, 0, "TCP 监控需要主机与端口"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(m.Port)), timeout)
	if err != nil {
		return false, 0, err.Error()
	}
	conn.Close()
	return true, int(time.Since(start).Milliseconds()), ""
}

var pingTimeRe = regexp.MustCompile(`time[=<]([\d.]+)\s*ms`)

func checkPing(m *model.Monitor, timeout time.Duration, start time.Time) (bool, int, string) {
	host := strings.TrimSpace(m.Target)
	if host == "" {
		return false, 0, "Ping 目标不能为空"
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w",
			strconv.Itoa(int(timeout.Milliseconds())), host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W",
			strconv.Itoa(int(timeout.Seconds())), host)
	}
	out, err := cmd.Output()
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		return false, ms, fmt.Sprintf("ping 失败: %v", err)
	}
	if mt := pingTimeRe.FindSubmatch(out); mt != nil {
		if f, e := strconv.ParseFloat(string(mt[1]), 64); e == nil {
			return true, int(f), ""
		}
	}
	return true, ms, ""
}

// RunMonitorOnce runs the monitor check, persists the result (status + heartbeat sample), then evaluates alert rules for push.
// Inside a maintenance window: a down sample is recorded as maint (excluded from availability, gray heartbeat) and no alert evaluation runs.
func RunMonitorOnce(m *model.Monitor) (bool, int, string) {
	up, ms, errMsg := CheckMonitor(m)
	status := "down"
	if up {
		status = "up"
	}
	oldStatus := m.LastStatus
	now := time.Now()
	inMaint := InMaintenanceWindow(now)
	sampleStatus := status
	if inMaint && status == "down" {
		sampleStatus = "maint"
	}
	updates := map[string]any{
		"last_status": status, "last_resp_ms": ms, "last_error": errMsg, "last_checked_at": now,
	}
	model.DB.Model(m).Updates(updates)
	model.DB.Create(&model.MonitorSample{MonitorID: m.ID, Status: sampleStatus, RespMs: ms, Error: errMsg, CreatedAt: now})
	if !inMaint {
		EvaluateAlertRules(m, oldStatus, status, ms, errMsg, now)
	}
	return up, ms, errMsg
}

// EvaluateAlertRules is the global alert rule engine (applies to all monitors):
//   - monitor down: alert only after the failure persists for the "grace" seconds; grace 0 = alert on first failure
//   - recovery notification is sent only if an alert was actually fired during the current failure period (global switch)
//   - host system reboot is detected automatically by the collection flow and pushed immediately and independently (coexists with threshold alerts, no interference)
func EvaluateAlertRules(m *model.Monitor, oldStatus, status string, ms int, errMsg string, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[alert] rule evaluate panic:", r)
		}
	}()
	rule := LoadAlertRule()
	if status == "down" {
		if m.DownSince == nil {
			m.DownSince = &now
			model.DB.Model(m).Update("down_since", now)
		}
		grace := time.Duration(rule.GraceSec) * time.Second // grace 0 = immediate
		fire := !m.AlertFired && now.Sub(*m.DownSince) >= grace
		if fire {
			m.AlertFired = true
			model.DB.Model(m).Update("alert_fired", true)
			SendMonitorAlert(m, "down", ms, errMsg)
		}
		return
	}
	// Recovery: reset failure timers; if an alert was actually fired during this failure period, send a recovery notification per the global switch
	if m.AlertFired && rule.NotifyRecovery {
		SendMonitorAlert(m, "up", ms, errMsg)
	}
	m.AlertFired = false
	m.DownSince = nil
	model.DB.Model(m).Updates(map[string]any{"alert_fired": false, "down_since": nil})
}

// AlertRule is the global alert rule (stored in system config, applies to all monitors)
type AlertRule struct {
	GraceSec       int  `json:"grace_sec"`       // down threshold in seconds, 0 = immediate
	NotifyRecovery bool `json:"notify_recovery"` // recovery notification switch
}

// LoadAlertRule reads the global alert rule
func LoadAlertRule() AlertRule {
	m := SystemConfigMap()
	r := AlertRule{NotifyRecovery: m["alert_rule_notify_recovery"] != "false"}
	fmt.Sscanf(m["alert_rule_grace_sec"], "%d", &r.GraceSec)
	if r.GraceSec < 0 {
		r.GraceSec = 60
	}
	return r
}

// SaveAlertRule saves the global alert rule
func SaveAlertRule(r AlertRule) error {
	if r.GraceSec < 0 || r.GraceSec > 86400 {
		return fmt.Errorf("阈值超出范围（0-86400 秒）")
	}
	for _, kv := range [][2]string{
		{"alert_rule_grace_sec", strconv.Itoa(r.GraceSec)},
		{"alert_rule_notify_recovery", map[bool]string{true: "true", false: "false"}[r.NotifyRecovery]},
	} {
		if err := model.DB.Save(&model.SystemConfig{Key: kv[0], Value: kv[1]}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ScanDueMonitors scans monitors that are due (called by the scheduler loop)
func ScanDueMonitors() {
	if !MonitorEnabled() {
		return
	}
	var monitors []model.Monitor
	if err := model.DB.Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		return
	}
	now := time.Now()
	for i := range monitors {
		m := monitors[i]
		interval := time.Duration(m.IntervalSec) * time.Second
		if interval < 15*time.Second {
			interval = 60 * time.Second
		}
		if m.NextRunAt != nil && now.Before(*m.NextRunAt) {
			continue
		}
		next := now.Add(interval)
		model.DB.Model(&m).Update("next_run_at", next)
		go func(m model.Monitor) {
			defer func() { recover() }()
			RunMonitorOnce(&m)
		}(m)
	}
}

// ---------- Host base resource collection (CPU / memory / disk) ----------

// ---------- Host base resource collection (CPU / memory / disk) ----------

// Resource collection command: one SSH session gathers boot_id (reboot detection) plus all three resource datasets; parsing happens server-side
const metricCmd = `cat /proc/sys/kernel/random/boot_id 2>/dev/null; echo ---BOOT---; grep '^cpu ' /proc/stat; sleep 1; grep '^cpu ' /proc/stat; echo ---MEM---; head -5 /proc/meminfo; echo ---DISK---; df -P 2>/dev/null`

type hostSample struct {
	CPU, Mem, Disk float64
	BootID         string
}

func parseMetricOutput(out string) (hostSample, bool) {
	var s hostSample
	boot := out
	if idx := strings.Index(out, "---BOOT---"); idx >= 0 {
		boot = out[:idx]
		out = out[idx+len("---BOOT---"):]
	} else {
		return s, false
	}
	s.BootID = strings.TrimSpace(boot)
	parts := strings.Split(out, "---MEM---")
	if len(parts) < 2 {
		return s, false
	}
	cpuLines := strings.Split(strings.TrimSpace(parts[0]), "\n")
	if len(cpuLines) >= 2 {
		// Standard CPU usage algorithm (same as top): 100 - idle ratio; iowait counts as idle
		f := func(line string) (idle, total uint64) {
			fl := strings.Fields(line)
			if len(fl) < 5 || fl[0] != "cpu" {
				return
			}
			for i, x := range fl[1:] {
				n, e := strconv.ParseUint(x, 10, 64)
				if e != nil {
					continue
				}
				total += n
				if i == 3 || i == 4 { // idle + iowait
					idle += n
				}
			}
			return
		}
		id1, t1 := f(cpuLines[0])
		id2, t2 := f(cpuLines[1])
		dt := float64(t2 - t1)
		if dt > 0 {
			s.CPU = round1(100 * (1 - float64(id2-id1)/dt))
		}
		if s.CPU < 0 {
			s.CPU = 0
		}
	}
	mem := parts[1]
	if diskIdx := strings.Index(mem, "---DISK---"); diskIdx >= 0 {
		mem = mem[:diskIdx]
	}
	total, avail := uint64(0), uint64(0)
	for _, line := range strings.Split(mem, "\n") {
		fl := strings.Fields(line)
		if len(fl) >= 4 {
			n, e := strconv.ParseUint(fl[1], 10, 64)
			if e == nil && strings.HasPrefix(fl[0], "MemTotal") {
				total = n
			}
			n2, e2 := strconv.ParseUint(fl[1], 10, 64)
			if e2 == nil && strings.HasPrefix(fl[0], "MemAvailable") {
				avail = n2
			}
		}
	}
	if total > 0 {
		s.Mem = round1(100 * float64(total-avail) / float64(total))
	}
	diskPart := ""
	if diskIdx := strings.Index(out, "---DISK---"); diskIdx >= 0 {
		diskPart = out[diskIdx+10:]
	}
	maxPct := 0.0
	for _, line := range strings.Split(diskPart, "\n") {
		fl := strings.Fields(line)
		// Filesystem 1024-blocks Used Available Capacity Mounted-on
		if len(fl) >= 5 && strings.HasPrefix(fl[0], "/dev/") {
			pct, e := strconv.ParseFloat(strings.TrimSuffix(fl[4], "%"), 64)
			if e == nil && pct > maxPct {
				maxPct = pct
			}
		}
	}
	s.Disk = round1(maxPct)
	return s, true
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

// lastMetricRun tracks the last collection time per host (accessed concurrently, must hold the lock)
var lastMetricMu sync.Mutex
var lastMetricRun = map[uint]time.Time{}

// CollectHostMetrics collects CPU/memory/disk for all hosts (throttled by the global interval)
func CollectHostMetrics() {
	if !MonitorEnabled() {
		return
	}
	interval := time.Duration(MonitorInterval()) * time.Second
	var hosts []model.Host
	model.DB.Find(&hosts)
	now := time.Now()
	lastMetricMu.Lock()
	due := hosts[:0]
	for _, h := range hosts {
		if last, ok := lastMetricRun[h.ID]; !ok || now.Sub(last) >= interval {
			due = append(due, h)
			lastMetricRun[h.ID] = now
		}
	}
	lastMetricMu.Unlock()
	if len(due) == 0 {
		return
	}
	sem := make(chan struct{}, 20)
	var wg sync.WaitGroup
	for _, h := range due {
		wg.Add(1)
		go func(h model.Host) {
			defer wg.Done()
			defer func() { recover() }() // a failed host must not affect the process or other hosts
			sem <- struct{}{}
			defer func() { <-sem }()
			// Windows host: collect via WinRM (no SSH channel)
			if IsWindows(&h) {
				collectWindowsMetrics(&h)
				return
			}
			cli, err := sshpool.ClientFor(&h)
			if err != nil {
				return // connection failed (e.g. host down): no data this round, no sample written
			}
			out, _, err := runCapture(cli, metricCmd)
			cli.Close()
			if err != nil {
				return
			}
			if s, ok := parseMetricOutput(out); ok {
				model.DB.Create(&model.HostMetric{
					HostID: h.ID, CPUPercent: s.CPU, MemPercent: s.Mem, DiskPercent: s.Disk,
					CollectedAt: time.Now(),
				})
				OOPushHostMetric(h.ID, h.Name, s.CPU, s.Mem, s.Disk, time.Now())
				EvaluateCmdAlerts(&h, s, time.Now())
				// Automatic host reboot detection: push when boot_id differs from the last one (and this is not the first collection)
				if s.BootID != "" {
					if h.LastBootID != "" && s.BootID != h.LastBootID {
						SendHostRebootAlert(&h, s.BootID)
						LogAlertEvent("host_reboot", "warn", h.Name, "主机系统重启（boot_id 变化）")
					}
					if h.LastBootID != s.BootID {
						model.DB.Model(&model.Host{}).Where("id = ?", h.ID).
							Update("last_boot_id", s.BootID)
					}
				}
			}
		}(h)
	}
	wg.Wait()
}

// PruneMonitorData prunes expired samples (host metrics and monitor samples are both kept for 30 days)
func PruneMonitorData() {
	ArchiveHostMetrics()
	model.DB.Where("collected_at < ?", time.Now().Add(-30*24*time.Hour)).Delete(&model.HostMetric{})
	model.DB.Where("created_at < ?", time.Now().Add(-30*24*time.Hour)).Delete(&model.MonitorSample{})
	PruneK8sCapacitySamples()
}

// StartMonitorLoop starts the monitor scheduling loop (checks for due items every 15 seconds)
func StartMonitorLoop() {
	go func() {
		lastPrune := time.Time{}
		for {
			func() {
				defer func() { recover() }()
				go ScanDueMonitors()
				go CollectHostMetrics()
				go CollectK8sClusters()
				go CollectK8sUsage()
				if time.Since(lastPrune) >= time.Hour {
					PruneMonitorData()
					lastPrune = time.Now()
				}
			}()
			time.Sleep(15 * time.Second)
		}
	}()
}
