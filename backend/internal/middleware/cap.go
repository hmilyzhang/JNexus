// JNexus Ops Platform — By JJ Zhang, Version 1.0
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

// RequireCap unified capability check: RequireCap(module, action)
func RequireCap(module, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		if !service.HasCap(u.Role, module, action) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
			return
		}
		c.Next()
	}
}

// RequireCapAny passes when the role has ANY of the listed actions on the module
func RequireCapAny(module string, actions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		for _, a := range actions {
			if service.HasCap(u.Role, module, a) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
	}
}
