// JNexus 运维平台 — By JJ Zhang, Version 1.0
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

// RequireCap 统一能力位校验：RequireCap(module, action)
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
