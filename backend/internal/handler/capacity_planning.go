// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
)

// K8S Pod 级容量趋势 + 主机容量规划（百分比维度）

type podTrend struct {
	Namespace string          `json:"namespace"`
	Name      string          `json:"name"`
	Points    []podTrendPoint `json:"points"`
	SlopeCPU  float64         `json:"slope_cpu_m_per_day"`
	SlopeMem  float64         `json:"slope_mem_mi_per_day"`
}

type podTrendPoint struct {
	Bucket time.Time `json:"t"`
	CPUM   float64   `json:"cpu_m"`
	MemMi  float64   `json:"mem_mi"`
}

// K8sPodCapacity GET /:id/capacity/pods?days=30|180|365
// 逐 Pod 趋势（仅被 Top10 采样覆盖的 Pod），30d 小时桶 / 更长天桶
func K8sPodCapacity(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if _, _, _, ok := k8sClusterAccess(c, id, "viewer"); !ok {
		return
	}
	days := 30
	if d, e := strconv.Atoi(c.Query("days")); e == nil {
		switch d {
		case 180:
			days = 180
		case 365:
			days = 365
		}
	}
	since := time.Now().AddDate(0, 0, -days)
	trunc := "'hour'"
	if days > 60 {
		trunc = "'day'"
	}

	var rows []struct {
		Namespace string    `json:"namespace"`
		Pod       string    `json:"pod"`
		Bucket    time.Time `json:"bucket"`
		CPUM      float64   `json:"cpu_m"`
		MemMi     float64   `json:"mem_mi"`
	}
	model.DB.Raw(`SELECT namespace, pod, date_trunc(`+trunc+`, collected_at) AS bucket,
			AVG(cpu_m) AS cpu_m, AVG(mem_mi) AS mem_mi
		FROM k8s_pod_samples WHERE cluster_id = ? AND collected_at > ?
		GROUP BY namespace, pod, bucket ORDER BY namespace, pod, bucket`, id, since).Scan(&rows)

	byKey := map[string]*podTrend{}
	order := []string{}
	for _, r := range rows {
		key := r.Namespace + "/" + r.Pod
		t, ok := byKey[key]
		if !ok {
			t = &podTrend{Namespace: r.Namespace, Name: r.Pod}
			byKey[key] = t
			order = append(order, key)
		}
		t.Points = append(t.Points, podTrendPoint{Bucket: r.Bucket, CPUM: r.CPUM, MemMi: r.MemMi})
	}
	out := make([]podTrend, 0, len(order))
	for _, key := range order {
		t := byKey[key]
		if len(t.Points) >= 2 {
			t.SlopeCPU, t.SlopeMem = podSlopePerDay(t.Points)
		}
		out = append(out, *t)
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "pods": out})
}

// podSlopePerDay 每日均值最小二乘斜率
func podSlopePerDay(points []podTrendPoint) (float64, float64) {
	if len(points) < 2 {
		return 0, 0
	}
	t0 := points[0].Bucket
	var sx, sxx, syC, syM, sxyC, sxyM float64
	n := float64(len(points))
	for _, p := range points {
		x := p.Bucket.Sub(t0).Hours() / 24
		sx += x
		sxx += x * x
		syC += p.CPUM
		syM += p.MemMi
		sxyC += x * p.CPUM
		sxyM += x * p.MemMi
	}
	den := n*sxx - sx*sx
	if math.Abs(den) < 1e-9 {
		return 0, 0
	}
	return (n*sxyC - sx*syC) / den, (n*sxyM - sx*syM) / den
}

// ---- 主机容量规划 ----

type hostCapacityPoint struct {
	Bucket      time.Time `json:"t"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"`
}

type hostForecast struct {
	SlopeCPUPct  float64  `json:"cpu_slope_pct_per_day"`
	SlopeMemPct  float64  `json:"mem_slope_pct_per_day"`
	SlopeDiskPct float64  `json:"disk_slope_pct_per_day"`
	CPUDaysTo90  *float64 `json:"cpu_days_to_90"`
	MemDaysTo90  *float64 `json:"mem_days_to_90"`
	DiskDaysTo90 *float64 `json:"disk_days_to_90"`
	CPUCurrent   float64  `json:"cpu_current"`
	MemCurrent   float64  `json:"mem_current"`
	DiskCurrent  float64  `json:"disk_current"`
}

// HostCapacityHistory GET /api/monitoring/hosts/:id/capacity?days=30|180|365
// 30d 走 raw 小时桶；更长走 hourly 表天桶；附达到 90% 水位预测
func HostCapacityHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	days := 30
	if d, e := strconv.Atoi(c.Query("days")); e == nil {
		switch d {
		case 180:
			days = 180
		case 365:
			days = 365
		}
	}
	since := time.Now().AddDate(0, 0, -days)
	var points []hostCapacityPoint

	if days <= 30 {
		model.DB.Raw(`SELECT date_trunc('hour', collected_at) AS bucket,
				AVG(cpu_percent) AS cpu_percent, AVG(mem_percent) AS mem_percent, AVG(disk_percent) AS disk_percent
			FROM host_metrics WHERE host_id = ? AND collected_at > ?
			GROUP BY bucket ORDER BY bucket`, id, since).Scan(&points)
	} else {
		model.DB.Raw(`SELECT date_trunc('day', bucket) AS bucket,
				AVG(cpu_percent) AS cpu_percent, AVG(mem_percent) AS mem_percent, AVG(disk_percent) AS disk_percent
			FROM host_metric_hourlies WHERE host_id = ? AND bucket > ?
			GROUP BY date_trunc('day', bucket) ORDER BY bucket`, id, since).Scan(&points)
	}
	if points == nil {
		points = []hostCapacityPoint{}
	}

	fc := hostForecast{}
	if len(points) >= 2 {
		t0 := points[0].Bucket
		var sx, sxx, sC, sM, sD, sxyC, sxyM, sxyD float64
		n := float64(len(points))
		for _, p := range points {
			x := p.Bucket.Sub(t0).Hours() / 24
			sx += x
			sxx += x * x
			sC += p.CPUPercent
			sM += p.MemPercent
			sD += p.DiskPercent
			sxyC += x * p.CPUPercent
			sxyM += x * p.MemPercent
			sxyD += x * p.DiskPercent
		}
		den := n*sxx - sx*sx
		if math.Abs(den) > 1e-9 {
			fc.SlopeCPUPct = (n*sxyC - sx*sC) / den
			fc.SlopeMemPct = (n*sxyM - sx*sM) / den
			fc.SlopeDiskPct = (n*sxyD - sx*sD) / den
		}
		last := points[len(points)-1]
		fc.CPUCurrent, fc.MemCurrent, fc.DiskCurrent = last.CPUPercent, last.MemPercent, last.DiskPercent
		fc.CPUDaysTo90 = daysToThreshold(last.CPUPercent, 90, fc.SlopeCPUPct)
		fc.MemDaysTo90 = daysToThreshold(last.MemPercent, 90, fc.SlopeMemPct)
		fc.DiskDaysTo90 = daysToThreshold(last.DiskPercent, 90, fc.SlopeDiskPct)
	}
	degraded := len(points) < 2
	c.JSON(http.StatusOK, gin.H{"days": days, "points": points, "forecast": fc, "degraded": degraded})
}

// daysToThreshold 百分比指标到达阈值的天数；斜率非正返回 nil
func daysToThreshold(current, threshold, slopePerDay float64) *float64 {
	if slopePerDay <= 0 {
		return nil
	}
	remaining := threshold - current
	if remaining <= 0 {
		z := 0.0
		return &z
	}
	d := remaining / slopePerDay
	if d > 3650 {
		d = 3650
	}
	return &d
}
