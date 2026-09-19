// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Linux security-log ingestion: pulls recent sshd/sudo auth entries from every
// reachable Linux host over SSH (journalctl) and dual-writes them to the
// linux_events stream for the security-department view. Throttled per host;
// incremental via per-host last-event timestamp. Failed-login bursts above the
// threshold are summarized to the configured security channels.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/sshpool"
)

var (
	linuxEventMu    sync.Mutex
	linuxEventPoll  = map[uint]time.Time{}
	linuxEventSince = map[uint]int64{}
)

// failed-login summary threshold per collection round per host
const linuxSecFailedThreshold = 5

// security-relevant message keywords (lowercase)
var linuxSecKeywords = []string{
	"failed password", "invalid user", "authentication failure",
	"accepted password", "accepted publickey",
	"session opened for user", "sudo:", "user not in sudoers",
}

type linuxJournalEntry struct {
	StampMs int64
	Unit    string
	Msg     string
	Failed  bool
}

// CollectLinuxEvents scheduler entry: throttled Linux security-log collection
func CollectLinuxEvents() {
	if !MonitorEnabled() || !ooIntegrationEnabled("linux_events") {
		return
	}
	if !LoadOOSettings().Enabled {
		return
	}
	interval := time.Duration(MonitorInterval()) * time.Second
	var hosts []model.Host
	model.DB.Where("os_type = ?", "linux").Find(&hosts)
	now := time.Now()

	linuxEventMu.Lock()
	var due []model.Host
	for _, h := range hosts {
		if last, ok := linuxEventPoll[h.ID]; !ok || now.Sub(last) >= interval {
			due = append(due, h)
			linuxEventPoll[h.ID] = now
		}
	}
	linuxEventMu.Unlock()
	if len(due) == 0 {
		return
	}

	for _, h := range due {
		h := h
		go func() {
			defer func() { recover() }()
			collectLinuxEventsFor(&h, now)
		}()
	}
}

func collectLinuxEventsFor(h *model.Host, now time.Time) {
	cli, err := sshpool.ClientFor(h)
	if err != nil {
		return
	}
	defer cli.Close()

	linuxEventMu.Lock()
	since, seen := linuxEventSince[h.ID]
	linuxEventMu.Unlock()
	if !seen {
		since = now.Add(-10 * time.Minute).Unix()
	}

	// journalctl JSON lines filtered to auth sources; hosts without journal
	// (no systemd) simply return nothing
	cmd := fmt.Sprintf("journalctl --since '@%d' -o json --no-pager -n 400 2>/dev/null | grep -Ei 'sshd|sudo' | head -c 300000", since)
	var out strings.Builder
	_, err = sshpool.RunCommand(context.Background(), cli, cmd, 45*time.Second, func(chunk string) { out.WriteString(chunk) })
	if err != nil || strings.TrimSpace(out.String()) == "" {
		return
	}

	var entries []linuxJournalEntry
	maxTs := since * 1000
	failed := 0
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var raw struct {
			StampUS string          `json:"__REALTIME_TIMESTAMP"`
			Msg     json.RawMessage `json:"MESSAGE"`
			Comm    string          `json:"_COMM"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		us, _ := strconv.ParseInt(raw.StampUS, 10, 64)
		if us == 0 {
			continue
		}
		msg := journalMessage(raw.Msg)
		low := strings.ToLower(msg)
		interesting := false
		failedHit := false
		for _, kw := range linuxSecKeywords {
			if strings.Contains(low, kw) {
				interesting = true
				if strings.Contains(low, "failed password") || strings.Contains(low, "invalid user") || strings.Contains(low, "authentication failure") {
					failedHit = true
				}
				break
			}
		}
		if !interesting {
			continue
		}
		unit := raw.Comm
		if unit == "" {
			unit = "auth"
		}
		entries = append(entries, linuxJournalEntry{
			StampMs: us / 1000, Unit: unit, Msg: msg, Failed: failedHit,
		})
		if us/1000 > maxTs {
			maxTs = us / 1000
		}
		if failedHit {
			failed++
		}
	}
	if len(entries) == 0 {
		return
	}

	for _, e := range entries {
		ooPushAsync("linux_events", map[string]any{
			"host_id": h.ID, "host": h.Name,
			"unit": e.Unit, "message": e.Msg,
			"event_time": time.UnixMilli(e.StampMs).UTC().Format(time.RFC3339),
		})
	}

	linuxEventMu.Lock()
	if maxTs > linuxEventSince[h.ID] {
		linuxEventSince[h.ID] = maxTs
	}
	linuxEventMu.Unlock()

	// failed-login burst: summarize to the configured security channels
	if failed >= linuxSecFailedThreshold {
		pushLinuxSecAlert(h, failed)
	}
}

// journalMessage flattens the MESSAGE field (string or array of strings)
func journalMessage(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return strings.Join(arr, " ")
	}
	return string(raw)
}

// pushLinuxSecAlert summarizes a failed-login burst to the security channels
func pushLinuxSecAlert(h *model.Host, failed int) {
	chCfg := strings.TrimSpace(SystemConfigMap()["sec_alert_channels"])
	if chCfg == "" {
		return
	}
	var chanIDs []uint
	for _, s := range strings.Split(chCfg, ",") {
		if n, e := strconv.Atoi(strings.TrimSpace(s)); e == nil && n > 0 {
			chanIDs = append(chanIDs, uint(n))
		}
	}
	var channels []model.AlertChannel
	model.DB.Where("id IN ? AND enabled = ?", chanIDs, true).Find(&channels)
	if len(channels) == 0 {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	vars := map[string]string{"host": h.Name, "ip": h.IP, "event": "Linux 登录失败爆发", "time": now}
	subject := fmt.Sprintf("🛡 [安全事件] %s：SSH 登录失败 %d 次", h.Name, failed)
	body := fmt.Sprintf("10 分钟窗口内检测到 %d 次登录失败/非法用户尝试。\n\n主机: %s（%s）\n时间: %s", failed, h.Name, h.IP, now)
	LogAlertEvent("sec_alert", "warn", h.Name, fmt.Sprintf("Linux 登录失败 %d 次（%s）", failed, h.Name))
	for i := range channels {
		ch := channels[i]
		go func() {
			defer func() { recover() }()
			_ = SendViaChannel(&ch, vars, subject, body)
		}()
	}
}

// pkgKept keeps the pkg import used (credential decrypt reserved for
// password-auth fallback paths)
var _ = pkg.Encrypt
