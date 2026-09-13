// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// OpenObserve integration endpoints: connection test (admin) and a search proxy
// (monitor viewers) so end users never talk to OpenObserve directly.

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

// OOConfigTest POST /api/system/oo/test — admin connection test for the stored settings
func OOConfigTest(c *gin.Context) {
	if err := service.OOConfigTest(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// OOSearchProxy POST /api/monitors/oo/search — proxy a SQL search to OpenObserve.
// Body: {sql, start_ms, end_ms, from, size}; time defaults to the last 24h.
func OOSearchProxy(c *gin.Context) {
	var req struct {
		SQL     string `json:"sql"`
		StartMs int64  `json:"start_ms"`
		EndMs   int64  `json:"end_ms"`
		From    int    `json:"from"`
		Size    int    `json:"size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SQL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sql is required"})
		return
	}
	if req.EndMs == 0 {
		req.EndMs = time.Now().UnixMilli()
	}
	if req.StartMs == 0 {
		req.StartMs = req.EndMs - 24*3600*1000
	}
	if req.From < 0 {
		req.From = 0
	}
	if req.Size <= 0 || req.Size > 1000 {
		req.Size = 100
	}
	out, err := service.OOSearch(req.SQL, req.StartMs, req.EndMs, req.From, req.Size)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
