// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"

	gossh "golang.org/x/crypto/ssh"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

const platformKeyName = "jnexus-platform"

// EnsurePlatformKey returns the platform pairing key (globally unique, lazily created).
// All auto-pairing shares this key: its public key is viewable in system config, and admins can pre-install it on target machines manually.
func EnsurePlatformKey() (*model.SSHKey, error) {
	var k model.SSHKey
	if err := model.DB.Where("name = ?", platformKeyName).First(&k).Error; err == nil {
		return &k, nil
	}
	if _, err := GenerateAndStoreKeyPair(platformKeyName, "jnexus-platform"); err != nil {
		return nil, err
	}
	// on concurrent creation of a duplicate name, reuse the existing record
	var k2 model.SSHKey
	if err := model.DB.Where("name = ?", platformKeyName).First(&k2).Error; err != nil {
		return nil, err
	}
	return &k2, nil
}

// PairAndCreateCredential logs in to the host with a password to install the platform public key, then registers the key credential.
// On success returns (true, cred, nil); on failure returns (false, nil, err) with no credential record created.
// The password is still stored (encrypted) on the key credential so the account stays password-rotatable:
// later rotations log in with the platform key and chpasswd, which needs no old password.
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
	encPwd, err := pkg.Encrypt(password)
	if err != nil {
		return false, nil, fmt.Errorf("加密密码失败: %w", err)
	}

	var cnt int64
	model.DB.Model(&model.HostCredential{}).Where("host_id = ?", host.ID).Count(&cnt)
	cred := model.HostCredential{
		HostID: host.ID, Username: username, AuthType: "key",
		SSHKeyID: &platformKey.ID, Label: label,
		IsDefault: cnt == 0 || makeDefault,
		Password:  encPwd,
	}
	if err := model.DB.Create(&cred).Error; err != nil {
		return false, nil, fmt.Errorf("创建凭据失败: %w", err)
	}
	RecordPasswordHistory(cred.ID, password, "created", "pairing")
	return true, &cred, nil
}

var _ = gossh.InsecureIgnoreHostKey
