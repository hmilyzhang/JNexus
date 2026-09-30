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
		&HostCredential{}, &CredentialPasswordHistory{}, &UserGroupCredential{}, &UserGroupCredRule{}, &CronJob{}, &Report{}, &ReportItem{},
		&WebAsset{}, &DBAccount{}, &CloudAccount{},
		&Monitor{}, &MonitorSample{}, &HostMetric{}, &TrustedCA{}, &K8sCapacitySample{}, &AlertEvent{}, &K8sPodSample{}, &HostMetricHourly{}, &CmdAlertState{}, &K8sCluster{}, &K8sClusterMember{}, &MaintenanceLog{}, &UserGroupApp{}, &ApiKey{}, &AlertChannel{}, &MonitorChannel{}, &DbSource{},
		&MonitorSampleHourly{},
	); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := MigrateHostCredentials(); err != nil {
		return fmt.Errorf("主机凭据迁移失败: %w", err)
	}
	if err := DropHostGroupNameUniqueIndex(); err != nil {
		return fmt.Errorf("主机分组索引迁移失败: %w", err)
	}
	if err := MigrateDBAccounts(); err != nil {
		return fmt.Errorf("数据库账号迁移失败: %w", err)
	}
	if err := NormalizeDefaultCredentials(); err != nil {
		return fmt.Errorf("默认账号归一失败: %w", err)
	}
	return nil
}

// NormalizeDefaultCredentials repairs hosts that carry more than one default
// OS account (legacy writes set is_default without clearing the others): the
// earliest account stays default, every later duplicate is cleared.
func NormalizeDefaultCredentials() error {
	var hostIDs []uint
	if err := DB.Model(&HostCredential{}).
		Where("is_default = ?", true).
		Group("host_id").
		Having("COUNT(*) > 1").
		Pluck("host_id", &hostIDs).Error; err != nil {
		return err
	}
	for _, hid := range hostIDs {
		var keep uint
		if err := DB.Model(&HostCredential{}).Where("host_id = ?", hid).
			Where("is_default = ?", true).Order("id").Limit(1).Pluck("id", &keep).Error; err != nil {
			return err
		}
		if err := DB.Model(&HostCredential{}).
			Where("host_id = ? AND is_default = ? AND id <> ?", hid, true, keep).
			Update("is_default", false).Error; err != nil {
			return err
		}
		log.Printf("[migrate] host %d had multiple default accounts; kept #%d", hid, keep)
	}
	return nil
}

// DropHostGroupNameUniqueIndex removes the legacy global-unique index on
// host_groups.name: group names are now unique per parent only (filesystem-style).
// GORM never drops old indexes, so any leftover unique index is dropped by name.
func DropHostGroupNameUniqueIndex() error {
	var idxs []string
	if err := DB.Raw(
		"SELECT indexname FROM pg_indexes WHERE tablename = 'host_groups' AND indexdef LIKE '%UNIQUE%' AND indexname <> 'host_groups_pkey'",
	).Scan(&idxs).Error; err != nil {
		return err
	}
	for _, n := range idxs {
		if err := DB.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", n)).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateDBAccounts copies each existing ingestion source's credentials into a
// DBAccount row (label 采集账号) so the workbench sees them; runs once per source.
func MigrateDBAccounts() error {
	var sources []DbSource
	DB.Find(&sources)
	for _, src := range sources {
		if src.Username == "" {
			continue
		}
		var cnt int64
		DB.Model(&DBAccount{}).Where("source_id = ?", src.ID).Count(&cnt)
		if cnt > 0 {
			continue
		}
		if err := DB.Create(&DBAccount{
			SourceID: src.ID, Username: src.Username, Password: src.Password,
			Label: "采集账号", CreatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
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
	return nil
}

// GenAESKey generates a 32-byte base64 master key (utility)
func GenAESKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func Now() *time.Time { t := time.Now(); return &t }
