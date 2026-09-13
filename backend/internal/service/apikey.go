// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
)

// API keys: for external system integrations. The full key is returned only once at creation; the server stores only its SHA-256 hash.

const apiKeyRateLimitPerMin = 120

// GenerateApiKey generates a new key: returns (full key, keyID, secretHash)
func GenerateApiKey() (full, keyID, secretHash string, err error) {
	raw := make([]byte, 20)
	if _, err = rand.Read(raw); err != nil {
		return
	}
	keyID = hex.EncodeToString(raw)[:12]
	raw2 := make([]byte, 24)
	if _, err = rand.Read(raw2); err != nil {
		return
	}
	secret := base64.RawURLEncoding.EncodeToString(raw2)
	full = "aok_" + keyID + "." + secret
	sum := sha256.Sum256([]byte(secret))
	secretHash = hex.EncodeToString(sum[:])
	return
}

// HashApiKeySecret computes the hash of the key's secret part
func HashApiKeySecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ParseApiKey splits a full key into keyID + secret (format aok_<keyID>.<secret>)
func ParseApiKey(full string) (keyID, secret string, ok bool) {
	if !strings.HasPrefix(full, "aok_") {
		return
	}
	rest := full[4:]
	idx := strings.IndexByte(rest, '.')
	if idx <= 0 || idx == len(rest)-1 {
		return
	}
	return rest[:idx], rest[idx+1:], true
}

// AuthenticateApiKey verifies a full key: returns the key row and its owner user
// checks: existence, hash match (constant-time compare), enabled, not expired, IP allowlist
func AuthenticateApiKey(full, clientIP string) (*model.ApiKey, *model.User, error) {
	keyID, secret, ok := ParseApiKey(full)
	if !ok {
		return nil, nil, fmt.Errorf("密钥格式错误")
	}
	var key model.ApiKey
	if err := model.DB.Where("key_id = ?", keyID).First(&key).Error; err != nil {
		return nil, nil, fmt.Errorf("密钥无效")
	}
	if subtle.ConstantTimeCompare([]byte(key.KeyHash), []byte(HashApiKeySecret(secret))) != 1 {
		return nil, nil, fmt.Errorf("密钥无效")
	}
	if !key.Enabled {
		return nil, nil, fmt.Errorf("密钥已停用")
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, nil, fmt.Errorf("密钥已过期")
	}
	if err := checkIPAllowlist(key.IPAllowlist, clientIP); err != nil {
		return nil, nil, err
	}
	var user model.User
	if err := model.DB.First(&user, key.OwnerUserID).Error; err != nil || user.Status != 1 {
		return nil, nil, fmt.Errorf("密钥属主账号不可用")
	}
	return &key, &user, nil
}

// checkIPAllowlist the allowlist is comma-separated IPs or CIDRs; empty = no restriction
func checkIPAllowlist(allowlist, clientIP string) error {
	allowlist = strings.TrimSpace(allowlist)
	if allowlist == "" {
		return nil
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return fmt.Errorf("来源 IP 非法")
	}
	for _, item := range strings.Split(allowlist, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if _, ipnet, err := net.ParseCIDR(item); err == nil && ipnet.Contains(ip) {
				return nil
			}
		} else if net.ParseIP(item) != nil && item == clientIP {
			return nil
		}
	}
	return fmt.Errorf("来源 IP %s 不在密钥白名单内", clientIP)
}

// ---------- Simple rate limiting (120 requests per key per minute) ----------

var (
	rateMu    sync.Mutex
	rateBlink int64 // start of the minute bucket (Unix minutes)
	rateCount = map[uint]int{}
)

// RateLimitApiKey returns false when over the limit
func RateLimitApiKey(keyID uint) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	bucket := time.Now().Unix() / 60
	if bucket != rateBlink {
		rateBlink = bucket
		rateCount = map[uint]int{}
	}
	if rateCount[keyID] >= apiKeyRateLimitPerMin {
		return false
	}
	rateCount[keyID]++
	return true
}

// TouchApiKey updates last-used time/IP (throttled: at most one DB write per minute)
var lastTouchMu sync.Mutex
var lastTouch = map[uint]time.Time{}

func TouchApiKey(key *model.ApiKey, ip string) {
	lastTouchMu.Lock()
	last, ok := lastTouch[key.ID]
	lastTouchMu.Unlock()
	if ok && time.Since(last) < time.Minute {
		return
	}
	lastTouchMu.Lock()
	lastTouch[key.ID] = time.Now()
	lastTouchMu.Unlock()
	now := time.Now()
	model.DB.Model(key).Updates(map[string]any{"last_used_at": now, "last_used_ip": ip})
}
