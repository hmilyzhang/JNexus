// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
)

// K8S 二期能力接口：YAML 查看 / Deployment 伸缩 / 资源使用率 / Helm 发布视图

// K8sResourceYAML 资源 YAML 查看（只读，viewer 即可）
func K8sResourceYAML(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	yml, err := api.ResourceYAML(c.Query("kind"), c.Query("namespace"), c.Query("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"yaml": yml})
}

// K8sScaleDeployment Deployment 副本伸缩（user 及以上，留痕）
func K8sScaleDeployment(c *gin.Context) {
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
	if err := api.ScaleDeployment(ns, name, *req.Replicas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "K8S", Resource: "SCALE DEPLOYMENT " + ns + "/" + name + " -> " + strconv.Itoa(*req.Replicas) + " @ " + cl.Name,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// K8sNodeMetrics 节点资源用量（未装 metrics-server 返回空数组）
func K8sNodeMetrics(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.NodeMetrics()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sPodMetrics Pod 资源用量（?namespace=）
func K8sPodMetrics(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.PodMetrics(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sHelmReleases Helm 发布列表（只读）
func K8sHelmReleases(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.HelmReleases(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}
