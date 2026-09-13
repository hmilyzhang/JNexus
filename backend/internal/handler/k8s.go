// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

// K8S cluster management:
//   - Permissions: roles set k8s_view (view) / k8s_manage (add/remove clusters and member management; admin always has it)
//   - Cluster members (K8SClusterMember) hold an admin/user role on that cluster and can view it
//   - Different clusters can bind different members and support contacts

func hasK8sView(u *model.User) bool {
	if u.IsAdmin() {
		return true
	}
	return service.HasK8sPerm(u.Role, "view")
}

func hasK8sManage(u *model.User) bool {
	if u.IsAdmin() {
		return true
	}
	return service.HasK8sPerm(u.Role, "manage")
}

// clusterAdmin reports whether the user is an admin member of the given cluster
func clusterAdmin(userID uint, clusterID uint) bool {
	var m model.K8sClusterMember
	if err := model.DB.Where("cluster_id = ? AND user_id = ?", clusterID, userID).
		First(&m).Error; err != nil {
		return false
	}
	return m.Role == "admin"
}

func canSeeCluster(u *model.User, clusterID uint) bool {
	if u.IsAdmin() || hasK8sView(u) {
		return true
	}
	var cnt int64
	model.DB.Model(&model.K8sClusterMember{}).
		Where("cluster_id = ? AND user_id = ?", clusterID, u.ID).Count(&cnt)
	return cnt > 0
}

// ListK8sClusters lists clusters (including my role and member count)
func ListK8sClusters(c *gin.Context) {
	user := currentUser(c)
	var clusters []model.K8sCluster
	model.DB.Order("name").Find(&clusters)
	out := []gin.H{}
	for _, cl := range clusters {
		if !canSeeCluster(user, cl.ID) {
			continue
		}
		var members []model.K8sClusterMember
		model.DB.Where("cluster_id = ?", cl.ID).Find(&members)
		myRole := "-"
		for _, mm := range members {
			if mm.UserID == user.ID {
				if mm.Role == "admin" {
					myRole = "admin"
				} else if myRole == "-" {
					myRole = mm.Role
				}
			}
		}
		if user.IsAdmin() || hasK8sManage(user) {
			myRole = "admin"
		}
		out = append(out, gin.H{
			"id": cl.ID, "name": cl.Name, "api_server": cl.ApiServer,
			"support": cl.Support, "description": cl.Description,
			"status": cl.Status, "version": cl.Version, "node_count": cl.NodeCount,
			"cert_expiry": cl.CertExpiry, "ca_expiry": cl.CAExpiry,
			"last_seen": cl.LastSeen, "enabled": cl.Enabled,
			"members": len(members), "my_role": myRole,
		})
	}
	c.JSON(http.StatusOK, out)
}

type k8sClusterReq struct {
	Name        string `json:"name" binding:"required"`
	ApiServer   string `json:"api_server" binding:"required"`
	Support     string `json:"support"`
	Description string `json:"description"`
	Kubeconfig  string `json:"kubeconfig"`
	CA          string `json:"ca"`
	ClientCert  string `json:"client_cert"`
	ClientKey   string `json:"client_key"`
	Enabled     *bool  `json:"enabled"`
}

// applyK8sCluster processes credentials (kubeconfig or the CA/cert/key triple, AES-encrypted)
func applyK8sCluster(c *model.K8sCluster, req k8sClusterReq, isUpdate bool) error {
	c.Name, c.ApiServer, c.Support, c.Description = req.Name, req.ApiServer, req.Support, req.Description
	if req.Kubeconfig != "" {
		server, ca, cert, key, err := service.ParseKubeconfig(req.Kubeconfig)
		if err != nil {
			return err
		}
		if c.ApiServer == "" {
			c.ApiServer = server
		}
		var e error
		if c.CA, e = service.EncryptK8sSecret(ca); e != nil {
			return e
		}
		if c.ClientCert, e = service.EncryptK8sSecret(cert); e != nil {
			return e
		}
		if c.ClientKey, e = service.EncryptK8sSecret(key); e != nil {
			return e
		}
		if c.Kubeconfig, e = service.EncryptK8sSecret(req.Kubeconfig); e != nil {
			return e
		}
		return nil
	}
	if req.CA != "" && req.ClientCert != "" && req.ClientKey != "" {
		if c.ApiServer == "" {
			return fmt.Errorf("API Server 不能为空")
		}
		var e error
		if c.CA, e = service.EncryptK8sSecret(req.CA); e != nil {
			return e
		}
		if c.ClientCert, e = service.EncryptK8sSecret(req.ClientCert); e != nil {
			return e
		}
		if c.ClientKey, e = service.EncryptK8sSecret(req.ClientKey); e != nil {
			return e
		}
		return nil
	}
	if !isUpdate {
		return fmt.Errorf("需要 kubeconfig 或 CA/客户端证书/私钥 三件套")
	}
	return nil // On update, allow metadata-only changes; keep existing credentials
}

// CreateK8sCluster adds a cluster and probes it immediately
func CreateK8sCluster(c *gin.Context) {
	user := currentUser(c)
	if !hasK8sManage(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无 K8S 管理权限"})
		return
	}
	var req k8sClusterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（名称/API Server 必填）"})
		return
	}
	cl := model.K8sCluster{Enabled: true, CreatedBy: user.Username}
	if err := applyK8sCluster(&cl, req, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := service.ProbeK8sCluster(&cl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接测试失败: " + err.Error()})
		return
	}
	cl.Status, cl.Version, cl.NodeCount = "online", res.Version, res.NodeCount
	now := time.Now()
	cl.LastSeen = &now
	if res.CertExp != nil {
		cl.CertExpiry = res.CertExp
	}
	if res.CAExp != nil {
		cl.CAExpiry = res.CAExp
	}
	if err := model.DB.Create(&cl).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "集群名已存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": cl.ID, "version": cl.Version, "node_count": cl.NodeCount, "cert_expiry": cl.CertExpiry})
}

// UpdateK8sCluster updates a cluster (metadata/credentials/enable-disable)
func UpdateK8sCluster(c *gin.Context) {
	user := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var cl model.K8sCluster
	if err := model.DB.First(&cl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}
	if !hasK8sManage(user) && !clusterAdmin(user.ID, cl.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该集群的管理权限"})
		return
	}
	var req k8sClusterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := applyK8sCluster(&cl, req, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Enabled != nil {
		cl.Enabled = *req.Enabled
	}
	// Reset certificate expiry reminder flags after a credential update
	if req.Kubeconfig != "" || (req.CA != "" && req.ClientCert != "" && req.ClientKey != "") {
		cl.Warn30Sent, cl.Warn7Sent = false, false
	}
	model.DB.Save(&cl)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteK8sCluster deletes a cluster (including member bindings)
func DeleteK8sCluster(c *gin.Context) {
	user := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var cl model.K8sCluster
	if err := model.DB.First(&cl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}
	if !hasK8sManage(user) && !clusterAdmin(user.ID, cl.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该集群的管理权限"})
		return
	}
	model.DB.Where("cluster_id = ?", id).Delete(&model.K8sClusterMember{})
	model.DB.Delete(&cl, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestK8sCluster re-tests connectivity immediately and refreshes certificate expiry
func TestK8sCluster(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cl model.K8sCluster
	if err := model.DB.First(&cl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}
	res, err := service.ProbeK8sCluster(&cl)
	if err != nil {
		model.DB.Model(&cl).Updates(map[string]any{"status": "offline", "last_seen": time.Now()})
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	now := time.Now()
	updates := map[string]any{"status": "online", "last_seen": now}
	if res.Version != "" {
		updates["version"] = res.Version
	}
	if res.NodeCount > 0 {
		updates["node_count"] = res.NodeCount
	}
	if res.CertExp != nil {
		updates["cert_expiry"] = res.CertExp
	}
	if res.CAExp != nil {
		updates["ca_expiry"] = res.CAExp
	}
	model.DB.Model(&cl).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"ok": true, "version": res.Version, "node_count": res.NodeCount,
		"cert_expiry": res.CertExp, "ca_expiry": res.CAExp})
}

// k8sClusterAccess combined check: returns (cluster, API client, member role, allowed)
// Role requirements: viewer read-only; user ops actions; admin cluster management
func k8sClusterAccess(c *gin.Context, clusterID int, needRole string) (*model.K8sCluster, *service.K8sAPI, string, bool) {
	user := currentUser(c)
	var cl model.K8sCluster
	if err := model.DB.First(&cl, clusterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return nil, nil, "", false
	}
	var m model.K8sClusterMember
	model.DB.Where("cluster_id = ? AND user_id = ?", clusterID, user.ID).First(&m)
	myRole := m.Role
	roleRank := map[string]int{"viewer": 1, "user": 2, "admin": 3}
	if user.IsAdmin() {
		myRole = "admin"
	} else if service.HasK8sPerm(user.Role, "manage") {
		myRole = "admin"
	} else if service.HasK8sPerm(user.Role, "view") && myRole == "" {
		myRole = "viewer"
	}
	if myRole == "" || roleRank[myRole] < roleRank[needRole] {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该集群 " + needRole + " 权限"})
		return nil, nil, "", false
	}
	api, err := service.K8sClientFor(&cl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, nil, "", false
	}
	return &cl, api, myRole, true
}

// K8sNodes lists cluster nodes
func K8sNodes(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cl, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	nodes, err := api.Nodes()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cluster": cl.Name, "nodes": nodes})
}

// K8sNamespaces lists namespaces
func K8sNamespaces(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	ns, err := api.Namespaces()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ns)
}

// K8sPods lists pods (?namespace=)
func K8sPods(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	pods, err := api.Pods(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pods)
}

// K8sPodLog returns pod logs
func K8sPodLog(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Query("namespace"), c.Query("pod")
	if ns == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace/pod 必填"})
		return
	}
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	text, err := api.PodLog(ns, name, c.Query("container"), 500)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"log": text})
}

// K8sDeletePod deletes a pod (requires user role or above)
func K8sDeletePod(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Param("namespace"), c.Param("name")
	cl, api, myRole, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.DeletePod(ns, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "DELETE POD " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	_ = myRole
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sClusterShell creates a temporary shell pod (busybox), waits for Running, then returns
// it so the frontend can open a terminal; the frontend cleans up via the existing delete-pod API on close
func K8sClusterShell(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	ns := c.Query("namespace")
	if ns == "" {
		ns = "default"
	}
	name := fmt.Sprintf("jnexus-shell-%05d", time.Now().UnixNano()%100000)
	if err := api.CreateShellPod(ns, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Wait for the pod to be Running (image pull may be slow); clean up and error out on failure
	ready := false
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		phase := api.GetPodPhase(ns, name)
		if phase == "Running" {
			ready = true
			break
		}
		if phase == "Failed" {
			break
		}
	}
	if !ready {
		_ = api.DeletePod(ns, name)
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Shell Pod 未就绪（镜像拉取慢或被 RBAC 拒绝？），请重试或检查 default 命名空间建 Pod 权限"})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "CREATE SHELL POD " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"namespace": ns, "name": name})
}

// K8sDeployments lists deployments (?namespace=)
func K8sDeployments(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	deps, err := api.Deployments(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deps)
}

// K8sRestartDeployment restarts a deployment (rolling restart; requires user role or above)
func K8sRestartDeployment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Param("namespace"), c.Param("name")
	cl, api, myRole, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.RestartDeployment(ns, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "RESTART DEPLOYMENT " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	_ = myRole
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sEvents lists cluster events
func K8sEvents(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	events, err := api.Events(100)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

// K8sConfigMaps lists ConfigMaps
func K8sConfigMaps(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns := c.Query("namespace")
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.ConfigMaps(ns)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sSecrets lists Secrets
func K8sSecrets(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns := c.Query("namespace")
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.Secrets(ns)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sDeleteConfig deletes a ConfigMap / Secret (kind: configmap / secret)
func K8sDeleteConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	kind, ns, name := c.Param("kind"), c.Param("namespace"), c.Param("name")
	if kind != "configmaps" && kind != "secrets" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind 必须是 configmaps / secrets"})
		return
	}
	cl, api, myRole, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	var err error
	if kind == "configmaps" {
		err = api.DeleteConfigMap(ns, name)
	} else {
		err = api.DeleteSecret(ns, name)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "DELETE " + strings.ToUpper(kind[:len(kind)-1]) + " " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	_ = myRole
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sCronJobs lists cron jobs (?namespace=)
func K8sCronJobs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns := c.Query("namespace")
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.CronJobs(ns)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sCreateCronJob creates a cron job
func K8sCreateCronJob(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Namespace string `json:"namespace" binding:"required"`
		Name      string `json:"name" binding:"required"`
		Schedule  string `json:"schedule" binding:"required"`
		Command   string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（namespace/name/schedule/command 必填）"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.CreateCronJob(req.Namespace, req.Name, req.Schedule, req.Command); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "CREATE CRONJOB " + req.Namespace + "/" + req.Name + " (" + req.Schedule + ") @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sSuspendCronJob suspends/resumes a cron job
func K8sSuspendCronJob(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Param("namespace"), c.Param("name")
	var req struct {
		Suspend bool `json:"suspend"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.SuspendCronJob(ns, name, req.Suspend); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	act := "RESUME"
	if req.Suspend {
		act = "SUSPEND"
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: act + " CRONJOB " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sDeleteCronJob deletes a cron job
func K8sDeleteCronJob(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Param("namespace"), c.Param("name")
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.DeleteCronJob(ns, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "DELETE CRONJOB " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sServiceAccounts lists service accounts (?namespace=)
func K8sServiceAccounts(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns := c.Query("namespace")
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.ServiceAccounts(ns)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sCreateServiceAccount creates a service account
func K8sCreateServiceAccount(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Namespace string `json:"namespace" binding:"required"`
		Name      string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace/name 必填"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.CreateServiceAccount(req.Namespace, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "CREATE SA " + req.Namespace + "/" + req.Name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sDeleteServiceAccount deletes a service account
func K8sDeleteServiceAccount(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Param("namespace"), c.Param("name")
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.DeleteServiceAccount(ns, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "DELETE SA " + ns + "/" + name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListClusterMembers lists cluster members
// ListClusterMembers lists cluster members
func ListClusterMembers(c *gin.Context) {
	clusterID, _ := strconv.Atoi(c.Param("id"))
	var members []model.K8sClusterMember
	model.DB.Where("cluster_id = ?", clusterID).Order("id").Find(&members)
	out := []gin.H{}
	for _, mm := range members {
		var u model.User
		name := ""
		if model.DB.First(&u, mm.UserID).Error == nil {
			name = u.Username
		}
		out = append(out, gin.H{"user_id": mm.UserID, "username": name, "role": mm.Role})
	}
	c.JSON(http.StatusOK, out)
}

type clusterMemberReq struct {
	UserID uint   `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

// SetClusterMembers replaces cluster members (roles: admin/user/viewer)
func SetClusterMembers(c *gin.Context) {
	operator := currentUser(c)
	clusterID, _ := strconv.Atoi(c.Param("id"))
	var cl model.K8sCluster
	if err := model.DB.First(&cl, clusterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}
	if !hasK8sManage(operator) && !clusterAdmin(operator.ID, cl.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该集群的成员管理权限"})
		return
	}
	var req struct {
		Members []clusterMemberReq `json:"members" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	for _, mm := range req.Members {
		if mm.Role != "admin" && mm.Role != "user" && mm.Role != "viewer" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "角色必须是 admin / user / viewer"})
			return
		}
	}
	model.DB.Where("cluster_id = ?", clusterID).Delete(&model.K8sClusterMember{})
	for _, mm := range req.Members {
		model.DB.Create(&model.K8sClusterMember{ClusterID: uint(clusterID), UserID: mm.UserID, Role: mm.Role})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
