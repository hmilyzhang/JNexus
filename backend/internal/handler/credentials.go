// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/pkg"
	"autoops/internal/service"
)

// BatchAddCredentials 批量为存量主机添加 OS 账号（异步任务）
func BatchAddCredentialsHandler(c *gin.Context) {
	var req service.BatchCredRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（账号名必填）"})
		return
	}
	taskID, err := service.BatchAddCredentials(currentUser(c), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": taskID})
}

// ListHostCredentials 主机的 OS 账号列表
func ListHostCredentials(c *gin.Context) {
	hostID, _ := strconv.Atoi(c.Param("id"))
	var creds []model.HostCredential
	model.DB.Preload("SSHKey").Where("host_id = ?", hostID).Order("is_default DESC, id ASC").Find(&creds)
	c.JSON(http.StatusOK, creds)
}

// UsableCredentials 当前用户在各主机上可用的 OS 账号（执行/终端选择用）
func UsableCredentialsHandler(c *gin.Context) {
	u := currentUser(c)
	var hosts []model.Host
	model.DB.Find(&hosts)
	out := []gin.H{}
	for i := range hosts {
		for _, cred := range service.UsableCredentials(u, &hosts[i]) {
			out = append(out, gin.H{
				"id": cred.ID, "host_id": hosts[i].ID, "username": cred.Username,
				"label": cred.Label, "is_default": cred.IsDefault,
				"host_name": hosts[i].Name, "host_ip": hosts[i].IP,
			})
		}
	}
	c.JSON(http.StatusOK, out)
}

type credReq struct {
	Username  string `json:"username" binding:"required"`
	AuthType  string `json:"auth_type"`
	SSHKeyID  *uint  `json:"ssh_key_id"`
	Password  string `json:"password"`
	Label     string `json:"label"`
	IsDefault bool   `json:"is_default"`
}

func (r *credReq) apply(cred *model.HostCredential) error {
	cred.Username = r.Username
	cred.AuthType = r.AuthType
	if cred.AuthType == "" {
		cred.AuthType = "key"
	}
	cred.SSHKeyID = r.SSHKeyID
	cred.Label = r.Label
	if r.Password != "" {
		enc, err := pkg.Encrypt(r.Password)
		if err != nil {
			return err
		}
		cred.Password = enc
	}
	return nil
}

// 设为唯一默认
func makeDefault(hostID, credID uint) {
	model.DB.Model(&model.HostCredential{}).Where("host_id = ?", hostID).Update("is_default", false)
	model.DB.Model(&model.HostCredential{}).Where("id = ?", credID).Update("is_default", true)
}

func CreateHostCredential(c *gin.Context) {
	hostID, _ := strconv.Atoi(c.Param("id"))
	var req credReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名必填"})
		return
	}
	if req.AuthType == "key" && req.SSHKeyID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密钥认证需要选择 SSH 密钥"})
		return
	}
	cred := model.HostCredential{HostID: uint(hostID)}
	if err := req.apply(&cred); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := model.DB.Create(&cred).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	var cnt int64
	model.DB.Model(&model.HostCredential{}).Where("host_id = ?", hostID).Count(&cnt)
	if cnt == 1 || req.IsDefault {
		makeDefault(uint(hostID), cred.ID)
	}
	c.JSON(http.StatusOK, cred)
}

func UpdateCredential(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	var req credReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名必填"})
		return
	}
	if req.AuthType == "key" && req.SSHKeyID == nil && (cred.SSHKeyID == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密钥认证需要选择 SSH 密钥"})
		return
	}
	if err := req.apply(&cred); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	model.DB.Save(&cred)
	if req.IsDefault {
		makeDefault(cred.HostID, cred.ID)
	}
	c.JSON(http.StatusOK, cred)
}

func DeleteCredential(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cnt int64
	model.DB.Model(&model.AppHost{}).Where("credential_id = ?", id).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该 OS 账号被应用发布配置引用，请先在应用管理中移除"})
		return
	}
	model.DB.Where("credential_id = ?", id).Delete(&model.UserGroupCredential{})
	model.DB.Delete(&model.HostCredential{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func SetDefaultCredential(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	makeDefault(cred.HostID, cred.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
