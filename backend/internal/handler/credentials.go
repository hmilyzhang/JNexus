// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// BatchAddCredentials 批量为存量主机添加 OS 账号（异步任务）
func BatchAddCredentialsHandler(c *gin.Context) {
	var req service.BatchCredRequest
	if err := c.ShouldBindJSON(&req); err != nil || (len(req.Accounts) == 0 && req.Username == "") {
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

// ListAllCredentials 全局 OS 账号列表（跨主机，供账号管理页使用）
// 筛选：host_id、keyword（账号/标签/主机名/IP）、rotation（failed/on/off）
func ListAllCredentials(c *gin.Context) {
	var creds []model.HostCredential
	model.DB.Preload("SSHKey").Order("host_id, is_default DESC, id ASC").Find(&creds)

	hosts := map[uint]model.Host{}
	var hs []model.Host
	model.DB.Find(&hs)
	for _, h := range hs {
		hosts[h.ID] = h
	}

	keyword := strings.ToLower(c.Query("keyword"))
	rotation := c.Query("rotation")

	out := make([]gin.H, 0, len(creds))
	for _, cr := range creds {
		h := hosts[cr.HostID]
		rowFailed := cr.RotateEnabled && strings.Contains(cr.LastRotationResult, "失败")
		match := true
		if keyword != "" &&
			!strings.Contains(strings.ToLower(cr.Username), keyword) &&
			!strings.Contains(strings.ToLower(cr.Label), keyword) &&
			!strings.Contains(strings.ToLower(h.Name), keyword) &&
			!strings.Contains(strings.ToLower(h.IP), keyword) {
			match = false
		}
		if match {
			switch rotation {
			case "failed":
				match = rowFailed
			case "ok":
				match = cr.RotateEnabled && !rowFailed
			case "on":
				match = cr.RotateEnabled
			case "off":
				match = !cr.RotateEnabled
			}
		}
		if !match {
			continue
		}
		keyName := ""
		if cr.SSHKey != nil {
			keyName = cr.SSHKey.Name
		}
		out = append(out, gin.H{
			"id": cr.ID, "host_id": cr.HostID,
			"host_name": h.Name, "host_ip": h.IP,
			"username": cr.Username, "label": cr.Label,
			"auth_type": cr.AuthType, "key_name": keyName,
			"is_default":     cr.IsDefault,
			"rotate_enabled": cr.RotateEnabled, "rotate_days": cr.RotateDays,
			"last_rotated_at": cr.LastRotatedAt, "last_rotation_result": cr.LastRotationResult,
			"is_ldap": cr.IsLDAP, "created_at": cr.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
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
	// 批量预取（固定 5-6 条查询），避免 400+ 主机时的 N+1
	usable := service.UsableCredentialsAll(u, hosts)
	out := []gin.H{}
	for i := range hosts {
		for _, cred := range usable[hosts[i].ID] {
			out = append(out, gin.H{
				"id": cred.ID, "host_id": hosts[i].ID, "username": cred.Username,
				"label": cred.Label, "is_default": cred.IsDefault,
				"auth_type": cred.AuthType,
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
		ID        uint   `json:"id"`
		HostID    uint   `json:"host_id"`
		Name      string `json:"name"` // 主机名-账号
		HostName  string `json:"host_name"`
		HostIP    string `json:"host_ip"`
		Username  string `json:"username"`
		Label     string `json:"label"`
		KeyName   string `json:"key_name"`
		PublicKey string `json:"public_key"`
		IsDefault bool   `json:"is_default"`
		CreatedAt string `json:"created_at"`
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
	Username      string `json:"username" binding:"required"`
	AuthType      string `json:"auth_type"`
	SSHKeyID      *uint  `json:"ssh_key_id"`
	Password      string `json:"password"`
	Label         string `json:"label"`
	IsDefault     bool   `json:"is_default"`
	AutoPair      bool   `json:"auto_pair"`      // 密码+自动配对：推送平台公钥后仅保留密钥凭据
	RotateEnabled bool   `json:"rotate_enabled"` // 密码定期轮换
	RotateDays    int    `json:"rotate_days"`
	IsLDAP        bool   `json:"is_ldap"` // LDAP/域账号标记（排除轮换）
}

func (r *credReq) apply(cred *model.HostCredential) error {
	cred.Username = r.Username
	cred.AuthType = r.AuthType
	if cred.AuthType == "" {
		cred.AuthType = "key"
	}
	cred.SSHKeyID = r.SSHKeyID
	cred.Label = r.Label
	cred.RotateEnabled = r.RotateEnabled
	cred.RotateDays = r.RotateDays // 0 = 跟随系统设置的全局周期
	cred.IsLDAP = r.IsLDAP
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

// BatchDeleteCredentials 批量删除 OS 賩号：逐条复用单删的引用校验
func BatchDeleteCredentials(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids 必填"})
		return
	}
	deleted, failed := []uint{}, map[string]string{}
	for _, id := range req.IDs {
		var cnt int64
		model.DB.Model(&model.AppHost{}).Where("credential_id = ?", id).Count(&cnt)
		if cnt > 0 {
			failed[fmt.Sprint(id)] = "被应用发布配置引用"
			continue
		}
		model.DB.Where("credential_id = ?", id).Delete(&model.UserGroupCredential{})
		if err := model.DB.Delete(&model.HostCredential{}, id).Error; err != nil {
			failed[fmt.Sprint(id)] = err.Error()
			continue
		}
		deleted = append(deleted, id)
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "failed": failed})
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

// RotateCredentialNow 立即轮换一次 OS 账号密码
func RotateCredentialNow(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	if cred.AuthType != "password" || cred.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅密码认证的账号支持轮换"})
		return
	}
	var host model.Host
	if err := model.DB.First(&host, cred.HostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "主机不存在"})
		return
	}
	if cred.IsLDAP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP/域账号不执行轮换"})
		return
	}
	_, err := service.RotateCredentialPassword(&host, &cred)
	result := "轮换成功"
	if err != nil {
		result = "轮换失败: " + err.Error()
	}
	model.DB.Model(&cred).Updates(map[string]any{
		"last_rotated_at":      time.Now(),
		"last_rotation_result": result,
	})
	service.NotifyRotationResult(cred, result, err)
	c.JSON(http.StatusOK, gin.H{"ok": err == nil, "result": result})
}

// RevealCredentialPassword 管理员查看账号密码明文（记录审计）
func RevealCredentialPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	if cred.AuthType != "password" || cred.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账号无密码（密钥认证）"})
		return
	}
	plain, err := pkg.Decrypt(cred.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解密失败: " + err.Error()})
		return
	}
	u := currentUser(c)
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "REVEAL", Resource: "/api/credentials/" + strconv.Itoa(id),
		Detail: `{"os_user":"` + cred.Username + `"}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"password": plain})
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

// ---- 凭据模板（host_id = 0）：LDAP/域账号密码存一次，添加主机/批量导入时引用 ----

// ListCredentialTemplates GET /api/credentials/templates
func ListCredentialTemplates(c *gin.Context) {
	var creds []model.HostCredential
	model.DB.Where("host_id = 0").Order("id DESC").Find(&creds)
	out := make([]gin.H, 0, len(creds))
	for _, cr := range creds {
		out = append(out, gin.H{
			"id": cr.ID, "username": cr.Username, "label": cr.Label,
			"is_ldap": cr.IsLDAP, "created_at": cr.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// SaveCredentialTemplate POST /api/credentials/templates（id 为空新建，否则更新密码）
func SaveCredentialTemplate(c *gin.Context) {
	var req struct {
		ID       *uint  `json:"id"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password"`
		Label    string `json:"label"`
		IsLDAP   bool   `json:"is_ldap"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "err.param"})
		return
	}
	if req.ID != nil {
		var cr model.HostCredential
		if err := model.DB.First(&cr, *req.ID).Error; err != nil || cr.HostID != 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
			return
		}
		cr.Username = req.Username
		cr.IsLDAP = req.IsLDAP
		if req.Label != "" {
			cr.Label = req.Label
		}
		if req.Password != "" {
			enc, err := pkg.Encrypt(req.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			cr.Password = enc
		}
		model.DB.Save(&cr)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "err.param"})
		return
	}
	enc, err := pkg.Encrypt(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	label := req.Label
	if label == "" {
		label = "模板"
	}
	cr := model.HostCredential{HostID: 0, Username: req.Username, AuthType: "password",
		Password: enc, Label: label, IsDefault: false, IsLDAP: req.IsLDAP}
	if err := model.DB.Create(&cr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": cr.ID})
}

// DeleteCredentialTemplate DELETE /api/credentials/templates/:id
func DeleteCredentialTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cr model.HostCredential
	if err := model.DB.First(&cr, id).Error; err != nil || cr.HostID != 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
		return
	}
	model.DB.Delete(&cr)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// resolveTemplatePassword 取模板的解密密码（模板必须存在且属于当前用户可用的范围）
func resolveTemplatePassword(id uint) (username, password string, isLDAP bool, err error) {
	var cr model.HostCredential
	if e := model.DB.First(&cr, id).Error; e != nil || cr.HostID != 0 {
		return "", "", false, fmt.Errorf("凭据模板不存在")
	}
	plain, e := pkg.Decrypt(cr.Password)
	if e != nil {
		return "", "", false, e
	}
	return cr.Username, plain, cr.IsLDAP, nil
}
