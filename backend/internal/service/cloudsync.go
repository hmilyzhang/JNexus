// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Cloud asset sync: pull instance inventory from a CSP, upsert hosts idempotently
// (identity = provider + instance id), report same-IP conflicts, support tag-based
// host grouping, scheduled sync and removal of hosts whose instance disappeared.

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

type CloudSyncSummary struct {
	Added       int      `json:"added"`
	Updated     int      `json:"updated"`
	SkippedIP   int      `json:"skipped_ip"`
	Removed     int      `json:"removed"`
	Credentials int      `json:"credentials"` // template accounts created during this sync
	KeyPaired   int      `json:"key_paired"`  // imported hosts auto-paired with the platform key
	Conflicts   []string `json:"conflicts"`   // "name (ip) -> existing host '<name>' id=<id>"
	Total       int      `json:"total"`
}

// applyCredentialTemplate creates the default OS account for an imported host from
// a credential template (HostCredential row with host_id = 0). The stored password
// ciphertext is reused as-is (same master key). Idempotent: skipped when the host
// already has an account with the template's username. Returns the new credential
// id (0 = nothing created); the key-pairing pass runs separately
// (runTemplatePairing) so slow SSH attempts never stall the import loop.
func applyCredentialTemplate(templateID *uint, hostID uint) uint {
	if templateID == nil || *templateID == 0 {
		return 0
	}
	var tpl model.HostCredential
	if err := model.DB.First(&tpl, *templateID).Error; err != nil || tpl.HostID != 0 || tpl.Password == "" {
		return 0
	}
	var cnt int64
	model.DB.Model(&model.HostCredential{}).Where("host_id = ? AND username = ?", hostID, tpl.Username).Count(&cnt)
	if cnt > 0 {
		return 0
	}
	nc := model.HostCredential{
		HostID: hostID, Username: tpl.Username, AuthType: "password",
		Password: tpl.Password, Label: tpl.Label, IsDefault: true, IsLDAP: tpl.IsLDAP,
	}
	if err := model.DB.Create(&nc).Error; err != nil {
		return 0
	}
	return nc.ID
}

// runTemplatePairing upgrades freshly imported template accounts to platform-key
// auth: the stored password installs the public key and a paired key credential
// becomes the default. Unreachable hosts keep the password account as default -
// run the credentials-page force-pair sweep later to catch them.
func runTemplatePairing(credIDs []uint) int {
	if len(credIDs) == 0 {
		return 0
	}
	paired := 0
	var mu sync.Mutex
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	for _, id := range credIDs {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			status, err := PairExisting(id)
			if err == nil && status == "paired" {
				mu.Lock()
				paired++
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	return paired
}

// DecryptCloudCredentials decrypts the stored credential JSON (exported for handlers)
func DecryptCloudCredentials(enc string) (string, error) {
	return pkg.Decrypt(enc)
}

// resolveCloudGroup finds or creates a host group by name (for tag-based grouping)
func resolveCloudGroup(name string) *uint {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var g model.HostGroup
	if err := model.DB.Where("name = ?", name).First(&g).Error; err == nil {
		return &g.ID
	}
	g = model.HostGroup{Name: name}
	if err := model.DB.Create(&g).Error; err != nil {
		return nil
	}
	return &g.ID
}

// SyncCloudAccount pulls the instance inventory and upserts hosts idempotently.
// Identity: (cloud_provider, cloud_instance_id). Same-IP manual hosts are reported
// as conflicts (merge via MergeCloudConflict). With AutoDelete, hosts stamped with
// this account whose instance no longer exists are removed.
func SyncCloudAccount(ca *model.CloudAccount, operator string) (*CloudSyncSummary, error) {
	instances, err := ListCloudInstances(ca.Provider, ca.Credentials, ca.Regions)
	if err != nil {
		return nil, err
	}
	sum := &CloudSyncSummary{Total: len(instances), Conflicts: []string{}}
	seen := map[string]bool{}     // instance ids present in the cloud
	var pairCands []uint          // fresh template credentials to key-pair after the loop

	for _, ci := range instances {
		if ci.State != "running" && !ca.ImportStopped {
			continue
		}
		seen[ci.InstanceID] = true

		// same-IP conflict against manual hosts (no cloud stamp)
		var manual model.Host
		if ci.PrivateIP != "" {
			model.DB.Where("ip = ? AND (cloud_instance_id = '' OR cloud_instance_id IS NULL)", ci.PrivateIP).First(&manual)
		}
		if manual.ID != 0 {
			sum.SkippedIP++
			sum.Conflicts = append(sum.Conflicts,
				fmt.Sprintf("%s (%s) -> 已存在手工主机「%s」id=%d", ci.Name, ci.PrivateIP, manual.Name, manual.ID))
			continue
		}

		var host model.Host
		err := model.DB.Where("cloud_provider = ? AND cloud_instance_id = ?", ca.Provider, ci.InstanceID).First(&host).Error
		if err == nil {
			// update mutable fields
			updates := map[string]any{
				"name": ci.Name, "ip": ci.PrivateIP, "cloud_region": ci.Region,
				"os_type": ci.OSType,
			}
			if ci.State == "stopped" {
				updates["status"] = "offline"
			}
			model.DB.Model(&host).Updates(updates)
			// template account: also covers hosts imported before a template was configured
			if cid := applyCredentialTemplate(ca.TemplateID, host.ID); cid > 0 {
				sum.Credentials++
				pairCands = append(pairCands, cid)
			}
			sum.Updated++
			continue
		}

		status := "online"
		if ci.State != "running" {
			status = "offline"
		}
		port := 22
		if ci.OSType == "windows" {
			port = 3389
		}
		host = model.Host{
			Name: ci.Name, IP: ci.PrivateIP, Port: port, OSType: ci.OSType,
			AuthType: "password", Status: status,
			CloudProvider: ca.Provider, CloudInstanceID: ci.InstanceID, CloudRegion: ci.Region,
		}
		// group: tag key wins, else the account's fixed target group
		groupID := ca.TargetGroupID
		if ca.TagGroupKey != "" {
			if gname := ci.Tags[ca.TagGroupKey]; gname != "" {
				groupID = resolveCloudGroup(gname)
			}
		}
		if groupID != nil {
			host.GroupID = groupID
		}
		if err := model.DB.Create(&host).Error; err != nil {
			sum.Conflicts = append(sum.Conflicts, fmt.Sprintf("%s: 创建失败 %v", ci.Name, err))
			continue
		}
		sum.Added++
		if cid := applyCredentialTemplate(ca.TemplateID, host.ID); cid > 0 {
			sum.Credentials++
			pairCands = append(pairCands, cid)
		}
	}

	// key pairing for freshly imported template accounts (concurrent, after the import loop)
	sum.KeyPaired = runTemplatePairing(pairCands)

	// auto-delete: hosts stamped with this account whose instance is gone
	if ca.AutoDelete {
		var hosts []model.Host
		model.DB.Where("cloud_provider = ? AND cloud_instance_id <> ''", ca.Provider).Find(&hosts)
		for _, h := range hosts {
			if !seen[h.CloudInstanceID] {
				model.DB.Delete(&model.Host{}, h.ID)
				model.DB.Where("host_id = ?", h.ID).Delete(&model.HostCredential{})
				sum.Removed++
			}
		}
	}
	return sum, nil
}

// CloudSyncConflicts returns the current same-IP conflicts for an account by
// re-listing the inventory (live check)
func CloudSyncConflicts(ca *model.CloudAccount) ([]string, error) {
	instances, err := ListCloudInstances(ca.Provider, ca.Credentials, ca.Regions)
	if err != nil {
		return nil, err
	}
	conflicts := []string{}
	for _, ci := range instances {
		if ci.PrivateIP == "" {
			continue
		}
		var manual model.Host
		model.DB.Where("ip = ? AND (cloud_instance_id = '' OR cloud_instance_id IS NULL)", ci.PrivateIP).First(&manual)
		if manual.ID != 0 {
			conflicts = append(conflicts, fmt.Sprintf("%s (%s) -> 手工主机「%s」id=%d", ci.Name, ci.PrivateIP, manual.Name, manual.ID))
		}
	}
	return conflicts, nil
}

// MergeCloudConflict links an existing manual host to a cloud instance identity
// instead of creating a duplicate, refreshing its cloud attributes
func MergeCloudConflict(ca *model.CloudAccount, instanceID string, hostID uint) error {
	instances, err := ListCloudInstances(ca.Provider, ca.Credentials, ca.Regions)
	if err != nil {
		return err
	}
	for _, ci := range instances {
		if ci.InstanceID != instanceID {
			continue
		}
		var host model.Host
		if err := model.DB.First(&host, hostID).Error; err != nil {
			return fmt.Errorf("主机不存在")
		}
		return model.DB.Model(&host).Updates(map[string]any{
			"cloud_provider": ca.Provider, "cloud_instance_id": ci.InstanceID,
			"cloud_region": ci.Region, "name": ci.Name, "ip": ci.PrivateIP,
			"os_type": ci.OSType,
		}).Error
	}
	return fmt.Errorf("云实例不存在")
}

// RunDueCloudSyncs runs scheduled syncs whose interval has elapsed (monitor loop)
func RunDueCloudSyncs() {
	var accounts []model.CloudAccount
	model.DB.Where("sync_interval_min > 0").Find(&accounts)
	now := time.Now()
	for i := range accounts {
		ca := accounts[i]
		if ca.LastSyncAt != nil && now.Sub(*ca.LastSyncAt) < time.Duration(ca.SyncIntervalMin)*time.Minute {
			continue
		}
		acc := ca
		go func() {
			defer func() { recover() }()
			sum, err := SyncCloudAccount(&acc, "system")
			result := "成功"
			if err != nil {
				result = "失败: " + err.Error()
			} else {
				result = "新增 " + fmt.Sprintf("%d", sum.Added) + " / 更新 " + fmt.Sprintf("%d", sum.Updated) +
					" / 跳过 " + fmt.Sprintf("%d", sum.SkippedIP) + " / 删除 " + fmt.Sprintf("%d", sum.Removed)
			}
			model.DB.Model(&acc).Updates(map[string]any{
				"last_sync_at": time.Now(), "last_sync_result": result,
			})
		}()
	}
}
