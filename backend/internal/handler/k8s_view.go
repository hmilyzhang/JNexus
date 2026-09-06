// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// K8S 管理页只读视图接口（viewer 权限即可）

// K8sSummary 集群概览统计
func K8sSummary(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cl, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	s, err := api.Summary()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cluster": cl.Name, "summary": s})
}

// K8sDaemonSets DaemonSet 列表
func K8sDaemonSets(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.DaemonSets(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sStatefulSets StatefulSet 列表
func K8sStatefulSets(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.StatefulSets(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sJobs Job 列表
func K8sJobs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.Jobs(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sServices Service 列表
func K8sServices(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.Services(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sIngresses Ingress 列表
func K8sIngresses(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.Ingresses(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sPVCs PVC 列表
func K8sPVCs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.PVCs(c.Query("namespace"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sPVs PV 列表
func K8sPVs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.PVs()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// K8sStorageClasses StorageClass 列表
func K8sStorageClasses(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, api, _, ok := k8sClusterAccess(c, id, "viewer")
	if !ok {
		return
	}
	list, err := api.StorageClasses()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}
