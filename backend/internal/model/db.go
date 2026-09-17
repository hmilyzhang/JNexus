// JNexus Ops Platform — By JJ Zhang, Version 1.0

package model

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect(dsn string) error {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	DB = db
	if err := DB.AutoMigrate(
		&User{}, &HostGroup{}, &SSHKey{}, &Host{}, &Script{},
		&Task{}, &TaskHostResult{},
		&Application{}, &AppHost{}, &Release{}, &ReleaseItem{},
		&DangerRule{}, &AuditLog{}, &UserHostGroup{}, &UserApp{},
		&SystemConfig{},
		&UserGroup{}, &UserGroupHost{}, &UserGroupHostGroup{}, &UserGroupMember{},
		&HostCredential{}, &UserGroupCredential{}, &UserGroupCredRule{}, &CronJob{}, &Report{}, &ReportItem{},
		&Monitor{}, &MonitorSample{}, &HostMetric{}, &K8sCapacitySample{}, &AlertEvent{}, &K8sPodSample{}, &HostMetricHourly{}, &CmdAlertState{}, &K8sCluster{}, &K8sClusterMember{}, &MaintenanceLog{}, &UserGroupApp{}, &ApiKey{}, &AlertChannel{}, &MonitorChannel{}, &DbSource{}, &ChangelogEntry{},
	); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := MigrateHostCredentials(); err != nil {
		return fmt.Errorf("主机凭据迁移失败: %w", err)
	}
	return nil
}

// MigrateHostCredentials auto-migration for existing hosts: creates a default credential for each host that already has an account
func MigrateHostCredentials() error {
	var hosts []Host
	DB.Find(&hosts)
	for _, h := range hosts {
		var cnt int64
		DB.Model(&HostCredential{}).Where("host_id = ?", h.ID).Count(&cnt)
		if cnt == 0 && h.Username != "" {
			if err := DB.Create(&HostCredential{
				HostID: h.ID, Username: h.Username, AuthType: h.AuthType,
				SSHKeyID: h.SSHKeyID, Password: h.Password, Label: "默认", IsDefault: true,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// SeedConfig initializes system config defaults
func SeedConfig() error {
	defaults := map[string]string{
		"system_name":                "JNexus 运维平台",
		"ldap_enabled":               "false",
		"ldap_host":                  "",
		"ldap_port":                  "389",
		"ldap_tls":                   "false",
		"ldap_bind_dn":               "",
		"ldap_bind_password":         "",
		"ldap_base_dn":               "",
		"ldap_user_filter":           "(uid=%s)",
		"ldap_attr_username":         "uid",
		"ldap_default_role":          string(RoleViewer),
		"ldap_group_check":           "false",
		"ldap_group_base_dn":         "",
		"ldap_group_filter":          "(member=%s)",
		"ldap_required_groups":       "",
		"monitor_enabled":            "true",
		"alert_rule_mode":            "grace",
		"alert_rule_grace_sec":       "60",
		"alert_rule_notify_recovery": "true",
		"monitor_interval_sec":       "60",
		"smtp_enabled":               "false",
		"smtp_host":                  "",
		"smtp_port":                  "25",
		"smtp_ssl":                   "false",
		"smtp_tls":                   "true",
		"smtp_username":              "",
		"smtp_password":              "",
		"smtp_from":                  "",
		"smtp_recipients":            "",
		"smtp_notify":                "true",
		"rotation_enabled":           "false",
		"rotation_length":            "20",
		"rotation_complexity":        "high",
		"rotation_days":              "90",
	}
	for k, v := range defaults {
		var cnt int64
		DB.Model(&SystemConfig{}).Where("key = ?", k).Count(&cnt)
		if cnt == 0 {
			if err := DB.Create(&SystemConfig{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// Seed initializes the default admin and dangerous command rules
func Seed() error {
	var cnt int64
	DB.Model(&User{}).Count(&cnt)
	if cnt == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := User{Username: "admin", Password: string(hash), Role: RoleAdmin, Status: 1, CreatedBy: "system", UpdatedBy: "system"}
		if err := DB.Create(&admin).Error; err != nil {
			return err
		}
		log.Println("已创建默认管理员 admin / admin123，请尽快修改密码")
	}

	DB.Model(&DangerRule{}).Count(&cnt)
	if cnt == 0 {
		defaults := []DangerRule{
			{Pattern: `rm\s+-[a-zA-Z]*[rf]{1,2}[a-zA-Z]*\s+/(?:\s|$)`, Desc: "递归强制删除根目录", Enabled: true},
			{Pattern: `rm\s+-[a-zA-Z]*r[a-zA-Z]*f?[a-zA-Z]*\s+(/|~|\*)`, Desc: "递归强制删除根/家目录/通配", Enabled: true},
			{Pattern: `mkfs(\.\w+)?\s`, Desc: "格式化文件系统", Enabled: true},
			{Pattern: `dd\s+.*of=/dev/`, Desc: "dd 直接写块设备", Enabled: true},
			{Pattern: `:\(\)\{.*\};:`, Desc: "Fork 炸弹", Enabled: true},
			{Pattern: `\b(shutdown|halt|poweroff|reboot|init\s+[06])\b`, Desc: "关机/重启类命令", Enabled: true},
			{Pattern: `\bdrop\s+(database|schema)\b`, Desc: "删除数据库", Enabled: true},
			{Pattern: `\btruncate\s+table\b`, Desc: "清空数据表", Enabled: true},
			{Pattern: `chmod\s+-R\s+777\s+/(?:\s|$)`, Desc: "根目录递归 777", Enabled: true},
			{Pattern: `>\s*/dev/sd[a-z]`, Desc: "覆写块设备", Enabled: true},
			{Pattern: `\bhistory\s+-c\b`, Desc: "清除历史记录", Enabled: true},
		}
		if err := DB.Create(&defaults).Error; err != nil {
			return err
		}
	}
	return SeedChangelog()
}

// SeedChangelog fills initial release notes once, so the Changelog page is not
// empty on first deploy; admins maintain entries from the UI afterwards
func SeedChangelog() error {
	var cnt int64
	DB.Model(&ChangelogEntry{}).Count(&cnt)
	if cnt > 0 {
		return nil
	}
	d := func(s string) time.Time {
		t, _ := time.ParseInLocation("2006-01-02", s, time.Local)
		return t
	}
	defaults := []ChangelogEntry{
		{Version: "1.280", Title: "AI 诊断迁移至监控中心", Details: "监控中心新增「AI 诊断」标签页（诊断配置 + 清理命令目录），与告警规则同页维护；系统设置 → AI 助手中的原入口移除。", ReleasedAt: d("2026-09-17")},
		{Version: "1.279", Title: "配对密钥页升级", Details: "新增平台密钥轮换配置（开关 / 周期 / 立即轮换）；配对凭据列表支持分页与关键词搜索，适配数百台规模。", ReleasedAt: d("2026-09-17")},
		{Version: "1.278", Title: "SSH 平台密钥周期轮换", Details: "支持按周期（如 30/60 天）自动生成新密钥对并推送到全部配对主机，可手动立即轮换，全程审计。", ReleasedAt: d("2026-09-17")},
		{Version: "1.277", Title: "Windows 主机终端", Details: "Shell 工作区支持通过 WinRM 打开 PowerShell 终端（需密码认证的 OS 账号），域名环境支持 Kerberos 认证。", ReleasedAt: d("2026-09-17")},
		{Version: "1.276", Title: "文档与手册", Details: "新增系统架构设计文档与用户手册（中英文），补充可观测集成、AI 诊断、Kerberos 与反向代理部署章节。", ReleasedAt: d("2026-09-17")},
		{Version: "1.275", Title: "月报打印修复", Details: "月度报告打印/导出 PDF 时样式丢失的问题修复，打印输出强制浅色版式。", ReleasedAt: d("2026-09-17")},
		{Version: "1.271", Title: "AI 告警诊断与受控磁盘清理", Details: "P1–P4 阈值告警触发后通过 SSH 采集系统状态，AI 分析根因/可疑进程并经告警通道推送；管理员可定义受控清理命令目录，逐字执行。", ReleasedAt: d("2026-09-16")},
		{Version: "1.266", Title: "日志检索", Details: "原生 OpenObserve 日志检索页：SQL 编辑、时间范围、host / event id 过滤、结果展开与 CSV 导出。", ReleasedAt: d("2026-09-15")},
	}
	return DB.Create(&defaults).Error
}

// GenAESKey generates a 32-byte base64 master key (utility)
func GenAESKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func Now() *time.Time { t := time.Now(); return &t }
