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

// CheckMonitor runs one monitor check: returns (ok, response ms, error message, cert expiry)
// cert expiry is non-nil only for https targets
func CheckMonitor(m *model.Monitor) (bool, int, string, *time.Time) {
	timeout := time.Duration(m.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	start := time.Now()
	switch m.Type {
	case "http":
		return checkHTTP(m, timeout, start)
	case "tcp":
		up, ms, msg := checkTCP(m, timeout, start)
		return up, ms, msg, nil
	case "ping":
		up, ms, msg := checkPing(m, timeout, start)
		return up, ms, msg, nil
	default:
		return false, 0, "未知监控类型: " + m.Type, nil
	}
}

func checkHTTP(m *model.Monitor, timeout time.Duration, start time.Time) (bool, int, string, *time.Time) {
	url := strings.TrimSpace(m.Target)
	if url == "" {
		return false, 0, "URL 不能为空", nil
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
		return false, 0, "URL 非法: " + err.Error(), nil
	}
	req.Header.Set("User-Agent", "JNexus-Monitor/1.0")
	transport := &http.Transport{TLSClientConfig: OutboundTLS()}
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return false, 0, err.Error(), nil
	}
	// HTTPS: capture the leaf certificate expiry for lifecycle tracking
	var certNotAfter *time.Time
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		na := resp.TLS.PeerCertificates[0].NotAfter
		certNotAfter = &na
	}
	defer resp.Body.Close()
	ms := int(time.Since(start).Milliseconds())
	if !statusAccepted(resp.StatusCode, m.AcceptedStatus) {
		return false, ms, fmt.Sprintf("HTTP %d 不在允许范围", resp.StatusCode), certNotAfter
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
			return false, ms, "响应包含不应出现的关键字: " + m.Keyword, certNotAfter
		}
		if m.KeywordType != "absent" && !contain {
			return false, ms, "响应未包含关键字: " + m.Keyword, certNotAfter
		}
	}
	return true, ms, "", certNotAfter
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
func RunMonitorOnce(m *model.Monitor) (bool, int, string, *time.Time) {
	up, ms, errMsg, certExp := CheckMonitor(m)
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
	OOPushMonitorSample(m, sampleStatus, ms, errMsg, now)
	if certExp != nil {
		model.DB.Model(m).Updates(map[string]any{"cert_not_after": certExp,
			"cert_warn_fired": false, "cert_crit_fired": false})
		if !inMaint {
			EvaluateCertExpiry(m, *certExp, now)
		}
	}
	if !inMaint {
		EvaluateAlertRules(m, oldStatus, status, ms, errMsg, now)
	}
	return up, ms, errMsg, certExp
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

// EvaluateCertExpiry checks the HTTPS certificate expiry against the global
// thresholds and pushes tiered alerts (warn / critical) to the monitor's bound
// channels. Flags on the monitor row dedupe repeated alerts per tier; they
// reset automatically once the certificate is renewed past the warn tier.
func EvaluateCertExpiry(m *model.Monitor, notAfter time.Time, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[cert-expiry] evaluate panic:", r)
		}
	}()
	rule := LoadAlertRule()
	days := int(time.Until(notAfter).Hours() / 24)
	level := 0 // 0 = ok, 1 = warn, 2 = critical, 3 = expired
	if days < 0 {
		level = 3
	} else if days <= rule.CertCritDays {
		level = 2
	} else if days <= rule.CertWarnDays {
		level = 1
	}

	if level == 0 {
		// renewed: clear fired flags silently
		if m.CertWarnFired || m.CertCritFired {
			model.DB.Model(m).Updates(map[string]any{"cert_warn_fired": false, "cert_crit_fired": false})
		}
		return
	}
	// dedupe per tier: re-alert only on escalation to critical
	if (level == 1 && m.CertWarnFired) || (level == 2 && m.CertCritFired) {
		return
	}
	SendCertAlert(m, level, days, notAfter)
	if level == 1 {
		model.DB.Model(m).Updates(map[string]any{"cert_warn_fired": true})
	} else {
		model.DB.Model(m).Updates(map[string]any{"cert_crit_fired": true})
	}
	LogAlertEvent("cert_expiry", map[int]string{1: "warn", 2: "crit", 3: "crit"}[level], m.Name,
		fmt.Sprintf("HTTPS 证书剩余 %d 天（%s 到期）", days, notAfter.Format("2006-01-02")))
}

// AlertRule is the global alert rule (stored in system config, applies to all monitors)
type AlertRule struct {
	GraceSec       int  `json:"grace_sec"`       // down threshold in seconds, 0 = immediate
	NotifyRecovery bool `json:"notify_recovery"` // recovery notification switch
	CertWarnDays   int  `json:"cert_warn_days"`  // HTTPS cert expiry warn threshold (days), 0 = 30
	CertCritDays   int  `json:"cert_crit_days"`  // HTTPS cert expiry critical threshold (days), 0 = 7
}

// LoadAlertRule reads the global alert rule
func LoadAlertRule() AlertRule {
	m := SystemConfigMap()
	r := AlertRule{NotifyRecovery: m["alert_rule_notify_recovery"] != "false", CertWarnDays: 30, CertCritDays: 7}
	fmt.Sscanf(m["alert_rule_grace_sec"], "%d", &r.GraceSec)
	if r.GraceSec < 0 {
		r.GraceSec = 60
	}
	fmt.Sscanf(m["alert_rule_cert_warn_days"], "%d", &r.CertWarnDays)
	fmt.Sscanf(m["alert_rule_cert_crit_days"], "%d", &r.CertCritDays)
	if r.CertWarnDays <= 0 {
		r.CertWarnDays = 30
	}
	if r.CertCritDays <= 0 {
		r.CertCritDays = 7
	}
	if r.CertCritDays > r.CertWarnDays {
		r.CertCritDays = r.CertWarnDays
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
		{"alert_rule_cert_warn_days", strconv.Itoa(r.CertWarnDays)},
		{"alert_rule_cert_crit_days", strconv.Itoa(r.CertCritDays)},
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
		// /proc/meminfo lines have 3 fields (Key: value kB); match any line with a value
		if len(fl) >= 2 {
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
	model.DB.Where("hour < ?", time.Now().Add(-35*24*time.Hour)).Delete(&model.MonitorSampleHourly{})
	PruneK8sCapacitySamples()
}

var sampleHourlyMu sync.Mutex
var sampleHourlyWatermark uint // last monitor_samples.id folded into the hourly rollup

// AggregateSampleHourly folds raw samples with id > watermark into the hourly
// rollup (id ranges are disjoint, so the += upsert counts every sample exactly
// once). First run does a full backfill when the rollup is empty. Cheap: runs
// on the monitor loop every few minutes, touching only the new id range.
func AggregateSampleHourly() {
	sampleHourlyMu.Lock()
	defer sampleHourlyMu.Unlock()
	var maxID uint
	model.DB.Model(&model.MonitorSample{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID)
	if maxID == 0 {
		return
	}
	var rollupRows int64
	model.DB.Model(&model.MonitorSampleHourly{}).Count(&rollupRows)
	if sampleHourlyWatermark == 0 && rollupRows == 0 {
		sampleHourlyWatermark = 0 // full backfill below
	}
	res := model.DB.Exec(`INSERT INTO monitor_sample_hourlies (monitor_id, hour, up_count, total_count)
		SELECT monitor_id, date_trunc('hour', created_at),
			SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END), COUNT(*)
		FROM monitor_samples
		WHERE id > ? AND id <= ? AND status IN ('up','down')
		GROUP BY monitor_id, date_trunc('hour', created_at)
		ON CONFLICT (monitor_id, hour) DO UPDATE SET
			up_count = monitor_sample_hourlies.up_count + EXCLUDED.up_count,
			total_count = monitor_sample_hourlies.total_count + EXCLUDED.total_count`,
		sampleHourlyWatermark, maxID)
	if res.Error != nil {
		fmt.Println("[monitor] hourly rollup failed:", res.Error)
		return
	}
	sampleHourlyWatermark = maxID
}

// StartMonitorLoop starts the monitor scheduling loop (checks for due items every 15 seconds)
func StartMonitorLoop() {
	go func() {
		lastPrune := time.Time{}
		lastAgg := time.Time{}
		for {
			func() {
				defer func() { recover() }()
				go ScanDueMonitors()
				go CollectHostMetrics()
				go CollectWindowsEvents()
				go CollectLinuxEvents()
				go RunDueDbSources()
				go RunDueCloudSyncs()
				go RunDueDBAccountRotations()
				go CheckKeyRotation()
				go CollectK8sClusters()
				go CollectK8sUsage()
				if time.Since(lastAgg) >= 2*time.Minute {
					AggregateSampleHourly()
					lastAgg = time.Now()
				}
				if time.Since(lastPrune) >= time.Hour {
					PruneMonitorData()
					lastPrune = time.Now()
				}
			}()
			time.Sleep(15 * time.Second)
		}
	}()
}
