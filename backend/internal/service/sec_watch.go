// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Security watchlists: the concrete items the security monitoring watches,
// each individually toggleable, managed from the observability admin page.
// Windows = Security-log event ids; Linux = message keyword patterns.
// "Immediate" items alert per hit (e.g. root login); others aggregate one
// summary per collection round.

import (
	"encoding/json"
	"strconv"
	"strings"

	"jnexus/internal/model"
)

type SecWatchWin struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	On        bool   `json:"on"`
	Immediate bool   `json:"immediate"`
}

type SecWatchLinux struct {
	KW        string `json:"kw"`
	Name      string `json:"name"`
	On        bool   `json:"on"`
	Immediate bool   `json:"immediate"`
}

const (
	secWatchWinKey   = "sec_watch_win"
	secWatchLinuxKey = "sec_watch_linux"
)

func defaultSecWatchWin() []SecWatchWin {
	return []SecWatchWin{
		{ID: 4625, Name: "登录失败", On: true},
		{ID: 4720, Name: "账号创建", On: true},
		{ID: 4726, Name: "账号删除", On: true},
		{ID: 4738, Name: "账号修改", On: false},
		{ID: 4740, Name: "账号锁定", On: true},
		{ID: 1102, Name: "审计日志清除", On: true, Immediate: true},
	}
}

func defaultSecWatchLinux() []SecWatchLinux {
	return []SecWatchLinux{
		{KW: "accepted password for root", Name: "root 密码登录", On: true, Immediate: true},
		{KW: "accepted publickey for root", Name: "root 公钥登录", On: true, Immediate: true},
		{KW: "session opened for user root", Name: "root 会话开启", On: true, Immediate: true},
		{KW: "failed password", Name: "SSH 登录失败", On: true},
		{KW: "invalid user", Name: "非法用户", On: true},
		{KW: "user not in sudoers", Name: "sudo 越权尝试", On: true},
		{KW: "authentication failure", Name: "认证失败", On: false},
		{KW: "sudo:", Name: "sudo 提权", On: false},
	}
}

func loadJSONList(key string, out any) bool {
	m := SystemConfigMap()
	raw := strings.TrimSpace(m[key])
	if raw == "" {
		return false
	}
	return json.Unmarshal([]byte(raw), out) == nil
}

// SecWatchWinList returns the Windows watchlist (defaults when unset)
func SecWatchWinList() []SecWatchWin {
	var out []SecWatchWin
	if loadJSONList(secWatchWinKey, &out) && len(out) > 0 {
		return out
	}
	return defaultSecWatchWin()
}

// SecWatchLinuxList returns the Linux keyword watchlist (defaults when unset)
func SecWatchLinuxList() []SecWatchLinux {
	var out []SecWatchLinux
	if loadJSONList(secWatchLinuxKey, &out) && len(out) > 0 {
		return out
	}
	return defaultSecWatchLinux()
}

// SaveSecWatch persists both watchlists
func SaveSecWatch(win []SecWatchWin, linux []SecWatchLinux) error {
	winJSON, err := json.Marshal(win)
	if err != nil {
		return err
	}
	linuxJSON, err := json.Marshal(linux)
	if err != nil {
		return err
	}
	return SetSystemConfigs(map[string]string{
		secWatchWinKey:   string(winJSON),
		secWatchLinuxKey: string(linuxJSON),
	})
}

// secAlertChannels resolves the configured security channels (enabled only)
func secAlertChannels() []model.AlertChannel {
	chCfg := strings.TrimSpace(SystemConfigMap()["sec_alert_channels"])
	if chCfg == "" {
		return nil
	}
	var ids []uint
	for _, s := range strings.Split(chCfg, ",") {
		if n, e := strconv.Atoi(strings.TrimSpace(s)); e == nil && n > 0 {
			ids = append(ids, uint(n))
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var channels []model.AlertChannel
	model.DB.Where("id IN ? AND enabled = ?", ids, true).Find(&channels)
	return channels
}

