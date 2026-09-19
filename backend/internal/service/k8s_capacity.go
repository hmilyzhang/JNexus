// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"log"
	"sort"
	"sync"
	"time"

	"jnexus/internal/model"
)

// K8S capacity planning: periodically samples cluster capacity/usage to support month/half-year/year trends and forecasts

var (
	k8sUsageMu   sync.Mutex
	k8sLastUsage = map[uint]time.Time{}
)

// CollectK8sUsage samples capacity per cluster with throttling (every 15 minutes), hooked into StartMonitorLoop
func CollectK8sUsage() {
	defer func() { recover() }()
	var clusters []model.K8sCluster
	if err := model.DB.Where("enabled = ?", true).Find(&clusters).Error; err != nil {
		return
	}
	now := time.Now()
	var wg sync.WaitGroup
	for _, c := range clusters {
		k8sUsageMu.Lock()
		last, seen := k8sLastUsage[c.ID]
		k8sUsageMu.Unlock()
		if seen && now.Sub(last) < 15*time.Minute {
			continue
		}
		k8sUsageMu.Lock()
		k8sLastUsage[c.ID] = now
		k8sUsageMu.Unlock()
		wg.Add(1)
		go func(c model.K8sCluster) {
			defer wg.Done()
			defer func() { recover() }()
			api, err := K8sClientFor(&c)
			if err != nil {
				return
			}
			u, err := api.ClusterUsage()
			if err != nil {
				return
			}
			sample := model.K8sCapacitySample{
				ClusterID:     c.ID,
				CPUCapacityM:  u.CPUCapacityM,
				CPUUsedM:      u.CPUUsedM,
				MemCapacityMi: u.MemCapacityMi,
				MemUsedMi:     u.MemUsedMi,
				PodReqCPUM:    u.PodReqCPUM,
				PodReqMemMi:   u.PodReqMemMi,
				CollectedAt:   time.Now(),
			}
			model.DB.Create(&sample)
			if ooIntegrationEnabled("k8s_capacity") {
				ooPushAsync("k8s_capacity", map[string]any{
					"cluster":        func() string { var cl model.K8sCluster; model.DB.Select("name").First(&cl, c.ID); return cl.Name }(),
					"cpu_capacity_m": sample.CPUCapacityM, "cpu_used_m": sample.CPUUsedM,
					"mem_capacity_mi": sample.MemCapacityMi, "mem_used_mi": sample.MemUsedMi,
					"collected_at": sample.CollectedAt.Format(time.RFC3339),
				})
			}

			// Pod level: sample top 10 by CPU usage (pod-dimension trends for capacity planning)
			top := make([]K8sPodUsage, len(u.Pods))
			copy(top, u.Pods)
			sort.Slice(top, func(i, j int) bool { return top[i].CPUM > top[j].CPUM })
			if len(top) > 10 {
				top = top[:10]
			}
			now := time.Now()
			for _, pu := range top {
				if pu.CPUM <= 0 && pu.MemMi <= 0 {
					continue
				}
				model.DB.Create(&model.K8sPodSample{
					ClusterID: c.ID, Namespace: pu.Namespace, Pod: pu.Name,
					CPUM: pu.CPUM, MemMi: pu.MemMi, CollectedAt: now,
				})
			}
		}(c)
	}
	wg.Wait()
}

// PruneK8sCapacitySamples deletes capacity samples older than 400 days (one-year horizon + margin)
func PruneK8sCapacitySamples() {
	defer func() { recover() }()
	cut := time.Now().AddDate(0, 0, -400)
	model.DB.Where("collected_at < ?", cut).Delete(&model.K8sCapacitySample{})
	model.DB.Where("collected_at < ?", cut).Delete(&model.K8sPodSample{})
}

// ArchiveHostMetrics aggregates host metrics older than 30 days into HostMetricHourly by hour (idempotent upsert); raw rows are then deleted by PruneMonitorData
func ArchiveHostMetrics() {
	defer func() { recover() }()
	cut := time.Now().Add(-30 * 24 * time.Hour)
	if err := model.DB.Exec(`INSERT INTO host_metric_hourlies (host_id, bucket, cpu_percent, mem_percent, disk_percent)
		SELECT host_id, date_trunc('hour', collected_at), AVG(cpu_percent), AVG(mem_percent), AVG(disk_percent)
		FROM host_metrics WHERE collected_at < ?
		GROUP BY host_id, date_trunc('hour', collected_at)
		ON CONFLICT (host_id, bucket) DO UPDATE SET cpu_percent = EXCLUDED.cpu_percent,
			mem_percent = EXCLUDED.mem_percent, disk_percent = EXCLUDED.disk_percent`, cut).Error; err != nil {
		log.Printf("[archive] host metrics hourly failed: %v", err)
	}
}
