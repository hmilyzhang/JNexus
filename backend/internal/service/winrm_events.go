// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Windows Event Log ingestion (OpenObserve builtin): pulls fresh System/Application
// error+warning events from every reachable Windows host over WinRM and dual-writes
// them to the windows_events stream. Throttled per host; incremental via per-host
// last-event timestamp.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

const winEventQueryMax = 50 // System/Application errors+warnings per host per round
const winEventSecMax = 30   // Security audit events per host per round

// buildWinEventCmd renders the PowerShell event-log query (PS 5.1 compatible).
// System/Application: errors & warnings only. Security: all audit events (logons,
// account management...) are Informational-level, so no level filter — capped lower.
func buildWinEventCmd(sinceMs int64) string {
	since := "[DateTime]::Parse('1970-01-01T00:00:00Z').ToUniversalTime().AddMilliseconds(" +
		strconv.FormatInt(sinceMs, 10) + ")"
	toRow := "ForEach-Object { $m = $_.Message; if ($m) { $m = ($m -replace \"`r`n\", ' ') -replace \"`n\", ' '; if ($m.Length -gt 400) { $m = $m.Substring(0, 400) } }; " +
		"[pscustomobject]@{ time = $_.TimeCreated.ToUniversalTime().Subtract([DateTime]'1970-01-01').TotalMilliseconds; log = $_.LogName; level = $_.LevelDisplayName; id = $_.Id; provider = $_.ProviderName; msg = $m } }"
	return `[Console]::OutputEncoding=[Text.Encoding]::UTF8; ` +
		"$sys = @(Get-WinEvent -FilterHashtable @{LogName=@('System','Application'); Level=1,2,3; StartTime=" + since + "} -MaxEvents " +
		strconv.Itoa(winEventQueryMax) + " -ErrorAction SilentlyContinue | " + toRow + "); " +
		"$sec = @(Get-WinEvent -FilterHashtable @{LogName='Security'; StartTime=" + since + "} -MaxEvents " +
		strconv.Itoa(winEventSecMax) + " -ErrorAction SilentlyContinue | " + toRow + "); " +
		"$e = @($sys) + @($sec); " +
		"ConvertTo-Json -InputObject $e -Compress"
}

type winEventRow struct {
	Time     float64 `json:"time"`
	Log      string  `json:"log"`
	Level    string  `json:"level"`
	Id       int     `json:"id"`
	Provider string  `json:"provider"`
	Msg      string  `json:"msg"`
}

// winEventPoll tracks the last event timestamp per host (ms epoch; concurrent access guarded)
var winEventMu sync.Mutex
var winEventSince = map[uint]int64{}
var winEventPoll = map[uint]time.Time{}

// CollectWindowsEvents pulls fresh Windows event-log entries for all Windows hosts
// (throttled to the global monitor interval, same cadence as metric collection)
func CollectWindowsEvents() {
	if !MonitorEnabled() || !ooIntegrationEnabled("windows_events") {
		return
	}
	if !LoadOOSettings().Enabled {
		return // events are only collected for OpenObserve ingestion
	}
	interval := time.Duration(MonitorInterval()) * time.Second
	var hosts []model.Host
	model.DB.Where("os_type = ?", "windows").Find(&hosts)
	now := time.Now()

	winEventMu.Lock()
	var due []model.Host
	for _, h := range hosts {
		if last, ok := winEventPoll[h.ID]; !ok || now.Sub(last) >= interval {
			due = append(due, h)
			winEventPoll[h.ID] = now
		}
	}
	winEventMu.Unlock()
	if len(due) == 0 {
		return
	}

	for _, h := range due {
		h := h
		go func() {
			defer func() { recover() }()
			cred := defaultCredential(&h)
			if cred == nil || cred.AuthType != "password" {
				return // WinRM needs a password account
			}
			pass, err := pkg.Decrypt(cred.Password)
			if err != nil {
				return
			}

			winEventMu.Lock()
			since, seen := winEventSince[h.ID]
			winEventMu.Unlock()
			if !seen {
				since = now.Add(-10 * time.Minute).UnixMilli()
			}

			out, _, err := WinRMRun(&h, cred.Username, pass, buildWinEventCmd(since), 45)
			if err != nil || strings.TrimSpace(out) == "" {
				return
			}
			var rows []winEventRow
			if json.Unmarshal([]byte(strings.TrimSpace(out)), &rows) != nil {
				return
			}

			maxTs := since
			for _, r := range rows {
				ts := int64(r.Time)
				if ts > maxTs {
					maxTs = ts
				}
				rec := map[string]any{
					"host_id": h.ID, "host": h.Name,
					"log_name": r.Log, "level": r.Level,
					"event_id": r.Id, "provider": r.Provider,
					"message":    r.Msg,
					"event_time": time.UnixMilli(ts).UTC().Format(time.RFC3339),
				}
				ooPushAsync("windows_events", rec)
			}
			if maxTs > since {
				winEventMu.Lock()
				if maxTs > winEventSince[h.ID] {
					winEventSince[h.ID] = maxTs
				}
				winEventMu.Unlock()
			}
			pushSecAlerts(&h, rows)
		}()
	}
}

// secAlertPush pushes one alert line to the configured security channels
func secAlertPush(h *model.Host, subject, body string) {
	channels := secAlertChannels()
	if len(channels) == 0 {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	vars := map[string]string{"host": h.Name, "ip": h.IP, "event": "安全事件监控", "time": now}
	for i := range channels {
		ch := channels[i]
		go func() {
			defer func() { recover() }()
			_ = SendViaChannel(&ch, vars, subject, body)
		}()
	}
}

// pushSecAlerts matches collected Security-log events against the admin
// watchlist: "immediate" items alert per hit, the rest aggregate into one
// summary per collection round per host.
func pushSecAlerts(h *model.Host, rows []winEventRow) {
	watch := SecWatchWinList()
	if len(watch) == 0 {
		return
	}
	byID := map[int]SecWatchWin{}
	for _, w := range watch {
		if w.On {
			byID[w.ID] = w
		}
	}
	if len(byID) == 0 {
		return
	}

	var agg []string
	for _, r := range rows {
		if !strings.EqualFold(r.Log, "Security") {
			continue
		}
		w, ok := byID[r.Id]
		if !ok {
			continue
		}
		line := fmt.Sprintf("[%s] %s（ID %d）%s", r.Level, w.Name, r.Id,
			time.UnixMilli(int64(r.Time)).Format("15:04:05"))
		if w.Immediate {
			LogAlertEvent("sec_alert", "warn", h.Name, line)
			secAlertPush(h, fmt.Sprintf("🛡 [安全告警] %s：%s", w.Name, h.Name),
				fmt.Sprintf("命中立即报警项：%s\n\n主机: %s（%s）\n明细: %s", w.Name, h.Name, h.IP, line))
			continue
		}
		agg = append(agg, line)
	}

	if len(agg) == 0 {
		return
	}
	if len(agg) > 5 {
		agg = agg[:5]
	}
	LogAlertEvent("sec_alert", "warn", h.Name, fmt.Sprintf("安全事件汇总 %d 条（%s）", len(agg), h.Name))
	secAlertPush(h, fmt.Sprintf("🛡 [安全事件] %s", h.Name),
		"Windows Security 日志检测到关注的安全事件：\n" + strings.Join(agg, "\n") +
			fmt.Sprintf("\n\n主机: %s（%s）", h.Name, h.IP))
}
