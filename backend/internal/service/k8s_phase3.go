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

// K8sClusterUsage 集群资源概况：容量/实际用量（来自 metrics-server）+ 全部 Pod 用量
type K8sClusterUsage struct {
	CPUCapacityM  int64           `json:"cpu_capacity_m"`  // 可分配 CPU 总量（毫核）
	CPUUsedM      int64           `json:"cpu_used_m"`      // 节点实际 CPU 用量合计（毫核）
	MemCapacityMi int64           `json:"mem_capacity_mi"` // 可分配内存总量（Mi）
	MemUsedMi     int64           `json:"mem_used_mi"`     // 节点实际内存用量合计（Mi）
	Pods          []K8sMetricInfo `json:"pods"`            // 全部 Pod 的实际用量
}

// ClusterUsage 并发拉取节点容量、节点用量与 Pod 用量（metrics-server 缺失时容量仍有值，用量为 0）
func (k *K8sAPI) ClusterUsage() (*K8sClusterUsage, error) {
	u := &K8sClusterUsage{Pods: []K8sMetricInfo{}}
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
		if ps, err := k.PodMetrics(""); err == nil {
			u.Pods = ps
		}
	})
	wg.Wait()
	return u, nil
}
