// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"

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
	if err := service.SendViaChannel(&ch, "🟢 AutoOps 测试消息", "这是一条来自 AutoOps 监控中心的测试通知。"); err != nil {
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
		Mode           string `json:"mode"`
		GraceSec       int    `json:"grace_sec"`
		NotifyRecovery bool   `json:"notify_recovery"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rule := service.AlertRule{Mode: req.Mode, GraceSec: req.GraceSec, NotifyRecovery: req.NotifyRecovery}
	if err := service.SaveAlertRule(rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, service.LoadAlertRule())
}
