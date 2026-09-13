// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// OpenObserve integration endpoints: connection test (admin) and a search proxy
// (monitor viewers) so end users never talk to OpenObserve directly.

import (
	"encoding/json"
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

// OOState GET /api/observe/state — lightweight enabled flag for menu visibility
// (any logged-in user; leaks nothing but the boolean)
func OOState(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": service.LoadOOSettings().Enabled})
}

// OOAdminStatus GET /api/system/oo/status — connection health + per-stream push stats (admin)
func OOAdminStatus(c *gin.Context) {
	c.JSON(http.StatusOK, service.OOStatus())
}

// OOAdminToggle POST /api/system/oo/integrations {stream, enabled} — per-stream toggle (admin)
func OOAdminToggle(c *gin.Context) {
	var req struct {
		Stream  string `json:"stream"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Stream == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stream is required"})
		return
	}
	if !service.OOStreamNameValid(req.Stream) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stream name"})
		return
	}
	if err := service.OOSetIntegration(req.Stream, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// OOAdminPush POST /api/system/oo/push {stream, records} — test push from the admin page
func OOAdminPush(c *gin.Context) {
	var req struct {
		Stream  string           `json:"stream"`
		Records []map[string]any `json:"records"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Stream == "" || len(req.Records) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stream and records are required"})
		return
	}
	if !service.OOStreamNameValid(req.Stream) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stream name"})
		return
	}
	n, err := service.OOIngestCustom(req.Stream, req.Records)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "successful": n})
}

// OOMonitorStreams GET /api/monitors/oo/streams — stream discovery for the search page
func OOMonitorStreams(c *gin.Context) {
	names, err := service.OOListStreams()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"streams": names})
}

// ExtOOPush POST /api/ext/oo/:stream — external custom push with API-key auth
func ExtOOPush(c *gin.Context) {
	stream := c.Param("stream")
	if !service.OOStreamNameValid(stream) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid stream name"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "JSON body required"})
		return
	}
	var records []map[string]any
	if json.Unmarshal(raw, &records) != nil {
		var one map[string]any
		if json.Unmarshal(raw, &one) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "body must be a JSON object or array of objects"})
			return
		}
		records = []map[string]any{one}
	}
	n, err := service.OOIngestCustom(stream, records)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "successful": n})
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
