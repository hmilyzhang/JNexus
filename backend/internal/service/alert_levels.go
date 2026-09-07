// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"time"

	"jnexus/internal/model"
)

// CMD 资源（CPU/内存/磁盘）P1-P4 分级阈值告警：
// 任一指标越过某级阈值即进入该级（取最严重级），持续满级内时长后向该级绑定通道推送；
// 级别变化时对旧级别发恢复（受全局恢复开关约束）；模板支持占位符自定义。

const cmdLevelsKey = "cmd_alert_levels"

// CmdLevel 单个告警级别配置
type CmdLevel struct {
	Level       string  `json:"level"` // P1 / P2 / P3 / P4
	CPU         float64 `json:"cpu"`   // 阈值 %，0 = 该指标不参与
	Mem         float64 `json:"mem"`
	Disk        float64 `json:"disk"`
	DurationSec int     `json:"duration_sec"` // 持续时长（秒），0 = 立即
	ChannelIDs  []uint  `json:"channel_ids"`
}

// DefaultCmdLevels P1 最严重 → P4 提示
func DefaultCmdLevels() []CmdLevel {
	return []CmdLevel{
		{Level: "P1", CPU: 95, Mem: 95, Disk: 95, DurationSec: 60},
		{Level: "P2", CPU: 90, Mem: 90, Disk: 90, DurationSec: 120},
		{Level: "P3", CPU: 80, Mem: 85, Disk: 85, DurationSec: 300},
		{Level: "P4", CPU: 70, Mem: 75, Disk: 75, DurationSec: 900},
	}
}

// LoadCmdLevels 读取分级配置（无配置时落库默认值）
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

// SaveCmdLevels 保存分级配置
func SaveCmdLevels(levels []CmdLevel) error {
	b, err := json.Marshal(levels)
	if err != nil {
		return err
	}
	return model.DB.Save(&model.SystemConfig{Key: cmdLevelsKey, Value: string(b)}).Error
}

// matchLevel 返回命中的最严重级别与越阈最显著的指标（P1 优先）
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

// EvaluateCmdAlerts 对一台主机的最新采样做分级评估（采集流程调用）
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
		// 级别未变化：未发过告警且持续满时长则触发
		if matched && hasState && !st.Fired && st.Since != nil &&
			now.Sub(*st.Since) >= time.Duration(lv.DurationSec)*time.Second {
			st.Fired = true
			model.DB.Model(&st).Update("fired", true)
			SendCmdLevelAlert(h, lv, metric, value, th, false)
		}
		return
	}

	// 级别变化（升级/降级/恢复）：旧级别已告警则发恢复（受全局恢复开关约束）
	if hasState && curLevel != "" && st.Fired && LoadAlertRule().NotifyRecovery {
		old := findLevel(levels, curLevel)
		SendCmdLevelAlert(h, old, metric, value, th, true)
	}

	// 写入新级别状态
	st.HostID = h.ID
	st.Level = newLevel
	if matched {
		st.Since = &now
	} else {
		st.Since = nil
	}
	st.Fired = false
	model.DB.Save(&st)

	// 新级别时长为 0：立即触发
	if matched && lv.DurationSec <= 0 {
		st.Fired = true
		model.DB.Model(&st).Update("fired", true)
		SendCmdLevelAlert(h, lv, metric, value, th, false)
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

// SendCmdLevelAlert 发送某级别的告警/恢复通知到该级别绑定的通道
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
	// 告警事件历史
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
