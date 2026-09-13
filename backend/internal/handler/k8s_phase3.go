// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// K8S phase 3: StatefulSet scaling / YAML update / real-time pod log streaming (WS relay)

// K8sScaleStatefulSet scales StatefulSet replicas (user role or above; audit-logged)
func K8sScaleStatefulSet(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ns, name := c.Param("namespace"), c.Param("name")
	var req struct {
		Replicas *int `json:"replicas"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Replicas == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "replicas 必填"})
		return
	}
	if *req.Replicas < 0 || *req.Replicas > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "replicas 取值 0-1000"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.ScaleStatefulSet(ns, name, *req.Replicas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "SCALE STATEFULSET " + ns + "/" + name + " -> " + strconv.Itoa(*req.Replicas) + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sUpdateYAML updates a resource with edited YAML (user role or above; audit-logged)
func K8sUpdateYAML(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		YAML      string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Kind == "" || req.Name == "" || req.YAML == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind/name/yaml 必填"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.UpdateResourceYAML(req.Kind, req.Namespace, req.Name, req.YAML); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "UPDATE YAML " + req.Kind + " " + req.Namespace + "/" + req.Name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sClusterUsage returns the cluster resource overview (capacity + usage + all pod usage; viewer role is enough)
func K8sClusterUsage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	u, err := api.ClusterUsage()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

// K8sCreateYAML creates a resource from YAML (user role or above; audit-logged)
func K8sCreateYAML(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		YAML      string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Kind == "" || req.YAML == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind/yaml 必填"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	created, err := api.CreateResourceYAML(req.Kind, req.Namespace, req.YAML)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "CREATE YAML " + req.Kind + " " + created + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "created": created})
}

// K8sDeleteResource generic resource deletion (user role or above; audit-logged; cluster-scoped resources rejected)
func K8sDeleteResource(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Kind == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind/name 必填"})
		return
	}
	cl, api, _, ok := k8sClusterAccess(c, id, "user")
	if !ok {
		return
	}
	if err := api.DeleteResource(req.Kind, req.Namespace, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "DELETE " + req.Kind + " " + req.Namespace + "/" + req.Name + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sLogWS GET /api/ws/k8s/logs/:clusterId?namespace=&pod=&container=&tail=&token=
// Real-time pod log streaming: relays the cluster API stream (follow=true) to the browser WS (viewer role is enough)
func K8sLogWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		if h := c.GetHeader("Authorization"); len(h) > 7 {
			token = h[7:]
		}
	}
	claims, err := pkg.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	var user model.User
	if err := model.DB.First(&user, claims.UserID).Error; err != nil || user.Status != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不可用"})
		return
	}
	clusterID, _ := strconv.Atoi(c.Param("clusterId"))
	var cl model.K8sCluster
	if err := model.DB.First(&cl, clusterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}
	ns, pod := c.Query("namespace"), c.Query("pod")
	if ns == "" || pod == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace/pod 必填"})
		return
	}
	// WS routes are not in the JWT middleware group (browser WS cannot send an Authorization header),
	// so cluster viewer authorization is done manually here using the parsed user
	var mm model.K8sClusterMember
	model.DB.Where("cluster_id = ? AND user_id = ?", cl.ID, user.ID).First(&mm)
	myRole := mm.Role
	if user.IsAdmin() || service.HasK8sPerm(user.Role, "manage") {
		myRole = "admin"
	} else if service.HasK8sPerm(user.Role, "view") && myRole == "" {
		myRole = "viewer"
	}
	if myRole == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该集群查看权限"})
		return
	}
	api, err := service.K8sClientFor(&cl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	container := c.Query("container")
	tail, _ := strconv.Atoi(c.Query("tail"))
	if tail <= 0 {
		tail = 100
	}

	ws, err := k8sExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Client disconnect → cancel the upstream request
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	body, err := api.FollowPodLog(ctx, ns, pod, container, tail)
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("["+err.Error()+"]"))
		return
	}
	defer body.Close()

	buf := make([]byte, 8192)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if werr := ws.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if ctx.Err() == nil {
				ws.WriteMessage(websocket.TextMessage, []byte("[stream closed]"))
			}
			return
		}
	}
}
