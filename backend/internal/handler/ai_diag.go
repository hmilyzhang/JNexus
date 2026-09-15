// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// AI alert diagnostics admin endpoints: config, cleanup catalog CRUD, run-now test

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

// OODiagConfigGet GET /api/system/ai/diag/config
func OODiagConfigGet(c *gin.Context) {
	all := service.SystemConfigMap()
	m := map[string]string{}
	for _, k := range []string{"ai_diag_enabled", "ai_diag_levels", "ai_diag_metrics", "ai_diag_cooldown_min", "ai_diag_prompt"} {
		m[k] = all[k]
	}
	c.JSON(http.StatusOK, m)
}

// OODiagConfigPut PUT /api/system/ai/diag/config
func OODiagConfigPut(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	allowed := map[string]bool{
		"ai_diag_enabled": true, "ai_diag_levels": true, "ai_diag_metrics": true,
		"ai_diag_cooldown_min": true, "ai_diag_prompt": true,
	}
	filtered := map[string]string{}
	for k, v := range req {
		if allowed[k] {
			filtered[k] = v
		}
	}
	if err := service.SetSystemConfigs(filtered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// OODiagCleanupList GET /api/system/ai/diag/cleanup
func OODiagCleanupList(c *gin.Context) {
	c.JSON(http.StatusOK, service.LoadCleanupList())
}

// OODiagCleanupSave POST /api/system/ai/diag/cleanup — replace the whole catalog
func OODiagCleanupSave(c *gin.Context) {
	var items []service.CleanupItem
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := service.SaveCleanupList(items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// OODiagCleanupTest POST /api/system/ai/diag/cleanup/:hostId/run — run the enabled
// cleanup commands on the given host right now (admin, audited via AlertEvent)
func OODiagCleanupTest(c *gin.Context) {
	hostID, _ := strconv.ParseUint(c.Param("hostId"), 10, 64)
	var h model.Host
	if err := model.DB.First(&h, hostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	go service.RunDiagnosis(&h, "MANUAL", "disk", 0, 0)
	c.JSON(http.StatusOK, gin.H{"ok": true, "note": "cleanup started; results are pushed through the bound alert channels"})
}
