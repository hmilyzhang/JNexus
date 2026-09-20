// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// BatchAddCredentials batch-adds OS accounts to existing hosts (async task)
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

// ListAllCredentials global OS account list (across hosts, for the account management page)
// Filters: host_id, keyword (account/label/hostname/IP), rotation (failed/on/off)
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
			"has_password": cr.Password != "",
		})
	}
	c.JSON(http.StatusOK, out)
}

// ListHostCredentials lists a host's OS accounts
func ListHostCredentials(c *gin.Context) {
	hostID, _ := strconv.Atoi(c.Param("id"))
	var creds []model.HostCredential
	model.DB.Preload("SSHKey").Where("host_id = ?", hostID).Order("is_default DESC, id ASC").Find(&creds)
	c.JSON(http.StatusOK, creds)
}

// UsableCredentials OS accounts available to the current user on each host (for exec/terminal selection)
func UsableCredentialsHandler(c *gin.Context) {
	u := currentUser(c)
	var hosts []model.Host
	model.DB.Find(&hosts)
	// Batch prefetch (fixed 5-6 queries) to avoid N+1 with 400+ hosts
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

// ListPairedCredentials paired key list: all key-auth OS accounts,
// name = hostname-account, to identify which machine each key is paired with.
// Supports keyword filter (host/IP/account/label) + page/page_size pagination.
func ListPairedCredentials(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))

	var creds []model.HostCredential
	model.DB.Preload("SSHKey").Where("auth_type = ?", "key").Order("id DESC").Find(&creds)
	type pairRow struct {
		ID        uint   `json:"id"`
		HostID    uint   `json:"host_id"`
		Name      string `json:"name"` // hostname-account
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
	var hs []model.Host
	model.DB.Find(&hs)
	for _, h := range hs {
		hostCache[h.ID] = h
	}
	out := make([]pairRow, 0, len(creds))
	for _, cr := range creds {
		h, ok := hostCache[cr.HostID]
		if !ok {
			model.DB.First(&h, cr.HostID)
			hostCache[cr.HostID] = h
		}
		if keyword != "" &&
			!strings.Contains(strings.ToLower(h.Name), keyword) &&
			!strings.Contains(strings.ToLower(h.IP), keyword) &&
			!strings.Contains(strings.ToLower(cr.Username), keyword) &&
			!strings.Contains(strings.ToLower(cr.Label), keyword) {
			continue
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
	total := len(out)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out[start:end], "total": total,
		"page": page, "page_size": pageSize,
	})
}

type credReq struct {
	Username      string `json:"username" binding:"required"`
	AuthType      string `json:"auth_type"`
	SSHKeyID      *uint  `json:"ssh_key_id"`
	Password      string `json:"password"`
	Label         string `json:"label"`
	IsDefault     bool   `json:"is_default"`
	AutoPair      bool   `json:"auto_pair"`      // password + auto-pair: push the platform public key and keep only the key credential
	RotateEnabled bool   `json:"rotate_enabled"` // periodic password rotation
	RotateDays    int    `json:"rotate_days"`
	IsLDAP        bool   `json:"is_ldap"` // LDAP/domain account flag (excluded from rotation)
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
	cred.RotateDays = r.RotateDays // 0 = follow the global period from system settings
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

// Set as the sole default
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

	// Password + auto-pair: push the platform public key; on success only the key credential is recorded, on failure no record is created
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
	if req.Password != "" {
		service.RecordPasswordHistory(cred.ID, req.Password, "created", currentUser(c).Username)
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
	if req.Password != "" {
		service.RecordPasswordHistory(cred.ID, req.Password, "manual", currentUser(c).Username)
	}
	c.JSON(http.StatusOK, cred)
}

// BatchDeleteCredentials batch-deletes OS accounts: reuses the single-delete reference check for each item
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

// RotateCredentialNow rotates an OS account password immediately
func RotateCredentialNow(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	if cred.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号未保存密码，无法轮换"})
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
	newPwd, err := service.RotateCredentialPassword(&host, &cred)
	result := "轮换成功"
	if err != nil {
		result = "轮换失败: " + err.Error()
	} else {
		u := currentUser(c)
		service.RecordPasswordHistory(cred.ID, newPwd, "manual", u.Username)
	}
	model.DB.Model(&cred).Updates(map[string]any{
		"last_rotated_at":      time.Now(),
		"last_rotation_result": result,
	})
	service.NotifyRotationResult(cred, result, err)
	c.JSON(http.StatusOK, gin.H{"ok": err == nil, "result": result})
}

// RevealCredentialPassword lets an admin view the account password in plain text (audited)
func RevealCredentialPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	if cred.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账号未保存密码"})
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

// PasswordHistory GET /api/credentials/:id/password-history — admin-only, audited.
// Returns the credential's archived passwords (decrypted for display), newest first;
// the newest entry is the current password.
func PasswordHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cred model.HostCredential
	if err := model.DB.First(&cred, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OS 账号不存在"})
		return
	}
	if cred.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账号未保存密码"})
		return
	}
	var rows []model.CredentialPasswordHistory
	model.DB.Where("credential_id = ?", id).Order("id DESC").Limit(50).Find(&rows)
	out := make([]gin.H, 0, len(rows))
	for i, r := range rows {
		plain, derr := pkg.Decrypt(r.Password)
		if derr != nil {
			plain = "(解密失败)"
		}
		out = append(out, gin.H{
			"changed_at": r.ChangedAt, "source": r.Source,
			"operator": r.Operator, "password": plain, "current": i == 0,
		})
	}
	u := currentUser(c)
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "REVEAL_HISTORY", Resource: "/api/credentials/" + strconv.Itoa(id),
		Detail: `{"os_user":"` + cred.Username + `"}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, out)
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

// ---- Credential templates (host_id = 0): LDAP/domain account passwords stored once, referenced when adding hosts / batch importing ----

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

// SaveCredentialTemplate POST /api/credentials/templates (empty id creates; otherwise updates the password)
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

// resolveTemplatePassword returns the decrypted template password (template must exist and be within the current user's usable scope)
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

// ---- Batch rotation ----

type rotateBatchState struct {
	Total   int     `json:"total"`
	Done    int     `json:"done"`
	OK      int     `json:"ok"`
	Failed  int     `json:"failed"`
	Running bool    `json:"running"`
	Results []gin.H `json:"results"`
}

var (
	rotateBatchesMu sync.Mutex
	rotateBatches   = map[string]*rotateBatchState{}
)

// RotateCredentialsBatch POST /api/credentials/rotate-batch  {ids: [credID...]}
// Rotates asynchronously one by one (300ms stagger); progress is queried via /rotate-batch/:batch.
func RotateCredentialsBatch(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids 必填"})
		return
	}
	if len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单批最多 100 个账号"})
		return
	}
	u := currentUser(c)
	batch := startRotationBatch(req.IDs, u, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"batch": batch, "total": len(req.IDs)})
}

// startRotationBatch starts an async batch rotation (serial per host, 300ms stagger); returns the batch ID
func startRotationBatch(ids []uint, u *model.User, ip string) string {
	batch := "rb-" + time.Now().Format("0102150405") + fmt.Sprintf("-%04d", time.Now().UnixNano()%10000)
	st := &rotateBatchState{Total: len(ids), Running: true, Results: []gin.H{}}
	rotateBatchesMu.Lock()
	rotateBatches[batch] = st
	for k := range rotateBatches { // keep only the most recent 20 batches
		if len(rotateBatches) > 20 {
			delete(rotateBatches, k)
		}
	}
	rotateBatchesMu.Unlock()

	go func() {
		defer func() { recover() }()
		rotateBatchesMu.Lock()
		st := rotateBatches[batch]
		rotateBatchesMu.Unlock()
		for _, id := range ids {
			var cred model.HostCredential
			var host model.Host
			hostDisp := ""
			result := ""
			ok := false
			if err := model.DB.First(&cred, id).Error; err != nil {
				result = "账号不存在"
			} else if cred.Password == "" {
				result = "账号未保存密码，无法轮换"
			} else if cred.IsLDAP {
				result = "LDAP/域账号跳过"
			} else if err := model.DB.First(&host, cred.HostID).Error; err != nil {
				result = "主机不存在"
			} else {
				if host.Name != "" {
					hostDisp = host.Name
				} else {
					hostDisp = host.IP
				}
				newPwd, rerr := service.RotateCredentialPassword(&host, &cred)
				if rerr != nil {
					result = "轮换失败: " + rerr.Error()
				} else {
					result = "轮换成功"
					ok = true
					service.RecordPasswordHistory(cred.ID, newPwd, "manual", u.Username)
				}
				model.DB.Model(&cred).Updates(map[string]any{
					"last_rotated_at":      time.Now(),
					"last_rotation_result": result,
				})
				service.NotifyRotationResult(cred, result, rerr)
				model.DB.Create(&model.AuditLog{
					UserID: u.ID, Username: u.Username,
					Action: "CRED_ROTATE", Resource: fmt.Sprintf("%s@%s", cred.Username, hostDisp),
					IP: ip, Status: map[bool]int{true: 200, false: 500}[ok], CreatedAt: time.Now(),
				})
			}
			rotateBatchesMu.Lock()
			st.Done++
			if ok {
				st.OK++
			} else {
				st.Failed++
			}
			st.Results = append(st.Results, gin.H{
				"id": id, "host": hostDisp, "username": cred.Username, "ok": ok, "result": result,
			})
			rotateBatchesMu.Unlock()
			time.Sleep(300 * time.Millisecond) // stagger to avoid hammering target hosts simultaneously
		}
		rotateBatchesMu.Lock()
		st.Running = false
		rotateBatchesMu.Unlock()
	}()

	return batch
}

// RotateCredentialsBatchStatus GET /api/credentials/rotate-batch/:batch — batch rotation progress
func RotateCredentialsBatchStatus(c *gin.Context) {
	rotateBatchesMu.Lock()
	st, ok := rotateBatches[c.Param("batch")]
	rotateBatchesMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "批次不存在"})
		return
	}
	c.JSON(http.StatusOK, st)
}
