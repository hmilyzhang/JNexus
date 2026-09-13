// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Windows Event Log ingestion (OpenObserve builtin): pulls fresh System/Application
// error+warning events from every reachable Windows host over WinRM and dual-writes
// them to the windows_events stream. Throttled per host; incremental via per-host
// last-event timestamp.

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

const winEventQueryMax = 50 // per host per round

// buildWinEventCmd renders the PowerShell event-log query (PS 5.1 compatible)
func buildWinEventCmd(sinceMs int64) string {
	since := "[DateTime]::Parse('1970-01-01T00:00:00Z').ToUniversalTime().AddMilliseconds(" +
		strconv.FormatInt(sinceMs, 10) + ")"
	return `[Console]::OutputEncoding=[Text.Encoding]::UTF8; ` +
		"$e = @(Get-WinEvent -FilterHashtable @{LogName=@('System','Application'); Level=1,2,3; StartTime=" + since + "} -MaxEvents " +
		strconv.Itoa(winEventQueryMax) + " -ErrorAction SilentlyContinue | " +
		"ForEach-Object { $m = $_.Message; if ($m) { $m = ($m -replace \"`r`n\", ' ') -replace \"`n\", ' '; if ($m.Length -gt 400) { $m = $m.Substring(0, 400) } }; " +
		"[pscustomobject]@{ time = $_.TimeCreated.ToUniversalTime().Subtract([DateTime]'1970-01-01').TotalMilliseconds; log = $_.LogName; level = $_.LevelDisplayName; id = $_.Id; provider = $_.ProviderName; msg = $m } }); " +
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
		}()
	}
}
