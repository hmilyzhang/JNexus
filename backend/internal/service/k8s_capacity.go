// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
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
		}(c)
	}
	wg.Wait()
}

// PruneK8sCapacitySamples 清理超过 400 天的容量样本（一年维度 + 余量）
func PruneK8sCapacitySamples() {
	defer func() { recover() }()
	model.DB.Where("collected_at < ?", time.Now().AddDate(0, 0, -400)).
		Delete(&model.K8sCapacitySample{})
}
