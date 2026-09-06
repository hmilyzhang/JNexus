// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"

	"autoops/internal/model"
	"autoops/internal/pkg"
)

// MFAStatus 当前账号 MFA 状态
func MFAStatus(c *gin.Context) {
	u := currentUser(c)
	c.JSON(http.StatusOK, gin.H{"enabled": u.MFAEnabled})
}

// MFASetup 生成 TOTP 密钥（待确认状态），返回密钥/otpauth 地址/二维码
func MFASetup(c *gin.Context) {
	u := currentUser(c)
	if u.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA 已启用，如需更换请先关闭"})
		return
	}
	secret, err := pkg.GenerateTOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成密钥失败"})
		return
	}
	enc, err := pkg.Encrypt(secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密钥加密失败"})
		return
	}
	if err := model.DB.Model(u).Update("mfa_secret", enc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存密钥失败"})
		return
	}
	uri := pkg.OTPAuthURL(u.Username, secret)
	png, err := qrcode.Encode(uri, qrcode.Medium, 220)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成二维码失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":      secret,
		"otpauth_url": uri,
		"qr":          "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

// MFAEnable 校验动态码后正式开启 MFA
func MFAEnable(c *gin.Context) {
	u := currentUser(c)
	if u.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA 已启用"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || u.MFASecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先生成密钥并扫码"})
		return
	}
	secret, err := pkg.Decrypt(u.MFASecret)
	if err != nil || !pkg.VerifyTOTP(secret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "动态验证码错误，请重试"})
		return
	}
	if err := model.DB.Model(u).Update("mfa_enabled", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MFADisable 关闭 MFA（需当前动态码）
func MFADisable(c *gin.Context) {
	u := currentUser(c)
	if !u.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA 未启用"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	secret, err := pkg.Decrypt(u.MFASecret)
	if err != nil || !pkg.VerifyTOTP(secret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "动态验证码错误"})
		return
	}
	if err := model.DB.Model(u).Updates(map[string]any{"mfa_enabled": false, "mfa_secret": ""}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AdminResetUserMFA 管理员重置用户 MFA（用户丢失验证器时解绑）
func AdminResetUserMFA(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u model.User
	if err := model.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err := model.DB.Model(&u).Updates(map[string]any{"mfa_enabled": false, "mfa_secret": ""}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
