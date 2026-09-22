// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

type alertChannelReq struct {
	Name    string `json:"name" binding:"required"`
	Type    string `json:"type" binding:"required"`
	Config  string `json:"config"`
	Enabled *bool  `json:"enabled"`
}

var alertChannelTypes = map[string]bool{
	"email": true, "webhook": true, "wecom": true, "dingtalk": true, "feishu": true, "telegram": true,
}

// ListAlertChannels lists channels
func ListAlertChannels(c *gin.Context) {
	var channels []model.AlertChannel
	model.DB.Order("id").Find(&channels)
	c.JSON(http.StatusOK, channels)
}

// CreateAlertChannel creates a channel
func CreateAlertChannel(c *gin.Context) {
	var req alertChannelReq
	if err := c.ShouldBindJSON(&req); err != nil || !alertChannelTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（名称/类型必填，类型须为 email/webhook/wecom/dingtalk/feishu/telegram）"})
		return
	}
	ch := model.AlertChannel{Name: req.Name, Type: req.Type, Config: req.Config, Enabled: true}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if err := model.DB.Create(&ch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, ch)
}

// UpdateAlertChannel updates a channel
func UpdateAlertChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch model.AlertChannel
	if err := model.DB.First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通道不存在"})
		return
	}
	var req alertChannelReq
	if err := c.ShouldBindJSON(&req); err != nil || !alertChannelTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	ch.Name, ch.Type, ch.Config = req.Name, req.Type, req.Config
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	model.DB.Model(&ch).Updates(map[string]any{
		"name": ch.Name, "type": ch.Type, "config": ch.Config, "enabled": ch.Enabled,
	})
	c.JSON(http.StatusOK, ch)
}

// DeleteAlertChannel deletes a channel (unbinds all monitors)
func DeleteAlertChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Where("channel_id = ?", id).Delete(&model.MonitorChannel{})
	model.DB.Delete(&model.AlertChannel{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestAlertChannel sends a test message
func TestAlertChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch model.AlertChannel
	if err := model.DB.First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通道不存在"})
		return
	}
	sampleVars := map[string]string{
		"level": "P2", "host": "demo-01", "ip": "10.0.0.8",
		"metric": "CPU", "value": "91.5", "threshold": "90",
		"monitor": "demo-monitor", "type": "http", "target": "http://10.0.0.8/health",
		"status": "DOWN", "resp_ms": "233", "error": "-",
		"event": "system rebooted (boot_id changed)", "time": time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := service.SendViaChannel(&ch, sampleVars, "🟢 JNexus 测试消息", "这是一条来自 JNexus 监控中心的测试通知。"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetAlertRule returns the global alert rule
func GetAlertRule(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadAlertRule())
}

// UpdateAlertRule saves the global alert rule (applies to all monitors)
func UpdateAlertRule(c *gin.Context) {
	var req struct {
		GraceSec       int  `json:"grace_sec"`
		NotifyRecovery bool `json:"notify_recovery"`
		CertWarnDays   int  `json:"cert_warn_days"`
		CertCritDays   int  `json:"cert_crit_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rule := service.AlertRule{GraceSec: req.GraceSec, NotifyRecovery: req.NotifyRecovery,
		CertWarnDays: req.CertWarnDays, CertCritDays: req.CertCritDays}
	if err := service.SaveAlertRule(rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, service.LoadAlertRule())
}

// GetAlertTemplates returns the global default notification templates
func GetAlertTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadAlertTemplates())
}

// UpdateAlertTemplates saves the global default notification templates (empty fields = restore built-in defaults)
func UpdateAlertTemplates(c *gin.Context) {
	var req service.AlertTemplates
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := service.SaveAlertTemplates(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, service.LoadAlertTemplates())
}

// PreviewAlertTemplates dry-run: renders templates with sample data and returns title and body (no actual send)
func PreviewAlertTemplates(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	sampleVars := map[string]string{
		"level": "P2", "host": "demo-01", "ip": "10.0.0.8",
		"metric": "CPU", "value": "91.5", "threshold": "90",
		"monitor": "demo-monitor", "type": "http", "target": "http://10.0.0.8/health",
		"status": "DOWN", "resp_ms": "233", "error": "-",
		"event": "system rebooted (boot_id changed)", "time": time.Now().Format("2006-01-02 15:04:05"),
	}
	c.JSON(http.StatusOK, gin.H{
		"title": service.RenderTemplate(req.Title, sampleVars),
		"body":  service.RenderTemplate(req.Body, sampleVars),
	})
}

// GetMaintenances lists global maintenance windows
func GetMaintenances(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadMaintenances())
}

// UpdateMaintenances saves global maintenance windows (applies to all monitors)
func UpdateMaintenances(c *gin.Context) {
	var req []service.MaintenanceWindow
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if _, err := service.ValidateMaintenances(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Submissions identical to the current config are treated as no-ops: not persisted, not logged
	cur := service.LoadMaintenances()
	curJSON, _ := json.Marshal(cur)
	newJSON, _ := json.Marshal(req)
	if strings.TrimSpace(string(curJSON)) == strings.TrimSpace(string(newJSON)) {
		c.JSON(http.StatusOK, gin.H{"unchanged": true})
		return
	}
	if err := service.SaveMaintenances(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	// Audit trail: mark old records inactive and write this change (operator/IP/windows/active flag)
	now := time.Now()
	model.DB.Model(&model.MaintenanceLog{}).Where("active = ?", true).Update("active", false)
	windowsJSON, _ := json.Marshal(req)
	model.DB.Create(&model.MaintenanceLog{
		Username: currentUser(c).Username, IP: c.ClientIP(),
		Windows: string(windowsJSON), Active: true, CreatedAt: now,
	})
	c.JSON(http.StatusOK, service.LoadMaintenances())
}

// GetMaintenanceLogs maintenance window change history (last 50 entries, with active status)
func GetMaintenanceLogs(c *gin.Context) {
	var logs []model.MaintenanceLog
	model.DB.Order("id DESC").Limit(50).Find(&logs)
	out := []gin.H{}
	for _, l := range logs {
		var wins []service.MaintenanceWindow
		_ = json.Unmarshal([]byte(l.Windows), &wins)
		out = append(out, gin.H{
			"username": l.Username, "ip": l.IP, "windows": wins,
			"active": l.Active, "created_at": l.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// MaintenanceStatus reports whether "now" falls inside any configured window,
// so the UI can distinguish "this snapshot is the current config" from
// "a maintenance window is actually in effect right now"
func MaintenanceStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"in_window": service.InMaintenanceWindow(time.Now()),
		"windows":   service.LoadMaintenances(),
	})
}

// DeleteMaintenanceLog removes one change-audit entry (admin); the current windows are untouched
func DeleteMaintenanceLog(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DB.Delete(&model.MaintenanceLog{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ClearMaintenanceLogs removes all change-audit entries (admin); the current windows are untouched
func ClearMaintenanceLogs(c *gin.Context) {
	if err := model.DB.Where("1 = 1").Delete(&model.MaintenanceLog{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清空失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetCmdLevels returns the CMD tiered threshold config
func GetCmdLevels(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadCmdLevels())
}

type cmdLevelReq struct {
	Level       string  `json:"level"`
	CPU         float64 `json:"cpu"`
	Mem         float64 `json:"mem"`
	Disk        float64 `json:"disk"`
	DurationSec int     `json:"duration_sec"`
	ChannelIDs  []uint  `json:"channel_ids"`
}

// UpdateCmdLevels saves the CMD tiered threshold config (all levels saved as a whole)
func UpdateCmdLevels(c *gin.Context) {
	var req []cmdLevelReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req) == 0 || len(req) > 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（需要级别数组）"})
		return
	}
	levels := make([]service.CmdLevel, 0, len(req))
	seen := map[string]bool{}
	for _, r := range req {
		if seen[r.Level] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "级别重复: " + r.Level})
			return
		}
		seen[r.Level] = true
		if r.CPU < 0 || r.CPU > 100 || r.Mem < 0 || r.Mem > 100 || r.Disk < 0 || r.Disk > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "阈值需在 0-100 之间"})
			return
		}
		if r.DurationSec < 0 || r.DurationSec > 86400 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "持续时长需在 0-86400 秒之间"})
			return
		}
		levels = append(levels, service.CmdLevel{
			Level: r.Level, CPU: r.CPU, Mem: r.Mem, Disk: r.Disk,
			DurationSec: r.DurationSec, ChannelIDs: r.ChannelIDs,
		})
	}
	if err := service.SaveCmdLevels(levels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, service.LoadCmdLevels())
}
