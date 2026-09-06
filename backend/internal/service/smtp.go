// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSettings 邮件服务器设置
type SMTPSettings struct {
	Enabled    bool
	Host       string
	Port       int
	SSL        bool // 直连 TLS (465)
	StartTLS   bool // STARTTLS (587/25)
	Username   string
	Password   string
	From       string
	Recipients []string
	Notify     bool // 任务结束后发送通知
}

func LoadSMTPSettings() SMTPSettings {
	m := SystemConfigMap()
	s := SMTPSettings{
		Enabled:  m["smtp_enabled"] == "true",
		Host:     m["smtp_host"],
		SSL:      m["smtp_ssl"] == "true",
		StartTLS: m["smtp_tls"] == "true",
		Username: m["smtp_username"],
		Password: m["smtp_password"],
		From:     m["smtp_from"],
		Notify:   m["smtp_notify"] == "true",
	}
	fmt.Sscanf(m["smtp_port"], "%d", &s.Port)
	if s.Port == 0 {
		s.Port = 25
	}
	for _, r := range strings.FieldsFunc(m["smtp_recipients"], func(r rune) bool { return r == ',' || r == 10 || r == 13 || r == 59 }) {
		if r = strings.TrimSpace(r); r != "" {
			s.Recipients = append(s.Recipients, r)
		}
	}
	return s
}

// SendMail 通过 SMTP 发送邮件（支持 SSL / STARTTLS / 认证）
func SendMail(s SMTPSettings, to []string, subject, htmlBody string) error {
	if s.Host == "" || len(to) == 0 {
		return fmt.Errorf("SMTP 未配置完整")
	}
	from := s.From
	if from == "" {
		from = s.Username
	}

	// 主题 UTF-8 Base64 编码，避免中文乱码
	encSubject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + strings.Join(to, ","),
		"Subject: " + encSubject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		htmlBody,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)

	send := func(cl *smtp.Client) error {
		if cl == nil {
			return fmt.Errorf("SMTP 连接失败")
		}
		defer cl.Close()
		if ok, _ := cl.Extension("AUTH"); ok && s.Username != "" {
			if err := cl.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
		if err := cl.Mail(from); err != nil {
			return err
		}
		for _, r := range to {
			if err := cl.Rcpt(r); err != nil {
				return fmt.Errorf("收件人 %s 被拒绝: %w", r, err)
			}
		}
		w, err := cl.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return cl.Quit()
	}

	if s.SSL {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.Host})
		if err != nil {
			return fmt.Errorf("SMTP SSL 连接失败: %w", err)
		}
		cl, err := smtp.NewClient(conn, s.Host)
		if err != nil {
			return err
		}
		return send(cl)
	}

	cl, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP 连接失败: %w", err)
	}
	if s.StartTLS {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
				return fmt.Errorf("STARTTLS 失败: %w", err)
			}
		}
	}
	return send(cl)
}
