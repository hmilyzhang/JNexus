// JNexus 运维平台 — By JJ Zhang, Version 1.0
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

// K8S 三期：StatefulSet 伸缩 / YAML 更新 / Pod 日志实时跟随（WS 中继）

// K8sScaleStatefulSet StatefulSet 副本伸缩（user 及以上，留痕）
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

// K8sUpdateYAML 用编辑后的 YAML 更新资源（user 及以上，留痕）
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


// K8sClusterUsage 集群资源概况（容量 + 用量 + 全部 Pod 用量，viewer 即可）
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

// K8sLogWS GET /api/ws/k8s/logs/:clusterId?namespace=&pod=&container=&tail=&token=
// Pod 日志实时跟随：集群 API(follow=true) 流式中继到浏览器 WS（viewer 即可）
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
	// WS 路由不在 JWT 中间件组内（浏览器 WS 无法带 Authorization 头），
	// 这里基于已解析的用户手动做集群 viewer 授权
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
	// 客户端断开 → 取消上游请求
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
