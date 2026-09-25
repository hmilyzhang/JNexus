// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
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
	// case-insensitive username lookup: Admin/admin log into the same account
	err := model.DB.Where("lower(username) = lower(?)", req.Username).First(&u).Error

	// Local account: verify with bcrypt
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
		// LDAP authentication (when the user does not exist or is an LDAP account)
		ldapUser, ldapErr := tryLDAPLogin(req.Username, req.Password, err != nil)
		if ldapErr != nil {
			// Log the failure stage server-side for troubleshooting (UI still shows a unified message to avoid exposing account info)
			if !errors.Is(ldapErr, errLDAPDisabled) {
				log.Printf("[ldap] login failed for %q: %v", req.Username, ldapErr)
			}
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

	// MFA-enabled accounts: after the password passes, issue a short-lived mfa_token first; the real token is only issued after the code is verified
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
		"user":  gin.H{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "role": u.Role, "auth_source": u.AuthSource},
	})
}

// LoginMFA login step 2: verify the TOTP code and exchange it for the real token
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
		"user":  gin.H{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "role": u.Role, "auth_source": u.AuthSource},
	})
}

// tryLDAPLogin LDAP login; auto-creates the account with the default role if it does not exist
func tryLDAPLogin(username, password string, autoCreate bool) (model.User, error) {
	settings := service.LoadLDAPSettings()
	if !settings.Enabled {
		return model.User{}, errLDAPDisabled
	}
	_, email, displayName, err := service.LDAPLogin(settings, username, password)
	if err != nil {
		return model.User{}, err
	}
	var u model.User
	if dbErr := model.DB.Where("lower(username) = lower(?)", username).First(&u).Error; dbErr != nil {
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
	if displayName != "" && u.DisplayName != displayName {
		model.DB.Model(&u).Update("display_name", displayName)
		u.DisplayName = displayName
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
	c.JSON(http.StatusOK, gin.H{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "role": u.Role,
		"auth_source": u.AuthSource, "email": u.Email, "last_login_at": lastLogin})
}

// UpdateMe lets the current user maintain basic info (email; LDAP/AD users are synced by the system and cannot be changed here)
func UpdateMe(c *gin.Context) {
	u := currentUser(c)
	if strings.EqualFold(u.AuthSource, "ldap") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP/AD 用户信息由系统自动同步，不可手动修改"})
		return
	}
	var req struct {
		Email       *string `json:"email"`
		DisplayName *string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]any{}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.DisplayName != nil {
		updates["display_name"] = strings.TrimSpace(*req.DisplayName)
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	model.DB.Model(u).Updates(updates)
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
