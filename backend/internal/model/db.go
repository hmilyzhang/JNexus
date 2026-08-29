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
	); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	return nil
}

// Seed 初始化默认管理员、危险命令规则
func Seed() error {
	var cnt int64
	DB.Model(&User{}).Count(&cnt)
	if cnt == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := User{Username: "admin", Password: string(hash), Role: RoleAdmin, Status: 1}
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

// GenAESKey 生成 32 字节 base64 主密钥（工具函数）
func GenAESKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func Now() *time.Time { t := time.Now(); return &t }
