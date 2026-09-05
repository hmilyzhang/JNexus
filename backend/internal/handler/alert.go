// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/service"
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

// ListAlertChannels 通道列表
func ListAlertChannels(c *gin.Context) {
	var channels []model.AlertChannel
	model.DB.Order("id").Find(&channels)
	c.JSON(http.StatusOK, channels)
}

// CreateAlertChannel 新建通道
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

// UpdateAlertChannel 编辑通道
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

// DeleteAlertChannel 删除通道（解绑所有监控项）
func DeleteAlertChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Where("channel_id = ?", id).Delete(&model.MonitorChannel{})
	model.DB.Delete(&model.AlertChannel{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestAlertChannel 发送测试消息
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
	if err := service.SendViaChannel(&ch, sampleVars, "🟢 AutoOps 测试消息", "这是一条来自 AutoOps 监控中心的测试通知。"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetAlertRule 全局报警规则
func GetAlertRule(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadAlertRule())
}

// UpdateAlertRule 保存全局报警规则（对所有监控项生效）
func UpdateAlertRule(c *gin.Context) {
	var req struct {
		GraceSec       int  `json:"grace_sec"`
		NotifyRecovery bool `json:"notify_recovery"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rule := service.AlertRule{GraceSec: req.GraceSec, NotifyRecovery: req.NotifyRecovery}
	if err := service.SaveAlertRule(rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, service.LoadAlertRule())
}

// GetAlertTemplates 全局默认通知模板
func GetAlertTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadAlertTemplates())
}

// UpdateAlertTemplates 保存全局默认通知模板（字段留空 = 恢复内建默认）
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

// PreviewAlertTemplates 模拟发送：用示例数据渲染模板，返回标题与正文（不实际发送）
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

// GetCmdLevels CMD 分级阈值配置
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

// UpdateCmdLevels 保存 CMD 分级阈值配置（4 级整体保存）
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
