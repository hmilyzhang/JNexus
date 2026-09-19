// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Ticketing integrations (admin config + ticket creation): ServiceNow cloud
// and ManageEngine ServiceDesk Plus on-premise. Config values live in system
// config; secrets are stored encrypted and masked over the API.

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// TicketingConfigGet GET /api/system/ticketing — masked config for the UI
func TicketingConfigGet(c *gin.Context) {
	sn := service.LoadServiceNowConfig()
	sdp := service.LoadSDPConfig()
	c.JSON(http.StatusOK, gin.H{
		"servicenow": gin.H{
			"url":   sn.URL,
			"user":  sn.User,
			"pass":  maskSecret(sn.Password),
		},
		"sdp": gin.H{
			"url":       sdp.URL,
			"token_set": sdp.Token != "",
			"requester": sdp.Requester,
		},
		"types": service.TicketTypesFor("servicenow"),
		"types_sdp": service.TicketTypesFor("sdp"),
	})
}

// TicketingConfigPut PUT /api/system/ticketing — save provider settings
func TicketingConfigPut(c *gin.Context) {
	var req struct {
		ServiceNow struct {
			URL      string `json:"url"`
			User     string `json:"user"`
			Password string `json:"password"` // masked = keep existing
		} `json:"servicenow"`
		SDP struct {
			URL       string `json:"url"`
			Token     string `json:"token"` // masked = keep existing
			Requester string `json:"requester"`
		} `json:"sdp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]string{}
	if u := strings.TrimSpace(req.ServiceNow.URL); u != "" || req.ServiceNow.User != "" {
		updates["ticket_servicenow_url"] = strings.TrimRight(strings.TrimSpace(req.ServiceNow.URL), "/")
		updates["ticket_servicenow_user"] = strings.TrimSpace(req.ServiceNow.User)
	}
	if p := req.ServiceNow.Password; p != "" && p != "******" {
		if enc, err := pkg.Encrypt(p); err == nil {
			updates["ticket_servicenow_pass"] = enc
		}
	}
	if u := strings.TrimSpace(req.SDP.URL); u != "" {
		updates["ticket_sdp_url"] = strings.TrimRight(u, "/")
	}
	if t := strings.TrimSpace(req.SDP.Token); t != "" && t != "******" {
		if enc, err := pkg.Encrypt(t); err == nil {
			updates["ticket_sdp_token"] = enc
		}
	}
	updates["ticket_sdp_requester"] = strings.TrimSpace(req.SDP.Requester)
	if err := service.SetSystemConfigs(updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TicketingTest POST /api/system/ticketing/test {provider} — verify connection
func TicketingTest(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var err error
	switch req.Provider {
	case "servicenow":
		err = service.TestServiceNow(service.LoadServiceNowConfig())
	case "sdp":
		err = service.TestSDP(service.LoadSDPConfig())
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知系统"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TicketCreate POST /api/tickets — create a ticket in the chosen system
func TicketCreate(c *gin.Context) {
	u := currentUser(c)
	var req struct {
		Provider    string `json:"provider" binding:"required"`
		Type        string `json:"type" binding:"required"`
		Priority    string `json:"priority"`
		Subject     string `json:"subject" binding:"required"`
		Description string `json:"description"`
		RequestedBy string `json:"requested_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Subject) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（系统/类型/标题必填）"})
		return
	}
	if !service.IsValidTicketType(req.Provider, req.Type) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该系统不支持此工单类型: " + req.Type})
		return
	}
	priority := req.Priority
	if priority == "" {
		priority = "moderate"
	}

	var number, link string
	var err error
	switch req.Provider {
	case "servicenow":
		cfg := service.LoadServiceNowConfig()
		if cfg.URL == "" || cfg.User == "" || cfg.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ServiceNow 未配置完整（地址/用户/密码）"})
			return
		}
		number, link, err = service.CreateServiceNowTicket(cfg, req.Type, priority, req.Subject, req.Description)
	case "sdp":
		cfg := service.LoadSDPConfig()
		if cfg.URL == "" || cfg.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ServiceDesk Plus 未配置完整（地址/技术员 Key）"})
			return
		}
		requester := req.RequestedBy
		if requester == "" {
			requester = u.Username
		}
		number, link, err = service.CreateSDPTicket(cfg, req.Type, priority, req.Subject, req.Description, requester)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知工单系统: " + req.Provider})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "number": number, "url": link})
}

// maskSecret shows a placeholder when a secret exists
func maskSecret(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return "******"
}
