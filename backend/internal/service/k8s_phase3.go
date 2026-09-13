// JNexus Ops Platform — By JJ Zhang, Version 1.0
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

// K8S phase 3: StatefulSet scaling / YAML updates / real-time pod log following

// ScaleStatefulSet adjusts StatefulSet replicas (scale subresource)
func (k *K8sAPI) ScaleStatefulSet(namespace, name string, replicas int) error {
	body, _ := json.Marshal(map[string]any{
		"apiVersion": "apps/v1", "kind": "Scale",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"replicas": replicas},
	})
	return k.do("PATCH", "/apis/apps/v1/namespaces/"+namespace+"/statefulsets/"+name+"/scale", body, nil)
}

// UpdateResourceYAML replaces the resource with the edited YAML (whole-object PUT)
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
	// Metadata consistency check to prevent submitting resource A's content to B
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

// FollowPodLog opens a follow=true log stream and returns the response body (caller is responsible for close and cancel)
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

// K8sPodUsage single pod: requests/limits + actual usage
type K8sPodUsage struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	CPUReqM   int64  `json:"cpu_req_m"`  // total requests (millicores)
	CPULimM   int64  `json:"cpu_lim_m"`  // total limits (millicores)
	MemReqMi  int64  `json:"mem_req_mi"` // total requests (Mi)
	MemLimMi  int64  `json:"mem_lim_mi"` // total limits (Mi)
	CPUM      int64  `json:"cpu_m"`      // actual usage (millicores)
	MemMi     int64  `json:"mem_mi"`     // actual usage (Mi)
}

// K8sClusterUsage cluster resource overview: capacity/actual usage (from metrics-server) + all pod requests and usage
type K8sClusterUsage struct {
	CPUCapacityM  int64         `json:"cpu_capacity_m"`  // total allocatable CPU (millicores)
	CPUUsedM      int64         `json:"cpu_used_m"`      // total actual node CPU usage (millicores)
	MemCapacityMi int64         `json:"mem_capacity_mi"` // total allocatable memory (Mi)
	MemUsedMi     int64         `json:"mem_used_mi"`     // total actual node memory usage (Mi)
	PodReqCPUM    int64         `json:"pod_req_cpu_m"`   // total CPU requests across all pods
	PodReqMemMi   int64         `json:"pod_req_mem_mi"`  // total memory requests across all pods
	Pods          []K8sPodUsage `json:"pods"`
}

// ClusterUsage concurrently fetches node capacity, node usage and pod usage (capacity still populated without metrics-server; usage is 0)
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
		// requests/limits of all pods
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

	// Merge actual usage into the pod list (matched by namespace/name; must run serially after pods are fetched)
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

// k8sCreatePaths collection paths for creatable resources (%s = namespace)
var k8sCreatePaths = map[string]string{
	"deployment":     "/apis/apps/v1/namespaces/%s/deployments",
	"daemonset":      "/apis/apps/v1/namespaces/%s/daemonsets",
	"statefulset":    "/apis/apps/v1/namespaces/%s/statefulsets",
	"job":            "/apis/batch/v1/namespaces/%s/jobs",
	"cronjob":        "/apis/batch/v1/namespaces/%s/cronjobs",
	"service":        "/api/v1/namespaces/%s/services",
	"ingress":        "/apis/networking.k8s.io/v1/namespaces/%s/ingresses",
	"pvc":            "/api/v1/namespaces/%s/persistentvolumeclaims",
	"configmap":      "/api/v1/namespaces/%s/configmaps",
	"secret":         "/api/v1/namespaces/%s/secrets",
	"serviceaccount": "/api/v1/namespaces/%s/serviceaccounts",
}

// DeleteResource generic resource deletion (rejects cluster-scoped resources to avoid accidentally deleting PVs/nodes)
func (k *K8sAPI) DeleteResource(kind, namespace, name string) error {
	switch kind {
	case "node", "pv", "storageclass":
		return fmt.Errorf("集群级资源 %s 不允许通过此接口删除", kind)
	}
	tpl, ok := k8sYAMLPaths[kind]
	if !ok {
		return fmt.Errorf("不支持的资源类型: %s", kind)
	}
	if namespace == "" {
		return fmt.Errorf("namespace 必填")
	}
	return k.do("DELETE", fmt.Sprintf(tpl, namespace, name), nil, nil)
}

// CreateResourceYAML creates a resource from YAML and returns the created resource name
func (k *K8sAPI) CreateResourceYAML(kind, defaultNS, yamlText string) (string, error) {
	tpl, ok := k8sCreatePaths[kind]
	if !ok {
		return "", fmt.Errorf("不支持的资源类型: %s", kind)
	}
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &obj); err != nil {
		return "", fmt.Errorf("YAML 解析失败: %w", err)
	}
	md, _ := obj["metadata"].(map[string]any)
	if md == nil {
		return "", fmt.Errorf("YAML 缺少 metadata")
	}
	name, _ := md["name"].(string)
	if name == "" {
		return "", fmt.Errorf("YAML 缺少 metadata.name")
	}
	ns, _ := md["namespace"].(string)
	if ns == "" {
		ns = defaultNS
	}
	if ns == "" {
		ns = "default"
	}
	md["namespace"] = ns
	body, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	if err := k.do("POST", fmt.Sprintf(tpl, ns), body, nil); err != nil {
		return "", err
	}
	return ns + "/" + name, nil
}
