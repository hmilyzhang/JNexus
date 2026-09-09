// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jnexus/internal/model"
)

// K8S 资源管理与日志：通过已存凭据代理调用集群 API

type K8sAPI struct {
	cli    *http.Client
	Server string
}

// K8sClientFor 构造指定集群的 API 客户端
func K8sClientFor(c *model.K8sCluster) (*K8sAPI, error) {
	caPEM, certPEM, keyPEM := decryptK8sCreds(c)
	cli, err := k8sTLSClient(caPEM, certPEM, keyPEM, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return &K8sAPI{cli: cli, Server: stringsTrimRightSlash(c.ApiServer)}, nil
}

func (k *K8sAPI) do(method, path string, body []byte, out any) error {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, k.Server+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := k.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("K8S API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

// K8sNodeInfo 节点信息
type K8sNodeInfo struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Version    string `json:"version"`
	InternalIP string `json:"internal_ip"`
	Roles      string `json:"roles"`
}

// K8sNodes 节点列表
func (k *K8sAPI) Nodes() ([]K8sNodeInfo, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				NodeInfo struct {
					KubeletVersion string `json:"kubeletVersion"`
				} `json:"nodeInfo"`
				Addresses []struct {
					Type    string `json:"type"`
					Address string `json:"address"`
				} `json:"addresses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := k.do("GET", "/api/v1/nodes", nil, &list); err != nil {
		return nil, err
	}
	out := []K8sNodeInfo{}
	for _, it := range list.Items {
		n := K8sNodeInfo{Name: it.Metadata.Name, Version: it.Status.NodeInfo.KubeletVersion}
		roles := []string{}
		for label := range it.Metadata.Labels {
			if strings.HasPrefix(label, "node-role.kubernetes.io/") {
				roles = append(roles, strings.TrimPrefix(label, "node-role.kubernetes.io/"))
			}
		}
		n.Roles = strings.Join(roles, ",")
		for _, addr := range it.Status.Addresses {
			if addr.Type == "InternalIP" {
				n.InternalIP = addr.Address
			}
		}
		for _, cond := range it.Status.Conditions {
			if cond.Type == "Ready" {
				if cond.Status == "True" {
					n.Status = "Ready"
				} else {
					n.Status = "NotReady"
				}
			}
		}
		out = append(out, n)
	}
	return out, nil
}

// K8sPodInfo Pod 信息
type K8sPodInfo struct {
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Node       string   `json:"node"`
	IP         string   `json:"ip"`
	Restarts   int      `json:"restarts"`
	StartedAt  string   `json:"started_at"`
	Containers []string `json:"containers"` // 容器名列表（多容器选择）
}

// K8sPods Pod 列表（namespace 为空 = 全部命名空间）
func (k *K8sAPI) Pods(namespace string) ([]K8sPodInfo, error) {
	path := "/api/v1/pods"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/pods"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				PodIP             string `json:"podIP"`
				Reason            string `json:"reason"`
				StartTime         string `json:"startTime"`
				ContainerStatuses []struct {
					RestartCount int `json:"restartCount"`
				} `json:"containerStatuses"`
			} `json:"status"`
			Spec struct {
				NodeName   string `json:"nodeName"`
				Containers []struct {
					Name string `json:"name"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sPodInfo{}
	for _, it := range list.Items {
		p := K8sPodInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Status: it.Status.Phase, Node: it.Spec.NodeName, IP: it.Status.PodIP,
			StartedAt: it.Status.StartTime,
		}
		if it.Status.Reason != "" {
			p.Status = it.Status.Reason
		}
		for _, cs := range it.Status.ContainerStatuses {
			p.Restarts += cs.RestartCount
		}
		for _, ct := range it.Spec.Containers {
			p.Containers = append(p.Containers, ct.Name)
		}
		out = append(out, p)
	}
	return out, nil
}

// DeletePod 删除 Pod
func (k *K8sAPI) DeletePod(namespace, name string) error {
	return k.do("DELETE", "/api/v1/namespaces/"+namespace+"/pods/"+name, nil, nil)
}

// RestartDeployment 重启 Deployment（kubectl rollout restart 语义）
func (k *K8sAPI) RestartDeployment(namespace, name string) error {
	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]string{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}
	b, _ := json.Marshal(patch)
	req, err := http.NewRequest("PATCH",
		k.Server+"/apis/apps/v1/namespaces/"+namespace+"/deployments/"+name,
		bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/strategic-merge-patch+json")
	resp, err := k.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("K8S API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// K8sDeploymentInfo Deployment 信息
type K8sDeploymentInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int    `json:"replicas"`
	Ready     int    `json:"ready"`
	Available int    `json:"available"`
}

// K8sDeployments Deployment 列表（namespace 为空 = 全部）
func (k *K8sAPI) Deployments(namespace string) ([]K8sDeploymentInfo, error) {
	path := "/apis/apps/v1/deployments"
	if namespace != "" {
		path = "/apis/apps/v1/namespaces/" + namespace + "/deployments"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Status struct {
				Replicas          int `json:"replicas"`
				ReadyReplicas     int `json:"readyReplicas"`
				AvailableReplicas int `json:"availableReplicas"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sDeploymentInfo{}
	for _, it := range list.Items {
		out = append(out, K8sDeploymentInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Replicas: it.Status.Replicas, Ready: it.Status.ReadyReplicas, Available: it.Status.AvailableReplicas,
		})
	}
	return out, nil
}

// PodLog 读取 Pod 日志
func (k *K8sAPI) PodLog(namespace, name, container string, tail int) (string, error) {
	path := "/api/v1/namespaces/" + namespace + "/pods/" + name + "/log"
	q := []string{}
	if container != "" {
		q = append(q, "container="+container)
	}
	if tail > 0 {
		q = append(q, "tailLines="+strconv.Itoa(tail))
	}
	if len(q) > 0 {
		path += "?" + strings.Join(q, "&")
	}
	resp, err := k.cli.Get(k.Server + path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("K8S API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

// K8sNamespaces 命名空间列表
func (k *K8sAPI) Namespaces() ([]string, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := k.do("GET", "/api/v1/namespaces", nil, &list); err != nil {
		return nil, err
	}
	out := []string{}
	for _, it := range list.Items {
		out = append(out, it.Metadata.Name)
	}
	return out, nil
}

// K8sEventInfo 事件
type K8sEventInfo struct {
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	LastSeen  string `json:"last_seen"`
}

// K8sEvents 最近事件
func (k *K8sAPI) Events(limit int) ([]K8sEventInfo, error) {
	path := "/api/v1/events?limit=" + strconv.Itoa(limit)
	var list struct {
		Items []struct {
			Metadata struct {
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Reason         string `json:"reason"`
			Type           string `json:"type"`
			Count          int    `json:"count"`
			LastTimestamp  string `json:"lastTimestamp"`
			InvolvedObject struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"involvedObject"`
			Message string `json:"message"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sEventInfo{}
	for _, it := range list.Items {
		out = append(out, K8sEventInfo{
			Namespace: it.Metadata.Namespace, Type: it.Type, Reason: it.Reason,
			Object:  it.InvolvedObject.Kind + "/" + it.InvolvedObject.Name,
			Message: it.Message, Count: it.Count, LastSeen: it.LastTimestamp,
		})
	}
	return out, nil
}

// K8sConfigInfo ConfigMap / Secret 条目
type K8sConfigInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	DataKeys  int    `json:"data_keys"`
	Age       string `json:"age"`
}

func (k *K8sAPI) ConfigMaps(namespace string) ([]K8sConfigInfo, error) {
	path := "/api/v1/configmaps"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/configmaps"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sConfigInfo{}
	for _, it := range list.Items {
		out = append(out, K8sConfigInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			DataKeys: len(it.Data), Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

func (k *K8sAPI) Secrets(namespace string) ([]K8sConfigInfo, error) {
	path := "/api/v1/secrets"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/secrets"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Type string `json:"type"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sConfigInfo{}
	for _, it := range list.Items {
		if strings.HasSuffix(it.Type, "helm.sh/release.v1") {
			continue // Helm release secrets 噪音过大，默认隐藏
		}
		out = append(out, K8sConfigInfo{Namespace: it.Metadata.Namespace, Name: it.Metadata.Name, Age: it.Metadata.CreationTimestamp})
	}
	return out, nil
}

// DeleteConfigMap 删除 ConfigMap
func (k *K8sAPI) DeleteConfigMap(namespace, name string) error {
	return k.do("DELETE", "/api/v1/namespaces/"+namespace+"/configmaps/"+name, nil, nil)
}

// DeleteSecret 删除 Secret
func (k *K8sAPI) DeleteSecret(namespace, name string) error {
	return k.do("DELETE", "/api/v1/namespaces/"+namespace+"/secrets/"+name, nil, nil)
}

// K8sNodeAbnormal 返回 NotReady 节点名列表（在线集群）
func (k *K8sAPI) NotReadyNodes() ([]string, error) {
	nodes, err := k.Nodes()
	if err != nil {
		return nil, err
	}
	bad := []string{}
	for _, n := range nodes {
		if n.Status != "Ready" {
			bad = append(bad, n.Name)
		}
	}
	return bad, nil
}

// K8sCronJobInfo 计划任务信息
type K8sCronJobInfo struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Schedule     string `json:"schedule"`
	Suspend      bool   `json:"suspend"`
	Active       int    `json:"active"`
	LastSchedule string `json:"last_schedule"`
}

// K8sCronJobs 计划任务列表
func (k *K8sAPI) CronJobs(namespace string) ([]K8sCronJobInfo, error) {
	path := "/apis/batch/v1/cronjobs"
	if namespace != "" {
		path = "/apis/batch/v1/namespaces/" + namespace + "/cronjobs"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name         string `json:"name"`
				Namespace    string `json:"namespace"`
				CreationTime string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Schedule string `json:"schedule"`
				Suspend  bool   `json:"suspend"`
			} `json:"spec"`
			Status struct {
				// batch/v1（K8S ≥1.21）的 active 是 ObjectReference 数组，取长度作为活跃 Job 数
				Active       []struct {
					Name string `json:"name"`
				} `json:"active"`
				LastSchedule string `json:"lastScheduleTime"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sCronJobInfo{}
	for _, it := range list.Items {
		out = append(out, K8sCronJobInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Schedule: it.Spec.Schedule, Suspend: it.Spec.Suspend,
			Active: len(it.Status.Active), LastSchedule: it.Status.LastSchedule,
		})
	}
	return out, nil
}

// CreateCronJob 创建计划任务（busybox 执行 shell 命令）
func (k *K8sAPI) CreateCronJob(namespace, name, schedule, command string) error {
	manifest := map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec": map[string]any{
			"schedule": schedule,
			"jobTemplate": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"restartPolicy": "OnFailure",
							"containers": []map[string]any{{
								"name":    name,
								"image":   "busybox:1.36",
								"command": []string{"sh", "-c", command},
							}},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(manifest)
	return k.do("POST", "/apis/batch/v1/namespaces/"+namespace+"/cronjobs", b, nil)
}

// SuspendCronJob 暂停/恢复计划任务
func (k *K8sAPI) SuspendCronJob(namespace, name string, suspend bool) error {
	b, _ := json.Marshal(map[string]any{"spec": map[string]any{"suspend": suspend}})
	req, err := http.NewRequest("PATCH",
		k.Server+"/apis/batch/v1/namespaces/"+namespace+"/cronjobs/"+name, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/merge-patch+json")
	resp, err := k.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("K8S API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// DeleteCronJob 删除计划任务
func (k *K8sAPI) DeleteCronJob(namespace, name string) error {
	return k.do("DELETE", "/apis/batch/v1/namespaces/"+namespace+"/cronjobs/"+name, nil, nil)
}

// K8sSAInfo 服务账号信息
type K8sSAInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Secrets   int    `json:"secrets"`
	Age       string `json:"age"`
}

// K8sServiceAccounts 服务账号列表
func (k *K8sAPI) ServiceAccounts(namespace string) ([]K8sSAInfo, error) {
	path := "/api/v1/serviceaccounts"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/serviceaccounts"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Secrets []any `json:"secrets"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sSAInfo{}
	for _, it := range list.Items {
		out = append(out, K8sSAInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Secrets: len(it.Secrets), Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

// CreateServiceAccount 创建服务账号
func (k *K8sAPI) CreateServiceAccount(namespace, name string) error {
	manifest := map[string]any{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}
	b, _ := json.Marshal(manifest)
	return k.do("POST", "/api/v1/namespaces/"+namespace+"/serviceaccounts", b, nil)
}

// DeleteServiceAccount 删除服务账号
func (k *K8sAPI) DeleteServiceAccount(namespace, name string) error {
	return k.do("DELETE", "/api/v1/namespaces/"+namespace+"/serviceaccounts/"+name, nil, nil)
}
