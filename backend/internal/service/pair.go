// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"fmt"

	gossh "golang.org/x/crypto/ssh"

	"autoops/internal/model"
)

const platformKeyName = "autoops-platform"

// EnsurePlatformKey 平台配对密钥（全局唯一、惰性创建）。
// 所有自动配对共用此密钥：系统配置可查看其公钥，管理员亦可手动预装到目标机。
func EnsurePlatformKey() (*model.SSHKey, error) {
	var k model.SSHKey
	if err := model.DB.Where("name = ?", platformKeyName).First(&k).Error; err == nil {
		return &k, nil
	}
	if _, err := GenerateAndStoreKeyPair(platformKeyName, "autoops-platform"); err != nil {
		return nil, err
	}
	// 并发下重名则取已有记录
	var k2 model.SSHKey
	if err := model.DB.Where("name = ?", platformKeyName).First(&k2).Error; err != nil {
		return nil, err
	}
	return &k2, nil
}

// PairAndCreateCredential 用密码登录主机推送平台公钥，成功后登记密钥凭据。
// 配对成功返回 (true, cred, nil)；失败返回 (false, nil, err)，不产生任何凭据记录。
func PairAndCreateCredential(host *model.Host, username, password, label string, makeDefault bool) (bool, *model.HostCredential, error) {
	platformKey, err := EnsurePlatformKey()
	if err != nil {
		return false, nil, fmt.Errorf("获取平台密钥失败: %w", err)
	}
	cli, err := dialWithPassword(*host, username, password)
	if err != nil {
		return false, nil, fmt.Errorf("密码登录失败: %w", err)
	}
	code, ierr := installPubKey(cli, KeyPairPublicLine(platformKey))
	cli.Close()
	if ierr != nil || code != 0 {
		return false, nil, fmt.Errorf("推送公钥失败(exit=%d): %v", code, ierr)
	}

	var cnt int64
	model.DB.Model(&model.HostCredential{}).Where("host_id = ?", host.ID).Count(&cnt)
	cred := model.HostCredential{
		HostID: host.ID, Username: username, AuthType: "key",
		SSHKeyID: &platformKey.ID, Label: label,
		IsDefault: cnt == 0 || makeDefault,
	}
	if err := model.DB.Create(&cred).Error; err != nil {
		return false, nil, fmt.Errorf("创建凭据失败: %w", err)
	}
	return true, &cred, nil
}

var _ = gossh.InsecureIgnoreHostKey
