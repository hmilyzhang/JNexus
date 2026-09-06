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

// K8S 容量规划：历史趋势（小时/天桶）+ 线性回归预测

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

// K8sCapacityHistory GET /:id/capacity/history?days=30|180|365
// 30d 按小时桶，180d/365d 按天桶；附最近 30 天线性回归预测
func K8sCapacityHistory(c *gin.Context) {
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
	var points []capacityPoint
	model.DB.Raw(`SELECT date_trunc(`+trunc+`, collected_at) AS bucket,
			AVG(cpu_used_m) AS cpu_used_m, AVG(mem_used_mi) AS mem_used_mi,
			AVG(pod_req_cpu_m) AS pod_req_cpu_m, AVG(pod_req_mem_mi) AS pod_req_mem_mi
		FROM k8s_capacity_samples WHERE cluster_id = ? AND collected_at > ?
		GROUP BY bucket ORDER BY bucket`, id, since).Scan(&points)
	if points == nil {
		points = []capacityPoint{}
	}

	// 容量取最近样本（节点扩缩容会变化）
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

	// 数据是否足以做趋势判断（采样 15 分钟一次，2 小时至少 8 点）
	degraded := len(points) < 2
	c.JSON(http.StatusOK, gin.H{
		"days":     days,
		"points":   points,
		"forecast": fc,
		"degraded": degraded, // true = 样本不足或 metrics-server 缺失
	})
}

// linearSlopePerDay 最小二乘斜率（单位/天）
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

// daysUntilFull 按当前增速估算到达容量的天数；不增长或已超容量返回 nil/0
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
		d = 3650 // 上限 10 年，避免"遥远未来"的夸张数字
	}
	return &d
}
