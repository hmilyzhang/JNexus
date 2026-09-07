// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/buildinfo"
	"jnexus/internal/model"
	"jnexus/internal/service"
)

// 系统配置键
var editableConfigKeys = []string{
	"system_name",
	"ldap_enabled", "ldap_host", "ldap_port", "ldap_tls",
	"ldap_bind_dn", "ldap_bind_password", "ldap_base_dn",
	"ldap_user_filter", "ldap_attr_username", "ldap_default_role",
	"ldap_group_check", "ldap_group_base_dn", "ldap_group_filter", "ldap_required_groups",
	"smtp_enabled", "smtp_host", "smtp_port", "smtp_ssl", "smtp_tls",
	"smtp_username", "smtp_password", "smtp_from", "smtp_recipients", "smtp_notify",
	"rotation_enabled", "rotation_length", "rotation_complexity", "rotation_days",
}

// GetSystemConfig 读取系统配置（admin），密码字段打码
func GetSystemConfig(c *gin.Context) {
	m := service.SystemConfigMap()
	for _, k := range []string{"ldap_bind_password", "smtp_password"} {
		if m[k] != "" {
			m[k] = "******"
		}
	}
	c.JSON(http.StatusOK, m)
}

// UpdateSystemConfig 更新系统配置（admin）；密码传空/打码则保持原值
func UpdateSystemConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	old := service.SystemConfigMap()
	filtered := map[string]string{}
	for _, k := range editableConfigKeys {
		v, ok := req[k]
		if !ok {
			continue
		}
		if (k == "ldap_bind_password" || k == "smtp_password") && (v == "" || v == "******") {
			continue // 保持原值
		}
		filtered[k] = v
	}
	if v, ok := filtered["ldap_enabled"]; ok && v != "true" && v != "false" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ldap_enabled 只能是 true/false"})
		return
	}
	if err := service.SetSystemConfigs(filtered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	_ = old
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetSystemRoles 角色设置（所有登录用户可读，用于菜单/界面过滤）
func GetSystemRoles(c *gin.Context) {
	c.JSON(http.StatusOK, service.GetRoleSettings())
}

// UpdateSystemRoles 保存角色设置（admin）
// ---- 自定义角色管理 ----

// builtinRoles 内置角色 key（禁止删除；admin 额外禁止一切写操作）
var builtinRoles = map[string]bool{
	"admin": true, "ops": true, "publisher": true,
	"viewer": true, "auditor": true, "k8s": true,
}

// CreateRole POST /api/system/roles  创建自定义角色（可复制现有角色的权限）
func CreateRole(c *gin.Context) {
	var req struct {
		Key    string              `json:"key" binding:"required"`
		Desc   string              `json:"desc"`
		CopyOf string              `json:"copy_of"`
		Perms  map[string][]string `json:"perms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色 key 必填"})
		return
	}
	key := strings.TrimSpace(req.Key)
	if len(key) < 2 || len(key) > 32 || strings.ContainsAny(key, " /\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色 key 须 2-32 位且不含空格或斜杠"})
		return
	}
	if builtinRoles[key] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "与内置角色重名"})
		return
	}
	settings := service.GetRoleSettings()
	if _, exists := settings[key]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "角色已存在"})
		return
	}

	// 基础配置：复制源角色 或 空白（仅 dashboard 菜单）
	rp := service.RolePerm{Desc: req.Desc, Menus: []string{"dashboard"}, Perms: map[string][]string{}}
	if req.CopyOf != "" {
		if src, ok := settings[req.CopyOf]; ok {
			rp.Desc = req.Desc
			rp.Menus = append([]string{}, src.Menus...)
			rp.Perms = map[string][]string{}
			for m, acts := range src.Perms {
				rp.Perms[m] = append([]string{}, acts...)
			}
			rp.Host = src.Host
			rp.Cred, rp.Report = src.Cred, src.Report
			rp.K8sView, rp.K8sManage = src.K8sView, src.K8sManage
		}
	}
	// 调用方显式提供的 perms 优先（前端矩阵可直接提交）
	if len(req.Perms) > 0 {
		rp.Perms = req.Perms
	}
	if rp.Perms == nil {
		rp.Perms = map[string][]string{}
	}
	settings[key] = rp
	if err := service.SetRoleSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "ROLE", Resource: "CREATE ROLE " + key,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "key": key})
}

// DeleteRole DELETE /api/system/roles/:key  删除自定义角色（内置禁止；被用户引用禁止）
func DeleteRole(c *gin.Context) {
	key := c.Param("key")
	if builtinRoles[key] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内置角色不可删除"})
		return
	}
	var cnt int64
	model.DB.Model(&model.User{}).Where("role = ?", key).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("该角色仍有 %d 个用户使用，请先改派角色", cnt), "count": cnt})
		return
	}
	settings := service.GetRoleSettings()
	if _, exists := settings[key]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	delete(settings, key)
	if err := service.SetRoleSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "ROLE", Resource: "DELETE ROLE " + key,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RoleUsers GET /api/system/roles/:key/users  该角色下的用户列表（删除前提示用）
func RoleUsers(c *gin.Context) {
	key := c.Param("key")
	var users []model.User
	model.DB.Where("role = ?", key).Select("id, username, status").Find(&users)
	c.JSON(http.StatusOK, users)
}

// GetSystemCapabilities 角色设置矩阵的模块/操作声明（前端自动渲染）
func GetSystemCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, service.CapabilitiesForFront())
}

func UpdateSystemRoles(c *gin.Context) {
	var req map[string]service.RolePerm
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	// admin 权限固定全开，防止误配置锁死
	admin := req[model.RoleAdmin]
	admin.Host.View, admin.Host.Create, admin.Host.Edit, admin.Host.Delete = true, true, true, true
	if len(admin.Menus) == 0 {
		def := service.DefaultRoleSettings()[model.RoleAdmin]
		admin.Menus = def.Menus
	}
	admin.Desc = "全部权限，含用户/系统管理"
	req[model.RoleAdmin] = admin
	if err := service.SetRoleSettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetPlatformKey 平台配对密钥公钥信息（admin）
func GetPlatformKey(c *gin.Context) {
	k, err := service.EnsurePlatformKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"name": k.Name, "public_key": k.PublicKey, "created_at": k.CreatedAt,
		"hint": "自动配对会将该公钥写入目标机 ~/.ssh/authorized_keys",
	})
}

// TestSMTPConfig 发送测试邮件
func TestSMTPConfig(c *gin.Context) {
	var req struct {
		To string `json:"to"`
	}
	_ = c.ShouldBindJSON(&req)
	smtpCfg := service.LoadSMTPSettings()
	to := strings.FieldsFunc(req.To, func(r rune) bool { return r == ',' || r == 10 || r == 13 || r == 59 })
	if len(to) == 0 {
		to = smtpCfg.Recipients
	}
	if len(to) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写收件邮箱"})
		return
	}
	subject := "[JNexus] SMTP 配置测试邮件"
	body := "<p>这是一封 JNexus 测试邮件，收到即表示 SMTP 配置正确。</p><p style='color:#909399;font-size:12px'>By JJ Zhang Version 1.0</p>"
	if err := service.SendMail(smtpCfg, to, subject, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestLDAPConfig 用当前已保存配置测试 LDAP 连通性
func TestLDAPConfig(c *gin.Context) {
	if err := service.TestLDAP(service.LoadLDAPSettings()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SystemInfo 公开接口：登录页展示系统名称/版本/作者
func SystemInfo(c *gin.Context) {
	m := service.SystemConfigMap()
	c.JSON(http.StatusOK, gin.H{
		"system_name": m["system_name"],
		"version":     buildinfo.Get(),
		"author":      "JJ Zhang",
	})
}

// Dashboard 登录后首页统计
func Dashboard(c *gin.Context) {
	u := currentUser(c)
	count := func(dst any, where string, args ...any) int64 {
		var n int64
		q := model.DB.Model(dst)
		if where != "" {
			q = q.Where(where, args...)
		}
		q.Count(&n)
		return n
	}
	online := count(&model.Host{}, "status = ?", "online")
	// 任务数范围：管理员/审计员看全量，其他人仅统计本人发起的任务
	taskQ := model.DB.Model(&model.Task{})
	if !u.IsAdmin() && u.Role != model.RoleAuditor {
		taskQ = taskQ.Where("operator = ?", u.Username)
	}
	var taskCnt int64
	taskQ.Count(&taskCnt)

	// 拦截规则/用户数为管理员维度，非管理员返回 0（前端隐藏对应卡片）
	dangerCnt, userCnt := count(&model.DangerRule{}, "enabled = ?", true), count(&model.User{}, "")
	if !u.IsAdmin() {
		dangerCnt, userCnt = 0, 0
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"username": u.Username, "role": u.Role, "auth_source": u.AuthSource,
			"last_login_at": u.LastLoginAt,
		},
		"hosts_total":  count(&model.Host{}, ""),
		"hosts_online": online,
		"host_groups":  count(&model.HostGroup{}, ""),
		"users":        userCnt,
		"tasks":        taskCnt,
		"scripts":      count(&model.Script{}, ""),
		"apps":         count(&model.Application{}, ""),
		"releases":     count(&model.Release{}, ""),
		"danger_rules": dangerCnt,
	})
}
