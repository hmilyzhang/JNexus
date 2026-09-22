// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"strconv"
	"time"

	"jnexus/internal/model"
)

// SendCertAlert notifies the monitor's bound channels about HTTPS certificate
// expiry (tiered: warn / critical). Mirrors SendMonitorAlert's channel fan-out.
func SendCertAlert(m *model.Monitor, level int, daysLeft int, notAfter time.Time) {
	var bindings []model.MonitorChannel
	model.DB.Where("monitor_id = ?", m.ID).Find(&bindings)
	if len(bindings) == 0 {
		return
	}
	var channelIDs []uint
	for _, b := range bindings {
		channelIDs = append(channelIDs, b.ChannelID)
	}
	var channels []model.AlertChannel
	model.DB.Where("id IN ? AND enabled = ?", channelIDs, true).Find(&channels)
	if len(channels) == 0 {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	emoji, event := "🟠", "CERT WARN"
	if level >= 2 {
		emoji, event = "🔴", "CERT EXPIRY"
	}
	kind := "即将到期"
	if daysLeft < 0 {
		kind = "已过期"
		daysLeft = -daysLeft
	}
	expiryStr := notAfter.Format("2006-01-02")
	vars := map[string]string{
		"monitor": m.Name, "type": m.Type, "target": monitorTargetText(m),
		"days": strconv.Itoa(daysLeft), "expiry": expiryStr, "time": now,
	}
	defSubject := fmt.Sprintf("%s [%s] %s 证书%s（剩余 %d 天）", emoji, event, m.Name, kind, daysLeft)
	defBody := fmt.Sprintf("HTTPS 证书生命周期提醒\nMonitor: %s\nTarget: %s\n到期时间: %s\n剩余天数: %d\nTime: %s",
		m.Name, monitorTargetText(m), expiryStr, daysLeft, now)

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
