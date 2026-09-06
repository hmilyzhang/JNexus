// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"autoops/internal/service"
)

// StartExec 批量执行命令
func StartExec(c *gin.Context) {
	var req service.ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	u := currentUser(c)
	taskID, hits, err := service.StartBatchExec(u, req)
	if err != nil {
		status := http.StatusBadRequest
		if len(hits) > 0 {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error(), "blocked_by": hits})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": taskID})
}

// ExecScript 从脚本中心发起执行
func ExecScript(c *gin.Context) {
	var req service.ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil || sid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "脚本 ID 非法"})
		return
	}
	scriptID := uint(sid)
	req.ScriptID = &scriptID

	u := currentUser(c)
	taskID, hits, err := service.StartBatchExec(u, req)
	if err != nil {
		status := http.StatusBadRequest
		if len(hits) > 0 {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error(), "blocked_by": hits})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": taskID})
}

// ---- 文件分发 ----

func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func UploadFile(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件"})
		return
	}
	name := sanitizeFileName(fh.Filename)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名非法"})
		return
	}
	dst := service.LocalUploadPath(name)
	if err := c.SaveUploadedFile(fh, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"file": name, "size": fh.Size})
}

func Distribute(c *gin.Context) {
	var req service.DistributeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.LocalFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少已上传文件名"})
		return
	}
	u := currentUser(c)
	taskID, _, err := service.DistributeFile(u, service.LocalUploadPath(req.LocalFile), req.LocalFile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": taskID})
}
