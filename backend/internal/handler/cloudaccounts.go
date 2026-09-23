// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Cloud accounts (CSP asset sync): CRUD for AWS/Azure/Huawei credential sets,
// credential test, manual sync, scheduled sync, conflict inspection and merge.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

func cloudAccountOut(ca model.CloudAccount) gin.H {
	return gin.H{
		"id": ca.ID, "provider": ca.Provider, "name": ca.Name,
		"regions": ca.Regions, "target_group_id": ca.TargetGroupID,
		"template_id": ca.TemplateID,
		"import_stopped": ca.ImportStopped, "tag_group_key": ca.TagGroupKey,
		"sync_interval_min": ca.SyncIntervalMin, "auto_delete": ca.AutoDelete,
		"last_sync_at": ca.LastSyncAt, "last_sync_result": ca.LastSyncResult,
		"creator": ca.Creator, "created_at": ca.CreatedAt,
		"has_credentials": ca.Credentials != "",
	}
}

// ListCloudAccounts GET /api/cloudaccounts
func ListCloudAccounts(c *gin.Context) {
	var accounts []model.CloudAccount
	model.DB.Order("id ASC").Find(&accounts)
	out := make([]gin.H, 0, len(accounts))
	for _, ca := range accounts {
		out = append(out, cloudAccountOut(ca))
	}
	c.JSON(http.StatusOK, out)
}

type cloudAccountReq struct {
	Provider        string         `json:"provider"`
	Name            string         `json:"name"`
	Credentials     map[string]any `json:"credentials"`
	Regions         string         `json:"regions"`
	TargetGroupID   *uint          `json:"target_group_id"`
	TemplateID      *uint          `json:"template_id"`
	ImportStopped   bool           `json:"import_stopped"`
	TagGroupKey     string         `json:"tag_group_key"`
	SyncIntervalMin int            `json:"sync_interval_min"`
	AutoDelete      bool           `json:"auto_delete"`
}

func (r *cloudAccountReq) valid(c *gin.Context) bool {
	switch strings.ToLower(r.Provider) {
	case "aws", "azure", "huawei":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 须为 aws / azure / huawei"})
		return false
	}
	if r.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称必填"})
		return false
	}
	return true
}

func encryptCredJSON(req *cloudAccountReq) (string, error) {
	keepMask := func(v any) string {
		s, _ := v.(string)
		return s
	}
	_ = keepMask
	b, err := json.Marshal(req.Credentials)
	if err != nil {
		return "", err
	}
	return pkg.Encrypt(string(b))
}

// CreateCloudAccount POST /api/cloudaccounts — admin only
func CreateCloudAccount(c *gin.Context) {
	var req cloudAccountReq
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid(c) {
		return
	}
	enc, err := encryptCredJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "凭据格式错误"})
		return
	}
	ca := model.CloudAccount{
		Provider: strings.ToLower(req.Provider), Name: req.Name,
		Credentials: enc, Regions: req.Regions,
		TargetGroupID: req.TargetGroupID, TemplateID: req.TemplateID, ImportStopped: req.ImportStopped,
		TagGroupKey: req.TagGroupKey, SyncIntervalMin: req.SyncIntervalMin,
		AutoDelete: req.AutoDelete, Creator: currentUser(c).Username,
	}
	if err := model.DB.Create(&ca).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败"})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "CLOUD_ACCOUNT", Resource: "CREATE " + ca.Provider + "/" + ca.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, cloudAccountOut(ca))
}

// UpdateCloudAccount PUT /api/cloudaccounts/:id — admin only; empty credentials keep stored
func UpdateCloudAccount(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ca model.CloudAccount
	if err := model.DB.First(&ca, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "云账号不存在"})
		return
	}
	var req cloudAccountReq
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid(c) {
		return
	}
	updates := map[string]any{
		"provider": strings.ToLower(req.Provider), "name": req.Name,
		"regions": req.Regions, "target_group_id": req.TargetGroupID,
		"import_stopped": req.ImportStopped, "template_id": req.TemplateID, "tag_group_key": req.TagGroupKey,
		"sync_interval_min": req.SyncIntervalMin, "auto_delete": req.AutoDelete,
	}
	if len(req.Credentials) > 0 {
		enc, err := encryptCredJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "凭据格式错误"})
			return
		}
		updates["credentials"] = enc
		ca.LastSyncAt = nil // new credentials: force a fresh sync
		updates["last_sync_at"] = nil
	}
	model.DB.Model(&ca).Updates(updates)
	c.JSON(http.StatusOK, cloudAccountOut(ca))
}

// DeleteCloudAccount DELETE /api/cloudaccounts/:id — admin only; imported hosts stay
func DeleteCloudAccount(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Delete(&model.CloudAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestCloudAccount POST /api/cloudaccounts/:id/test — admin only; lists a preview
func TestCloudAccount(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ca model.CloudAccount
	if err := model.DB.First(&ca, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "云账号不存在"})
		return
	}
	instances, err := service.ListCloudInstances(ca.Provider, ca.Credentials, ca.Regions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preview := make([]gin.H, 0, len(instances))
	for _, ci := range instances {
		preview = append(preview, gin.H{
			"name": ci.Name, "ip": ci.PrivateIP, "os": ci.OSType,
			"state": ci.State, "region": ci.Region, "type": ci.InstanceType,
		})
	}
	c.JSON(http.StatusOK, gin.H{"total": len(instances), "instances": preview})
}

// SyncCloudAccountByID POST /api/cloudaccounts/:id/sync — pulls inventory and
// upserts hosts; requires hosts:create capability (aligned with host creation)
func SyncCloudAccountByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ca model.CloudAccount
	if err := model.DB.First(&ca, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "云账号不存在"})
		return
	}
	u := currentUser(c)
	sum, err := service.SyncCloudAccount(&ca, u.Username)
	result := "成功"
	if err != nil {
		result = "失败: " + err.Error()
	} else {
		result = "新增 " + strconv.Itoa(sum.Added) + " / 更新 " + strconv.Itoa(sum.Updated) +
			" / 跳过 " + strconv.Itoa(sum.SkippedIP) + " / 删除 " + strconv.Itoa(sum.Removed)
	}
	model.DB.Model(&ca).Updates(map[string]any{
		"last_sync_at": time.Now(), "last_sync_result": result,
	})
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "CLOUD_SYNC", Resource: ca.Provider + "/" + ca.Name,
		Detail: `{"result":` + jsonStringOf(result) + `}`,
		IP:     c.ClientIP(), Status: map[bool]int{true: 200, false: 500}[err == nil], CreatedAt: time.Now(),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sum)
}

// CloudSyncConflicts GET /api/cloudaccounts/:id/conflicts — live same-IP conflicts
func CloudSyncConflicts(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ca model.CloudAccount
	if err := model.DB.First(&ca, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "云账号不存在"})
		return
	}
	conflicts, err := service.CloudSyncConflicts(&ca)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts})
}

// MergeCloudConflict POST /api/cloudaccounts/:id/merge {instance_id, host_id} —
// links an existing manual host to the cloud instance (no duplicate created)
func MergeCloudConflict(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ca model.CloudAccount
	if err := model.DB.First(&ca, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "云账号不存在"})
		return
	}
	var req struct {
		InstanceID string `json:"instance_id" binding:"required"`
		HostID     uint   `json:"host_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	u := currentUser(c)
	if err := service.MergeCloudConflict(&ca, req.InstanceID, req.HostID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "CLOUD_MERGE", Resource: ca.Provider + "/" + ca.Name,
		Detail: `{"instance":` + jsonStringOf(req.InstanceID) + `,"host_id":` + strconv.Itoa(int(req.HostID)) + `}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
