// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/service"
)

// K8S 集群管理：
//   - 权限：角色设置 k8s_view（查看）/ k8s_manage（添加/删除集群与成员管理，admin 恒有）
//   - 集群成员（K8SClusterMember）拥有该集群的 admin/user 角色，可查看该集群
//   - 不同集群可绑定不同的成员与支持人

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

// clusterAdmin 判断用户是否为指定集群的管理成员
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

// ListK8sClusters 集群列表（含我的角色与成员数）
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

// applyK8sCluster 处理凭据（kubeconfig 或三件套，AES 加密）
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
	return nil // 更新时允许仅改元数据，凭据保留
}

// CreateK8sCluster 添加集群并立即探测
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

// UpdateK8sCluster 更新集群（元数据/凭据/启停）
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
	// 凭据更新后重置证书到期提醒标记
	if req.Kubeconfig != "" || (req.CA != "" && req.ClientCert != "" && req.ClientKey != "") {
		cl.Warn30Sent, cl.Warn7Sent = false, false
	}
	model.DB.Save(&cl)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteK8sCluster 删除集群（含成员绑定）
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

// TestK8sCluster 立即重测连通性并刷新证书有效期
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

// k8sClusterAccess 组合检查：返回 (集群, API 客户端, 成员角色, 是否允许)
// role 需求：viewer 只读；user 运维操作；admin 集群管理
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

// K8sNodes 集群节点列表
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

// K8sNamespaces 命名空间列表
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

// K8sPods Pod 列表（?namespace=）
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

// K8sPodLog Pod 日志
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

// K8sDeletePod 删除 Pod（需要 user 及以上角色）
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

// K8sDeployments Deployment 列表（?namespace=）
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

// K8sRestartDeployment 重启 Deployment（滚动重启，需要 user 及以上角色）
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

// K8sEvents 集群事件
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

// K8sConfigMaps ConfigMap 列表
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

// K8sSecrets Secret 列表
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

// K8sDeleteConfig 删除 ConfigMap / Secret（kind: configmap / secret）
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


// K8sCronJobs 计划任务列表（?namespace=）
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

// K8sCreateCronJob 创建计划任务
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

// K8sSuspendCronJob 暂停/恢复计划任务
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

// K8sDeleteCronJob 删除计划任务
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

// K8sServiceAccounts 服务账号列表（?namespace=）
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

// K8sCreateServiceAccount 创建服务账号
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

// K8sDeleteServiceAccount 删除服务账号
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

// ListClusterMembers 集群成员列表
// ListClusterMembers 集群成员列表
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

// SetClusterMembers 重写集群成员（角色 admin/user/viewer）
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
