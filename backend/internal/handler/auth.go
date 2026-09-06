// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"errors"
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
			if errors.Is(ldapErr, service.ErrLDAPGroupDenied) {
				c.JSON(http.StatusForbidden, gin.H{"error": ldapErr.Error()})
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
		u = ldapUser
		if u.Status != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用"})
			return
		}
	}

	// 开启 MFA 的账号：密码通过后先发短时 mfa_token，验证动态码后才发正式 token
	if u.MFAEnabled {
		mfaToken, err := pkg.GenMFAToken(u.ID, u.Username, u.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"mfa_required": true,
			"mfa_token":    mfaToken,
			"username":     u.Username,
		})
		return
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

// LoginMFA 登录第二步：校验 TOTP 动态码，换取正式 token
func LoginMFA(c *gin.Context) {
	var req struct {
		MFAToken string `json:"mfa_token" binding:"required"`
		Code     string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	claims, err := pkg.ParseToken(req.MFAToken)
	if err != nil || claims.Purpose != "mfa" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证已超时，请重新登录"})
		return
	}
	var u model.User
	if err := model.DB.First(&u, claims.UserID).Error; err != nil || u.Status != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不可用"})
		return
	}
	secret, err := pkg.Decrypt(u.MFASecret)
	if err != nil || !pkg.VerifyTOTP(secret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "动态验证码错误"})
		return
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
			AuthSource: "ldap", Email: email, Status: 1, CreatedBy: "LDAP",
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
	var lastLogin *time.Time
	if u.LastLoginAt != nil {
		t := *u.LastLoginAt
		lastLogin = &t
	}
	c.JSON(http.StatusOK, gin.H{"id": u.ID, "username": u.Username, "role": u.Role,
		"auth_source": u.AuthSource, "email": u.Email, "last_login_at": lastLogin})
}

// UpdateMe 当前用户自助维护基本信息（邮箱；LDAP/AD 用户由系统同步，不可改）
func UpdateMe(c *gin.Context) {
	u := currentUser(c)
	if strings.EqualFold(u.AuthSource, "ldap") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP/AD 用户信息由系统自动同步，不可手动修改"})
		return
	}
	var req struct {
		Email *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	model.DB.Model(u).Update("email", *req.Email)
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
