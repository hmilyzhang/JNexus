// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"sync"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// Force-pairing for existing stored-password credentials: the vaulted password
// logs in, the platform public key is installed and a paired key credential is
// registered — no manual password re-entry needed. Windows hosts are out of scope
// (WinRM has no SSH keys); rotation of the paired account keeps working because
// PairAndCreateCredential stores the password on the key credential.

// PairExisting pairs one stored-password credential. Returns
// ("paired" | "already", nil) or ("", err) with a user-facing reason.
func PairExisting(credID uint) (string, error) {
	var cred model.HostCredential
	if err := model.DB.First(&cred, credID).Error; err != nil {
		return "", fmt.Errorf("账号不存在")
	}
	var h model.Host
	if err := model.DB.First(&h, cred.HostID).Error; err != nil {
		return "", fmt.Errorf("主机不存在")
	}
	if h.OSType == "windows" {
		return "", fmt.Errorf("Windows 主机走 WinRM，不适用 SSH 密钥配对")
	}
	if cred.AuthType != "password" || cred.Password == "" {
		return "", fmt.Errorf("该账号没有存储密码，无法自动配对")
	}
	platformKey, err := EnsurePlatformKey()
	if err != nil {
		return "", err
	}
	var cnt int64
	model.DB.Model(&model.HostCredential{}).
		Where("host_id = ? AND username = ? AND auth_type = 'key' AND ssh_key_id = ?",
			h.ID, cred.Username, platformKey.ID).
		Count(&cnt)
	if cnt > 0 {
		return "already", nil
	}
	plain, err := pkg.Decrypt(cred.Password)
	if err != nil {
		return "", fmt.Errorf("解密密码失败: %w", err)
	}
	if _, _, err := PairAndCreateCredential(&h, cred.Username, plain, cred.Label, true); err != nil {
		return "", err
	}
	return "paired", nil
}

// PairResult is one credential's force-pair outcome in PairAllPasswords.
type PairResult struct {
	CredentialID uint   `json:"credential_id"`
	Host         string `json:"host"`
	Username     string `json:"username"`
	Status       string `json:"status"` // paired / already / failed
	Detail       string `json:"detail"`
}

// PairAllPasswords force-pairs every eligible stored-password credential on Linux
// hosts (8 workers). Already-paired accounts are reported as "already".
func PairAllPasswords() []PairResult {
	type row struct {
		CredID   uint
		Host     string
		Username string
	}
	var rows []row
	model.DB.Table("host_credentials cr").
		Select("cr.id AS cred_id, COALESCE(h.name, '') AS host, cr.username AS username").
		Joins("JOIN hosts h ON h.id = cr.host_id").
		// is_ldap accounts are included: pairing only logs in with the stored password
		// and installs a key - it never changes any password (unlike rotation).
		Where("cr.auth_type = 'password' AND cr.password <> '' AND h.os_type = 'linux'").
		Order("cr.id").Scan(&rows)

	results := make([]PairResult, len(rows))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, r := range rows {
		wg.Add(1)
		go func(i int, r row) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res := PairResult{CredentialID: r.CredID, Host: r.Host, Username: r.Username}
			status, err := PairExisting(r.CredID)
			switch {
			case err != nil:
				res.Status = "failed"
				res.Detail = err.Error()
			case status == "already":
				res.Status = "already"
				res.Detail = "已配对"
			default:
				res.Status = "paired"
				res.Detail = "配对成功"
			}
			results[i] = res
		}(i, r)
	}
	wg.Wait()
	return results
}
