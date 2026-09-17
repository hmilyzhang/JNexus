// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

func KeyRotationConfigGet(c *gin.Context) {
	all := service.SystemConfigMap()
	cfg := service.LoadKeyRotationConfig()
	c.JSON(http.StatusOK, gin.H{
		"ssh_key_rotation_enabled": all["ssh_key_rotation_enabled"],
		"ssh_key_rotation_days":    all["ssh_key_rotation_days"],
		"ssh_key_rotation_last":    all["ssh_key_rotation_last"],
		"days":                     cfg.Days,
		"last_run":                 cfg.LastRun,
		"running":                  service.KeyRotationRunning(),
	})
}

func KeyRotationConfigPut(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	allowed := map[string]bool{"ssh_key_rotation_enabled": true, "ssh_key_rotation_days": true}
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

// KeyRotationRun starts a manual rotation in the background: with hundreds of
// paired hosts the push loop can take minutes, so the request returns at once.
// Progress/completion lands in the audit log via the ssh_key_rotation event.
func KeyRotationRun(c *gin.Context) {
	if service.KeyRotationRunning() {
		c.JSON(http.StatusConflict, gin.H{"error": "rotation already in progress"})
		return
	}
	go func() {
		if _, _, err := service.RotatePlatformSSHKey(); err != nil {
			fmt.Println("[key-rotation] manual run error:", err.Error())
		}
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true, "started": true})
}
