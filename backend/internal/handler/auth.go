// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"autoops/internal/model"
	"autoops/internal/pkg"
	"autoops/internal/service"
)

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var u model.User
	err := model.DB.Where("username = ?", req.Username).First(&u).Error

	// 本地账号：bcrypt 校验
	if err == nil && u.AuthSource != "ldap" {
		if u.Status != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用"})
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
	} else {
		// LDAP 认证（用户不存在或为 LDAP 账号时）
		ldapUser, ldapErr := tryLDAPLogin(req.Username, req.Password, err != nil)
		if ldapErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
		u = ldapUser
		if u.Status != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用"})
			return
		}
	}

	token, err := pkg.GenToken(u.ID, u.Username, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}
	model.DB.Model(&u).Update("last_login_at", time.Now())
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  gin.H{"id": u.ID, "username": u.Username, "role": u.Role, "auth_source": u.AuthSource},
	})
}

// tryLDAPLogin LDAP 登录；账号不存在时按默认角色自动建号
func tryLDAPLogin(username, password string, autoCreate bool) (model.User, error) {
	settings := service.LoadLDAPSettings()
	if !settings.Enabled {
		return model.User{}, errLDAPDisabled
	}
	_, email, err := service.LDAPLogin(settings, username, password)
	if err != nil {
		return model.User{}, err
	}
	var u model.User
	if dbErr := model.DB.Where("username = ?", username).First(&u).Error; dbErr != nil {
		if !autoCreate {
			return model.User{}, errLDAPDisabled
		}
		u = model.User{
			Username: username, Role: settings.DefaultRole,
			AuthSource: "ldap", Email: email, Status: 1,
		}
		if err := model.DB.Create(&u).Error; err != nil {
			return model.User{}, err
		}
		return u, nil
	}
	if email != "" && u.Email != email {
		model.DB.Model(&u).Update("email", email)
		u.Email = email
	}
	return u, nil
}

var errLDAPDisabled = &ldapDisabledError{}

type ldapDisabledError struct{}

func (*ldapDisabledError) Error() string { return "LDAP 认证未启用" }

func Me(c *gin.Context) {
	u := currentUser(c)
	c.JSON(http.StatusOK, gin.H{"id": u.ID, "username": u.Username, "role": u.Role, "auth_source": u.AuthSource})
}

func ChangePassword(c *gin.Context) {
	u := currentUser(c)
	if strings.EqualFold(u.AuthSource, "ldap") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP 账号请在 LDAP 系统中修改密码"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少 6 位"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码错误"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	model.DB.Model(u).Update("password", string(hash))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func currentUser(c *gin.Context) *model.User {
	return c.MustGet("user").(*model.User)
}
