// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"autoops/internal/service"
)

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
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足：当前角色无主机" + action + "权限"})
	}
}
