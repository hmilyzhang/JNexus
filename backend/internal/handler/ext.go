// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/middleware"
	"autoops/internal/model"
	"autoops/internal/service"
)

// 外部集成 API（/api/ext/*）：仅接受 API 密钥，以密钥属主用户身份执行，
// 角色权限与数据级授权全部沿用现有体系。

func extOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

// ExtHosts GET /api/ext/hosts —— 主机列表 + 在线状态 + 最新资源
func ExtHosts(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var hosts []model.Host
	model.DB.Order("name").Find(&hosts)
	metrics := map[uint]model.HostMetric{}
	var latest []model.HostMetric
	model.DB.Raw(`SELECT DISTINCT ON (host_id) * FROM host_metrics ORDER BY host_id, collected_at DESC`).Scan(&latest)
	for _, m := range latest {
		metrics[m.HostID] = m
	}
	out := []gin.H{}
	for _, h := range hosts {
		if !user.IsAdmin() && !service.CanExecHost(user, h.ID, h.GroupID) && !userCanViewHost(user, &h) {
			continue
		}
		item := gin.H{"id": h.ID, "name": h.Name, "ip": h.IP, "port": h.Port, "status": h.Status}
		if h.GroupID != nil {
			var g model.HostGroup
			if model.DB.First(&g, *h.GroupID).Error == nil {
				item["group"] = g.Name
			}
		}
		if m, ok := metrics[h.ID]; ok {
			item["cpu"] = m.CPUPercent
			item["mem"] = m.MemPercent
			item["disk"] = m.DiskPercent
			item["metrics_at"] = m.CollectedAt
		}
		out = append(out, item)
	}
	extOK(c, out)
}

func userCanViewHost(user *model.User, h *model.Host) bool {
	return user.Role == model.RoleViewer || user.Role == model.RoleAuditor
}

// ExtMonitors GET /api/ext/monitors —— 应用监控项 + 最新状态 + 24h 可用率
func ExtMonitors(c *gin.Context) {
	var monitors []model.Monitor
	model.DB.Order("id").Find(&monitors)
	out := []gin.H{}
	now := time.Now()
	for _, m := range monitors {
		item := gin.H{
			"id": m.ID, "name": m.Name, "type": m.Type, "target": m.Target,
			"enabled": m.Enabled, "status": m.LastStatus, "resp_ms": m.LastRespMs,
			"last_checked_at": m.LastCheckedAt,
		}
		var up float64
		model.DB.Raw(`SELECT COALESCE(AVG(CASE WHEN status = 'up' THEN 100.0 ELSE 0 END), -1)
			FROM monitor_samples WHERE monitor_id = ? AND created_at > ?`, m.ID, now.Add(-24*time.Hour)).Scan(&up)
		if up >= 0 {
			item["uptime_24h"] = up
		}
		out = append(out, item)
	}
	extOK(c, out)
}

// ExtTask GET /api/ext/tasks/:id —— 任务结果（权限：admin/auditor 全部；他人仅本人任务）
func ExtTask(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var task model.Task
	if err := model.DB.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "任务不存在"})
		return
	}
	if !user.IsAdmin() && user.Role != model.RoleAuditor && task.Operator != user.Username {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "无权查看该任务"})
		return
	}
	var results []model.TaskHostResult
	model.DB.Where("task_id = ?", id).Order("id").Find(&results)
	items := []gin.H{}
	for _, r := range results {
		items = append(items, gin.H{
			"host": r.HostName, "ip": r.HostIP, "account": r.OsUser,
			"status": r.Status, "exit_code": r.ExitCode, "output": r.Output,
		})
	}
	extOK(c, gin.H{
		"id": task.ID, "type": task.Type, "operator": task.Operator,
		"status": task.Status, "params": task.Params,
		"created_at": task.CreatedAt, "finished_at": task.FinishedAt, "results": items,
	})
}

// ExtExec POST /api/ext/exec —— 触发批量命令执行（异步），返回任务 ID
// body: {"host_ids":[1,2], "command":"uptime", "timeout_sec":60, "wait":false}
func ExtExec(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user.Role != model.RoleAdmin && user.Role != model.RoleOps {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "密钥属主角色无执行权限（需 admin/ops）"})
		return
	}
	var req struct {
		HostIDs    []uint `json:"host_ids" binding:"required"`
		Command    string `json:"command"`
		ScriptID   *uint  `json:"script_id"`
		TimeoutSec int    `json:"timeout_sec"`
		Credential *uint  `json:"credential_id"`
		Wait       bool   `json:"wait"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Command == "" && req.ScriptID == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "需要 host_ids 与 command 或 script_id"})
		return
	}
	timeout := req.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	taskID, _, err := service.StartBatchExec(user, service.ExecRequest{
		HostIDs: req.HostIDs, Command: req.Command, ScriptID: req.ScriptID,
		TimeoutSec: timeout, CredentialID: req.Credential, Concurrency: 10,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	if !req.Wait {
		extOK(c, gin.H{"task_id": taskID})
		return
	}
	// wait=true：最长等 120 秒直到任务结束
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		var t model.Task
		if model.DB.First(&t, taskID).Error == nil && t.Status != "running" {
			break
		}
	}
	var t model.Task
	model.DB.First(&t, taskID)
	var results []model.TaskHostResult
	model.DB.Where("task_id = ?", taskID).Order("id").Find(&results)
	items := []gin.H{}
	for _, r := range results {
		items = append(items, gin.H{
			"host": r.HostName, "ip": r.HostIP, "status": r.Status,
			"exit_code": r.ExitCode, "output": r.Output,
		})
	}
	extOK(c, gin.H{"task_id": taskID, "status": t.Status, "results": items})
}

// ---------- 密钥管理（系统设置，仅 admin） ----------

type apiKeyReq struct {
	Name        string `json:"name" binding:"required"`
	OwnerUserID uint   `json:"owner_user_id" binding:"required"`
	ExpiresAt   string `json:"expires_at"` // RFC3339 或空
	IPAllowlist string `json:"ip_allowlist"`
}

// ListApiKeys 密钥列表（不返回哈希）
func ListApiKeys(c *gin.Context) {
	var keys []model.ApiKey
	model.DB.Order("id").Find(&keys)
	out := []gin.H{}
	for _, k := range keys {
		out = append(out, gin.H{
			"id": k.ID, "name": k.Name, "key_id": k.KeyID, "owner": k.OwnerName,
			"expires_at": k.ExpiresAt, "ip_allowlist": k.IPAllowlist,
			"enabled": k.Enabled, "last_used_at": k.LastUsedAt, "last_used_ip": k.LastUsedIP,
			"created_at": k.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// CreateApiKey 创建密钥：完整密钥仅此一次返回
func CreateApiKey(c *gin.Context) {
	var req apiKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称与属主用户必填"})
		return
	}
	var owner model.User
	if err := model.DB.First(&owner, req.OwnerUserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "属主用户不存在"})
		return
	}
	full, keyID, hash, err := service.GenerateApiKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成失败"})
		return
	}
	key := model.ApiKey{
		Name: req.Name, KeyID: keyID, KeyHash: hash,
		OwnerUserID: owner.ID, OwnerName: owner.Username,
		IPAllowlist: req.IPAllowlist, Enabled: true, CreatedBy: currentUser(c).Username,
	}
	if req.ExpiresAt != "" {
		if t, e := time.Parse(time.RFC3339, req.ExpiresAt); e == nil {
			key.ExpiresAt = &t
		}
	}
	if err := model.DB.Create(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": key.ID, "key_id": keyID, "key": full})
}

// UpdateApiKey 启用/停用
func UpdateApiKey(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var key model.ApiKey
	if err := model.DB.First(&key, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "密钥不存在"})
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	model.DB.Model(&key).Update("enabled", *req.Enabled)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteApiKey 删除密钥
func DeleteApiKey(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Delete(&model.ApiKey{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

var _ = middleware.ApiKeyFromContext
