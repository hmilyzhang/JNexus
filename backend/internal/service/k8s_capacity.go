// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"sort"
	"sync"
	"time"

	"jnexus/internal/model"
)

// K8S 容量规划：周期采样集群容量/用量，支撑 月/半年/年 趋势与预测

var (
	k8sUsageMu   sync.Mutex
	k8sLastUsage = map[uint]time.Time{}
)

// CollectK8sUsage 按集群节流采样容量样本（15 分钟一次），挂在 StartMonitorLoop
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

			// Pod 级：按 CPU 用量 Top10 采样（容量规划 Pod 维度趋势）
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

// PruneK8sCapacitySamples 清理超过 400 天的容量样本（一年维度 + 余量）
func PruneK8sCapacitySamples() {
	defer func() { recover() }()
	cut := time.Now().AddDate(0, 0, -400)
	model.DB.Where("collected_at < ?", cut).Delete(&model.K8sCapacitySample{})
	model.DB.Where("collected_at < ?", cut).Delete(&model.K8sPodSample{})
}

// ArchiveHostMetrics 把 30 天前的主机指标按小时聚合进 HostMetricHourly（幂等覆盖），随后 raw 由 PruneMonitorData 删除
func ArchiveHostMetrics() {
	defer func() { recover() }()
	cut := time.Now().Add(-30 * 24 * time.Hour)
	model.DB.Exec(`INSERT INTO host_metric_hourlies (host_id, bucket, cpu_percent, mem_percent, disk_percent, created_at)
		SELECT host_id, date_trunc('hour', collected_at), AVG(cpu_percent), AVG(mem_percent), AVG(disk_percent), date_trunc('hour', collected_at)
		FROM host_metrics WHERE collected_at < ?
		GROUP BY host_id, date_trunc('hour', collected_at)
		ON CONFLICT (host_id, bucket) DO UPDATE SET cpu_percent = EXCLUDED.cpu_percent,
			mem_percent = EXCLUDED.mem_percent, disk_percent = EXCLUDED.disk_percent`, cut)
}
