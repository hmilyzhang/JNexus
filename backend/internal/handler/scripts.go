// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
)

func ListScripts(c *gin.Context) {
	var scripts []model.Script
	model.DB.Order("id DESC").Find(&scripts)
	c.JSON(http.StatusOK, scripts)
}

func CreateScript(c *gin.Context) {
	var s model.Script
	if err := c.ShouldBindJSON(&s); err != nil || s.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "脚本名称必填"})
		return
	}
	s.Creator = currentUser(c).Username
	if err := model.DB.Create(&s).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func UpdateScript(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var s model.Script
	if err := model.DB.First(&s, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "脚本不存在"})
		return
	}
	var req model.Script
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	model.DB.Model(&s).Updates(map[string]any{
		"name": req.Name, "description": req.Description, "content": req.Content,
		"content_ps": req.ContentPS, // empty clears the Windows variant
	})
	s.ContentPS = req.ContentPS
	c.JSON(http.StatusOK, s)
}

func DeleteScript(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Delete(&model.Script{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
