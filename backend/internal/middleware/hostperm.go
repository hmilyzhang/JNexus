// JNexus 运维平台 — By JJ Zhang, Version 1.0
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

// RequireCredPerm 校验 OS 账号管理权限（角色设置可配；admin 恒通过）
func RequireCredPerm() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		if service.HasCredPerm(u.Role) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "err.noCredPerm", "action": "os_accounts"})
	}
}

// RequireReportPerm 校验报告模块权限（角色设置可配；admin 恒通过）
func RequireReportPerm() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		if service.HasReportPerm(u.Role) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "err.noReportPerm", "action": "reports"})
	}
}

// RequireHostPerm 按角色配置校验主机细粒度权限（view/create/edit/delete）
func RequireHostPerm(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		if service.HasHostPerm(u.Role, action) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "err.noHostPerm", "action": action})
	}
}
