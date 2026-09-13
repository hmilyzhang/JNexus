// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// K8S phase-2 capabilities: YAML viewer / Deployment scaling / metrics-server usage / Helm release view

// k8sYAMLPaths kind → resource path template (%s = namespace, %s = name)
var k8sYAMLPaths = map[string]string{
	"pod":            "/api/v1/namespaces/%s/pods/%s",
	"service":        "/api/v1/namespaces/%s/services/%s",
	"configmap":      "/api/v1/namespaces/%s/configmaps/%s",
	"secret":         "/api/v1/namespaces/%s/secrets/%s",
	"pvc":            "/api/v1/namespaces/%s/persistentvolumeclaims/%s",
	"serviceaccount": "/api/v1/namespaces/%s/serviceaccounts/%s",
	"node":           "/api/v1/nodes/%s",
	"pv":             "/api/v1/persistentvolumes/%s",
	"deployment":     "/apis/apps/v1/namespaces/%s/deployments/%s",
	"daemonset":      "/apis/apps/v1/namespaces/%s/daemonsets/%s",
	"statefulset":    "/apis/apps/v1/namespaces/%s/statefulsets/%s",
	"job":            "/apis/batch/v1/namespaces/%s/jobs/%s",
	"cronjob":        "/apis/batch/v1/namespaces/%s/cronjobs/%s",
	"ingress":        "/apis/networking.k8s.io/v1/namespaces/%s/ingresses/%s",
	"storageclass":   "/apis/storage.k8s.io/v1/storageclasses/%s",
}

// ResourceYAML fetches the resource's raw JSON and converts it to YAML (read-only)
func (k *K8sAPI) ResourceYAML(kind, namespace, name string) (string, error) {
	tpl, ok := k8sYAMLPaths[kind]
	if !ok {
		return "", fmt.Errorf("不支持的资源类型: %s", kind)
	}
	var path string
	if kind == "node" || kind == "pv" || kind == "storageclass" {
		path = fmt.Sprintf(tpl, name)
	} else {
		if namespace == "" {
			return "", fmt.Errorf("namespace 必填")
		}
		path = fmt.Sprintf(tpl, namespace, name)
	}
	var obj map[string]any
	if err := k.do("GET", path, nil, &obj); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ScaleDeployment adjusts Deployment replicas (scale subresource)
func (k *K8sAPI) ScaleDeployment(namespace, name string, replicas int) error {
	body, _ := json.Marshal(map[string]any{
		"apiVersion": "apps/v1", "kind": "Scale",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"replicas": replicas},
	})
	return k.do("PATCH", "/apis/apps/v1/namespaces/"+namespace+"/deployments/"+name+"/scale", body, nil)
}

// ---- metrics-server resource usage (graceful degradation when metrics-server is not installed) ----

// K8sMetricInfo resource usage of a single object (CPU millicores / memory Mi)
type K8sMetricInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	CPUM      int64  `json:"cpu_m"`  // millicores
	MemMi     int64  `json:"mem_mi"` // MiB
}

// k8sQuantityCPU parses CPU usage into millicores (supports n/u/m/bare cores)
func k8sQuantityCPU(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	switch {
	case strings.HasSuffix(v, "m"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "m"), 10, 64)
		return n
	case strings.HasSuffix(v, "u"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "u"), 10, 64)
		return n / 1000
	case strings.HasSuffix(v, "n"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "n"), 10, 64)
		return n / 1000000
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1000)
}

// k8sQuantityMem parses memory usage into bytes (supports Ki/Mi/Gi/Ti/K/M/G and bare bytes)
func k8sQuantityMem(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	mult := int64(1)
	suffixes := map[string]int64{"Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40,
		"K": 1000, "M": 1000 << 10, "G": 1000 << 20, "T": 1000 << 30}
	for suf, m := range suffixes {
		if strings.HasSuffix(v, suf) {
			mult, v = m, strings.TrimSuffix(v, suf)
			break
		}
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return int64(n * float64(mult))
}

// NodeMetrics node resource usage (returns an empty list when metrics-server is missing)
func (k *K8sAPI) NodeMetrics() ([]K8sMetricInfo, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"items"`
	}
	if err := k.do("GET", "/apis/metrics.k8s.io/v1beta1/nodes", nil, &list); err != nil {
		return []K8sMetricInfo{}, nil // metrics-server not installed: degrade silently
	}
	out := []K8sMetricInfo{}
	for _, it := range list.Items {
		out = append(out, K8sMetricInfo{
			Name: it.Metadata.Name,
			CPUM: k8sQuantityCPU(it.Usage.CPU), MemMi: k8sQuantityMem(it.Usage.Memory) >> 20,
		})
	}
	return out, nil
}

// PodMetrics pod resource usage (summed over containers)
func (k *K8sAPI) PodMetrics(namespace string) ([]K8sMetricInfo, error) {
	path := "/apis/metrics.k8s.io/v1beta1/pods"
	if namespace != "" {
		path = "/apis/metrics.k8s.io/v1beta1/namespaces/" + namespace + "/pods"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Containers []struct {
				Usage struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"usage"`
			} `json:"containers"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return []K8sMetricInfo{}, nil
	}
	out := []K8sMetricInfo{}
	for _, it := range list.Items {
		m := K8sMetricInfo{Name: it.Metadata.Name, Namespace: it.Metadata.Namespace}
		for _, ct := range it.Containers {
			m.CPUM += k8sQuantityCPU(ct.Usage.CPU)
			m.MemMi += k8sQuantityMem(ct.Usage.Memory) >> 20
		}
		out = append(out, m)
	}
	return out, nil
}

// ---- Helm release view (decodes helm.sh/release.v1 secrets, read-only) ----

// K8sHelmReleaseInfo Helm release info
type K8sHelmReleaseInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Chart     string `json:"chart"`
	Version   string `json:"version"`
	Revision  int    `json:"revision"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

// K8sHelmReleases Helm release list (latest revision per release)
func (k *K8sAPI) HelmReleases(namespace string) ([]K8sHelmReleaseInfo, error) {
	path := "/api/v1/secrets"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/secrets"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Namespace string            `json:"namespace"`
				Labels    map[string]string `json:"labels"`
			} `json:"metadata"`
			Type string            `json:"type"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	latest := map[string]K8sHelmReleaseInfo{}
	for _, it := range list.Items {
		if it.Type != "helm.sh/release.v1" || it.Metadata.Labels["owner"] != "helm" {
			continue
		}
		labels := it.Metadata.Labels
		rel := K8sHelmReleaseInfo{
			Namespace: it.Metadata.Namespace, Name: labels["name"],
			Status: labels["status"], UpdatedAt: labels["updated_at"],
		}
		rel.Revision, _ = strconv.Atoi(labels["version"])
		if payload, ok := it.Data["release"]; ok {
			if chart, ver, deployed, err := decodeHelmRelease(payload); err == nil {
				rel.Chart, rel.Version, rel.UpdatedAt = chart, ver, deployed
			}
		}
		key := rel.Namespace + "/" + rel.Name
		if old, exists := latest[key]; !exists || rel.Revision > old.Revision {
			latest[key] = rel
		}
	}
	out := []K8sHelmReleaseInfo{}
	for _, rel := range latest {
		out = append(out, rel)
	}
	return out, nil
}

// decodeHelmRelease decodes the release secret payload (base64 → gzip → JSON) and extracts chart name/version/deploy time
func decodeHelmRelease(payload string) (chart, version, deployed string, err error) {
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return
	}
	defer zr.Close()
	b, err := io.ReadAll(io.LimitReader(zr, 4<<20))
	if err != nil {
		return
	}
	var rel struct {
		Chart struct {
			Metadata struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"metadata"`
		} `json:"chart"`
		Info struct {
			LastDeployed string `json:"last_deployed"`
		} `json:"info"`
	}
	if err = json.Unmarshal(b, &rel); err != nil {
		return
	}
	return rel.Chart.Metadata.Name, rel.Chart.Metadata.Version, rel.Info.LastDeployed, nil
}
