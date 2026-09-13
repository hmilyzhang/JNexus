// JNexus Ops Platform — By JJ Zhang, Version 1.0

package pkg

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP implementation (RFC 6238: HMAC-SHA1, 6 digits, 30s step), pure stdlib with no external dependencies

const totpPeriod = 30 * time.Second
const totpDigits = 6

// GenerateTOTPSecret generates a 20-byte random secret and returns it as base32 (unpadded)
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// totpAt computes the TOTP code for the step containing t
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
	// RFC 4226 dynamic truncation
	offset := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff) % 1000000
	return fmt.Sprintf("%0*d", totpDigits, code), nil
}

// VerifyTOTP verifies the dynamic code, allowing ±1 step (±30s) of clock drift
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

// OTPAuthURL builds an otpauth:// URL for authenticator apps to scan
func OTPAuthURL(username, secret string) string {
	label := url.PathEscape("JNexus:" + username)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", "JNexus")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + label + "?" + q.Encode()
}
