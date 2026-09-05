// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"autoops/internal/model"
)

// 告警通知通道分发：邮件 / Webhook / 企业微信 / 钉钉 / 飞书 / Telegram（参考 Uptime Kuma）

func postJSON(url string, payload any, timeout time.Duration) error {
	body, _ := json.Marshal(payload)
	cli := &http.Client{Timeout: timeout}
	resp, err := cli.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// renderTpl 模板占位符渲染：{key} 替换为 vars 值
func renderTpl(tpl string, vars map[string]string) string {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// SendViaChannel 通过指定通道发送通知。
// vars 为模板占位符变量（各告警源提供）；通道配置里的 title_tpl / body_tpl
// 非空时优先渲染，否则使用调用方给出的默认标题/正文。
func SendViaChannel(ch *model.AlertChannel, vars map[string]string, defSubject, defBody string) error {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
		return fmt.Errorf("通道配置不是合法 JSON: %v", err)
	}
	subject := defSubject
	if t := strings.TrimSpace(cfg["title_tpl"]); t != "" {
		subject = renderTpl(t, vars)
	}
	text := defBody
	if t := strings.TrimSpace(cfg["body_tpl"]); t != "" {
		text = renderTpl(t, vars)
	}
	timeout := 10 * time.Second
	switch ch.Type {
	case "email":
		smtp := LoadSMTPSettings()
		if !smtp.Enabled {
			return fmt.Errorf("系统 SMTP 未启用（系统设置 → 邮件 SMTP）")
		}
		recipients := strings.Split(strings.Trim(cfg["recipients"], ","), ",")
		to := recipients[:0]
		for _, r := range recipients {
			if r = strings.TrimSpace(r); r != "" {
				to = append(to, r)
			}
		}
		if len(to) == 0 {
			return fmt.Errorf("收件人未配置")
		}
		return SendMail(smtp, to, subject, "<pre style='font-family:monospace'>"+text+"</pre>")
	case "webhook":
		url := strings.TrimSpace(cfg["url"])
		if url == "" {
			return fmt.Errorf("Webhook URL 未配置")
		}
		payload := map[string]any{
			"title": subject, "message": text, "timestamp": time.Now().Format(time.RFC3339),
		}
		return postJSON(url, payload, timeout)
	case "wecom": // 企业微信群机器人
		url := strings.TrimSpace(cfg["url"])
		if url == "" {
			return fmt.Errorf("企业微信机器人 URL 未配置")
		}
		return postJSON(url, map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": fmt.Sprintf("**%s**\n%s", subject, text),
			},
		}, timeout)
	case "dingtalk": // 钉钉群机器人
		url := strings.TrimSpace(cfg["url"])
		if url == "" {
			return fmt.Errorf("钉钉机器人 URL 未配置")
		}
		return postJSON(url, map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": subject,
				"text":  fmt.Sprintf("### %s\n\n%s", subject, text),
			},
		}, timeout)
	case "feishu": // 飞书自定义机器人
		url := strings.TrimSpace(cfg["url"])
		if url == "" {
			return fmt.Errorf("飞书机器人 URL 未配置")
		}
		return postJSON(url, map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": subject + "\n" + text},
		}, timeout)
	case "telegram":
		token := strings.TrimSpace(cfg["bot_token"])
		chatID := strings.TrimSpace(cfg["chat_id"])
		if token == "" || chatID == "" {
			return fmt.Errorf("Telegram bot token / chat id 未配置")
		}
		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
		return postJSON(url, map[string]any{
			"chat_id": chatID, "text": subject + "\n" + text,
		}, timeout)
	default:
		return fmt.Errorf("未知通道类型: %s", ch.Type)
	}
}

// SendMonitorAlert 按报警规则的判定结果向绑定通道发送告警/恢复通知
func SendMonitorAlert(m *model.Monitor, status string, respMs int, errMsg string) {
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
	down := status == "down"
	emoji := "🟢"
	event := "RECOVERY"
	if down {
		emoji = "🔴"
		event = "ALERT"
	}
	vars := map[string]string{
		"monitor": m.Name, "type": m.Type, "target": monitorTargetText(m),
		"status": strings.ToUpper(status), "resp_ms": strconv.Itoa(respMs),
		"error": errMsg, "time": now,
	}
	defSubject := fmt.Sprintf("%s [%s] %s", emoji, event, m.Name)
	defBody := fmt.Sprintf("Monitor: %s\nType: %s\nTarget: %s\nStatus: %s\nResponse: %dms\nTime: %s",
		m.Name, m.Type, monitorTargetText(m), strings.ToUpper(status), respMs, now)
	tpl := LoadAlertTemplates()
	if down {
		defSubject, defBody = renderTpl(tpl.MonitorAlertTitle, vars), renderTpl(tpl.MonitorAlertBody, vars)
	} else {
		defSubject, defBody = renderTpl(tpl.MonitorRecoveryTitle, vars), renderTpl(tpl.MonitorRecoveryBody, vars)
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

// SendHostRebootAlert 主机重启自动告警（boot_id 变化时触发），广播到所有启用的通知通道
func SendHostRebootAlert(h *model.Host, newBootID string) {
	var channels []model.AlertChannel
	model.DB.Where("enabled = ?", true).Find(&channels)
	if len(channels) == 0 {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	vars := map[string]string{
		"host": h.Name, "ip": h.IP, "event": "system rebooted (boot_id changed)", "time": now,
	}
	tpl := LoadAlertTemplates()
	defSubject, defBody := renderTpl(tpl.RebootTitle, vars), renderTpl(tpl.RebootBody, vars)
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

func monitorTargetText(m *model.Monitor) string {
	if m.Type == "tcp" {
		return fmt.Sprintf("%s:%d", m.Target, m.Port)
	}
	return m.Target
}
