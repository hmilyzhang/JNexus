// JNexus 运维平台 — By JJ Zhang, Version 1.0
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

	"autoops/internal/model"
)

// API 密钥：外部系统集成用。完整密钥仅创建时返回一次，服务端只保存 SHA-256 哈希。

const apiKeyRateLimitPerMin = 120

// GenerateApiKey 生成新密钥：返回 (完整密钥, keyID, secretHash)
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

// HashApiKeySecret 计算密钥 secret 部分的哈希
func HashApiKeySecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ParseApiKey 拆分完整密钥为 keyID + secret（格式 aok_<keyID>.<secret>）
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

// AuthenticateApiKey 校验完整密钥：返回密钥行与属主用户
// checks: 存在、哈希一致（恒定时间比较）、启用、未过期、IP 白名单
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

// checkIPAllowlist 白名单为逗号分隔的 IP 或 CIDR；空 = 不限制
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

// ---------- 简单限流（每密钥每分钟 120 次） ----------

var (
	rateMu    sync.Mutex
	rateBlink int64 // 分钟桶起点（Unix 分钟）
	rateCount = map[uint]int{}
)

// RateLimitApiKey 超限返回 false
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

// TouchApiKey 更新最后使用时间/IP（节流：最多每分钟写一次库）
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
