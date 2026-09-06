// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jnexus/internal/model"
)

// 全局维护窗口：窗口内的 downtime 不计入可用率、不触发告警。
// 全局设置（对所有监控项生效），存系统配置 JSON。
// 每个窗口：{date_start/date_end: "YYYY-MM-DD"（日历范围），start/end: "HH:MM"}，end < start 视为跨午夜。

const maintenanceKey = "maintenance_windows"

type MaintenanceWindow struct {
	DateStart string `json:"date_start"` // YYYY-MM-DD
	DateEnd   string `json:"date_end"`   // YYYY-MM-DD
	Start     string `json:"start"`      // HH:MM
	End       string `json:"end"`        // HH:MM
}

// LoadMaintenances 读取全局维护窗口
func LoadMaintenances() []MaintenanceWindow {
	m := SystemConfigMap()
	raw := m[maintenanceKey]
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []MaintenanceWindow
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// SaveMaintenances 保存全局维护窗口
func SaveMaintenances(wins []MaintenanceWindow) error {
	b, err := json.Marshal(wins)
	if err != nil {
		return err
	}
	return model.DB.Save(&model.SystemConfig{Key: maintenanceKey, Value: string(b)}).Error
}

func parseHM(s string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ValidateMaintenances 校验前端提交的窗口列表
func ValidateMaintenances(wins []MaintenanceWindow) (string, error) {
	for i, w := range wins {
		ds, ok1 := parseDate(w.DateStart)
		de, ok2 := parseDate(w.DateEnd)
		if !ok1 || !ok2 {
			return "", fmt.Errorf("窗口 %d：日期非法（需 YYYY-MM-DD）", i+1)
		}
		if de.Before(ds) {
			return "", fmt.Errorf("窗口 %d：结束日期早于开始日期", i+1)
		}
		if _, ok := parseHM(w.Start); !ok {
			return "", fmt.Errorf("窗口 %d：开始时间非法（HH:MM）", i+1)
		}
		if _, ok := parseHM(w.End); !ok {
			return "", fmt.Errorf("窗口 %d：结束时间非法（HH:MM）", i+1)
		}
	}
	b, err := json.Marshal(wins)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// InMaintenanceWindow 判断时刻 t 是否处于任一全局维护窗口（对所有监控项生效）
func InMaintenanceWindow(t time.Time) bool {
	windows := LoadMaintenances()
	if len(windows) == 0 {
		return false
	}
	day := t.Format("2006-01-02")
	minutes := t.Hour()*60 + t.Minute()
	for _, w := range windows {
		if day < w.DateStart || day > w.DateEnd {
			continue
		}
		sm, ok1 := parseHM(w.Start)
		em, ok2 := parseHM(w.End)
		if !ok1 || !ok2 {
			continue
		}
		if sm <= em {
			if minutes >= sm && minutes < em {
				return true
			}
		} else if minutes >= sm || minutes < em { // 跨午夜
			return true
		}
	}
	return false
}
