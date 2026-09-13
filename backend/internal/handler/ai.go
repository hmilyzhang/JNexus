// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/service"
)

const aiDefaultSystemPrompt = "You are the JNexus AI assistant for an ops platform. Answer concisely, professionally, and actionably; reply in the same language the user writes in."

// ---- AI roles (stored in system config ai_roles as a JSON array; can be added/deleted/edited) ----

type aiRole struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// aiBuiltinRoles built-in role seeds (seeded once only when ai_roles is not yet set; afterwards config takes precedence, all roles editable/deletable)
func aiBuiltinRoles() []aiRole {
	return []aiRole{
		{Key: "sre", Name: "SRE Reliability Engineer", Prompt: "You are a senior SRE (Site Reliability Engineer) skilled at troubleshooting, root cause analysis, capacity planning and SLO design. Give actionable answers with concrete commands and troubleshooting steps."},
		{Key: "dba", Name: "DBA Administrator", Prompt: "You are a senior database administrator (DBA) skilled at MySQL/PostgreSQL/Redis operations, SQL optimization, backup and recovery, replication and slow query analysis. Prioritize safety and warn before destructive operations."},
		{Key: "devops", Name: "DevOps Engineer", Prompt: "You are a DevOps engineer skilled at CI/CD, containerization, infrastructure as code and operations automation. Focus on efficiency and best practices."},
		{Key: "security", Name: "Security Analyst", Prompt: "You are a security analyst skilled at vulnerability assessment, intrusion detection, hardening advice and compliance audit. Focus on risk levels and remediation priority."},
		{Key: "general", Name: "General Assistant", Prompt: "You are a general operations assistant who answers technical questions and ops scenario consultations."},
	}
}

// aiRoleList reads the role list; seeds built-in roles when ai_roles is missing (migrates legacy ai_system_prompt to general)
func aiRoleList() []aiRole {
	m := service.SystemConfigMap()
	if raw := strings.TrimSpace(m["ai_roles"]); raw != "" {
		var roles []aiRole
		if json.Unmarshal([]byte(raw), &roles) == nil {
			return roles
		}
	}
	roles := aiBuiltinRoles()
	if p := strings.TrimSpace(m["ai_system_prompt"]); p != "" {
		for i := range roles {
			if roles[i].Key == "general" {
				roles[i].Prompt = p
			}
		}
	}
	if b, err := json.Marshal(roles); err == nil {
		_ = service.SetSystemConfigs(map[string]string{"ai_roles": string(b)})
	}
	return roles
}

// aiRolePrompt returns the role prompt by key, or empty if not found
func aiRolePrompt(key string) string {
	for _, r := range aiRoleList() {
		if r.Key == key {
			return r.Prompt
		}
	}
	return ""
}

// GetAIRoles GET /api/ai/roles — role list (readable by logged-in users, for chat selection)
func GetAIRoles(c *gin.Context) {
	c.JSON(http.StatusOK, aiRoleList())
}

// UpdateAIRoles POST /api/ai/roles — save the whole role table (admin; frontend submits after add/delete/rename/prompt edits)
func UpdateAIRoles(c *gin.Context) {
	var roles []aiRole
	if err := c.ShouldBindJSON(&roles); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	out := make([]aiRole, 0, len(roles))
	seen := map[string]bool{}
	for _, r := range roles {
		r.Key = strings.TrimSpace(r.Key)
		r.Name = strings.TrimSpace(r.Name)
		if r.Key == "" || seen[r.Key] {
			continue
		}
		seen[r.Key] = true
		if r.Name == "" {
			r.Name = r.Key
		}
		out = append(out, r)
	}
	b, err := json.Marshal(out)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := service.SetSystemConfigs(map[string]string{"ai_roles": string(b)}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "roles": out})
}

// getSystemPrompt reads the default System Prompt (used when no role is selected), falling back to the built-in default
func getSystemPrompt() string {
	m := service.SystemConfigMap()
	if p := strings.TrimSpace(m["ai_system_prompt"]); p != "" {
		return p
	}
	return aiDefaultSystemPrompt
}

// pageContexts route path → page description (helps the AI know which module the user is in)
var pageContexts = map[string]string{
	"/dashboard":   "Dashboard: system overview",
	"/hosts":       "Hosts: manage SSH/WinRM hosts, groups and credentials",
	"/os-accounts": "OS Accounts: manage OS accounts on hosts (passwords/keys) and password rotation",
	"/exec":        "Command Execution: run Shell/PowerShell commands across multiple hosts",
	"/files":       "File Distribution: batch upload files to multiple hosts",
	"/scripts":     "Scripts: manage and run operations scripts",
	"/crons":       "Scheduled Jobs: cron job management and scheduling",
	"/reports":     "Reports: view and export monthly operation reports and check results",
	"/monitor":     "Monitoring: host CPU/memory/disk monitoring, probes, alert rules, maintenance windows, monthly report",
	"/k8s":         "K8S Clusters: multi-cluster management and switching",
	"/apps":        "Applications: app catalog and deployment config",
	"/releases":    "Release Center: release records, rollback and history",
	"/users":       "Users: create/disable users and assign roles",
	"/danger":      "Dangerous Commands: manage dangerous command interception rules",
	"/audit":       "Audit Log: full operation audit and search",
	"/system":      "System Settings: AI assistant, LDAP, SMTP email, password rotation policy, platform keys",
	"/shell":       "Web Shell: multi-tab SSH/WinRM interactive terminal",
	"/logtail":     "Log Viewer: follow new content of host log files in real time",
	"/profile":     "Profile: personal info and TOTP MFA",
	"/login":       "Login page",
	"/k8s/manage":  "K8S Cluster Management: nodes, workloads, pods, services, config, Helm, capacity planning",
}

// AIChat POST /api/ai/chat  {prompt, page, role} — AI chat
// page: current route path auto-sent by the frontend (e.g. /monitor), letting the AI sense the user's module
// role: role key selected in the frontend chat box; if matched, that role's System Prompt is used
func AIChat(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
		Page   string `json:"page"`
		Role   string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}
	s := service.LoadAISettings()
	if !s.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI is not enabled (System Settings → AI Assistant)"})
		return
	}

	systemP := getSystemPrompt()
	if req.Role != "" {
		if p := aiRolePrompt(req.Role); p != "" {
			systemP = p
		}
	}
	// Page awareness: inject module context based on the current route
	if desc, ok := pageContexts[req.Page]; ok {
		systemP += "\n\n[用户当前所在页面] " + desc
	}

	start := time.Now()
	reply, err := service.AIChat(s, systemP, strings.TrimSpace(req.Prompt))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reply": reply, "elapsed_ms": time.Since(start).Milliseconds()})
}

// AITest POST /api/system/ai/test — admin tests AI service connectivity
func AITest(c *gin.Context) {
	s := service.LoadAISettings()
	if s.BaseURL == "" || s.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Set the base URL and model first"})
		return
	}
	start := time.Now()
	reply, err := service.AIChat(s, "connectivity test", "Reply: OK")
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error(), "elapsed_ms": elapsed})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "reply": reply, "elapsed_ms": elapsed})
}
