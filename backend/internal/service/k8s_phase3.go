// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// K8S 三期：StatefulSet 伸缩 / YAML 更新 / Pod 日志实时跟随

// ScaleStatefulSet 调整 StatefulSet 副本数（scale 子资源）
func (k *K8sAPI) ScaleStatefulSet(namespace, name string, replicas int) error {
	body, _ := json.Marshal(map[string]any{
		"apiVersion": "apps/v1", "kind": "Scale",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"replicas": replicas},
	})
	return k.do("PATCH", "/apis/apps/v1/namespaces/"+namespace+"/statefulsets/"+name+"/scale", body, nil)
}

// UpdateResourceYAML 用编辑后的 YAML 替换资源（整对象 PUT）
func (k *K8sAPI) UpdateResourceYAML(kind, namespace, name, yamlText string) error {
	tpl, ok := k8sYAMLPaths[kind]
	if !ok {
		return fmt.Errorf("不支持的资源类型: %s", kind)
	}
	var path string
	if kind == "node" || kind == "pv" || kind == "storageclass" {
		path = fmt.Sprintf(tpl, name)
	} else {
		if namespace == "" {
			return fmt.Errorf("namespace 必填")
		}
		path = fmt.Sprintf(tpl, namespace, name)
	}
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &obj); err != nil {
		return fmt.Errorf("YAML 解析失败: %w", err)
	}
	if obj == nil {
		return fmt.Errorf("YAML 内容为空")
	}
	// 元数据一致性校验，防止把 A 资源的内容提交到 B
	if md, ok := obj["metadata"].(map[string]any); ok {
		if n, _ := md["name"].(string); n != "" && n != name {
			return fmt.Errorf("YAML 中 metadata.name(%s) 与目标资源(%s)不一致", n, name)
		}
		if ns, _ := md["namespace"].(string); namespace != "" && ns != "" && ns != namespace {
			return fmt.Errorf("YAML 中 metadata.namespace(%s) 与目标命名空间(%s)不一致", ns, namespace)
		}
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return k.do("PUT", path, body, nil)
}

// FollowPodLog 打开 follow=true 的日志流，返回响应体（调用方负责 close 与 cancel）
func (k *K8sAPI) FollowPodLog(ctx context.Context, namespace, name, container string, tail int) (io.ReadCloser, error) {
	path := "/api/v1/namespaces/" + namespace + "/pods/" + name + "/log?follow=true"
	if container != "" {
		path += "&container=" + container
	}
	if tail > 0 {
		path += "&tailLines=" + strconv.Itoa(tail)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", k.Server+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := k.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, fmt.Errorf("K8S API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return resp.Body, nil
}

// K8sPodUsage 单个 Pod：requests/limits 申请量 + 实际用量
type K8sPodUsage struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	CPUReqM   int64  `json:"cpu_req_m"`  // requests 合计（毫核）
	CPULimM   int64  `json:"cpu_lim_m"`  // limits 合计（毫核）
	MemReqMi  int64  `json:"mem_req_mi"` // requests 合计（Mi）
	MemLimMi  int64  `json:"mem_lim_mi"` // limits 合计（Mi）
	CPUM      int64  `json:"cpu_m"`      // 实际用量（毫核）
	MemMi     int64  `json:"mem_mi"`     // 实际用量（Mi）
}

// K8sClusterUsage 集群资源概况：容量/实际用量（来自 metrics-server）+ 全部 Pod 申请量与用量
type K8sClusterUsage struct {
	CPUCapacityM  int64          `json:"cpu_capacity_m"`  // 可分配 CPU 总量（毫核）
	CPUUsedM      int64          `json:"cpu_used_m"`      // 节点实际 CPU 用量合计（毫核）
	MemCapacityMi int64          `json:"mem_capacity_mi"` // 可分配内存总量（Mi）
	MemUsedMi     int64          `json:"mem_used_mi"`     // 节点实际内存用量合计（Mi）
	PodReqCPUM    int64          `json:"pod_req_cpu_m"`   // 全部 Pod requests CPU 合计
	PodReqMemMi   int64          `json:"pod_req_mem_mi"`  // 全部 Pod requests 内存合计
	Pods          []K8sPodUsage  `json:"pods"`
}

// ClusterUsage 并发拉取节点容量、节点用量与 Pod 用量（metrics-server 缺失时容量仍有值，用量为 0）
func (k *K8sAPI) ClusterUsage() (*K8sClusterUsage, error) {
	u := &K8sClusterUsage{Pods: []K8sPodUsage{}}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	run := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem; recover() }()
			f()
		}()
	}
	run(func() {
		var list struct {
			Items []struct {
				Status struct {
					Allocatable map[string]string `json:"allocatable"`
				} `json:"status"`
			} `json:"items"`
		}
		if err := k.do("GET", "/api/v1/nodes", nil, &list); err == nil {
			for _, it := range list.Items {
				u.CPUCapacityM += k8sQuantityCPU(it.Status.Allocatable["cpu"])
				u.MemCapacityMi += k8sQuantityMem(it.Status.Allocatable["memory"]) >> 20
			}
		}
	})
	run(func() {
		if ms, err := k.NodeMetrics(); err == nil {
			for _, m := range ms {
				u.CPUUsedM += m.CPUM
				u.MemUsedMi += m.MemMi
			}
		}
	})
	run(func() {
		// 全部 Pod 的 requests/limits 申请量
		var list struct {
			Items []struct {
				Metadata struct {
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"metadata"`
				Status struct {
					Phase string `json:"phase"`
				} `json:"status"`
				Spec struct {
					Containers []struct {
						Resources struct {
							Requests map[string]string `json:"requests"`
							Limits   map[string]string `json:"limits"`
						} `json:"resources"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"items"`
		}
		if err := k.do("GET", "/api/v1/pods", nil, &list); err == nil {
			for _, it := range list.Items {
				pu := K8sPodUsage{Namespace: it.Metadata.Namespace, Name: it.Metadata.Name, Phase: it.Status.Phase}
				for _, ct := range it.Spec.Containers {
					pu.CPUReqM += k8sQuantityCPU(ct.Resources.Requests["cpu"])
					pu.CPULimM += k8sQuantityCPU(ct.Resources.Limits["cpu"])
					pu.MemReqMi += k8sQuantityMem(ct.Resources.Requests["memory"]) >> 20
					pu.MemLimMi += k8sQuantityMem(ct.Resources.Limits["memory"]) >> 20
				}
				u.Pods = append(u.Pods, pu)
				u.PodReqCPUM += pu.CPUReqM
				u.PodReqMemMi += pu.MemReqMi
			}
		}
	})
	wg.Wait()

	// 实际用量合并进 Pod 列表（按 namespace/name 匹配；须在 Pods 拉取完成后串行执行）
	if ps, err := k.PodMetrics(""); err == nil {
		idx := make(map[string]int, len(u.Pods))
		for i := range u.Pods {
			idx[u.Pods[i].Namespace+"/"+u.Pods[i].Name] = i
		}
		for _, m := range ps {
			if i, ok := idx[m.Namespace+"/"+m.Name]; ok {
				u.Pods[i].CPUM = m.CPUM
				u.Pods[i].MemMi = m.MemMi
			}
		}
	}
	return u, nil
}
