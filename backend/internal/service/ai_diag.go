// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// AI alert diagnostics + controlled disk cleanup:
// When a P1-P4 threshold alert fires for disk/mem/cpu, a fixed read-only diagnostic
// script is gathered over SSH (Linux) or WinRM (Windows) and handed to the
// configured AI for root-cause analysis. The analysis is pushed through the
// alert channels bound to that level.
// On disk alerts (Linux), admin-predefined cleanup commands (the "cleanup
// catalog") may run — executed VERBATIM from the catalog; the AI never
// generates or alters commands. App monitors get server-side network diagnosis
// (ai_diag_monitor.go).

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/sshpool"

	gossh "golang.org/x/crypto/ssh"
)

// aiDiagDefaultPrompt built-in diagnosis system prompt (overridable via the
// ai_diag_prompt setting). Shared by host alerts (SSH/WinRM gather) and app
// monitor alerts (server-side network probes).
const aiDiagDefaultPrompt = "你是资深 SRE。JNexus 运维平台检测到告警，以下是采集到的诊断信息。" +
	"请输出：1) 根因分析 2) 可疑进程/服务 3) 处置建议。用中文，简洁分点。" +
	"严格约束：只做分析，不得生成任何可执行命令或脚本。"

// ---- Configuration ----

type AiDiagConfig struct {
	Enabled     bool
	Levels      []string // P1..P4
	Metrics     []string // disk / cpu / mem
	CooldownMin int
}

func LoadAiDiagConfig() AiDiagConfig {
	m := SystemConfigMap()
	cfg := AiDiagConfig{Enabled: m["ai_diag_enabled"] == "true", CooldownMin: 30}
	if v := strings.TrimSpace(m["ai_diag_levels"]); v != "" {
		for _, lv := range strings.Split(v, ",") {
			if lv = strings.ToUpper(strings.TrimSpace(lv)); lv != "" {
				cfg.Levels = append(cfg.Levels, lv)
			}
		}
	}
	if v := strings.TrimSpace(m["ai_diag_metrics"]); v != "" {
		for _, mt := range strings.Split(v, ",") {
			if mt = strings.ToLower(strings.TrimSpace(mt)); mt != "" {
				cfg.Metrics = append(cfg.Metrics, mt)
			}
		}
	}
	if n, e := strconv.Atoi(m["ai_diag_cooldown_min"]); e == nil && n >= 0 {
		cfg.CooldownMin = n
	}
	return cfg
}

func (c AiDiagConfig) levelEnabled(lv string) bool {
	for _, l := range c.Levels {
		if l == strings.ToUpper(lv) {
			return true
		}
	}
	return false
}

func (c AiDiagConfig) metricEnabled(mt string) bool {
	for _, m := range c.Metrics {
		if m == strings.ToLower(mt) {
			return true
		}
	}
	return false
}

// ---- Cleanup catalog (admin-defined; executed verbatim) ----

type CleanupItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
	Enabled bool   `json:"enabled"`
}

func LoadCleanupList() []CleanupItem {
	m := SystemConfigMap()
	var out []CleanupItem
	if raw := strings.TrimSpace(m["ai_diag_cleanup"]); raw != "" {
		if json.Unmarshal([]byte(raw), &out) == nil && len(out) > 0 {
			return out
		}
		out = nil
	}
	// Defaults (safe, scoped cleanup commands); also re-seeded when the stored
	// catalog is an empty array — admins trim rather than empty it
	out = []CleanupItem{
		{ID: "tmp-old-files", Name: "清理 /tmp 7 天未访问文件", Command: "find /tmp -xdev -type f -atime +7 -delete", Enabled: true},
		{ID: "vartmp-old-files", Name: "清理 /var/tmp 7 天未访问文件", Command: "find /var/tmp -xdev -type f -atime +7 -delete", Enabled: true},
		{ID: "log-gz-14d", Name: "清理 /var/log 14 天前的压缩归档", Command: `find /var/log -type f -name "*.gz" -mtime +14 -delete`, Enabled: true},
		{ID: "journal-200m", Name: "journald 日志压缩到 200M", Command: "journalctl --vacuum-size=200M", Enabled: true},
		{ID: "apt-cache", Name: "清理 apt 下载缓存（Debian/Ubuntu）", Command: "apt-get clean", Enabled: true},
		{ID: "yum-cache", Name: "清理 yum 缓存（RHEL/CentOS）", Command: "yum clean all", Enabled: false},
		{ID: "docker-prune", Name: "清理 Docker 未使用资源", Command: "docker system prune -af", Enabled: false},
	}
	if b, err := json.Marshal(out); err == nil {
		_ = SetSystemConfigs(map[string]string{"ai_diag_cleanup": string(b)})
	}
	return out
}

// SaveCleanupList sanitizes and persists the catalog. Hard guard: one simple
// statement per command — no chaining (; & |), no line breaks.
func SaveCleanupList(items []CleanupItem) error {
	clean := make([]CleanupItem, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		it.ID = strings.TrimSpace(it.ID)
		it.Command = strings.TrimSpace(it.Command)
		if it.ID == "" || seen[it.ID] {
			continue
		}
		it.Command = strings.ReplaceAll(it.Command, "\n", " ")
		if it.Command == "" || strings.ContainsAny(it.Command, ";&|") {
			return fmt.Errorf("命令包含非法字符（禁止 ; & | 或换行）: %s", it.Name)
		}
		seen[it.ID] = true
		it.Name = strings.TrimSpace(it.Name)
		if it.Name == "" {
			it.Name = it.ID
		}
		clean = append(clean, it)
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	return SetSystemConfigs(map[string]string{"ai_diag_cleanup": string(b)})
}

// ---- Per-host cooldown ----

var aiDiagMu sync.Mutex
var aiDiagLast = map[uint]time.Time{}

func aiDiagCoolingDown(hostID uint, min int) bool {
	aiDiagMu.Lock()
	cooling := false
	if last, ok := aiDiagLast[hostID]; ok && time.Since(last) < time.Duration(min)*time.Minute {
		cooling = true
	} else {
		aiDiagLast[hostID] = time.Now()
	}
	aiDiagMu.Unlock()
	return cooling
}

// ---- Fixed read-only diagnostics (PowerShell-free, POSIX only) ----

const aiDiagBaseScript = `echo '---DF---'; df -hP; echo '---MEM---'; free -m; echo '---UPTIME---'; uptime; echo '---TOPCPU---'; ps aux --sort=-%cpu | head -12; echo '---TOPMEM---'; ps aux --sort=-%mem | head -12`

const aiDiagDiskScript = aiDiagBaseScript + `; echo '---DU---'; du -xsh /tmp /var/tmp /var/log 2>/dev/null`

// ---- Entry point ----

// AutoDiagnose runs the alert-driven diagnosis pipeline. Called async after a
// CMD alert fires; every gate failure is a silent no-op. Linux hosts gather
// over SSH, Windows hosts over WinRM.
func AutoDiagnose(h *model.Host, level, metric string, value, threshold float64) {
	defer func() { recover() }()
	metric = strings.ToLower(metric) // fire path passes capitalized metric names (Disk/CPU/Mem)
	cfg := LoadAiDiagConfig()
	ai := LoadAISettings()
	if !cfg.Enabled || !ai.Enabled || ai.BaseURL == "" {
		return
	}
	if !cfg.levelEnabled(level) || !cfg.metricEnabled(metric) {
		return
	}
	if aiDiagCoolingDown(h.ID, cfg.CooldownMin) {
		return
	}
	if IsWindows(h) {
		RunWindowsDiagnosis(h, level, metric, value, threshold)
		return
	}
	RunDiagnosis(h, level, metric, value, threshold)
}

// RunDiagnosis executes the ungated diagnosis core (also used by the admin
// cleanup-test endpoint with level="MANUAL")
func RunDiagnosis(h *model.Host, level, metric string, value, threshold float64) {
	defer func() { recover() }()
	ai := LoadAISettings()
	if ai.BaseURL == "" {
		return
	}

	cli, err := sshpool.ClientFor(h)
	if err != nil {
		return
	}
	defer cli.Close()

	script := aiDiagBaseScript
	if metric == "disk" {
		script = aiDiagDiskScript
	}
	diag, _, err := runCapture(cli, script)
	if err != nil || strings.TrimSpace(diag) == "" {
		return
	}

	analysis, aerr := aiDiagAnalyze(ai, h, level, metric, value, threshold, diag)
	if aerr != nil {
		analysis = "AI 分析失败: " + aerr.Error()
	}

	// Phase 2: disk alerts may run the enabled, admin-defined cleanup commands
	cleanupReport := ""
	if metric == "disk" && aerr == nil {
		cleanupReport = aiDiagCleanupRun(cli)
	}

	SendAiDiagReport(h, level, metric, value, threshold, analysis, cleanupReport)

	summary := analysis
	if len(summary) > 400 {
		summary = summary[:400]
	}
	LogAlertEvent("ai_diag", level, h.Name, summary)
}

// aiDiagAnalyze sends the alert context + diagnostics to the AI and returns the analysis
func aiDiagAnalyze(ai AISettings, h *model.Host, level, metric string, value, threshold float64, diag string) (string, error) {
	sys := strings.TrimSpace(SystemConfigMap()["ai_diag_prompt"])
	if sys == "" {
		sys = aiDiagDefaultPrompt
	}
	guard := AIInjectionGuardEnabled()
	if guard {
		sys += AISecurityGuard
	}
	// gathered machine output is untrusted: wrap it so embedded text cannot
	// act as instructions (indirect prompt injection); markers follow the guard toggle
	data := diag
	if guard {
		data = "<<<UNTRUSTED_MACHINE_OUTPUT 开始>>>\n" + diag + "\n<<<UNTRUSTED_MACHINE_OUTPUT 结束>>>"
	}
	user := fmt.Sprintf("告警: 等级=%s 主机=%s(%s) 指标=%s 当前值=%.1f%% 阈值=%.1f%%\n\n系统状态:\n%s",
		level, h.Name, h.IP, metric, value, threshold, data)
	return AIChat(ai, sys, user)
}

// aiDiagCleanupRun runs the enabled cleanup catalog commands verbatim (disk alerts only)
func aiDiagCleanupRun(cli *gossh.Client) string {
	enabled := []CleanupItem{}
	for _, it := range LoadCleanupList() {
		if it.Enabled {
			enabled = append(enabled, it)
		}
	}
	if len(enabled) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n--- 磁盘清理（预定义命令，逐字执行）---")
	for _, it := range enabled {
		var out strings.Builder
		onOut := func(chunk string) { out.WriteString(chunk) }
		_, err := sshpool.RunCommand(context.Background(), cli, it.Command, 60*time.Second, onOut)
		if err != nil {
			b.WriteString("\n[" + it.Name + "] 失败: " + err.Error())
			continue
		}
		o := strings.TrimSpace(out.String())
		if len(o) > 400 {
			o = o[:400]
		}
		if o != "" {
			b.WriteString("\n[" + it.Name + "] " + it.Command + "\n" + o)
		} else {
			b.WriteString("\n[" + it.Name + "] " + it.Command + " — 完成")
		}
	}
	return b.String()
}

// SendAiDiagReport pushes the analysis + cleanup report through the alert channels
// bound to the fired alert level (reuses the CMD alert templates/vars mechanism)
func SendAiDiagReport(h *model.Host, level, metric string, value, threshold float64, analysis, cleanupReport string) {
	for _, cl := range LoadCmdLevels() {
		if cl.Level != strings.ToUpper(level) {
			continue
		}
		if len(cl.ChannelIDs) == 0 {
			return
		}
		var channels []model.AlertChannel
		model.DB.Where("id IN ? AND enabled = ?", cl.ChannelIDs, true).Find(&channels)
		vars := map[string]string{
			"level": cl.Level, "host": h.Name, "ip": h.IP,
			"metric": metric, "value": fmt.Sprintf("%.1f", value),
			"threshold": fmt.Sprintf("%.1f", threshold),
			"time":      time.Now().Format("2006-01-02 15:04:05"),
		}
		defSubject := fmt.Sprintf("AI 诊断 [%s][%s] %s=%.1f%%", cl.Level, h.Name, metric, value)
		defBody := analysis + cleanupReport
		for i := range channels {
			ch := channels[i]
			go func() {
				defer func() { recover() }()
				_ = SendViaChannel(&ch, vars, defSubject, defBody)
			}()
		}
		return
	}
}
