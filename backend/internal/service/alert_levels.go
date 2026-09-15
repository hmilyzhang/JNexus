// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"time"

	"jnexus/internal/model"
)

// P1-P4 tiered threshold alerting for CMD resources (CPU/memory/disk):
// any metric crossing a level's threshold puts the host at that level (most severe wins); once it persists
// for the level's duration, notifications are pushed to the channels bound to that level;
// when the level changes, a recovery is sent for the old level (subject to the global recovery switch); templates support placeholder customization.

const cmdLevelsKey = "cmd_alert_levels"

// CmdLevel configuration for a single alert level
type CmdLevel struct {
	Level       string  `json:"level"` // P1 / P2 / P3 / P4
	CPU         float64 `json:"cpu"`   // threshold %, 0 = metric not checked
	Mem         float64 `json:"mem"`
	Disk        float64 `json:"disk"`
	DurationSec int     `json:"duration_sec"` // duration in seconds, 0 = immediate
	ChannelIDs  []uint  `json:"channel_ids"`
}

// DefaultCmdLevels P1 most severe → P4 informational
func DefaultCmdLevels() []CmdLevel {
	return []CmdLevel{
		{Level: "P1", CPU: 95, Mem: 95, Disk: 95, DurationSec: 60},
		{Level: "P2", CPU: 90, Mem: 90, Disk: 90, DurationSec: 120},
		{Level: "P3", CPU: 80, Mem: 85, Disk: 85, DurationSec: 300},
		{Level: "P4", CPU: 70, Mem: 75, Disk: 75, DurationSec: 900},
	}
}

// LoadCmdLevels loads the level config (persists defaults when none exists)
func LoadCmdLevels() []CmdLevel {
	m := SystemConfigMap()
	raw := m[cmdLevelsKey]
	if raw == "" {
		def := DefaultCmdLevels()
		SaveCmdLevels(def)
		return def
	}
	var levels []CmdLevel
	if err := json.Unmarshal([]byte(raw), &levels); err != nil || len(levels) == 0 {
		return DefaultCmdLevels()
	}
	return levels
}

// SaveCmdLevels saves the level config
func SaveCmdLevels(levels []CmdLevel) error {
	b, err := json.Marshal(levels)
	if err != nil {
		return err
	}
	return model.DB.Save(&model.SystemConfig{Key: cmdLevelsKey, Value: string(b)}).Error
}

// matchLevel returns the most severe matched level and the metric exceeding its threshold by the largest margin (P1 first)
func matchLevel(host *model.Host, s hostSample, levels []CmdLevel) (CmdLevel, string, float64, float64, bool) {
	for _, lv := range levels {
		type cand struct {
			metric string
			value  float64
			th     float64
		}
		var worst *cand
		for _, c := range []cand{
			{"CPU", s.CPU, lv.CPU}, {"Memory", s.Mem, lv.Mem}, {"Disk", s.Disk, lv.Disk},
		} {
			if c.th > 0 && c.value >= c.th {
				if worst == nil || c.value/c.th > worst.value/worst.th {
					c := c
					worst = &c
				}
			}
		}
		if worst != nil {
			return lv, worst.metric, worst.value, worst.th, true
		}
	}
	return CmdLevel{}, "", 0, 0, false
}

// EvaluateCmdAlerts evaluates the latest sample of one host against the levels (called by the collection pipeline)
func EvaluateCmdAlerts(h *model.Host, s hostSample, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[alert] cmd evaluate panic:", r)
		}
	}()
	levels := LoadCmdLevels()
	var st model.CmdAlertState
	hasState := model.DB.Where("host_id = ?", h.ID).First(&st).Error == nil

	lv, metric, value, th, matched := matchLevel(h, s, levels)
	newLevel := ""
	if matched {
		newLevel = lv.Level
	}

	curLevel := ""
	if hasState {
		curLevel = st.Level
	}
	if curLevel == newLevel {
		// Level unchanged: trigger if not yet fired and the duration has been met
		if matched && hasState && !st.Fired && st.Since != nil &&
			now.Sub(*st.Since) >= time.Duration(lv.DurationSec)*time.Second {
			st.Fired = true
			model.DB.Model(&st).Update("fired", true)
			SendCmdLevelAlert(h, lv, metric, value, th, false)
			go AutoDiagnose(h, lv.Level, metric, value, th)
		}
		return
	}

	// Level changed (escalate/degrade/recover): send recovery for the old level if it had fired (subject to the global recovery switch)
	if hasState && curLevel != "" && st.Fired && LoadAlertRule().NotifyRecovery {
		old := findLevel(levels, curLevel)
		SendCmdLevelAlert(h, old, metric, value, th, true)
	}

	// Persist the new level state
	st.HostID = h.ID
	st.Level = newLevel
	if matched {
		st.Since = &now
	} else {
		st.Since = nil
	}
	st.Fired = false
	model.DB.Save(&st)

	// New level has zero duration: fire immediately
	if matched && lv.DurationSec <= 0 {
		st.Fired = true
		model.DB.Model(&st).Update("fired", true)
		SendCmdLevelAlert(h, lv, metric, value, th, false)
		go AutoDiagnose(h, lv.Level, metric, value, th)
	}
}

func findLevel(levels []CmdLevel, key string) CmdLevel {
	for _, lv := range levels {
		if lv.Level == key {
			return lv
		}
	}
	return CmdLevel{Level: key}
}

// SendCmdLevelAlert sends the alert/recovery notification for a level to its bound channels
func SendCmdLevelAlert(h *model.Host, lv CmdLevel, metric string, value, th float64, recovery bool) {
	var channels []model.AlertChannel
	if len(lv.ChannelIDs) > 0 {
		model.DB.Where("id IN ? AND enabled = ?", lv.ChannelIDs, true).Find(&channels)
	}
	if len(channels) == 0 {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	tpl := LoadAlertTemplates()
	vars := map[string]string{
		"level": lv.Level, "host": h.Name, "ip": h.IP,
		"metric": metric, "value": fmt.Sprintf("%.1f", value), "threshold": fmt.Sprintf("%.0f", th),
		"time": now,
	}
	// Alert event history
	if recovery {
		LogAlertRecovery("cmd_level", h.Name, time.Now())
	} else {
		LogAlertEvent("cmd_level", lv.Level, h.Name,
			fmt.Sprintf("%s %s %.1f%% 超过阈值 %.0f%%", h.Name, metric, value, th))
	}
	defSubject, defBody := renderTpl(tpl.CmdAlertTitle, vars), renderTpl(tpl.CmdAlertBody, vars)
	if recovery {
		defSubject, defBody = renderTpl(tpl.CmdRecoveryTitle, vars), renderTpl(tpl.CmdRecoveryBody, vars)
	}
	for _, ch := range channels {
		ch := ch
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[alert] channel %s send panic: %v\n", ch.Name, r)
				}
			}()
			if err := SendViaChannel(&ch, vars, defSubject, defBody); err != nil {
				fmt.Printf("[alert] channel %s(%s) send failed: %v\n", ch.Name, ch.Type, err)
			}
		}()
	}
}
