// JNexus 运维平台 — By JJ Zhang, Version 1.0

package config

import (
	"encoding/base64"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int    `yaml:"port"`
		Mode string `yaml:"mode"` // debug / release
	} `yaml:"server"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Auth struct {
		JWTSecret string `yaml:"jwt_secret"`
		// 32 字节 base64 编码的主密钥，用于加密落库的 SSH 私钥
		AESKey string `yaml:"aes_key"`
	} `yaml:"auth"`
	Storage struct {
		UploadDir string `yaml:"upload_dir"`
	} `yaml:"storage"`
}

var Cfg Config

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// 容器部署可不带配置文件：环境变量提供数据库连接即可启动
		if os.IsNotExist(err) && (os.Getenv("AUTOOPS_DSN") != "" || os.Getenv("AUTOOPS_DB_HOST") != "") {
			data = nil
		} else {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &Cfg); err != nil {
			return fmt.Errorf("解析配置文件失败: %w", err)
		}
	}
	// 环境变量覆盖
	if v := os.Getenv("AUTOOPS_DSN"); v != "" {
		Cfg.Database.DSN = v
	}
	// 外部数据库分项配置：AUTOOPS_DB_HOST/PORT/USER/PASSWORD/NAME
	// 设置了 AUTOOPS_DB_HOST 且未显式给 DSN 时，自动拼装 DSN
	if Cfg.Database.DSN == "" && os.Getenv("AUTOOPS_DB_HOST") != "" {
		host := os.Getenv("AUTOOPS_DB_HOST")
		port := os.Getenv("AUTOOPS_DB_PORT")
		user := os.Getenv("AUTOOPS_DB_USER")
		pass := os.Getenv("AUTOOPS_DB_PASSWORD")
		name := os.Getenv("AUTOOPS_DB_NAME")
		if port == "" {
			port = "5432"
		}
		if name == "" {
			name = "autoops"
		}
		Cfg.Database.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, pass, name)
	}
	if v := os.Getenv("AUTOOPS_JWT_SECRET"); v != "" {
		Cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("AUTOOPS_AES_KEY"); v != "" {
		Cfg.Auth.AESKey = v
	}
	if Cfg.Server.Port == 0 {
		Cfg.Server.Port = 8080
	}
	if Cfg.Storage.UploadDir == "" {
		Cfg.Storage.UploadDir = "./uploads"
	}
	return nil
}

// AESKeyBytes 返回解码后的 32 字节主密钥
func AESKeyBytes() ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(Cfg.Auth.AESKey)
	if err != nil {
		return nil, fmt.Errorf("aes_key 必须是 32 字节的 base64 编码: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("aes_key 解码后长度必须为 32 字节, 当前 %d", len(raw))
	}
	return raw, nil
}
