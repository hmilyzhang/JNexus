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

// Monthly ops report: asset overview / alert stats / host & K8S capacity / config changes, for management reporting

type dayRow struct {
	HostID uint
	Bucket time.Time
	CPU    float64
	Mem    float64
	Disk   float64
}

type repAlertRow struct {
	Kind      string     `json:"kind"`
	Level     string     `json:"level"`
	Target    string     `json:"target"`
	Message   string     `json:"message"`
	FiredAt   time.Time  `json:"fired_at"`
	Recovered *time.Time `json:"recovered_at"`
	Duration  *int       `json:"duration_sec"`
}

type repHostRow struct {
	HostID    uint     `json:"host_id"`
	Name      string   `json:"name"`
	IP        string   `json:"ip"`
	AvgCPU    float64  `json:"avg_cpu"`
	PeakCPU   float64  `json:"peak_cpu"`
	AvgMem    float64  `json:"avg_mem"`
	PeakMem   float64  `json:"peak_mem"`
	Disk      float64  `json:"disk"`
	SlopeCPU  float64  `json:"slope_cpu_pct_day"`
	DaysTo90C *float64 `json:"cpu_days_to_90"`
	DaysTo90M *float64 `json:"mem_days_to_90"`
	Risk      string   `json:"risk"` // red / yellow / green
}

type repK8sRow struct {
	Cluster     string      `json:"cluster"`
	Nodes       int         `json:"nodes"`
	CPUCapM     int64       `json:"cpu_capacity_m"`
	CPUUsedAvg  float64     `json:"cpu_used_avg_m"`
	CPUUsedEnd  float64     `json:"cpu_used_end_m"`
	MemCapMi    int64       `json:"mem_capacity_mi"`
	MemUsedAvg  float64     `json:"mem_used_avg_mi"`
	MemUsedEnd  float64     `json:"mem_used_end_mi"`
	SlopeCPU    float64     `json:"cpu_slope_m_day"`
	DaysToFullC *float64    `json:"cpu_days_to_full"`
	DaysToFullM *float64    `json:"mem_days_to_full"`
	TopPods     []repPodRow `json:"top_pods"`
}

type repPodRow struct {
	Namespace string  `json:"namespace"`
	Pod       string  `json:"pod"`
	AvgCPU    float64 `json:"avg_cpu_m"`
	AvgMem    float64 `json:"avg_mem_mi"`
}

// MonthlyReport GET /api/report/monthly?year=&month= (defaults to last month)
func MonthlyReport(c *gin.Context) {
	now := time.Now()
	year, month := now.Year(), int(now.Month())-1
	if month == 0 {
		year, month = year-1, 12
	}
	if y, e := strconv.Atoi(c.Query("year")); e == nil {
		year = y
	}
	if m, e := strconv.Atoi(c.Query("month")); e == nil && m >= 1 && m <= 12 {
		month = m
	}
	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 1, 0)

	// ---- Asset overview ----
	var hostsTotal, hostsOnline int64
	model.DB.Model(&model.Host{}).Count(&hostsTotal)
	model.DB.Model(&model.Host{}).Where("status = ?", "online").Count(&hostsOnline)
	var k8sTotal int64
	model.DB.Model(&model.K8sCluster{}).Where("enabled = ?", true).Count(&k8sTotal)
	var monTotal int64
	model.DB.Model(&model.Monitor{}).Count(&monTotal)

	// ---- Alert stats ----
	var events []repAlertRow
	model.DB.Table("alert_events").Where("fired_at >= ? AND fired_at < ?", from, to).
		Order("fired_at DESC").Limit(2000).Scan(&events)
	byLevel, byKind := map[string]int{}, map[string]int{}
	topMap := map[string]int{}
	for _, e := range events {
		byLevel[e.Level]++
		byKind[e.Kind]++
		topMap[e.Target]++
	}
	totalAlerts := len(events)
	topTargets := sortCountMap(topMap, 10)

	// ---- Host capacity (union of hourly archive + raw tables) ----
	var hostRows []repHostRow
	model.DB.Raw(`SELECT h.id AS host_id, h.name, h.ip,
			COALESCE(ROUND(AVG(t.cpu_percent)::numeric, 1), 0)::float8 AS avg_cpu,
			COALESCE(MAX(t.cpu_percent), 0)::float8 AS peak_cpu,
			COALESCE(ROUND(AVG(t.mem_percent)::numeric, 1), 0)::float8 AS avg_mem,
			COALESCE(MAX(t.mem_percent), 0)::float8 AS peak_mem,
			0::float8 AS disk
		FROM hosts h LEFT JOIN (
			SELECT host_id, cpu_percent, mem_percent, disk_percent FROM host_metric_hourlies
				WHERE bucket >= ? AND bucket < ?
			UNION ALL
			SELECT host_id, cpu_percent, mem_percent, disk_percent FROM host_metrics
				WHERE collected_at >= ? AND collected_at < ?
		) t ON t.host_id = h.id
		GROUP BY h.id, h.name, h.ip ORDER BY h.name`, from, to, from, to).Scan(&hostRows)

	// Daily series for the month (for slope calculation) + end-of-month disk level

	var days []dayRow
	model.DB.Raw(`SELECT host_id, date_trunc('day', collected_at) AS bucket,
			AVG(cpu_percent) AS cpu, AVG(mem_percent) AS mem, AVG(disk_percent) AS disk
		FROM host_metrics WHERE collected_at >= ? AND collected_at < ?
		GROUP BY host_id, bucket ORDER BY host_id, bucket`, from, to).Scan(&days)

	perHostDays := map[uint][]dayRow{}
	for _, d := range days {
		perHostDays[d.HostID] = append(perHostDays[d.HostID], d)
	}
	for i := range hostRows {
		h := &hostRows[i]
		series := perHostDays[h.HostID]
		if len(series) > 0 {
			h.Disk = series[len(series)-1].Disk
		}
		slope := slopeOf(series, "cpu")
		h.SlopeCPU = slope
		h.DaysTo90C = daysToPct(avgLast(series, "cpu"), 90, slope)
		h.DaysTo90M = daysToPct(avgLast(series, "mem"), 90, slopeOf(series, "mem"))
		h.Risk = riskOf(h.DaysTo90C, h.DaysTo90M, h.Disk)
	}

	// ---- K8S capacity ----
	var clusters []model.K8sCluster
	model.DB.Where("enabled = ?", true).Find(&clusters)
	k8sRows := make([]repK8sRow, 0, len(clusters))
	for _, cl := range clusters {
		row := repK8sRow{Cluster: cl.Name, TopPods: []repPodRow{}}
		var agg struct {
			AvgCPU  float64
			AvgMem  float64
			MaxCapC int64
			MaxCapM int64
		}
		model.DB.Raw(`SELECT COALESCE(AVG(cpu_used_m),0) AS avg_cpu, COALESCE(AVG(mem_used_mi),0) AS avg_mem,
				COALESCE(MAX(cpu_capacity_m),0) AS max_cap_c, COALESCE(MAX(mem_capacity_mi),0) AS max_cap_m
			FROM k8s_capacity_samples WHERE cluster_id = ? AND collected_at >= ? AND collected_at < ?`,
			cl.ID, from, to).Scan(&agg)
		row.CPUCapM, row.MemCapMi = agg.MaxCapC, agg.MaxCapM
		row.CPUUsedAvg, row.MemUsedAvg = agg.AvgCPU, agg.AvgMem
		var last model.K8sCapacitySample
		if err := model.DB.Where("cluster_id = ?", cl.ID).Order("collected_at DESC").First(&last).Error; err == nil {
			row.CPUUsedEnd = float64(last.CPUUsedM)
			row.MemUsedEnd = float64(last.MemUsedMi)
			row.CPUCapM, row.MemCapMi = last.CPUCapacityM, last.MemCapacityMi
		}
		// Slope from the in-month daily average series
		var series []struct {
			Bucket time.Time `json:"bucket"`
			CPU    float64   `json:"cpu"`
			Mem    float64   `json:"mem"`
		}
		model.DB.Raw(`SELECT date_trunc('day', collected_at) AS bucket, AVG(cpu_used_m) AS cpu, AVG(mem_used_mi) AS mem
			FROM k8s_capacity_samples WHERE cluster_id = ? AND collected_at >= ? AND collected_at < ?
			GROUP BY bucket ORDER BY bucket`, cl.ID, from, to).Scan(&series)
		fc := hostForecast{}
		if len(series) >= 2 {
			t0 := series[0].Bucket
			var sx, sxx, sC, sM, sxyC, sxyM float64
			n := float64(len(series))
			for _, p := range series {
				x := p.Bucket.Sub(t0).Hours() / 24
				sx += x
				sxx += x * x
				sC += p.CPU
				sM += p.Mem
				sxyC += x * p.CPU
				sxyM += x * p.Mem
			}
			den := n*sxx - sx*sx
			if math.Abs(den) > 1e-9 {
				fc.SlopeCPUPct = (n*sxyC - sx*sC) / den
				fc.SlopeMemPct = (n*sxyM - sx*sM) / den
			}
		}
		row.SlopeCPU = fc.SlopeCPUPct
		row.DaysToFullC = daysToCapacity(row.CPUUsedEnd, float64(row.CPUCapM), fc.SlopeCPUPct)
		row.DaysToFullM = daysToCapacity(row.MemUsedEnd, float64(row.MemCapMi), fc.SlopeMemPct)
		// Top 10 pods (by monthly average CPU)
		model.DB.Raw(`SELECT namespace, pod, AVG(cpu_m) AS avg_cpu_m, AVG(mem_mi) AS avg_mem_mi
			FROM k8s_pod_samples WHERE cluster_id = ? AND collected_at >= ? AND collected_at < ?
			GROUP BY namespace, pod ORDER BY avg_cpu_m DESC LIMIT 10`, cl.ID, from, to).Scan(&row.TopPods)
		// Node count
		var nn int64
		model.DB.Model(&model.K8sClusterMember{}).Where("cluster_id = ?", cl.ID).Count(&nn)
		row.Nodes = int(nn)
		k8sRows = append(k8sRows, row)
	}

	// ---- Config change summary ----
	var hostsAdded, hostsDeleted, credsAdded int64
	model.DB.Model(&model.AuditLog{}).Where("action = ? AND resource LIKE ? AND created_at >= ? AND created_at < ?", "POST", "%/hosts", from, to).Count(&hostsAdded)
	model.DB.Model(&model.AuditLog{}).Where("action = ? AND resource LIKE ? AND created_at >= ? AND created_at < ?", "DELETE", "%/hosts/%", from, to).Count(&hostsDeleted)
	model.DB.Model(&model.AuditLog{}).Where("action = ? AND resource LIKE ? AND created_at >= ? AND created_at < ?", "POST", "%/credentials%", from, to).Count(&credsAdded)

	c.JSON(http.StatusOK, gin.H{
		"period": gin.H{"year": year, "month": month, "from": from.Format("2006-01-02"), "to": to.AddDate(0, 0, -1).Format("2006-01-02")},
		"assets": gin.H{"hosts_total": hostsTotal, "hosts_online": hostsOnline, "k8s_clusters": k8sTotal, "monitors": monTotal},
		"alerts": gin.H{
			"total": totalAlerts, "by_level": byLevel, "by_kind": byKind,
			"top_targets": topTargets,
			"timeline":    filterP12(events),
			"details":     events,
		},
		"hosts":   hostRows,
		"k8s":     k8sRows,
		"changes": gin.H{"hosts_added": hostsAdded, "hosts_deleted": hostsDeleted, "credentials_added": credsAdded},
	})
}

func filterP12(events []repAlertRow) []repAlertRow {
	out := []repAlertRow{}
	for _, e := range events {
		if e.Level == "P1" || e.Level == "P2" {
			out = append(out, e)
		}
	}
	return out
}

func sortCountMap(m map[string]int, n int) []gin.H {
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(m))
	for k, v := range m {
		arr = append(arr, kv{k, v})
	}
	for i := 0; i < len(arr); i++ {
		for j := i + 1; j < len(arr); j++ {
			if arr[j].v > arr[i].v {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}
	out := []gin.H{}
	for i, x := range arr {
		if i >= n {
			break
		}
		out = append(out, gin.H{"target": x.k, "count": x.v})
	}
	return out
}

type dayVal struct {
	bucket time.Time
	cpu    float64
	mem    float64
	disk   float64
}

// slopeOf computes the least-squares slope of a daily percentage series (% per day)
func slopeOf(series []dayRow, metric string) float64 {
	if len(series) < 2 {
		return 0
	}
	t0 := series[0].Bucket
	var sx, sxx, sy, sxy float64
	n := float64(len(series))
	for _, p := range series {
		x := p.Bucket.Sub(t0).Hours() / 24
		v := map[bool]float64{true: p.CPU, false: p.Mem}[metric == "cpu"]
		sx += x
		sxx += x * x
		sy += v
		sxy += x * v
	}
	den := n*sxx - sx*sx
	if math.Abs(den) < 1e-9 {
		return 0
	}
	return (n*sxy - sx*sy) / den
}

func avgLast(series []dayRow, metric string) float64 {
	if len(series) == 0 {
		return 0
	}
	last := series[len(series)-1]
	return map[bool]float64{true: last.CPU, false: last.Mem}[metric == "cpu"]
}

func daysToPct(current, threshold, slopePctDay float64) *float64 {
	if slopePctDay <= 0 {
		return nil
	}
	remaining := threshold - current
	if remaining <= 0 {
		z := 0.0
		return &z
	}
	d := remaining / slopePctDay
	if d > 3650 {
		d = 3650
	}
	return &d
}

func daysToCapacity(current, capacity, slopePerDay float64) *float64 {
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
		d = 3650
	}
	return &d
}

func riskOf(dC, dM *float64, disk float64) string {
	days := func(d *float64) float64 {
		if d == nil {
			return 9999
		}
		return *d
	}
	minD := math.Min(days(dC), days(dM))
	if minD < 90 || disk >= 90 {
		return "red"
	}
	if minD < 180 || disk >= 80 {
		return "yellow"
	}
	return "green"
}
