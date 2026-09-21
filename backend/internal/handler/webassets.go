// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// Web assets: PAM-style web application assets (URL + vaulted credentials).
// Anyone signed in may use an asset (open); only admins manage them. Every
// open/reveal is written to the audit log.

func webAssetOut(w model.WebAsset) gin.H {
	return gin.H{
		"id": w.ID, "name": w.Name, "url": w.URL, "username": w.Username,
		"description": w.Description, "creator": w.Creator, "created_at": w.CreatedAt,
		"updated_at": w.UpdatedAt, "has_password": w.Password != "",
	}
}

// ListWebAssets GET /api/webassets — all signed-in users see the assets they may open
func ListWebAssets(c *gin.Context) {
	var assets []model.WebAsset
	model.DB.Order("id ASC").Find(&assets)
	out := make([]gin.H, 0, len(assets))
	for _, w := range assets {
		out = append(out, webAssetOut(w))
	}
	c.JSON(http.StatusOK, out)
}

type webAssetReq struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Description string `json:"description"`
}

func (r *webAssetReq) valid(c *gin.Context) bool {
	if r.Name == "" || r.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称和 URL 必填"})
		return false
	}
	return true
}

// CreateWebAsset POST /api/webassets — admin only
func CreateWebAsset(c *gin.Context) {
	var req webAssetReq
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid(c) {
		return
	}
	u := currentUser(c)
	w := model.WebAsset{
		Name: req.Name, URL: req.URL, Username: req.Username,
		Description: req.Description, Creator: u.Username,
	}
	if req.Password != "" {
		enc, err := pkg.Encrypt(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		w.Password = enc
	}
	if err := model.DB.Create(&w).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, webAssetOut(w))
}

// UpdateWebAsset PUT /api/webassets/:id — admin only; empty password keeps the stored one
func UpdateWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var stored model.WebAsset
	if err := model.DB.First(&stored, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	var req webAssetReq
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid(c) {
		return
	}
	updates := map[string]any{
		"name": req.Name, "url": req.URL, "username": req.Username,
		"description": req.Description,
	}
	if req.Password != "" {
		enc, err := pkg.Encrypt(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updates["password"] = enc
	}
	model.DB.Model(&stored).Updates(updates)
	stored.Name, stored.URL, stored.Username, stored.Description = req.Name, req.URL, req.Username, req.Description
	c.JSON(http.StatusOK, webAssetOut(stored))
}

// DeleteWebAsset DELETE /api/webassets/:id — admin only
func DeleteWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DB.Delete(&model.WebAsset{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// OpenWebAsset POST /api/webassets/:id/open — signed-in users; audited.
// Returns the target URL and account for the confirm card. The password is
// never returned here (use reveal, admin-only, if a manual paste is needed).
func OpenWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var w model.WebAsset
	if err := model.DB.First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	u := currentUser(c)
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "WEBASSET_OPEN", Resource: "/api/webassets/" + strconv.Itoa(id),
		Detail: `{"name":"` + w.Name + `","account":"` + w.Username + `"}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"url": w.URL, "username": w.Username})
}

// RevealWebAssetPassword POST /api/webassets/:id/reveal — admin only, audited
func RevealWebAssetPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var w model.WebAsset
	if err := model.DB.First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	if w.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该资产未保存密码"})
		return
	}
	plain, err := pkg.Decrypt(w.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解密失败: " + err.Error()})
		return
	}
	u := currentUser(c)
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "WEBASSET_REVEAL", Resource: "/api/webassets/" + strconv.Itoa(id),
		Detail: `{"name":"` + w.Name + `"}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"password": plain})
}
