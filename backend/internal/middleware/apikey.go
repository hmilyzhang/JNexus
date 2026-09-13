// JNexus Ops Platform — By JJ Zhang, Version 1.0
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

// APIKeyAuth external API key authentication:
// Authorization: Bearer aok_<keyID>.<secret>
// Validation order: rate limit -> key validity -> owner account; on success stores the user in context and records an audit entry
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) < 8 || !strings.EqualFold(auth[:7], "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 API 密钥（Authorization: Bearer aok_...）"})
			return
		}
		full := strings.TrimSpace(auth[7:])
		if !strings.HasPrefix(full, "aok_") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "外部 API 仅接受 aok_ 前缀的 API 密钥，不接受登录令牌"})
			return
		}
		key, user, err := service.AuthenticateApiKey(full, c.ClientIP())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if !service.RateLimitApiKey(key.ID) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁（每密钥每分钟 120 次上限）"})
			return
		}
		c.Set("user", user)
		c.Set("api_key", key)
		c.Next()
		// Audit: record key name, path, and status code
		model.DB.Create(&model.AuditLog{
			UserID: user.ID, Username: user.Username + " [key:" + key.Name + "]",
			Action: "API", Resource: c.Request.Method + " " + c.FullPath(),
			IP: c.ClientIP(), Status: c.Writer.Status(), CreatedAt: time.Now(),
		})
		service.TouchApiKey(key, c.ClientIP())
	}
}

// ApiKeyFromContext returns the current request's API key (nil if none)
func ApiKeyFromContext(c *gin.Context) *model.ApiKey {
	v, _ := c.Get("api_key")
	k, _ := v.(*model.ApiKey)
	return k
}

var _ = time.Now
