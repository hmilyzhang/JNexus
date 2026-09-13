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

const aiDefaultSystemPrompt = "你是 JNexus 运维平台的 AI 助手。回答简洁、专业、可执行；使用与用户提问相同的语言。"

// ---- AI 角色（存于系统配置 ai_roles，JSON 数组；可添加/删除/编辑） ----

type aiRole struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// aiBuiltinRoles 内置角色种子（仅 ai_roles 尚未写入时播种一次，之后以配置为准，均可编辑删除）
func aiBuiltinRoles() []aiRole {
	return []aiRole{
		{Key: "sre", Name: "SRE 可靠性工程师", Prompt: "你是一名资深 SRE（站点可靠性工程师），擅长故障排查、根因分析、容量规划和 SLO 制定。回答注重可操作性，给出具体命令和排查步骤。"},
		{Key: "dba", Name: "DBA 数据库管理员", Prompt: "你是一名资深数据库管理员（DBA），擅长 MySQL/PostgreSQL/Redis 的运维、SQL 优化、备份恢复、主从复制和慢查询分析。回答注重安全性，涉及破坏性操作时提醒确认。"},
		{Key: "devops", Name: "DevOps 工程师", Prompt: "你是一名 DevOps 工程师，擅长 CI/CD、容器化、基础设施即代码和自动化运维。回答注重效率和最佳实践。"},
		{Key: "security", Name: "安全分析师", Prompt: "你是一名安全分析师，擅长漏洞评估、入侵检测、加固建议和合规审计。回答注重风险等级和修复优先级。"},
		{Key: "general", Name: "通用助手", Prompt: "你是一名通用运维助手，能回答各类技术问题和运维场景咨询。"},
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := service.SetSystemConfigs(map[string]string{"ai_roles": string(b)}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
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
	"/dashboard":   "仪表盘：系统总览",
	"/hosts":       "主机管理：查看和管理 SSH/WinRM 主机、分组、凭据",
	"/os-accounts": "OS 账号：管理主机上的操作系统账号（密码/密钥）、密码轮换",
	"/exec":        "命令执行：批量在多台主机上执行 Shell/PowerShell 命令",
	"/files":       "文件分发：批量上传文件到多台主机",
	"/scripts":     "脚本中心：管理和执行运维脚本",
	"/crons":       "计划任务：Cron 定时任务管理和调度",
	"/reports":     "运营报告：查看和导出月度运营报告、检查项结果",
	"/monitor":     "监控中心：主机 CPU/内存/磁盘 监控、拨测、告警规则、维护窗口、月度报告",
	"/k8s":         "K8S 集群：多集群管理和切换",
	"/apps":        "应用管理：应用目录和部署配置",
	"/releases":    "发布中心：发布记录、回滚和发布历史",
	"/users":       "用户管理：创建/禁用用户、分配角色",
	"/danger":      "危险命令：管理危险命令拦截规则",
	"/audit":       "审计日志：全量操作审计和检索",
	"/system":      "系统设置：AI 助手、LDAP、SMTP 邮件、密码轮换策略、平台密钥",
	"/shell":       "Web 终端：多标签 SSH/WinRM 交互式终端",
	"/logtail":     "日志跟随：实时查看主机日志文件新增内容",
	"/profile":     "个人中心：个人信息和 MFA 两步验证",
	"/login":       "登录页",
	"/k8s/manage":  "K8S 集群管理：节点、工作负载、Pod、服务、配置、Helm、容量规划",
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt 必填"})
		return
	}
	s := service.LoadAISettings()
	if !s.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未启用（系统设置 → AI 助手）"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写服务地址与模型"})
		return
	}
	start := time.Now()
	reply, err := service.AIChat(s, "测试连通性", "回复：OK")
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error(), "elapsed_ms": elapsed})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "reply": reply, "elapsed_ms": elapsed})
}
