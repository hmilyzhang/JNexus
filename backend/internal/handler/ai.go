// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

const aiSystemPrompt = "你是 JNexus 运维平台的 AI 助手。回答简洁、专业、可执行；使用与用户提问相同的语言。"

// AIChat POST /api/ai/chat  {prompt} — AI 对话（所有登录用户可用，需管理员先启用并配置）
func AIChat(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt 必填"})
		return
	}
	s := service.LoadAISettings()
	if !s.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未启用（系统设置 → AI 助手）"})
		return
	}
	start := time.Now()
	reply, err := service.AIChat(s, aiSystemPrompt, strings.TrimSpace(req.Prompt))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reply": reply, "elapsed_ms": time.Since(start).Milliseconds()})
}

// AITest POST /api/system/ai/test — 管理员测试 AI 服务连通性
func AITest(c *gin.Context) {
	s := service.LoadAISettings()
	if s.BaseURL == "" || s.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写服务地址与模型"})
		return
	}
	start := time.Now()
	reply, err := service.AIChat(s, "测试连通性", "回复：OK")
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error(), "elapsed_ms": elapsed})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "reply": reply, "elapsed_ms": elapsed})
}
