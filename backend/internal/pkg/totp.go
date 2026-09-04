// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package pkg

import (
	"crypto/rand"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP 实现（RFC 6238：HMAC-SHA1、6 位、30 秒步长），纯标准库无外部依赖

const totpPeriod = 30 * time.Second
const totpDigits = 6

// GenerateTOTPSecret 生成 20 字节随机密钥，返回 base32（无填充）
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// totpAt 计算 time 对应步长的 TOTP 码
func totpAt(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", fmt.Errorf("密钥格式错误: %w", err)
	}
	counter := uint64(t.Unix()) / uint64(totpPeriod/time.Second)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	// RFC 4226 动态截断
	offset := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff) % 1000000
	return fmt.Sprintf("%0*d", totpDigits, code), nil
}

// VerifyTOTP 校验动态验证码，允许 ±1 步（±30s）时钟偏移
func VerifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	now := time.Now()
	for _, d := range []time.Duration{-totpPeriod, 0, totpPeriod} {
		want, err := totpAt(secret, now.Add(d))
		if err != nil {
			return false
		}
		if hmac.Equal([]byte(want), []byte(code)) {
			return true
		}
	}
	return false
}

// OTPAuthURL 生成验证器 App 扫码用的 otpauth:// 地址
func OTPAuthURL(username, secret string) string {
	label := url.PathEscape("AutoOps:" + username)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", "AutoOps")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + label + "?" + q.Encode()
}
