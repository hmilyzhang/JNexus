// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
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

func KeyRotationRun(c *gin.Context) {
	ok, fail, err := service.RotatePlatformSSHKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "hosts_ok": ok, "hosts_fail": fail})
}
