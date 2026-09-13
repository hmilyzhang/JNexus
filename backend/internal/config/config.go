// JNexus Ops Platform — By JJ Zhang, Version 1.0

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
		// 32-byte base64-encoded master key used to encrypt SSH private keys stored in the DB
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
		// Container deployments may omit the config file: a DB connection via env vars is enough to start
		if os.IsNotExist(err) && (os.Getenv("JNEXUS_DSN") != "" || os.Getenv("JNEXUS_DB_HOST") != "") {
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
	// Environment variable overrides
	if v := os.Getenv("JNEXUS_DSN"); v != "" {
		Cfg.Database.DSN = v
	}
	// External database individual settings: JNEXUS_DB_HOST/PORT/USER/PASSWORD/NAME
	// If JNEXUS_DB_HOST is set and no explicit DSN given, assemble the DSN automatically
	if Cfg.Database.DSN == "" && os.Getenv("JNEXUS_DB_HOST") != "" {
		host := os.Getenv("JNEXUS_DB_HOST")
		port := os.Getenv("JNEXUS_DB_PORT")
		user := os.Getenv("JNEXUS_DB_USER")
		pass := os.Getenv("JNEXUS_DB_PASSWORD")
		name := os.Getenv("JNEXUS_DB_NAME")
		if port == "" {
			port = "5432"
		}
		if name == "" {
			name = "jnexus"
		}
		Cfg.Database.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, pass, name)
	}
	if v := os.Getenv("JNEXUS_JWT_SECRET"); v != "" {
		Cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("JNEXUS_AES_KEY"); v != "" {
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

// AESKeyBytes returns the decoded 32-byte master key
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
