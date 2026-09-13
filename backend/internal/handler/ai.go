// JNexus 运维平台 — By JJ Zhang, Version 1.0
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

// ---- AI 角色（存于系统配置 ai_roles，JSON 数组；可添加/删除/编辑） ----

type aiRole struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// aiBuiltinRoles 内置角色种子（仅 ai_roles 尚未写入时播种一次，之后以配置为准，均可编辑删除）
func aiBuiltinRoles() []aiRole {
	return []aiRole{
		{Key: "sre", Name: "SRE Reliability Engineer", Prompt: "You are a senior SRE (Site Reliability Engineer) skilled at troubleshooting, root cause analysis, capacity planning and SLO design. Give actionable answers with concrete commands and troubleshooting steps."},
		{Key: "dba", Name: "DBA Administrator", Prompt: "You are a senior database administrator (DBA) skilled at MySQL/PostgreSQL/Redis operations, SQL optimization, backup and recovery, replication and slow query analysis. Prioritize safety and warn before destructive operations."},
		{Key: "devops", Name: "DevOps Engineer", Prompt: "You are a DevOps engineer skilled at CI/CD, containerization, infrastructure as code and operations automation. Focus on efficiency and best practices."},
		{Key: "security", Name: "Security Analyst", Prompt: "You are a security analyst skilled at vulnerability assessment, intrusion detection, hardening advice and compliance audit. Focus on risk levels and remediation priority."},
		{Key: "general", Name: "General Assistant", Prompt: "You are a general operations assistant who answers technical questions and ops scenario consultations."},
	}
}

// aiRoleList 读取角色列表；ai_roles 缺失时用内置角色播种（迁移旧 ai_system_prompt 到 general）
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

// aiRolePrompt 按 key 取角色 Prompt，未命中返回空
func aiRolePrompt(key string) string {
	for _, r := range aiRoleList() {
		if r.Key == key {
			return r.Prompt
		}
	}
	return ""
}

// GetAIRoles GET /api/ai/roles — 角色列表（登录用户可读，供对话选择）
func GetAIRoles(c *gin.Context) {
	c.JSON(http.StatusOK, aiRoleList())
}

// UpdateAIRoles POST /api/ai/roles — 整表保存角色（admin，前端添加/删除/改名/改 Prompt 后提交）
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

// getSystemPrompt 读取默认 System Prompt（未选角色时使用），无则用内置默认
func getSystemPrompt() string {
	m := service.SystemConfigMap()
	if p := strings.TrimSpace(m["ai_system_prompt"]); p != "" {
		return p
	}
	return aiDefaultSystemPrompt
}

// pageContexts 路由路径 → 页面描述（让 AI 了解用户当前所在模块）
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

// AIChat POST /api/ai/chat  {prompt, page, role} — AI 对话
// page: 前端自动传入的当前路由路径（如 /monitor），让 AI 感知用户所在模块
// role: 前端对话框选择的角色 key，命中则用该角色的 System Prompt
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
	// 页面感知：根据当前路由注入模块上下文
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

// AITest POST /api/system/ai/test — 管理员测试 AI 服务连通性
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
