// AutoOps 运维平台 — By JJ Zhang, Version 1.0
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

	"autoops/internal/model"
	"autoops/internal/sshpool"
)

// 监控组件：应用监控（HTTP/TCP/Ping，Uptime Kuma 风格）+ 主机基础资源（CPU/内存/磁盘）

func MonitorEnabled() bool { return SystemConfigMap()["monitor_enabled"] != "false" }
func MonitorInterval() int {
	n := 0
	fmt.Sscanf(SystemConfigMap()["monitor_interval_sec"], "%d", &n)
	if n < 15 {
		n = 60
	}
	return n
}

// ---------- 应用监控检查 ----------

// statusAccepted 判定 HTTP 状态码是否在 accepted 范围（如 "200-299,301"）
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

// CheckMonitor 执行一次监控项检查：返回 (是否正常, 响应毫秒, 错误信息)
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
	req.Header.Set("User-Agent", "AutoOps-Monitor/1.0")
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

// RunMonitorOnce 执行监控项并落库（状态 + 心跳样本）；状态变化时向绑定通道推送告警
func RunMonitorOnce(m *model.Monitor) (bool, int, string) {
	up, ms, errMsg := CheckMonitor(m)
	status := "down"
	if up {
		status = "up"
	}
	oldStatus := m.LastStatus
	now := time.Now()
	updates := map[string]any{
		"last_status": status, "last_resp_ms": ms, "last_error": errMsg, "last_checked_at": now,
	}
	model.DB.Model(m).Updates(updates)
	model.DB.Create(&model.MonitorSample{MonitorID: m.ID, Status: status, RespMs: ms, Error: errMsg, CreatedAt: now})
	if oldStatus != status {
		NotifyMonitorStatusChange(m, oldStatus, status, ms, errMsg)
	}
	return up, ms, errMsg
}

// ScanDueMonitors 到期监控扫描（调度循环调用）
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

// ---------- 主机基础资源采集（CPU / 内存 / 磁盘） ----------

// 资源采集命令：一次 SSH 会话取齐三块数据，解析在服务端完成
const metricCmd = `grep '^cpu ' /proc/stat; sleep 1; grep '^cpu ' /proc/stat; echo ---MEM---; head -5 /proc/meminfo; echo ---DISK---; df -P 2>/dev/null`

type hostSample struct {
	CPU, Mem, Disk float64
}

func parseMetricOutput(out string) (hostSample, bool) {
	var s hostSample
	parts := strings.Split(out, "---MEM---")
	if len(parts) < 2 {
		return s, false
	}
	cpuLines := strings.Split(strings.TrimSpace(parts[0]), "\n")
	if len(cpuLines) >= 2 {
		// 标准 CPU 占用算法（与 top 一致）：100 - 空闲时间占比；iowait 计入空闲
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

// lastMetricRun 记录每台主机上次采集时间（多协程并发访问，须持锁）
var lastMetricMu sync.Mutex
var lastMetricRun = map[uint]time.Time{}

// CollectHostMetrics 采集全部主机的 CPU/内存/磁盘（按全局间隔节流）
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
			defer func() { recover() }() // 单台采集失败不影响进程与其余主机
			sem <- struct{}{}
			defer func() { <-sem }()
			cli, err := sshpool.ClientFor(&h)
			if err != nil {
				return // 连接失败（如主机关机）：本轮无数据，不写样本
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
			}
		}(h)
	}
	wg.Wait()
}

// PruneMonitorData 清理过期采样（主机指标 24h、监控样本 7d）
func PruneMonitorData() {
	model.DB.Where("collected_at < ?", time.Now().Add(-24*time.Hour)).Delete(&model.HostMetric{})
	model.DB.Where("created_at < ?", time.Now().Add(-7*24*time.Hour)).Delete(&model.MonitorSample{})
}

// StartMonitorLoop 监控调度循环（每 15 秒检查到期项）
func StartMonitorLoop() {
	go func() {
		lastPrune := time.Time{}
		for {
			func() {
				defer func() { recover() }()
				go ScanDueMonitors()
				go CollectHostMetrics()
				if time.Since(lastPrune) >= time.Hour {
					PruneMonitorData()
					lastPrune = time.Now()
				}
			}()
			time.Sleep(15 * time.Second)
		}
	}()
}
