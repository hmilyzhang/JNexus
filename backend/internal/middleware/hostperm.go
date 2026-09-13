// JNexus Ops Platform — By JJ Zhang, Version 1.0
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

// RequireCredPerm checks OS account management permission (configurable per role; admin always passes)
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

// RequireReportPerm checks report module permission (configurable per role; admin always passes)
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

// RequireHostPerm checks fine-grained host permissions per role config (view/create/edit/delete)
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
