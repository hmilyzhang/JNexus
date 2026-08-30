// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"

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
}

// GetSystemConfig 读取系统配置（admin），密码字段打码
func GetSystemConfig(c *gin.Context) {
	m := service.SystemConfigMap()
	if m["ldap_bind_password"] != "" {
		m["ldap_bind_password"] = "******"
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
		if k == "ldap_bind_password" && (v == "" || v == "******") {
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
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"username": u.Username, "role": u.Role, "auth_source": u.AuthSource,
			"last_login_at": u.LastLoginAt,
		},
		"hosts_total":  count(&model.Host{}, ""),
		"hosts_online": online,
		"host_groups":  count(&model.HostGroup{}, ""),
		"users":        count(&model.User{}, ""),
		"tasks":        count(&model.Task{}, ""),
		"scripts":      count(&model.Script{}, ""),
		"apps":         count(&model.Application{}, ""),
		"releases":     count(&model.Release{}, ""),
		"danger_rules": count(&model.DangerRule{}, "enabled = ?", true),
	})
}
