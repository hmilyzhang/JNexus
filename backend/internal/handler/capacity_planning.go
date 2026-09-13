// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
)

// K8S per-Pod capacity trends + host capacity planning (percentage-based)

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

// K8sPodCapacity GET /:id/capacity/pods?hours=6|24|168|720|4320|8760
// Per-Pod trends (only pods covered by Top10 sampling); ≤48h raw samples / 7d·30d hourly buckets / 180d·1y daily buckets
func K8sPodCapacity(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if _, _, _, ok := k8sClusterAccess(c, id, "viewer"); !ok {
		return
	}
	hours := capacityHours(c)
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var rows []struct {
		Namespace string    `json:"namespace"`
		Pod       string    `json:"pod"`
		Bucket    time.Time `json:"bucket"`
		CPUM      float64   `json:"cpu_m"`
		MemMi     float64   `json:"mem_mi"`
	}
	if hours <= 48 {
		model.DB.Raw(`SELECT namespace, pod, collected_at AS bucket, cpu_m, mem_mi
			FROM k8s_pod_samples WHERE cluster_id = ? AND collected_at > ?
			ORDER BY namespace, pod, collected_at`, id, since).Scan(&rows)
	} else {
		trunc := "'hour'"
		if hours > 720 {
			trunc = "'day'"
		}
		model.DB.Raw(`SELECT namespace, pod, date_trunc(`+trunc+`, collected_at) AS bucket,
				AVG(cpu_m) AS cpu_m, AVG(mem_mi) AS mem_mi
			FROM k8s_pod_samples WHERE cluster_id = ? AND collected_at > ?
			GROUP BY namespace, pod, bucket ORDER BY namespace, pod, bucket`, id, since).Scan(&rows)
	}

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
	c.JSON(http.StatusOK, gin.H{"hours": hours, "pods": out})
}

// podSlopePerDay least-squares slope per day
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

// ---- Host capacity planning ----

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
// 30d uses raw hourly buckets; longer ranges use daily buckets from the hourly table; includes days-to-90% forecast
func HostCapacityHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	// hours: 6/24/168/720 use raw hourly buckets; 4320/8760 use daily buckets from the archive table (aligned with monitoring 6h/24h/7d/30d/180d/1y)
	hours := 720
	if h, e := strconv.Atoi(c.Query("hours")); e == nil {
		switch h {
		case 6, 24, 168, 720, 4320, 8760:
			hours = h
		}
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	var points []hostCapacityPoint

	if hours <= 720 {
		model.DB.Raw(`SELECT date_trunc('hour', collected_at) AS bucket,
				AVG(cpu_percent) AS cpu_percent, AVG(mem_percent) AS mem_percent, AVG(disk_percent) AS disk_percent
			FROM host_metrics WHERE host_id = ? AND collected_at > ?
			GROUP BY bucket ORDER BY bucket`, id, since).Scan(&points)
	} else {
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		model.DB.Raw(`SELECT date_trunc('day', t.at) AS bucket,
				AVG(t.cpu_percent) AS cpu_percent, AVG(t.mem_percent) AS mem_percent, AVG(t.disk_percent) AS disk_percent
			FROM (
				SELECT collected_at AS at, cpu_percent, mem_percent, disk_percent
					FROM host_metrics WHERE host_id = ? AND collected_at > ?
				UNION ALL
				SELECT bucket AS at, cpu_percent, mem_percent, disk_percent
					FROM host_metric_hourlies WHERE host_id = ? AND bucket > ? AND bucket <= ?
			) t GROUP BY bucket ORDER BY bucket`, id, cutoff, id, cutoff, since).Scan(&points)
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
	c.JSON(http.StatusOK, gin.H{"hours": hours, "points": points, "forecast": fc, "degraded": degraded})
}

// daysToThreshold days until a percentage metric reaches the threshold; returns nil for non-positive slope
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
