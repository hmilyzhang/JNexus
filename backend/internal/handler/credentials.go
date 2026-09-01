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

// ListPairedCredentials 配对密钥列表：全部密钥认证的 OS 账号，
// 名称 = 主机名-账号，便于识别配对到哪台机器
func ListPairedCredentials(c *gin.Context) {
	var creds []model.HostCredential
	model.DB.Preload("SSHKey").Where("auth_type = ?", "key").Order("id DESC").Find(&creds)
	type pairRow struct {
		ID         uint   `json:"id"`
		HostID     uint   `json:"host_id"`
		Name       string `json:"name"` // 主机名-账号
		HostName   string `json:"host_name"`
		HostIP     string `json:"host_ip"`
		Username   string `json:"username"`
		Label      string `json:"label"`
		KeyName    string `json:"key_name"`
		PublicKey  string `json:"public_key"`
		IsDefault  bool   `json:"is_default"`
		CreatedAt  string `json:"created_at"`
	}
	hostCache := map[uint]model.Host{}
	out := make([]pairRow, 0, len(creds))
	for _, cr := range creds {
		h, ok := hostCache[cr.HostID]
		if !ok {
			model.DB.First(&h, cr.HostID)
			hostCache[cr.HostID] = h
		}
		keyName := ""
		pubKey := ""
		if cr.SSHKey != nil {
			keyName = cr.SSHKey.Name
			pubKey = cr.SSHKey.PublicKey
		}
		name := h.Name + "-" + cr.Username
		if h.Name == "" {
			name = cr.Username
		}
		out = append(out, pairRow{
			ID: cr.ID, HostID: cr.HostID, Name: name,
			HostName: h.Name, HostIP: h.IP,
			Username: cr.Username, Label: cr.Label,
			KeyName: keyName, PublicKey: pubKey,
			IsDefault: cr.IsDefault, CreatedAt: cr.CreatedAt.Format("2006-01-02 15:04:05"),
		})
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
	AutoPair  bool   `json:"auto_pair"` // 密码+自动配对：推送平台公钥后仅保留密钥凭据
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
	var host model.Host
	if err := model.DB.First(&host, hostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "主机不存在"})
		return
	}
	var req credReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名必填"})
		return
	}
	label := req.Label
	if label == "" {
		label = "默认"
	}

	// 密码 + 自动配对：推送平台公钥，成功仅登记密钥凭据；失败不产生记录
	if req.AutoPair && req.Password != "" {
		paired, cred, err := service.PairAndCreateCredential(&host, req.Username, req.Password, label, req.IsDefault)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "配对失败: " + err.Error()})
			return
		}
		_ = paired
		c.JSON(http.StatusOK, cred)
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
