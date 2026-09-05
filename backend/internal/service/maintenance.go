// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"autoops/internal/model"
)

// 维护窗口：窗口内的 downtime 不计入可用率、不触发告警。
// 每个监控项可配置多个窗口：{days: [1-7]（周一=1），start/end: "HH:MM"}，end < start 视为跨午夜。

type MaintenanceWindow struct {
	Days  []int  `json:"days"`  // ISO 星期：1=周一 ... 7=周日
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

func parseMaintenances(raw string) []MaintenanceWindow {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []MaintenanceWindow
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
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

// InMaintenance 判断监控项在时刻 t 是否处于维护窗口
func InMaintenance(m *model.Monitor, t time.Time) bool {
	windows := parseMaintenances(m.Maintenances)
	if len(windows) == 0 {
		return false
	}
	iso := int(t.Weekday()) // Sun=0
	if iso == 0 {
		iso = 7
	}
	minutes := t.Hour()*60 + t.Minute()
	for _, w := range windows {
		dayOK := false
		for _, d := range w.Days {
			if d == iso {
				dayOK = true
				break
			}
		}
		if !dayOK {
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

// ValidateMaintenances 校验前端提交的维护窗口
func ValidateMaintenances(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || string(raw) == "null" {
		return "", nil
	}
	var wins []MaintenanceWindow
	if err := json.Unmarshal(raw, &wins); err != nil {
		return "", fmt.Errorf("维护窗口格式错误")
	}
	for _, w := range wins {
		if _, ok := parseHM(w.Start); !ok {
			return "", fmt.Errorf("维护窗口开始时间非法: %s", w.Start)
		}
		if _, ok := parseHM(w.End); !ok {
			return "", fmt.Errorf("维护窗口结束时间非法: %s", w.End)
		}
		for _, d := range w.Days {
			if d < 1 || d > 7 {
				return "", fmt.Errorf("维护窗口日期需在 1-7（周一至周日）")
			}
		}
	}
	b, err := json.Marshal(wins)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
