// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

func ListApps(c *gin.Context) {
	var apps []model.Application
	model.DB.Preload("AppHosts.Host").Order("id DESC").Find(&apps)
	// Data-level filtering: non-platform roles only see apps bound to their user groups
	ids, unrestricted := service.VisibleAppIDs(currentUser(c))
	if !unrestricted {
		filtered := make([]model.Application, 0, len(apps))
		for _, a := range apps {
			if ids[a.ID] {
				filtered = append(filtered, a)
			}
		}
		apps = filtered
	}
	c.JSON(http.StatusOK, apps)
}

type appReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	AppHosts    []struct {
		HostID         uint   `json:"host_id" binding:"required"`
		CredentialID   *uint  `json:"credential_id"`
		DeployDir      string `json:"deploy_dir" binding:"required"`
		JarName        string `json:"jar_name" binding:"required"`
		StopCmd        string `json:"stop_cmd"`
		StartCmd       string `json:"start_cmd"`
		BackupDir      string `json:"backup_dir"`
		HealthCheckURL string `json:"health_check_url"`
	} `json:"app_hosts"`
}

func saveAppHosts(appID uint, req *appReq) error {
	model.DB.Where("app_id = ?", appID).Delete(&model.AppHost{})
	for _, ah := range req.AppHosts {
		rec := model.AppHost{
			AppID: appID, HostID: ah.HostID, CredentialID: ah.CredentialID,
			DeployDir: ah.DeployDir, JarName: ah.JarName,
			StopCmd: ah.StopCmd, StartCmd: ah.StartCmd, BackupDir: ah.BackupDir, HealthCheckURL: ah.HealthCheckURL,
		}
		if err := model.DB.Create(&rec).Error; err != nil {
			return err
		}
	}
	return nil
}

func CreateApp(c *gin.Context) {
	var req appReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（名称、主机的部署目录与 jar 名必填）"})
		return
	}
	app := model.Application{Name: req.Name, Description: req.Description}
	if err := model.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用名已存在"})
		return
	}
	if err := saveAppHosts(app.ID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存部署配置失败"})
		return
	}
	model.DB.Preload("AppHosts.Host").First(&app, app.ID)
	c.JSON(http.StatusOK, app)
}

func UpdateApp(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !service.CanOperateApp(currentUser(c), uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	var app model.Application
	if err := model.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}
	var req appReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	model.DB.Model(&app).Updates(map[string]any{"name": req.Name, "description": req.Description})
	if err := saveAppHosts(app.ID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存部署配置失败"})
		return
	}
	model.DB.Preload("AppHosts.Host").First(&app, app.ID)
	c.JSON(http.StatusOK, app)
}

func DeleteApp(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Where("app_id = ?", id).Delete(&model.AppHost{})
	model.DB.Where("app_id = ?", id).Delete(&model.UserApp{})
	model.DB.Delete(&model.Application{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- Releases ----

func ListReleases(c *gin.Context) {
	var releases []model.Release
	q := model.DB
	if appID := c.Query("app_id"); appID != "" {
		q = q.Where("app_id = ?", appID)
	}
	q.Order("id DESC").Limit(100).Find(&releases)
	ids, unrestricted := service.VisibleAppIDs(currentUser(c))
	if !unrestricted {
		filtered := make([]model.Release, 0, len(releases))
		for _, r := range releases {
			if ids[r.AppID] {
				filtered = append(filtered, r)
			}
		}
		releases = filtered
	}
	c.JSON(http.StatusOK, releases)
}

func GetRelease(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rel model.Release
	if err := model.DB.First(&rel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "发布单不存在"})
		return
	}
	var items []model.ReleaseItem
	model.DB.Where("release_id = ?", id).Find(&items)
	c.JSON(http.StatusOK, gin.H{"release": rel, "items": items})
}

func CreateReleaseHandler(c *gin.Context) {
	var req service.CreateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	u := currentUser(c)
	if !service.CanOperateApp(u, req.AppID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	relID, err := service.StartRelease(u, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"release_id": relID})
}

func RollbackHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	u := currentUser(c)
	var rel model.Release
	if err := model.DB.First(&rel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "发布单不存在"})
		return
	}
	if !service.CanOperateApp(u, rel.AppID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	newID, err := service.RollbackRelease(u, uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"release_id": newID})
}
