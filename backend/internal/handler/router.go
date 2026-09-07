// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"jnexus/internal/config"
	"jnexus/internal/middleware"
	"jnexus/internal/model"
	"jnexus/internal/ws"
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Audit())

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.Static("/uploads", config.Cfg.Storage.UploadDir)

	api := r.Group("/api")
	{
		api.POST("/login", Login)
		api.POST("/login/mfa", LoginMFA)
		api.GET("/system/info", SystemInfo)
		// Web 终端（自带 token 鉴权，不走 hub）
		api.GET("/ws/term/:hostId", WebTerminal)
		api.GET("/ws/k8s/:clusterId", K8sExecWS)
		api.GET("/ws/k8s/logs/:clusterId", K8sLogWS)
		api.GET("/ws/task/:id", ws.Handler(func(c *gin.Context) string {
			return "task-" + c.Param("id")
		}))
		api.GET("/ws/release/:id", ws.Handler(func(c *gin.Context) string {
			return "release-" + c.Param("id")
		}))
	}

	// 外部集成 API（仅 API 密钥认证；密码登录令牌不可用）
	ext := api.Group("/ext", middleware.APIKeyAuth())
	{
		ext.GET("/hosts", ExtHosts)
		ext.GET("/monitors", ExtMonitors)
		ext.GET("/tasks/:id", ExtTask)
		ext.POST("/exec", ExtExec)
	}

	auth := api.Group("", middleware.JWT())
	{
		auth.GET("/me", Me)
		auth.PUT("/me", UpdateMe)
		auth.POST("/change_password", ChangePassword)

		// MFA（TOTP 两步验证）自助管理
		// 监控：应用监控项 + 主机资源
		mon := auth.Group("/monitors", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor))
		{
			mon.GET("", ListMonitors)
			mon.POST("", middleware.RequireRole(model.RoleAdmin, model.RoleOps), CreateMonitor)
			mon.PUT("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateMonitor)
			mon.DELETE("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), DeleteMonitor)
			mon.POST("/:id/test", middleware.RequireRole(model.RoleAdmin, model.RoleOps), TestMonitor)
			mon.GET("/:id/history", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor), MonitorHistory)
		}
		arule := auth.Group("/alert_rules", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor))
		{
			arule.GET("", GetAlertRule)
			arule.PUT("", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateAlertRule)
		}
		apikeys := auth.Group("/api_keys", middleware.RequireRole())
		{
			apikeys.GET("", ListApiKeys)
			apikeys.POST("", middleware.RequireRole(model.RoleAdmin), CreateApiKey)
			apikeys.PUT("/:id", middleware.RequireRole(model.RoleAdmin), UpdateApiKey)
			apikeys.DELETE("/:id", middleware.RequireRole(model.RoleAdmin), DeleteApiKey)
		}

		// K8S 集群管理（查看：所有登录用户中可见者；管理：角色权限/集群管理员）
		k8sg := auth.Group("/k8s/clusters")
		{
			_ = k8sg
			k8sg.GET("", ListK8sClusters)
			k8sg.POST("", middleware.RequireRole(model.RoleAdmin, model.RoleOps), CreateK8sCluster)
			k8sg.PUT("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateK8sCluster)
			k8sg.DELETE("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), DeleteK8sCluster)
			k8sg.POST("/:id/test", middleware.RequireRole(model.RoleAdmin, model.RoleOps), TestK8sCluster)
			k8sg.GET("/:id/nodes", K8sNodes)
			k8sg.GET("/:id/namespaces", K8sNamespaces)
			k8sg.GET("/:id/pods", K8sPods)
			k8sg.GET("/:id/events", K8sEvents)
			k8sg.GET("/:id/configmaps", K8sConfigMaps)
			k8sg.GET("/:id/secrets", K8sSecrets)
			k8sg.DELETE("/:id/configmaps/:namespace/:name", K8sDeleteConfig)
			k8sg.DELETE("/:id/secrets/:namespace/:name", K8sDeleteConfig)
			k8sg.GET("/:id/deployments", K8sDeployments)
			k8sg.POST("/:id/deployments/:namespace/:name/restart", K8sRestartDeployment)
			k8sg.GET("/:id/cronjobs", K8sCronJobs)
			k8sg.POST("/:id/cronjobs", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RoleK8s), K8sCreateCronJob)
			k8sg.PUT("/:id/cronjobs/:namespace/:name/suspend", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RoleK8s), K8sSuspendCronJob)
			k8sg.DELETE("/:id/cronjobs/:namespace/:name", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RoleK8s), K8sDeleteCronJob)
			k8sg.GET("/:id/serviceaccounts", K8sServiceAccounts)
			k8sg.POST("/:id/serviceaccounts/:namespace", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RoleK8s), K8sCreateServiceAccount)
			k8sg.DELETE("/:id/serviceaccounts/:namespace/:name", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RoleK8s), K8sDeleteServiceAccount)
			k8sg.GET("/:id/podlog", K8sPodLog)
			k8sg.DELETE("/:id/pods/:namespace/:name", K8sDeletePod)
			// 管理页只读视图（概览 + 新增资源列表）
			k8sg.GET("/:id/summary", K8sSummary)
			k8sg.GET("/:id/daemonsets", K8sDaemonSets)
			k8sg.GET("/:id/statefulsets", K8sStatefulSets)
			k8sg.GET("/:id/jobs", K8sJobs)
			k8sg.GET("/:id/services", K8sServices)
			k8sg.GET("/:id/ingresses", K8sIngresses)
			k8sg.GET("/:id/pvcs", K8sPVCs)
			k8sg.GET("/:id/pvs", K8sPVs)
			k8sg.GET("/:id/storageclasses", K8sStorageClasses)
			// 二期：YAML 查看 / 伸缩 / 资源使用率 / Helm 发布
			k8sg.GET("/:id/yaml", K8sResourceYAML)
			k8sg.POST("/:id/deployments/:namespace/:name/scale", K8sScaleDeployment)
			k8sg.POST("/:id/statefulsets/:namespace/:name/scale", K8sScaleStatefulSet)
			k8sg.PUT("/:id/yaml", K8sUpdateYAML)
			k8sg.POST("/:id/yaml", K8sCreateYAML)
			k8sg.POST("/:id/delete", K8sDeleteResource)
			k8sg.GET("/:id/usage", K8sClusterUsage)
			k8sg.GET("/:id/capacity/history", K8sCapacityHistory)
			k8sg.GET("/:id/capacity/pods", K8sPodCapacity)
			k8sg.GET("/:id/nodemetrics", K8sNodeMetrics)
			k8sg.GET("/:id/podmetrics", K8sPodMetrics)
			k8sg.GET("/:id/helmreleases", K8sHelmReleases)
			k8sg.GET("/:id/members", ListClusterMembers)
			k8sg.PUT("/:id/members", middleware.RequireRole(model.RoleAdmin, model.RoleOps), SetClusterMembers)
		}

		mwin := auth.Group("/maintenance_windows", middleware.RequireRole())
		{
			mwin.GET("", GetMaintenances)
			mwin.GET("/logs", GetMaintenanceLogs)
			mwin.PUT("", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateMaintenances)
		}

		arule.GET("/cmd", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor), GetCmdLevels)
		arule.GET("/templates", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor), GetAlertTemplates)
		arule.POST("/templates/preview", PreviewAlertTemplates)
		arule.PUT("/templates", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateAlertTemplates)
		arule.PUT("/cmd", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateCmdLevels)

		ach := auth.Group("/alert_channels", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor))
		{
			ach.GET("", ListAlertChannels)
			ach.POST("", middleware.RequireRole(model.RoleAdmin, model.RoleOps), CreateAlertChannel)
			ach.PUT("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), UpdateAlertChannel)
			ach.DELETE("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), DeleteAlertChannel)
			ach.POST("/:id/test", middleware.RequireRole(model.RoleAdmin, model.RoleOps), TestAlertChannel)
		}

		mg := auth.Group("/monitoring", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor))
		{
			mg.GET("/hosts", HostMetricsList)
			mg.GET("/hosts/:id/history", HostMetricHistory)
		}

		auth.GET("/monitoring/hosts/:id/capacity", middleware.RequireRole(model.RoleOps, model.RoleAuditor), HostCapacityHistory)
		rep := auth.Group("/report", middleware.RequireRole(model.RoleOps, model.RoleAuditor))
		{
			rep.GET("/monthly", MonthlyReport)
		}
		auth.GET("/mfa/status", MFAStatus)
		auth.POST("/mfa/setup", MFASetup)
		auth.POST("/mfa/enable", MFAEnable)
		auth.POST("/mfa/disable", MFADisable)

		// 用户管理（admin）
		users := auth.Group("/users", middleware.RequireRole())
		{
			users.GET("", ListUsers)
			users.POST("", CreateUser)
			users.PUT("/:id", UpdateUser)
			users.DELETE("/:id", DeleteUser)
			users.PUT("/:id/grants", SetUserGrants)
			users.GET("/:id/grants", GetUserGrants)
			users.POST("/:id/mfa_reset", AdminResetUserMFA)
			users.POST("/ldap_sync_emails", SyncLdapEmails)
		}

		// 主机分组
		groups := auth.Group("/host_groups", middleware.RequireRole(model.RoleOps, model.RolePublisher))
		{
			groups.GET("", ListGroups)
			groups.POST("", CreateGroup)
			groups.PUT("/:id", UpdateGroup)
			groups.DELETE("/:id", DeleteGroup)
		}

		// SSH 密钥（admin / ops）
		keys := auth.Group("/ssh_keys", middleware.RequireRole(model.RoleOps))
		{
			keys.GET("", ListKeys)
			keys.POST("", CreateKey)
			keys.DELETE("/:id", DeleteKey)
		}

		// 主机
		hosts := auth.Group("/hosts", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer))
		{
			hosts.GET("", ListHosts)
			hosts.POST("", middleware.RequireHostPerm("create"), CreateHost)
			hosts.PUT("/:id", middleware.RequireHostPerm("edit"), UpdateHost)
			hosts.DELETE("/:id", middleware.RequireHostPerm("delete"), DeleteHost)
			hosts.POST("/batch-delete", middleware.RequireRole(model.RoleAdmin, model.RoleOps), BatchDeleteHosts)
			hosts.POST("/import", ImportHosts)
			hosts.POST("/probe", ProbeHostsHandler)
			// OS 账号（凭据）管理
			hosts.GET("/:id/credentials", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer), ListHostCredentials)
			hosts.POST("/:id/credentials", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), CreateHostCredential)
		}
		// 配对密钥列表：管理员/运维可见
		auth.GET("/credentials/paired", middleware.RequireRole(), ListPairedCredentials)

		// 报告模块：权限由角色设置「查看报告」控制
		reports := auth.Group("/reports", middleware.RequireReportPerm())
		{
			reports.GET("/templates", ListReportTemplates)
			reports.GET("", ListReports)
			reports.GET("/:id", GetReport)
			reports.GET("/:id/export", ExportReport)
			reports.POST("", CreateReport)
			reports.DELETE("/:id", middleware.RequireRole(), DeleteReport)
		}

		// 计划任务：管理员/运维可管理
		crons := auth.Group("/crons", middleware.RequireRole(model.RoleOps))
		{
			crons.GET("", ListCrons)
			crons.POST("", CreateCron)
			crons.PUT("/:id", UpdateCron)
			crons.DELETE("/:id", DeleteCron)
			crons.POST("/:id/toggle", ToggleCron)
			crons.POST("/:id/run", RunCronNow)
			crons.GET("/:id/history", CronHistory)
		}

		creds := auth.Group("/credentials", middleware.RequireRole(model.RoleOps, model.RolePublisher))
		{
			creds.GET("", ListAllCredentials)
			// 凭据模板（添加主机/批量导入引用，免重复输密码）
			creds.GET("/templates", ListCredentialTemplates)
			creds.POST("/templates", middleware.RequireRole(model.RoleOps), SaveCredentialTemplate)
			creds.DELETE("/templates/:id", middleware.RequireRole(model.RoleOps), DeleteCredentialTemplate)
			creds.GET("/usable", UsableCredentialsHandler)
			// OS 账号管理：需角色开启「账号管理」权限
			creds.POST("/batch", middleware.RequireRole(model.RoleOps), middleware.RequireHostPerm("edit"), middleware.RequireCredPerm(), BatchAddCredentialsHandler)
			creds.POST("/batch-delete", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), BatchDeleteCredentials)
			creds.PUT("/:id", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), UpdateCredential)
			creds.POST("/:id/default", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), SetDefaultCredential)
			creds.POST("/:id/rotate", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), RotateCredentialNow)
			// 查看密码明文：仅系统管理员（记录审计）
			creds.POST("/:id/reveal", middleware.RequireRole(), RevealCredentialPassword)
			// 删除：仅系统管理员
			creds.DELETE("/:id", middleware.RequireRole(), DeleteCredential)
		}

		// 批量执行
		exec := auth.Group("/exec", middleware.RequireRole(model.RoleOps, model.RolePublisher))
		{
			exec.POST("", StartExec)
		}
		// 执行记录：管理员/审计员看全量，运维/发布员仅本人任务（handler 内过滤）
		tasks := auth.Group("/tasks", middleware.RequireRole(model.RoleAuditor, model.RoleOps, model.RolePublisher, model.RoleViewer))
		{
			tasks.GET("", ListTasks)
			tasks.GET("/:id", GetTask)
			tasks.GET("/:id/export", ExportTask)
		}

		// 文件分发
		files := auth.Group("/files", middleware.RequireRole(model.RoleOps, model.RolePublisher))
		{
			files.POST("/upload", UploadFile)
			files.POST("/distribute", Distribute)
		}

		// 脚本中心
		scripts := auth.Group("/scripts", middleware.RequireRole(model.RoleOps, model.RolePublisher))
		{
			scripts.GET("", ListScripts)
			scripts.POST("", CreateScript)
			scripts.PUT("/:id", UpdateScript)
			scripts.DELETE("/:id", DeleteScript)
			scripts.POST("/:id/exec", ExecScript)
		}

		// 应用与发布
		apps := auth.Group("/apps", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer))
		{
			apps.GET("", ListApps)
			apps.POST("", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RolePublisher), CreateApp)
			apps.PUT("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RolePublisher), UpdateApp)
			apps.DELETE("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleOps), DeleteApp)
		}
		releases := auth.Group("/releases", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer))
		{
			releases.GET("", ListReleases)
			releases.GET("/:id", GetRelease)
			releases.POST("", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RolePublisher), CreateReleaseHandler)
			releases.POST("/:id/rollback", middleware.RequireRole(model.RoleAdmin, model.RoleOps, model.RolePublisher), RollbackHandler)
		}

		// 审计日志：仅管理员/审计员
		audit := auth.Group("/audit", middleware.RequireRole(model.RoleAuditor))
		{
			audit.GET("", ListAudit)
		}

		// 危险命令规则（admin）
		danger := auth.Group("/danger_rules", middleware.RequireRole())
		{
			danger.GET("", ListDangerRules)
			danger.POST("", CreateDangerRule)
			danger.PUT("/:id", UpdateDangerRule)
			danger.DELETE("/:id", DeleteDangerRule)
		}

		// 权限分配数据源
		auth.GET("/grants/options", GrantsOptions)

		// 用户组管理（admin）
		ug := auth.Group("/user_groups", middleware.RequireRole())
		{
			ug.GET("", ListUserGroups)
			ug.POST("", CreateUserGroup)
			ug.PUT("/:id", UpdateUserGroup)
			ug.DELETE("/:id", DeleteUserGroup)
			ug.GET("/:id", GetUserGroup)
			ug.PUT("/:id/links", UpdateUserGroupLinks)
		}

		// Dashboard
		auth.GET("/dashboard", Dashboard)

		// 系统配置（admin）
		auth.GET("/system/roles", GetSystemRoles) // 所有登录用户可读（前端菜单渲染依赖）
		sysCfg := auth.Group("/system", middleware.RequireRole())
		{
			sysCfg.GET("/config", GetSystemConfig)
			sysCfg.PUT("/config", UpdateSystemConfig)
			sysCfg.POST("/ldap/test", TestLDAPConfig)
			sysCfg.POST("/smtp/test", TestSMTPConfig)
			sysCfg.GET("/platform_key", GetPlatformKey)
			sysCfg.PUT("/roles", UpdateSystemRoles)
		}
	}

	// 托管前端 SPA（frontend/dist）：兼容本地开发（backend/ 下相对路径）与容器（/app/frontend/dist）
	candidates := []string{
		filepath.Join("..", "frontend", "dist"), // 本地：从 backend/ 启动
		filepath.Join("frontend", "dist"),       // 容器：WORKDIR /app
		"/app/frontend/dist",                    // 容器：绝对路径兜底
	}
	for _, frontDist := range candidates {
		abs, err := filepath.Abs(frontDist)
		if err != nil {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			continue
		}
		r.StaticFile("/favicon.ico", filepath.Join(abs, "favicon.ico"))
		if apiDoc := filepath.Join(abs, "docs", "api.html"); fileExists(apiDoc) {
			r.GET("/docs/api", func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, "/docs/api.html")
			})
			r.StaticFile("/docs/api.html", apiDoc)
		}
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			// 未知 API 路径返回 404 JSON，不能兜底成 SPA 页面
			if strings.HasPrefix(p, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
				return
			}
			if p != "/" && !strings.HasPrefix(p, "/assets") {
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.File(filepath.Join(abs, "index.html"))
				return
			}
			if strings.HasPrefix(p, "/assets") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			c.File(filepath.Join(abs, strings.TrimPrefix(p, "/")))
		})
		break
	}

	return r
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
