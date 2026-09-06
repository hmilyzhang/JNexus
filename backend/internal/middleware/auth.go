// JNexus 运维平台 — By JJ Zhang, Version 1.0

package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/pkg"
)

// JWT 解析 token 并把用户信息写入 context
func JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) < 8 || !strings.EqualFold(auth[:7], "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		claims, err := pkg.ParseToken(strings.TrimSpace(auth[7:]))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			return
		}
		if claims.Purpose == "mfa" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先完成两步验证"})
			return
		}
		var u model.User
		if err := model.DB.First(&u, claims.UserID).Error; err != nil || u.Status != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "账号不可用"})
			return
		}
		c.Set("user", &u)
		c.Next()
	}
}

// CurrentUser 从 context 取当前用户
func CurrentUser(c *gin.Context) *model.User {
	v, _ := c.Get("user")
	u, _ := v.(*model.User)
	return u
}

// RequireRole 路由级 RBAC：admin 恒通过
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		if u.IsAdmin() {
			c.Next()
			return
		}
		for _, r := range roles {
			if u.Role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
	}
}

// UserCanExecGroup 用户是否对主机组有执行权限
func UserCanExecGroup(user *model.User, groupID *uint) bool {
	if user.IsAdmin() {
		return true
	}
	if user.Role != model.RoleOps && user.Role != model.RolePublisher {
		return false
	}
	if groupID == nil {
		// 未分组主机：仅 admin 可执行
		return false
	}
	var cnt int64
	model.DB.Model(&model.UserHostGroup{}).
		Where("user_id = ? AND group_id = ? AND can_exec = ?", user.ID, *groupID, true).
		Count(&cnt)
	return cnt > 0
}
