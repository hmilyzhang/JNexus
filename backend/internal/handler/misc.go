// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

// ---- 审计日志 ----

func ListAudit(c *gin.Context) {
	var logs []model.AuditLog
	q := model.DB
	if username := c.Query("username"); username != "" {
		q = q.Where("username ILIKE ?", "%"+username+"%")
	}
	if action := c.Query("action"); action != "" {
		q = q.Where("action = ?", action)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("resource ILIKE ? OR detail ILIKE ?", like, like)
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	var total int64
	q.Model(&model.AuditLog{}).Count(&total)
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"total": total, "items": logs})
}

// ---- 危险命令规则 ----

func ListDangerRules(c *gin.Context) {
	var rules []model.DangerRule
	model.DB.Order("id").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

func CreateDangerRule(c *gin.Context) {
	var r model.DangerRule
	if err := c.ShouldBindJSON(&r); err != nil || r.Pattern == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "正则表达式必填"})
		return
	}
	if _, err := compileCheck(r.Pattern); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法正则: " + err.Error()})
		return
	}
	if err := model.DB.Create(&r).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	_ = service.LoadDangerRules()
	c.JSON(http.StatusOK, r)
}

func UpdateDangerRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var r model.DangerRule
	if err := model.DB.First(&r, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}
	var req model.DangerRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if _, err := compileCheck(req.Pattern); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法正则: " + err.Error()})
		return
	}
	model.DB.Model(&r).Updates(map[string]any{
		"pattern": req.Pattern, "desc": req.Desc, "enabled": req.Enabled,
	})
	_ = service.LoadDangerRules()
	c.JSON(http.StatusOK, r)
}

func DeleteDangerRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Delete(&model.DangerRule{}, id)
	_ = service.LoadDangerRules()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
