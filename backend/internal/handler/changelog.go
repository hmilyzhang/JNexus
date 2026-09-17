// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Changelog page: release notes with a short summary per update.
// Read for every logged-in user; entries are managed by admins.

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/buildinfo"
	"jnexus/internal/model"
)

type changelogReq struct {
	Version    string `json:"version"`
	Title      string `json:"title" binding:"required"`
	Details    string `json:"details"`
	ReleasedAt string `json:"released_at"` // YYYY-MM-DD, optional (defaults to today)
}

func (r *changelogReq) releasedAt() time.Time {
	if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(r.ReleasedAt), time.Local); err == nil {
		return t
	}
	return time.Now()
}

// ListChangelog GET /changelog — newest first
func ListChangelog(c *gin.Context) {
	var rows []model.ChangelogEntry
	model.DB.Order("released_at DESC, id DESC").Limit(200).Find(&rows)
	c.JSON(http.StatusOK, gin.H{"version": buildinfo.Get(), "items": rows})
}

// CreateChangelog POST /changelog (admin)
func CreateChangelog(c *gin.Context) {
	var req changelogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（标题必填）"})
		return
	}
	row := model.ChangelogEntry{
		Version:    strings.TrimSpace(req.Version),
		Title:      strings.TrimSpace(req.Title),
		Details:    strings.TrimSpace(req.Details),
		ReleasedAt: req.releasedAt(),
	}
	if err := model.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, row)
}

// UpdateChangelog PUT /changelog/:id (admin)
func UpdateChangelog(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var row model.ChangelogEntry
	if err := model.DB.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	var req changelogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（标题必填）"})
		return
	}
	row.Version = strings.TrimSpace(req.Version)
	row.Title = strings.TrimSpace(req.Title)
	row.Details = strings.TrimSpace(req.Details)
	row.ReleasedAt = req.releasedAt()
	if err := model.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, row)
}

// DeleteChangelog DELETE /changelog/:id (admin)
func DeleteChangelog(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DB.Delete(&model.ChangelogEntry{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
