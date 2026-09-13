// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jnexus/internal/model"
)

// Global maintenance windows: downtime inside a window does not count against availability and does not trigger alerts.
// Global setting (applies to all monitors), stored as JSON in system config.
// Each window Type:
//   once    one-shot: date_start/date_end calendar range (inclusive on both ends)
//   daily   daily: active every day
//   weekly  weekly: active on weekdays that match (0=Sunday..6=Saturday)
//   monthly month-end: active on the last day of each month
// start/end in HH:MM; end < start is treated as crossing midnight. Empty Type falls back to once for legacy data.

const maintenanceKey = "maintenance_windows"

type MaintenanceWindow struct {
	Type      string `json:"type"`       // once / daily / weekly / monthly
	DateStart string `json:"date_start"` // once: YYYY-MM-DD
	DateEnd   string `json:"date_end"`   // once: YYYY-MM-DD
	Start     string `json:"start"`      // HH:MM
	End       string `json:"end"`        // HH:MM
	Weekdays  []int  `json:"weekdays"`   // weekly: 0=Sunday..6=Saturday
}

// LoadMaintenances loads global maintenance windows
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

// SaveMaintenances saves global maintenance windows
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

func maintType(w MaintenanceWindow) string {
	if w.Type == "" {
		return "once"
	}
	return w.Type
}

// ValidateMaintenances validates the window list submitted by the frontend (validated per type)
func ValidateMaintenances(wins []MaintenanceWindow) (string, error) {
	for i, w := range wins {
		switch maintType(w) {
		case "once":
			ds, ok1 := parseDate(w.DateStart)
			de, ok2 := parseDate(w.DateEnd)
			if !ok1 || !ok2 {
				return "", fmt.Errorf("窗口 %d：日期非法（需 YYYY-MM-DD）", i+1)
			}
			if de.Before(ds) {
				return "", fmt.Errorf("窗口 %d：结束日期早于开始日期", i+1)
			}
		case "daily", "monthly":
			// daily and month-end need no extra fields
		case "weekly":
			if len(w.Weekdays) == 0 {
				return "", fmt.Errorf("窗口 %d：每周维护需选择生效星期", i+1)
			}
			for _, d := range w.Weekdays {
				if d < 0 || d > 6 {
					return "", fmt.Errorf("窗口 %d：星期非法", i+1)
				}
			}
		default:
			return "", fmt.Errorf("窗口 %d：类型非法", i+1)
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

// isMonthEnd reports whether t is the last day of its month
func isMonthEnd(t time.Time) bool {
	next := t.AddDate(0, 0, 1)
	return next.Month() != t.Month()
}

// InMaintenanceWindow reports whether time t falls within any global maintenance window (applies to all monitors)
func InMaintenanceWindow(t time.Time) bool {
	windows := LoadMaintenances()
	if len(windows) == 0 {
		return false
	}
	day := t.Format("2006-01-02")
	minutes := t.Hour()*60 + t.Minute()
	for _, w := range windows {
		switch maintType(w) {
		case "once":
			if day < w.DateStart || day > w.DateEnd {
				continue
			}
		case "weekly":
			wd := int(t.Weekday())
			hit := false
			for _, d := range w.Weekdays {
				if d == wd {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		case "monthly":
			if !isMonthEnd(t) {
				continue
			}
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
		} else if minutes >= sm || minutes < em { // crosses midnight
			return true
		}
	}
	return false
}
