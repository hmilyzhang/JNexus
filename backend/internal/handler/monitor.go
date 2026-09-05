// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/service"
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
	ChannelIDs     []uint `json:"channel_ids"` // 告警通知通道绑定
}

// saveMonitorBindings 重写监控项的通知通道绑定
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

// ListMonitors 监控列表：最新状态 + 24h 可用率 + 最近心跳（最多 50 条）
func ListMonitors(c *gin.Context) {
	var monitors []model.Monitor
	model.DB.Order("id").Find(&monitors)

	since24 := time.Now().Add(-24 * time.Hour)
	type uptimeRow struct {
		MonitorID uint
		Uptime    float64
	}
	var uptimes []uptimeRow
	model.DB.Raw(`SELECT monitor_id, AVG(CASE WHEN status = 'up' THEN 100.0 ELSE 0 END) AS uptime
		FROM monitor_samples WHERE created_at > ? GROUP BY monitor_id`, since24).Scan(&uptimes)
	uptimeMap := map[uint]float64{}
	for _, u := range uptimes {
		uptimeMap[u.MonitorID] = math.Round(u.Uptime*10) / 10
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
			"monitor": m, "uptime24h": uptimeMap[m.ID], "recent": recentMap[m.ID],
			"channel_ids": chMap[m.ID],
		})
	}
	c.JSON(http.StatusOK, out)
}

// CreateMonitor 新建监控项
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
	if err := model.DB.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	saveMonitorBindings(m.ID, req.ChannelIDs)
	c.JSON(http.StatusOK, m)
}

// UpdateMonitor 编辑监控项
func UpdateMonitor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.Monitor
	if err := model.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控项不存在"})
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
	m.NextRunAt = nil // 立即重新调度
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

// DeleteMonitor 删除监控项（样本级联删除）
func DeleteMonitor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Where("monitor_id = ?", id).Delete(&model.MonitorSample{})
	model.DB.Delete(&model.Monitor{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestMonitor 立即执行一次检查（结果同时落库，便于看到即时状态）
func TestMonitor(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.Monitor
	if err := model.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控项不存在"})
		return
	}
	up, ms, errMsg := service.RunMonitorOnce(&m)
	c.JSON(http.StatusOK, gin.H{"up": up, "resp_ms": ms, "error": errMsg})
}

// MonitorHistory 单个监控项的采样历史（?hours=24）
func MonitorHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	hours := 24
	if h, e := strconv.Atoi(c.Query("hours")); e == nil && h > 0 && h <= 24*7 {
		hours = h
	}
	var samples []model.MonitorSample
	model.DB.Where("monitor_id = ? AND created_at > ?", id, time.Now().Add(-time.Duration(hours)*time.Hour)).
		Order("id").Limit(2000).Find(&samples)
	c.JSON(http.StatusOK, samples)
}

// HostMetricsList 全部主机最新资源（CPU/内存/磁盘）
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
	// 也列出尚无采样数据的主机（前端显示"暂无数据"）
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

// HostMetricHistory 单台主机资源历史（?hours=6，供趋势图）
func HostMetricHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	hours := 6
	if h, e := strconv.Atoi(c.Query("hours")); e == nil && h > 0 && h <= 24 {
		hours = h
	}
	var rows []model.HostMetric
	model.DB.Where("host_id = ? AND collected_at > ?", id, time.Now().Add(-time.Duration(hours)*time.Hour)).
		Order("collected_at").Limit(2000).Find(&rows)
	c.JSON(http.StatusOK, rows)
}
