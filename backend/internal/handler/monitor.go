// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

type monitorReq struct {
	Name           string `json:"name" binding:"required"`
	Type           string `json:"type" binding:"required"`
	Target         string `json:"target" binding:"required"`
	Port           int    `json:"port"`
	Method         string `json:"method"`
	AcceptedStatus string `json:"accepted_status"`
	Keyword        string `json:"keyword"`
	KeywordType    string `json:"keyword_type"`
	IntervalSec    int    `json:"interval_sec"`
	TimeoutSec     int    `json:"timeout_sec"`
	Enabled        *bool  `json:"enabled"`
	ChannelIDs     []uint `json:"channel_ids"` // Alert notification channel bindings
}

// saveMonitorBindings replaces a monitor's notification channel bindings
func saveMonitorBindings(monitorID uint, channelIDs []uint) {
	model.DB.Where("monitor_id = ?", monitorID).Delete(&model.MonitorChannel{})
	for _, cid := range channelIDs {
		model.DB.Create(&model.MonitorChannel{MonitorID: monitorID, ChannelID: cid})
	}
}

func applyMonitorReq(m *model.Monitor, req monitorReq) error {
	switch req.Type {
	case "http", "tcp", "ping":
	default:
		return fmt.Errorf("监控类型必须是 http / tcp / ping")
	}
	m.Name, m.Type, m.Target = req.Name, req.Type, req.Target
	m.Port, m.Method, m.AcceptedStatus = req.Port, req.Method, req.AcceptedStatus
	m.Keyword, m.KeywordType = req.Keyword, req.KeywordType
	if req.KeywordType == "" {
		m.KeywordType = "contain"
	}
	m.IntervalSec, m.TimeoutSec = req.IntervalSec, req.TimeoutSec
	if m.IntervalSec <= 0 {
		m.IntervalSec = service.MonitorInterval()
	}
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = 10
	}
	if m.Method == "" {
		m.Method = "GET"
	}
	return nil
}

// ListMonitors monitor list: latest status + 24h uptime + recent heartbeats (up to 50)
func ListMonitors(c *gin.Context) {
	u := currentUser(c)
	_, app, _, manage := service.MonitorCaps(u.Role)

	var monitors []model.Monitor
	model.DB.Order("id").Find(&monitors)
	// department scoping: without manage, only global and own-group monitors are listed
	if !manage {
		groups := service.UserGroupIDsOf(u.ID)
		filtered := monitors[:0]
		for _, m := range monitors {
			if m.OwnerGroupID == nil || (app && containsUint(groups, *m.OwnerGroupID)) {
				filtered = append(filtered, m)
			}
		}
		monitors = filtered
	}

	since24 := time.Now().Add(-24 * time.Hour)
	since30 := time.Now().Add(-30 * 24 * time.Hour)
	type uptimeRow struct {
		MonitorID uint
		Uptime    float64
	}
	var uptimes []uptimeRow
	model.DB.Raw(`SELECT monitor_id, AVG(CASE WHEN status = 'up' THEN 100.0 ELSE 0 END) AS uptime
		FROM monitor_samples WHERE created_at > ? AND status IN ('up','down') GROUP BY monitor_id`, since24).Scan(&uptimes)
	uptimeMap := map[uint]float64{}
	for _, u := range uptimes {
		uptimeMap[u.MonitorID] = math.Round(u.Uptime*10) / 10
	}
	var uptimes30 []uptimeRow
	model.DB.Raw(`SELECT monitor_id, AVG(CASE WHEN status = 'up' THEN 100.0 ELSE 0 END) AS uptime
		FROM monitor_samples WHERE created_at > ? AND status IN ('up','down') GROUP BY monitor_id`, since30).Scan(&uptimes30)
	uptimeMap30 := map[uint]float64{}
	for _, u := range uptimes30 {
		uptimeMap30[u.MonitorID] = math.Round(u.Uptime*10) / 10
	}

	type sampleRow struct {
		MonitorID uint
		ID        uint
		Status    string
		RespMs    int
		CreatedAt time.Time
	}
	var samples []sampleRow
	model.DB.Raw(`SELECT monitor_id, id, status, resp_ms, created_at FROM (
		SELECT monitor_id, id, status, resp_ms, created_at,
			ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY id DESC) AS rn
		FROM monitor_samples WHERE created_at > ?
	) t WHERE rn <= 50 ORDER BY monitor_id, id`, time.Now().Add(-2*24*time.Hour)).Scan(&samples)
	recentMap := map[uint][]gin.H{}
	for _, s := range samples {
		recentMap[s.MonitorID] = append(recentMap[s.MonitorID], gin.H{
			"status": s.Status, "resp_ms": s.RespMs, "at": s.CreatedAt,
		})
	}

	var bindings []model.MonitorChannel
	model.DB.Find(&bindings)
	chMap := map[uint][]uint{}
	for _, b := range bindings {
		chMap[b.MonitorID] = append(chMap[b.MonitorID], b.ChannelID)
	}

	out := []gin.H{}
	for _, m := range monitors {
		out = append(out, gin.H{
			"monitor": m, "uptime24h": uptimeMap[m.ID], "uptime_30d": uptimeMap30[m.ID],
			"recent": recentMap[m.ID], "channel_ids": chMap[m.ID],
		})
	}
	c.JSON(http.StatusOK, out)
}

// CreateMonitor creates a monitor
func CreateMonitor(c *gin.Context) {
	var req monitorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（名称/类型/目标必填）"})
		return
	}
	m := model.Monitor{CreatedBy: currentUser(c).Username, Enabled: true}
	if err := applyMonitorReq(&m, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// department self-service: creators without monitor.manage own the monitor
	// through their first user group, so only department members and infra manage it
	u := currentUser(c)
	if _, _, _, manage := service.MonitorCaps(u.Role); !manage {
		groups := service.UserGroupIDsOf(u.ID)
		if len(groups) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "自建监控需要先加入用户组（用于归属部门）"})
			return
		}
		m.OwnerGroupID = &groups[0]
	}
	if err := model.DB.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	saveMonitorBindings(m.ID, req.ChannelIDs)
	c.JSON(http.StatusOK, m)
}

// canEditMonitor: monitor.manage, or view_app on a monitor owned by the
// caller's user group (department self-service monitors)
func canEditMonitor(u *model.User, m *model.Monitor) bool {
	_, app, _, manage := service.MonitorCaps(u.Role)
	if manage {
		return true
	}
	if !app || m.OwnerGroupID == nil {
		return false
	}
	for _, g := range service.UserGroupIDsOf(u.ID) {
		if g == *m.OwnerGroupID {
			return true
		}
	}
	return false
}

// UpdateMonitor edits a monitor
func UpdateMonitor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.Monitor
	if err := model.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控项不存在"})
		return
	}
	u := currentUser(c)
	if !canEditMonitor(u, &m) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改该监控项（仅部门成员与 infra 可管理）"})
		return
	}
	var req monitorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := applyMonitorReq(&m, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}
	m.NextRunAt = nil // reschedule immediately
	updates := map[string]any{
		"name": m.Name, "type": m.Type, "target": m.Target, "port": m.Port,
		"method": m.Method, "accepted_status": m.AcceptedStatus,
		"keyword": m.Keyword, "keyword_type": m.KeywordType,
		"interval_sec": m.IntervalSec, "timeout_sec": m.TimeoutSec,
		"enabled": m.Enabled, "next_run_at": nil,
	}
	model.DB.Model(&m).Updates(updates)
	saveMonitorBindings(m.ID, req.ChannelIDs)
	c.JSON(http.StatusOK, m)
}

// DeleteMonitor deletes a monitor (samples are cascade-deleted)
func DeleteMonitor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.Monitor
	if err := model.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控项不存在"})
		return
	}
	if !canEditMonitor(currentUser(c), &m) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该监控项（仅部门成员与 infra 可管理）"})
		return
	}
	model.DB.Where("monitor_id = ?", id).Delete(&model.MonitorSample{})
	model.DB.Delete(&model.Monitor{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestMonitor runs a check immediately (the result is also persisted so the latest status is visible)
func TestMonitor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.Monitor
	if err := model.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控项不存在"})
		return
	}
	if !canEditMonitor(currentUser(c), &m) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权测试该监控项"})
		return
	}
	up, ms, errMsg := service.RunMonitorOnce(&m)
	c.JSON(http.StatusOK, gin.H{"up": up, "resp_ms": ms, "error": errMsg})
}

// MonitorHistory sample history for one monitor (?hours=24, max 720; aggregated hourly beyond 48h)
func MonitorHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	hours := 24
	if h, e := strconv.Atoi(c.Query("hours")); e == nil && h > 0 && h <= 24*30 {
		hours = h
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	if hours > 48 {
		type bucket struct {
			Bucket time.Time `json:"at"`
			Up     int       `json:"up"`
			Down   int       `json:"down"`
			Maint  int       `json:"maint"`
		}
		var rows []bucket
		model.DB.Raw(`SELECT date_trunc('hour', created_at) AS bucket,
				SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) AS up,
				SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END) AS down,
				SUM(CASE WHEN status = 'maint' THEN 1 ELSE 0 END) AS maint
			FROM monitor_samples WHERE monitor_id = ? AND created_at > ?
			GROUP BY bucket ORDER BY bucket`, id, since).Scan(&rows)
		c.JSON(http.StatusOK, rows)
		return
	}
	var samples []model.MonitorSample
	model.DB.Where("monitor_id = ? AND created_at > ?", id, since).
		Order("id").Limit(5000).Find(&samples)
	c.JSON(http.StatusOK, samples)
}

// HostMetricsList returns the latest resources of all hosts (CPU/memory/disk)
func HostMetricsList(c *gin.Context) {
	var rows []struct {
		HostID      uint
		Name        string
		IP          string
		GroupName   *string
		Status      string
		CPUPercent  float64
		MemPercent  float64
		DiskPercent float64
		CollectedAt time.Time
	}
	model.DB.Raw(`SELECT DISTINCT ON (hm.host_id)
			hm.host_id, h.name, h.ip, g.name AS group_name, h.status,
			hm.cpu_percent, hm.mem_percent, hm.disk_percent, hm.collected_at
		FROM host_metrics hm
		JOIN hosts h ON h.id = hm.host_id
		LEFT JOIN host_groups g ON g.id = h.group_id
		ORDER BY hm.host_id, hm.collected_at DESC`).Scan(&rows)
	// Also list hosts that have no samples yet (frontend shows "no data")
	var hosts []model.Host
	model.DB.Order("name").Find(&hosts)
	seen := map[uint]bool{}
	for _, r := range rows {
		seen[r.HostID] = true
	}
	out := []gin.H{}
	for _, r := range rows {
		out = append(out, gin.H{
			"host_id": r.HostID, "name": r.Name, "ip": r.IP, "group": r.GroupName,
			"status": r.Status, "cpu": r.CPUPercent, "mem": r.MemPercent,
			"disk": r.DiskPercent, "collected_at": r.CollectedAt,
		})
	}
	for _, h := range hosts {
		if !seen[h.ID] {
			out = append(out, gin.H{
				"host_id": h.ID, "name": h.Name, "ip": h.IP,
				"status": h.Status, "collected_at": nil,
			})
		}
	}
	c.JSON(http.StatusOK, out)
}

// HostMetricHistory resource history for one host (?hours=6, max 720; aggregated hourly beyond 48h)
func HostMetricHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	hours := 6
	if h, e := strconv.Atoi(c.Query("hours")); e == nil && h > 0 && h <= 24*400 {
		hours = h
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	if hours > 24*30 {
		// Long ranges (180d/1y): last 30 days from raw data in day buckets, older from the hourly archive in day buckets, merged into one series
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		type dayBucket struct {
			Bucket      time.Time `json:"collected_at"`
			CPUPercent  float64   `json:"cpu_percent"`
			MemPercent  float64   `json:"mem_percent"`
			DiskPercent float64   `json:"disk_percent"`
		}
		// Last 30 days: raw data in day buckets; older: hourly archive in day buckets; both merged into one series
		var rows []dayBucket
		model.DB.Raw(`SELECT date_trunc('day', t.at) AS bucket,
				AVG(t.cpu_percent) AS cpu_percent, AVG(t.mem_percent) AS mem_percent, AVG(t.disk_percent) AS disk_percent
			FROM (
				SELECT collected_at AS at, cpu_percent, mem_percent, disk_percent
					FROM host_metrics WHERE host_id = ? AND collected_at >= ?
				UNION ALL
				SELECT bucket AS at, cpu_percent, mem_percent, disk_percent
					FROM host_metric_hourlies WHERE host_id = ? AND bucket >= ? AND bucket < ?
			) t GROUP BY bucket ORDER BY bucket`, id, cutoff, id, since, cutoff).Scan(&rows)
		if rows == nil {
			rows = []dayBucket{}
		}
		c.JSON(http.StatusOK, rows)
		return
	}
	if hours > 48 {
		type bucket struct {
			Bucket      time.Time `json:"collected_at"`
			CPUPercent  float64   `json:"cpu_percent"`
			MemPercent  float64   `json:"mem_percent"`
			DiskPercent float64   `json:"disk_percent"`
		}
		var rows []bucket
		model.DB.Raw(`SELECT date_trunc('hour', collected_at) AS bucket,
				AVG(cpu_percent) AS cpu_percent, AVG(mem_percent) AS mem_percent, AVG(disk_percent) AS disk_percent
			FROM host_metrics WHERE host_id = ? AND collected_at > ?
			GROUP BY bucket ORDER BY bucket`, id, since).Scan(&rows)
		c.JSON(http.StatusOK, rows)
		return
	}
	var rows []model.HostMetric
	model.DB.Where("host_id = ? AND collected_at > ?", id, since).
		Order("collected_at").Limit(5000).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

// MonitorScreen GET /api/monitoring/screen — aggregated monitoring dashboard data (single request fetches all; frontend polls every 15s)
func MonitorScreen(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Aggregate hosts by group
	var hosts []model.Host
	model.DB.Select("name", "status", "group_id").Find(&hosts)
	var groups []model.HostGroup
	model.DB.Find(&groups)
	groupNames := map[uint]string{}
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}
	type grpRow struct {
		Name   string `json:"name"`
		Total  int64  `json:"total"`
		Online int64  `json:"online"`
	}
	grpMap := map[uint]*grpRow{}
	order := []uint{}
	ensure := func(id uint) *grpRow {
		if r, ok := grpMap[id]; ok {
			return r
		}
		r := &grpRow{Name: "未分组"}
		if id != 0 {
			if n, ok := groupNames[id]; ok {
				r.Name = n
			}
		}
		grpMap[id] = r
		order = append(order, id)
		return r
	}
	onlineHosts := 0
	for _, h := range hosts {
		id := uint(0)
		if h.GroupID != nil {
			id = *h.GroupID
		}
		r := ensure(id)
		r.Total++
		if h.Status == "online" {
			r.Online++
			onlineHosts++
		}
	}
	groupsOut := make([]grpRow, 0, len(order))
	for _, id := range order {
		groupsOut = append(groupsOut, *grpMap[id])
	}
	sort.Slice(groupsOut, func(i, j int) bool { return groupsOut[i].Total > groupsOut[j].Total })

	// Probe monitor status
	var monitors []model.Monitor
	model.DB.Select("name", "type", "target", "port", "enabled", "last_status", "last_resp_ms").Find(&monitors)
	type monRow struct {
		Name   string  `json:"name"`
		Type   string  `json:"type"`
		Target string  `json:"target"`
		Status string  `json:"status"` // up / down / paused / unknown
		RespMs int     `json:"resp_ms"`
		Uptime float64 `json:"uptime24h"`
	}
	mons := make([]monRow, 0, len(monitors))
	up, down, paused := 0, 0, 0
	// 24h uptime (same calculation as the list page)
	var uptimeRows []struct {
		MonitorID uint
		Uptime    float64
	}
	model.DB.Raw(`SELECT monitor_id, AVG(CASE WHEN status = 'up' THEN 100.0 ELSE 0 END) AS uptime
		FROM monitor_samples WHERE created_at > ? AND status IN ('up','down') GROUP BY monitor_id`,
		now.Add(-24*time.Hour)).Scan(&uptimeRows)
	uptimes := map[uint]float64{}
	for _, u := range uptimeRows {
		uptimes[u.MonitorID] = math.Round(u.Uptime*10) / 10
	}
	for _, m := range monitors {
		st := "unknown"
		if !m.Enabled {
			st = "paused"
			paused++
		} else if m.LastStatus == "up" {
			st = "up"
			up++
		} else if m.LastStatus == "down" {
			st = "down"
			down++
		}
		mons = append(mons, monRow{Name: m.Name, Type: m.Type, Target: m.Target, Status: st, RespMs: m.LastRespMs, Uptime: uptimes[m.ID]})
	}

	// Alert events: today's count + latest 20 from the last 7 days
	var todayAlerts int64
	model.DB.Model(&model.AlertEvent{}).Where("fired_at >= ?", todayStart).Count(&todayAlerts)
	var recent []model.AlertEvent
	model.DB.Where("fired_at >= ?", now.AddDate(0, 0, -7)).Order("fired_at DESC").Limit(20).Find(&recent)

	// Tasks today
	var tasksToday int64
	model.DB.Model(&model.Task{}).Where("created_at >= ?", todayStart).Count(&tasksToday)

	// Host average CPU/memory trend (last 24h in hourly buckets, Grafana-style timeline)
	type trendRow struct {
		Bucket time.Time `json:"t"`
		CPU    float64   `json:"cpu"`
		Mem    float64   `json:"mem"`
	}
	var trend []trendRow
	model.DB.Raw(`SELECT date_trunc('hour', collected_at) AS bucket,
			AVG(cpu_percent) AS cpu, AVG(mem_percent) AS mem
		FROM host_metrics WHERE collected_at > ?
		GROUP BY bucket ORDER BY bucket`, now.Add(-24*time.Hour)).Scan(&trend)

	// Probe average response trend (last 24h in hourly buckets, successful samples only)
	type respRow struct {
		Bucket time.Time `json:"t"`
		Ms     float64   `json:"ms"`
	}
	var respTrend []respRow
	model.DB.Raw(`SELECT date_trunc('hour', created_at) AS bucket, AVG(resp_ms) AS ms
		FROM monitor_samples WHERE created_at > ? AND status = 'up'
		GROUP BY bucket ORDER BY bucket`, now.Add(-24*time.Hour)).Scan(&respTrend)

	// Daily alert counts for the last 14 days (missing days filled with 0)
	type dayRow struct {
		Day string `json:"d"`
		N   int64  `json:"n"`
	}
	var alertDays []dayRow
	model.DB.Raw(`SELECT to_char(date_trunc('day', fired_at), 'YYYY-MM-DD') AS d, COUNT(*) AS n
		FROM alert_events WHERE fired_at > ?
		GROUP BY d ORDER BY d`, now.AddDate(0, 0, -13)).Scan(&alertDays)
	dayMap := map[string]int64{}
	for _, r := range alertDays {
		dayMap[r.Day] = r.N
	}
	alertsDaily := make([]dayRow, 0, 14)
	for i := 13; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		alertsDaily = append(alertsDaily, dayRow{Day: d, N: dayMap[d]})
	}

	c.JSON(http.StatusOK, gin.H{
		"generated_at": now,
		"maintenance":  service.InMaintenanceWindow(now),
		"hosts": gin.H{
			"total": len(hosts), "online": onlineHosts,
			"groups": groupsOut,
		},
		"monitors": gin.H{
			"total": len(monitors), "up": up, "down": down, "paused": paused,
			"rows": mons,
		},
		"alerts":       gin.H{"today": todayAlerts, "recent": recent},
		"trend":        trend,
		"resp_trend":   respTrend,
		"alerts_daily": alertsDaily,
		"tasks_today":  tasksToday,
	})
}

func containsUint(list []uint, v uint) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// SecLogs POST /monitors/sec/logs - security-department log query over the
// whitelisted security streams only (no free SQL). Body:
// {stream, start_ms, end_ms, from, size, host, event_id, keyword}
func SecLogs(c *gin.Context) {
	var req struct {
		Stream  string `json:"stream"`
		StartMs int64  `json:"start_ms"`
		EndMs   int64  `json:"end_ms"`
		From    int    `json:"from"`
		Size    int    `json:"size"`
		Host    string `json:"host"`
		EventID int    `json:"event_id"`
		Keyword string `json:"keyword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	// stream whitelist: security-relevant streams only
	cols := map[string]string{
		"windows_events": "_timestamp, host, log_name, level, event_id, provider, event_time, message",
		"linux_events":   "_timestamp, host, kind, unit, level, message, event_time",
		"db_audit":       "_timestamp, username, action, resource, ip, status, detail",
		"alert_events":   "_timestamp, kind, level, target, message",
	}
	collist, ok := cols[req.Stream]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持安全相关日志流（windows_events / linux_events / db_audit / alert_events）"})
		return
	}
	if req.EndMs == 0 {
		req.EndMs = time.Now().UnixMilli()
	}
	if req.StartMs == 0 {
		req.StartMs = req.EndMs - 24*3600*1000
	}
	if req.From < 0 {
		req.From = 0
	}
	if req.Size <= 0 || req.Size > 500 {
		req.Size = 100
	}

	q := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	where := " WHERE 1=1"
	switch req.Stream {
	case "windows_events":
		if req.Host != "" {
			where += fmt.Sprintf(" AND host = '%s'", q(req.Host))
		}
		if req.EventID > 0 {
			where += fmt.Sprintf(" AND event_id = %d", req.EventID)
		}
	case "linux_events":
		if req.Host != "" {
			where += fmt.Sprintf(" AND host = '%s'", q(req.Host))
		}
	case "db_audit":
		if req.Host != "" {
			where += fmt.Sprintf(" AND ip = '%s'", q(req.Host))
		}
	case "alert_events":
		if req.Host != "" {
			where += fmt.Sprintf(" AND target = '%s'", q(req.Host))
		}
	}
	if req.Keyword != "" {
		where += fmt.Sprintf(" AND message LIKE '%%%s%%'", q(req.Keyword))
	}
	sql := fmt.Sprintf("SELECT %s FROM \"%s\"%s ORDER BY _timestamp DESC", collist, req.Stream, where)

	out, err := service.OOSearch(sql, req.StartMs, req.EndMs, req.From, req.Size)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
