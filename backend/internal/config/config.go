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
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	if err := yaml.Unmarshal(data, &Cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	// 环境变量覆盖
	if v := os.Getenv("AUTOOPS_DSN"); v != "" {
		Cfg.Database.DSN = v
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
