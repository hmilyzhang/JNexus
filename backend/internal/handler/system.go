// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/service"
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
	subject := "[AutoOps] SMTP 配置测试邮件"
	body := "<p>这是一封 AutoOps 测试邮件，收到即表示 SMTP 配置正确。</p><p style='color:#909399;font-size:12px'>By JJ Zhang Version 1.0</p>"
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
		"version":     "1.0",
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
