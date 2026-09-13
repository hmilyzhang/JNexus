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

// K8S capacity planning: historical trends (hour/day buckets) + linear regression forecast

type capacityPoint struct {
	Bucket      time.Time `json:"t"`
	CPUUsedM    float64   `json:"cpu_used_m"`
	MemUsedMi   float64   `json:"mem_used_mi"`
	PodReqCPUM  float64   `json:"pod_req_cpu_m"`
	PodReqMemMi float64   `json:"pod_req_mem_mi"`
}

type capacityForecast struct {
	SlopeCPUMPerDay  float64  `json:"cpu_slope_m_per_day"`
	SlopeMemMiPerDay float64  `json:"mem_slope_mi_per_day"`
	CPUDaysLeft      *float64 `json:"cpu_days_left"`
	MemDaysLeft      *float64 `json:"mem_days_left"`
	CPUCapacityM     int64    `json:"cpu_capacity_m"`
	MemCapacityMi    int64    `json:"mem_capacity_mi"`
	CPUCurrentM      float64  `json:"cpu_current_m"`
	MemCurrentMi     float64  `json:"mem_current_mi"`
}

// K8sCapacityHistory GET /:id/capacity/history?hours=6|24|168|720|4320|8760
// ≤48h raw samples; 7d/30d hourly buckets; 180d/1y daily buckets (time ranges aligned with the monitoring center); with linear regression forecast
func K8sCapacityHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if _, _, _, ok := k8sClusterAccess(c, id, "viewer"); !ok {
		return
	}
	hours := capacityHours(c)
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var points []capacityPoint
	if hours <= 48 {
		model.DB.Raw(`SELECT collected_at AS bucket, cpu_used_m, mem_used_mi, pod_req_cpu_m, pod_req_mem_mi
			FROM k8s_capacity_samples WHERE cluster_id = ? AND collected_at > ?
			ORDER BY collected_at`, id, since).Scan(&points)
	} else {
		trunc := "'hour'"
		if hours > 720 {
			trunc = "'day'"
		}
		model.DB.Raw(`SELECT date_trunc(`+trunc+`, collected_at) AS bucket,
				AVG(cpu_used_m) AS cpu_used_m, AVG(mem_used_mi) AS mem_used_mi,
				AVG(pod_req_cpu_m) AS pod_req_cpu_m, AVG(pod_req_mem_mi) AS pod_req_mem_mi
			FROM k8s_capacity_samples WHERE cluster_id = ? AND collected_at > ?
			GROUP BY bucket ORDER BY bucket`, id, since).Scan(&points)
	}
	if points == nil {
		points = []capacityPoint{}
	}

	// Capacity from the latest sample (changes as nodes scale up/down)
	var latest model.K8sCapacitySample
	model.DB.Where("cluster_id = ?", id).Order("collected_at DESC").First(&latest)

	fc := capacityForecast{
		CPUCapacityM:  latest.CPUCapacityM,
		MemCapacityMi: latest.MemCapacityMi,
		CPUCurrentM:   float64(latest.CPUUsedM),
		MemCurrentMi:  float64(latest.MemUsedMi),
	}
	if len(points) >= 2 {
		slopeCPU, slopeMem := linearSlopePerDay(points)
		fc.SlopeCPUMPerDay = slopeCPU
		fc.SlopeMemMiPerDay = slopeMem
		fc.CPUDaysLeft = daysUntilFull(fc.CPUCurrentM, float64(fc.CPUCapacityM), slopeCPU)
		fc.MemDaysLeft = daysUntilFull(fc.MemCurrentMi, float64(fc.MemCapacityMi), slopeMem)
	}

	// Whether there is enough data for trend analysis (sampled every 15 min, at least 8 points in 2 hours)
	degraded := len(points) < 2
	c.JSON(http.StatusOK, gin.H{
		"hours":    hours,
		"points":   points,
		"forecast": fc,
		"degraded": degraded, // true = insufficient samples or missing metrics-server
	})
}

// capacityHours parses the time range parameter (aligned with the monitoring center: 6h/24h/7d/30d/180d/1y)
func capacityHours(c *gin.Context) int {
	hours := 720
	if h, e := strconv.Atoi(c.Query("hours")); e == nil {
		switch h {
		case 6, 24, 168, 720, 4320, 8760:
			hours = h
		}
	}
	return hours
}

// linearSlopePerDay computes the least-squares slope (units per day)
func linearSlopePerDay(points []capacityPoint) (float64, float64) {
	if len(points) < 2 {
		return 0, 0
	}
	t0 := points[0].Bucket
	var sx, syC, syM, sxx float64
	n := float64(len(points))
	for i, p := range points {
		x := p.Bucket.Sub(t0).Hours() / 24
		sx += x
		sxx += x * x
		syC += p.CPUUsedM
		syM += p.MemUsedMi
		_ = i
	}
	den := n*sxx - sx*sx
	if math.Abs(den) < 1e-9 {
		return 0, 0
	}
	var sxyC, sxyM float64
	for i, p := range points {
		x := p.Bucket.Sub(t0).Hours() / 24
		sxyC += x * p.CPUUsedM
		sxyM += x * p.MemUsedMi
		_ = i
	}
	return (n*sxyC - sx*syC) / den, (n*sxyM - sx*syM) / den
}

// daysUntilFull estimates days until full capacity at the current growth rate; returns nil/0 if not growing or already over capacity
func daysUntilFull(current, capacity float64, slopePerDay float64) *float64 {
	if slopePerDay <= 0 || capacity <= 0 {
		return nil
	}
	remaining := capacity - current
	if remaining <= 0 {
		z := 0.0
		return &z
	}
	d := remaining / slopePerDay
	if d > 3650 {
		d = 3650 // cap at 10 years to avoid exaggerated "distant future" numbers
	}
	return &d
}
