// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Ticketing integrations: create tickets of a specified type in ServiceNow
// (cloud) or ManageEngine ServiceDesk Plus (on-premise). Connection settings
// live in system config; both providers can be configured side by side and
// the creator picks the target system per ticket.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"jnexus/internal/pkg"
)

// unified ticket priorities
var ticketPriorities = map[string]string{
	"critical": "1", "high": "2", "moderate": "3", "low": "4",
}

// sdpPriorityNames maps unified priorities to SDP v3 priority names
var sdpPriorityNames = map[string]string{
	"critical": "Very High", "high": "High", "moderate": "Moderate", "low": "Low",
}

// ticket type catalogs per provider (UI mirrors these)
var ticketTypes = map[string][]string{
	"servicenow": {"incident", "problem", "change_request"},
	"sdp":        {"request", "incident", "problem", "change"},
}

// TicketTypesFor returns the valid ticket types for a provider
func TicketTypesFor(provider string) []string {
	return ticketTypes[provider]
}

// IsValidTicketType checks the type against the provider catalog
func IsValidTicketType(provider, t string) bool {
	for _, v := range ticketTypes[provider] {
		if v == t {
			return true
		}
	}
	return false
}

// ---- configuration ----

type ServiceNowConfig struct {
	URL      string
	User     string
	Password string // decrypted
}

type SDPConfig struct {
	URL       string
	Token     string // technician API key
	Requester string // requester name recorded on tickets
}

// LoadServiceNowConfig reads ServiceNow settings (password decrypted)
func LoadServiceNowConfig() ServiceNowConfig {
	m := SystemConfigMap()
	pass, _ := pkg.Decrypt(m["ticket_servicenow_pass"])
	return ServiceNowConfig{
		URL:      strings.TrimRight(strings.TrimSpace(m["ticket_servicenow_url"]), "/"),
		User:     strings.TrimSpace(m["ticket_servicenow_user"]),
		Password: pass,
	}
}

// LoadSDPConfig reads ServiceDesk Plus settings (token decrypted)
func LoadSDPConfig() SDPConfig {
	m := SystemConfigMap()
	tok, _ := pkg.Decrypt(m["ticket_sdp_token"])
	return SDPConfig{
		URL:       strings.TrimRight(strings.TrimSpace(m["ticket_sdp_url"]), "/"),
		Token:     tok,
		Requester: strings.TrimSpace(m["ticket_sdp_requester"]),
	}
}

// ---- ServiceNow ----

type nowResult struct {
	Result struct {
		SysID   string `json:"sys_id"`
		Number  string `json:"number"`
		Link    string `json:"links,omitempty"`
		Details string `json:"error,omitempty"`
	} `json:"result"`
}

func snRequest(cfg ServiceNowConfig, method, path string, body any) (int, []byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(method, cfg.URL+path, bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(cfg.User, cfg.Password)
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: OutboundTLS()}}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, nil
}

// TestServiceNow verifies the instance URL and credentials
func TestServiceNow(cfg ServiceNowConfig) error {
	if cfg.URL == "" || cfg.User == "" {
		return fmt.Errorf("请填写 ServiceNow 地址与用户")
	}
	code, data, err := snRequest(cfg, http.MethodGet, "/api/now/table/incident?sysparm_limit=1&sysparm_fields=sys_id", nil)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	if code == 401 {
		return fmt.Errorf("认证失败（401）：请核对用户与密码")
	}
	if code >= 400 {
		return fmt.Errorf("ServiceNow 返回 HTTP %d: %s", code, truncateOO(data))
	}
	return nil
}

// CreateServiceNowTicket creates a ticket in the given table
func CreateServiceNowTicket(cfg ServiceNowConfig, table, priority, subject, description string) (number, link string, err error) {
	if table == "" {
		table = "incident"
	}
	body := map[string]any{
		"short_description": subject,
		"description":       description,
		"impact":            "3",
		"urgency":           "2",
	}
	if p, ok := ticketPriorities[strings.ToLower(priority)]; ok && p != "" {
		body["priority"] = p
		body["urgency"] = map[string]string{"1": "1", "2": "1", "3": "2", "4": "3"}[p]
		body["impact"] = map[string]string{"1": "1", "2": "2", "3": "2", "4": "3"}[p]
	}
	code, data, err := snRequest(cfg, http.MethodPost, "/api/now/table/"+table, body)
	if err != nil {
		return "", "", fmt.Errorf("连接失败: %w", err)
	}
	if code >= 400 {
		return "", "", fmt.Errorf("ServiceNow HTTP %d: %s", code, truncateOO(data))
	}
	var out nowResult
	if json.Unmarshal(data, &out) == nil && out.Result.Number != "" {
		return out.Result.Number, cfg.URL + "/" + table + ".do?sys_id=" + out.Result.SysID, nil
	}
	return "", "", nil
}

// ---- ServiceDesk Plus (on-premise) ----

func sdpRequest(cfg SDPConfig, method, path string, body any) (int, []byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(method, cfg.URL+path, bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authtoken", cfg.Token)
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: OutboundTLS()}}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, nil
}

// TestSDP verifies the URL and technician key
func TestSDP(cfg SDPConfig) error {
	if cfg.URL == "" || cfg.Token == "" {
		return fmt.Errorf("请填写 ServiceDesk Plus 地址与技术员 Key")
	}
	code, data, err := sdpRequest(cfg, http.MethodGet, "/api/v3/requests?list_size=1", nil)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	if code == 401 {
		return fmt.Errorf("认证失败（401）：请核对技术员 Key")
	}
	if code >= 400 {
		return fmt.Errorf("ServiceDesk Plus 返回 HTTP %d: %s", code, truncateOO(data))
	}
	return nil
}

// CreateSDPTicket creates a request/incident/problem/change in SDP (v3 API)
func CreateSDPTicket(cfg SDPConfig, module, priority, subject, description, requester string) (idStr, link string, err error) {
	if requester == "" {
		requester = "administrator"
	}
	if module == "" {
		module = "request"
	}
	path := "/api/v3/" + module + "s"
	prioName := sdpPriorityNames[strings.ToLower(priority)]
	reqBody := map[string]any{
		"request": map[string]any{
			"subject":     subject,
			"description": description,
			"requester":   map[string]any{"name": requester},
			"status":      map[string]any{"name": "Open"},
		},
	}
	if prioName != "" {
		reqBody["request"].(map[string]any)["priority"] = map[string]any{"name": prioName}
	}
	code, data, err := sdpRequest(cfg, http.MethodPost, path, reqBody)
	if err != nil {
		return "", "", fmt.Errorf("连接失败: %w", err)
	}
	if code >= 400 {
		return "", "", fmt.Errorf("ServiceDesk Plus HTTP %d: %s", code, truncateOO(data))
	}
	var out struct {
		Request struct {
			ID   int64 `json:"id"`
			Path string `json:"path"`
		} `json:"request"`
	}
	if json.Unmarshal(data, &out) == nil && out.Request.ID != 0 {
		return fmt.Sprintf("%d", out.Request.ID), cfg.URL + "/WorkOrder.do?woID=" + fmt.Sprintf("%d", out.Request.ID), nil
	}
	return "", "", nil
}
