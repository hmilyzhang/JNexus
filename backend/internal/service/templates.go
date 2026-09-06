// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"

	"jnexus/internal/model"
)

// 全局默认通知模板（占位符渲染；通道级 title_tpl/body_tpl 优先级更高）。
// 五类通知源 × 标题/正文，存系统配置 JSON；字段留空使用内建默认。

const alertTemplatesKey = "alert_templates"

// AlertTemplates 各通知源的默认标题/正文模板（占位符形式存储）
type AlertTemplates struct {
	MonitorAlertTitle    string `json:"monitor_alert_title"`
	MonitorAlertBody     string `json:"monitor_alert_body"`
	MonitorRecoveryTitle string `json:"monitor_recovery_title"`
	MonitorRecoveryBody  string `json:"monitor_recovery_body"`
	RebootTitle          string `json:"reboot_title"`
	RebootBody           string `json:"reboot_body"`
	CmdAlertTitle        string `json:"cmd_alert_title"`
	CmdAlertBody         string `json:"cmd_alert_body"`
	CmdRecoveryTitle     string `json:"cmd_recovery_title"`
	CmdRecoveryBody      string `json:"cmd_recovery_body"`
	EmailTitle           string `json:"email_title"` // 邮件通道专用（优先于通用模板）
	EmailBody            string `json:"email_body"`  // 支持 HTML
}

// BuiltinAlertTemplates 内建默认模板（与历史行为一致）
func BuiltinAlertTemplates() AlertTemplates {
	return AlertTemplates{
		MonitorAlertTitle:    "🔴 [ALERT] {monitor}",
		MonitorAlertBody:     "Monitor: {monitor}\nType: {type}\nTarget: {target}\nStatus: {status}\nResponse: {resp_ms}ms\nTime: {time}",
		MonitorRecoveryTitle: "🟢 [RECOVERY] {monitor}",
		MonitorRecoveryBody:  "Monitor: {monitor}\nType: {type}\nTarget: {target}\nStatus: UP\nTime: {time}",
		RebootTitle:          "🔄 [REBOOT] {host}",
		RebootBody:           "Host: {host}\nIP: {ip}\nEvent: system rebooted (boot_id changed)\nDetected: {time}",
		CmdAlertTitle:        "🔴 [{level}][{metric}] {host}",
		CmdAlertBody:         "[{level}] {host} ({ip})\n{metric}: {value}%（阈值 {threshold}%）\n时间: {time}",
		CmdRecoveryTitle:     "🟢 [{level} 恢复] {host}",
		CmdRecoveryBody:      "{host} ({ip})\n{metric}: {value}%\n时间: {time}",
	}
}

// LoadAlertTemplates 读取生效模板（未配置的字段回落内建默认）
func LoadAlertTemplates() AlertTemplates {
	def := BuiltinAlertTemplates()
	m := SystemConfigMap()
	raw := m[alertTemplatesKey]
	if raw == "" {
		return def
	}
	var stored AlertTemplates
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return def
	}
	if stored.MonitorAlertTitle != "" {
		def.MonitorAlertTitle = stored.MonitorAlertTitle
	}
	if stored.MonitorAlertBody != "" {
		def.MonitorAlertBody = stored.MonitorAlertBody
	}
	if stored.MonitorRecoveryTitle != "" {
		def.MonitorRecoveryTitle = stored.MonitorRecoveryTitle
	}
	if stored.MonitorRecoveryBody != "" {
		def.MonitorRecoveryBody = stored.MonitorRecoveryBody
	}
	if stored.RebootTitle != "" {
		def.RebootTitle = stored.RebootTitle
	}
	if stored.RebootBody != "" {
		def.RebootBody = stored.RebootBody
	}
	if stored.CmdAlertTitle != "" {
		def.CmdAlertTitle = stored.CmdAlertTitle
	}
	if stored.CmdAlertBody != "" {
		def.CmdAlertBody = stored.CmdAlertBody
	}
	if stored.CmdRecoveryTitle != "" {
		def.CmdRecoveryTitle = stored.CmdRecoveryTitle
	}
	if stored.CmdRecoveryBody != "" {
		def.CmdRecoveryBody = stored.CmdRecoveryBody
	}
	if stored.EmailTitle != "" {
		def.EmailTitle = stored.EmailTitle
	}
	if stored.EmailBody != "" {
		def.EmailBody = stored.EmailBody
	}
	return def
}

// SaveAlertTemplates 保存模板（字段留空 = 恢复内建默认）
func SaveAlertTemplates(t AlertTemplates) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return model.DB.Save(&model.SystemConfig{Key: alertTemplatesKey, Value: string(b)}).Error
}
