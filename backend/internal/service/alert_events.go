// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"time"

	"jnexus/internal/model"
)

// AlertEvent 落库：所有告警通知发出时同步记录（月报/告警统计的数据源）

func LogAlertEvent(kind, level, target, message string) {
	defer func() { recover() }()
	model.DB.Create(&model.AlertEvent{
		Kind: kind, Level: level, Target: target, Message: message, FiredAt: time.Now(),
	})
}

// LogAlertRecovery 记录恢复事件：给最近一条同 kind+target 未恢复的事件补恢复时间
func LogAlertRecovery(kind, target string, recoveredAt time.Time) {
	defer func() { recover() }()
	var ev model.AlertEvent
	if err := model.DB.Where("kind = ? AND target = ? AND recovered_at IS NULL", kind, target).
		Order("fired_at DESC").First(&ev).Error; err != nil {
		return
	}
	dur := int(recoveredAt.Sub(ev.FiredAt).Seconds())
	model.DB.Model(&ev).Updates(map[string]any{
		"recovered_at": recoveredAt, "duration_sec": dur,
		"message": ev.Message + " [已恢复]",
	})
}
