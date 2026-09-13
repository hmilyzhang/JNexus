// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
)

// K8S cluster read-only views: overview stats + DaemonSet/StatefulSet/Job/Service/Ingress/PVC/PV/StorageClass lists
// (data source for the K8S admin page's left menu; all read-only endpoints accessible with viewer permission)

// K8sWorkloadInfo generic workload entry (DaemonSet / StatefulSet / Job)
type K8sWorkloadInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Desired   int    `json:"desired"`
	Ready     int    `json:"ready"`
	Status    string `json:"status"` // Job case: Succeeded/Failed count description
	Age       string `json:"age"`
}

func (k *K8sAPI) workloadList(path string, kind string) ([]K8sWorkloadInfo, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Replicas    *int `json:"replicas"`
				Completions *int `json:"completions"`
			} `json:"spec"`
			Status struct {
				DesiredNumberScheduled int `json:"desiredNumberScheduled"`
				NumberReady            int `json:"numberReady"`
				ReadyReplicas          int `json:"readyReplicas"`
				Succeeded              int `json:"succeeded"`
				Failed                 int `json:"failed"`
				Active                 int `json:"active"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sWorkloadInfo{}
	for _, it := range list.Items {
		w := K8sWorkloadInfo{Namespace: it.Metadata.Namespace, Name: it.Metadata.Name, Age: it.Metadata.CreationTimestamp}
		switch kind {
		case "daemonset":
			w.Desired, w.Ready = it.Status.DesiredNumberScheduled, it.Status.NumberReady
		case "statefulset":
			if it.Spec.Replicas != nil {
				w.Desired = *it.Spec.Replicas
			}
			w.Ready = it.Status.ReadyReplicas
		case "job":
			if it.Spec.Completions != nil {
				w.Desired = *it.Spec.Completions
			}
			w.Ready = it.Status.Succeeded
			w.Status = formatJobStatus(it.Status.Active, it.Status.Succeeded, it.Status.Failed)
		}
		out = append(out, w)
	}
	return out, nil
}

func formatJobStatus(active, succeeded, failed int) string {
	parts := []string{}
	if active > 0 {
		parts = append(parts, "Running")
	}
	if succeeded > 0 {
		parts = append(parts, "Succeeded")
	}
	if failed > 0 {
		parts = append(parts, "Failed")
	}
	if len(parts) == 0 {
		return "Pending"
	}
	return strings.Join(parts, " / ")
}

// DaemonSets DaemonSet list
func (k *K8sAPI) DaemonSets(namespace string) ([]K8sWorkloadInfo, error) {
	path := "/apis/apps/v1/daemonsets"
	if namespace != "" {
		path = "/apis/apps/v1/namespaces/" + namespace + "/daemonsets"
	}
	return k.workloadList(path, "daemonset")
}

// StatefulSets StatefulSet list
func (k *K8sAPI) StatefulSets(namespace string) ([]K8sWorkloadInfo, error) {
	path := "/apis/apps/v1/statefulsets"
	if namespace != "" {
		path = "/apis/apps/v1/namespaces/" + namespace + "/statefulsets"
	}
	return k.workloadList(path, "statefulset")
}

// Jobs Job list
func (k *K8sAPI) Jobs(namespace string) ([]K8sWorkloadInfo, error) {
	path := "/apis/batch/v1/jobs"
	if namespace != "" {
		path = "/apis/batch/v1/namespaces/" + namespace + "/jobs"
	}
	return k.workloadList(path, "job")
}

// K8sServiceInfo service info
type K8sServiceInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

// K8sServices service list
func (k *K8sAPI) Services(namespace string) ([]K8sServiceInfo, error) {
	path := "/api/v1/services"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/services"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Type      string `json:"type"`
				ClusterIP string `json:"clusterIP"`
				Ports     []struct {
					Port       int    `json:"port"`
					Protocol   string `json:"protocol"`
					NodePort   int    `json:"nodePort"`
					TargetPort any    `json:"targetPort"`
				} `json:"ports"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sServiceInfo{}
	for _, it := range list.Items {
		ports := []string{}
		for _, pt := range it.Spec.Ports {
			s := strconv.Itoa(pt.Port) + "/" + pt.Protocol
			if pt.NodePort > 0 {
				s += ":" + strconv.Itoa(pt.NodePort)
			}
			ports = append(ports, s)
		}
		out = append(out, K8sServiceInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Type: it.Spec.Type, ClusterIP: it.Spec.ClusterIP,
			Ports: strings.Join(ports, ","), Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

// K8sIngressInfo ingress info
type K8sIngressInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Hosts     string `json:"hosts"`
	Age       string `json:"age"`
}

// K8sIngresses ingress list
func (k *K8sAPI) Ingresses(namespace string) ([]K8sIngressInfo, error) {
	path := "/apis/networking.k8s.io/v1/ingresses"
	if namespace != "" {
		path = "/apis/networking.k8s.io/v1/namespaces/" + namespace + "/ingresses"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Rules []struct {
					Host string `json:"host"`
				} `json:"rules"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sIngressInfo{}
	for _, it := range list.Items {
		hosts := []string{}
		for _, r := range it.Spec.Rules {
			if r.Host != "" {
				hosts = append(hosts, r.Host)
			}
		}
		out = append(out, K8sIngressInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Hosts: strings.Join(hosts, ","), Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

// K8sPVCInfo PersistentVolumeClaim info
type K8sPVCInfo struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Capacity     string `json:"capacity"`
	StorageClass string `json:"storage_class"`
	Age          string `json:"age"`
}

// K8sPVCs PVC list
func (k *K8sAPI) PVCs(namespace string) ([]K8sPVCInfo, error) {
	path := "/api/v1/persistentvolumeclaims"
	if namespace != "" {
		path = "/api/v1/namespaces/" + namespace + "/persistentvolumeclaims"
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				StorageClassName string `json:"storageClassName"`
			} `json:"spec"`
			Status struct {
				Phase     string `json:"phase"`
				Resources struct {
					Requests map[string]string `json:"requests"`
				} `json:"resources"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := k.do("GET", path, nil, &list); err != nil {
		return nil, err
	}
	out := []K8sPVCInfo{}
	for _, it := range list.Items {
		out = append(out, K8sPVCInfo{
			Namespace: it.Metadata.Namespace, Name: it.Metadata.Name,
			Phase: it.Status.Phase, Capacity: it.Status.Resources.Requests["storage"],
			StorageClass: it.Spec.StorageClassName, Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

// K8sPVInfo PersistentVolume info
type K8sPVInfo struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Capacity     string `json:"capacity"`
	Claim        string `json:"claim"`
	StorageClass string `json:"storage_class"`
	Age          string `json:"age"`
}

// K8sPVs PV list (cluster-scoped resources)
func (k *K8sAPI) PVs() ([]K8sPVInfo, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				StorageClassName string `json:"storageClassName"`
				ClaimRef         struct {
					Namespace string `json:"namespace"`
					Name      string `json:"name"`
				} `json:"claimRef"`
				Capacity map[string]string `json:"capacity"`
			} `json:"spec"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := k.do("GET", "/api/v1/persistentvolumes", nil, &list); err != nil {
		return nil, err
	}
	out := []K8sPVInfo{}
	for _, it := range list.Items {
		claim := ""
		if it.Spec.ClaimRef.Name != "" {
			claim = it.Spec.ClaimRef.Namespace + "/" + it.Spec.ClaimRef.Name
		}
		out = append(out, K8sPVInfo{
			Name: it.Metadata.Name, Phase: it.Status.Phase,
			Capacity: it.Spec.Capacity["storage"], Claim: claim,
			StorageClass: it.Spec.StorageClassName, Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

// K8sStorageClassInfo StorageClass info
type K8sStorageClassInfo struct {
	Name        string `json:"name"`
	Provisioner string `json:"provisioner"`
	Age         string `json:"age"`
}

// K8sStorageClasses StorageClass list (cluster-scoped resources)
func (k *K8sAPI) StorageClasses() ([]K8sStorageClassInfo, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Provisioner string `json:"provisioner"`
		} `json:"items"`
	}
	if err := k.do("GET", "/apis/storage.k8s.io/v1/storageclasses", nil, &list); err != nil {
		return nil, err
	}
	out := []K8sStorageClassInfo{}
	for _, it := range list.Items {
		out = append(out, K8sStorageClassInfo{
			Name: it.Metadata.Name, Provisioner: it.Provisioner, Age: it.Metadata.CreationTimestamp,
		})
	}
	return out, nil
}

// K8sClusterSummary cluster overview stats
type K8sClusterSummary struct {
	Version         string `json:"version"`
	CreatedAt       string `json:"created_at"` // creation time of the earliest node, i.e. when the cluster came up
	Nodes           int    `json:"nodes"`
	NodesReady      int    `json:"nodes_ready"`
	Namespaces      int    `json:"namespaces"`
	Pods            int    `json:"pods"`
	RunningPods     int    `json:"running_pods"`
	Deployments     int    `json:"deployments"`
	DaemonSets      int    `json:"daemonsets"`
	StatefulSets    int    `json:"statefulsets"`
	Jobs            int    `json:"jobs"`
	CronJobs        int    `json:"cronjobs"`
	Services        int    `json:"services"`
	Ingresses       int    `json:"ingresses"`
	PVs             int    `json:"pvs"`
	PVCs            int    `json:"pvcs"`
	ConfigMaps      int    `json:"configmaps"`
	Secrets         int    `json:"secrets"`
	ServiceAccounts int    `json:"serviceaccounts"`
}

// k8sRawItem loose list entry used for overview counting
type k8sRawItem struct {
	Metadata struct {
		CreationTimestamp string `json:"creationTimestamp"`
	} `json:"metadata"`
	Status struct {
		Phase      string `json:"phase"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
	} `json:"status"`
}

// countList fetches a list and counts it (fn is the filter, nil = all); retries once on transient failure
func (k *K8sAPI) countList(path string, fn func(it k8sRawItem) bool) (int, string, error) {
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	err := k.do("GET", path, nil, &list)
	if err != nil {
		time.Sleep(300 * time.Millisecond)
		err = k.do("GET", path, nil, &list)
	}
	if err != nil {
		return 0, "", err
	}
	n := 0
	created := ""
	for _, raw := range list.Items {
		var it k8sRawItem
		if json.Unmarshal(raw, &it) != nil {
			continue
		}
		if fn != nil && !fn(it) {
			continue
		}
		n++
		if it.Metadata.CreationTimestamp != "" &&
			(created == "" || it.Metadata.CreationTimestamp < created) {
			created = it.Metadata.CreationTimestamp
		}
	}
	return n, created, nil
}

// Summary cluster overview: concurrently fetches counts of each resource (individual failures don't affect the whole; counts stay 0)
func (k *K8sAPI) Summary() (*K8sClusterSummary, error) {
	s := &K8sClusterSummary{}
	var ver struct {
		GitVersion string `json:"gitVersion"`
	}
	if err := k.do("GET", "/version", nil, &ver); err == nil {
		s.Version = ver.GitVersion
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // limit concurrency to avoid a spike in cluster API connections
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
		n, created, _ := k.countList("/api/v1/nodes", func(it k8sRawItem) bool { return true })
		ready, _, _ := k.countList("/api/v1/nodes", func(it k8sRawItem) bool {
			for _, c := range it.Status.Conditions {
				if c.Type == "Ready" {
					return c.Status == "True"
				}
			}
			return false
		})
		s.Nodes, s.NodesReady, s.CreatedAt = n, ready, created
	})
	run(func() { s.Namespaces, _, _ = k.countList("/api/v1/namespaces", nil) })
	run(func() {
		n, _, _ := k.countList("/api/v1/pods", nil)
		s.Pods = n
	})
	run(func() {
		n, _, _ := k.countList("/api/v1/pods", func(it k8sRawItem) bool { return it.Status.Phase == "Running" })
		s.RunningPods = n
	})
	run(func() { s.Deployments, _, _ = k.countList("/apis/apps/v1/deployments", nil) })
	run(func() { s.DaemonSets, _, _ = k.countList("/apis/apps/v1/daemonsets", nil) })
	run(func() { s.StatefulSets, _, _ = k.countList("/apis/apps/v1/statefulsets", nil) })
	run(func() { s.Jobs, _, _ = k.countList("/apis/batch/v1/jobs", nil) })
	run(func() { s.CronJobs, _, _ = k.countList("/apis/batch/v1/cronjobs", nil) })
	run(func() { s.Services, _, _ = k.countList("/api/v1/services", nil) })
	run(func() { s.Ingresses, _, _ = k.countList("/apis/networking.k8s.io/v1/ingresses", nil) })
	run(func() { s.PVs, _, _ = k.countList("/api/v1/persistentvolumes", nil) })
	run(func() { s.PVCs, _, _ = k.countList("/api/v1/persistentvolumeclaims", nil) })
	run(func() { s.ConfigMaps, _, _ = k.countList("/api/v1/configmaps", nil) })
	run(func() {
		n, _, _ := k.countList("/api/v1/secrets", nil)
		s.Secrets = n
	})
	run(func() { s.ServiceAccounts, _, _ = k.countList("/api/v1/serviceaccounts", nil) })
	wg.Wait()
	return s, nil
}
