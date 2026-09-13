// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"time"

	"jnexus/internal/model"
)

// AlertEvent persistence: recorded whenever an alert notification is sent (data source for monthly reports and alert statistics)

func LogAlertEvent(kind, level, target, message string) {
	defer func() { recover() }()
	model.DB.Create(&model.AlertEvent{
		Kind: kind, Level: level, Target: target, Message: message, FiredAt: time.Now(),
	})
}

// LogAlertRecovery records a recovery event: fills in the recovery time on the latest unrecovered event with the same kind+target
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
