// JNexus Ops Platform — By JJ Zhang, Version 1.0

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
	// RDP gateway WebSocket proxy (same-origin: browser needs no direct gateway access;
	// the one-time encrypted q= token is the credential)
	r.GET("/rdp-gw", ProxyRDPGateway)
	r.GET("/rdp-gw/", ProxyRDPGateway) // tolerate the trailing-slash variant (a 301 here would kill the WS handshake)
	r.Static("/uploads", config.Cfg.Storage.UploadDir)

		api := r.Group("/api")
		{
			api.POST("/login", Login)
			api.POST("/login/mfa", LoginMFA)
			// Web asset headless-browser session (token auth via query; validated in-handler)
			api.GET("/webassets/:id/stream", StreamWebAsset)
			api.GET("/system/info", SystemInfo)
		// Web terminal (has its own token auth; does not go through the hub)
		api.GET("/ws/term/:hostId", WebTerminal)
		api.GET("/ws/winrm/:hostId", WinRMTerminal)
		api.GET("/ws/tail", LogTailWS)
		api.GET("/ws/k8s/:clusterId", K8sExecWS)
		api.GET("/ws/k8s/logs/:clusterId", K8sLogWS)
		api.GET("/ws/task/:id", ws.Handler(func(c *gin.Context) string {
			return "task-" + c.Param("id")
		}))
		api.GET("/ws/release/:id", ws.Handler(func(c *gin.Context) string {
			return "release-" + c.Param("id")
		}))
	}

	// External integration API (API key auth only; password login tokens are not accepted)
	ext := api.Group("/ext", middleware.APIKeyAuth())
	{
		ext.GET("/hosts", ExtHosts)
		ext.GET("/monitors", ExtMonitors)
		ext.GET("/tasks/:id", ExtTask)
		ext.POST("/exec", ExtExec)
		ext.POST("/oo/:stream", ExtOOPush)
	}

	auth := api.Group("", middleware.JWT())
	{
		// Ticketing: create tickets in the configured external system
		auth.POST("/tickets", middleware.RequireRole(model.RoleOps, model.RoleAdmin), TicketCreate)
		auth.GET("/me", Me)
		auth.PUT("/me", UpdateMe)
		auth.POST("/change_password", ChangePassword)

		// AI assistant: chat (any logged-in user) + role list (readable when logged in) + role save (admin)
		auth.POST("/ai/chat", middleware.RequireCap("ai", "chat"), AIChat)
		auth.GET("/ai/roles", GetAIRoles)
		auth.POST("/ai/roles", middleware.RequireRole(model.RoleAdmin), UpdateAIRoles)

		// OpenObserve enabled flag (any logged-in user; drives the Log Search menu visibility)
		auth.GET("/observe/state", OOState)

		// MFA (TOTP two-step verification) self-service management
		// Monitoring: app monitors + host resources
		auth.GET("/monitoring/screen", middleware.RequireCap("monitor", "view_host"), MonitorScreen)
		mon := auth.Group("/monitors", middleware.RequireCapAny("monitor", "view_host", "view_app", "view_sec"))
		{
			mon.GET("", ListMonitors)
			// create/edit/delete/test enforce department ownership in-handler
			// (monitor.manage, or view_app on own-group monitors)
			mon.POST("", CreateMonitor)
			mon.PUT("/:id", UpdateMonitor)
			mon.DELETE("/:id", DeleteMonitor)
			mon.POST("/:id/test", TestMonitor)
			mon.GET("/:id/history", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor), MonitorHistory)
			// OpenObserve search proxy (long-term storage / full-text log search)
			mon.POST("/oo/search", OOSearchProxy)
			mon.GET("/oo/streams", OOMonitorStreams)
			// security-department log query (whitelisted security streams)
			mon.POST("/sec/logs", middleware.RequireCap("monitor", "view_sec"), SecLogs)
		}
		arule := auth.Group("/alert_rules", middleware.RequireCapAny("monitor", "view_host", "view_app", "view_sec"))
		{
			arule.GET("", GetAlertRule)
			arule.PUT("", middleware.RequireCap("monitor", "manage"), UpdateAlertRule)
		}
		apikeys := auth.Group("/api_keys", middleware.RequireRole())
		{
			apikeys.GET("", ListApiKeys)
			apikeys.POST("", middleware.RequireRole(model.RoleAdmin), CreateApiKey)
			apikeys.PUT("/:id", middleware.RequireRole(model.RoleAdmin), UpdateApiKey)
			apikeys.DELETE("/:id", middleware.RequireRole(model.RoleAdmin), DeleteApiKey)
		}

		// K8S cluster management (view: visible to any logged-in user; manage: role permission / cluster admins)
		k8sg := auth.Group("/k8s/clusters")
		{
			_ = k8sg
			k8sg.GET("", ListK8sClusters)
			k8sg.POST("", middleware.RequireCap("k8s", "manage"), CreateK8sCluster)
			k8sg.PUT("/:id", middleware.RequireCap("k8s", "manage"), UpdateK8sCluster)
			k8sg.DELETE("/:id", middleware.RequireCap("k8s", "manage"), DeleteK8sCluster)
			k8sg.POST("/:id/test", middleware.RequireCap("k8s", "manage"), TestK8sCluster)
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
			k8sg.POST("/:id/cronjobs", middleware.RequireCap("k8s", "manage"), K8sCreateCronJob)
			k8sg.PUT("/:id/cronjobs/:namespace/:name/suspend", middleware.RequireCap("k8s", "manage"), K8sSuspendCronJob)
			k8sg.DELETE("/:id/cronjobs/:namespace/:name", middleware.RequireCap("k8s", "manage"), K8sDeleteCronJob)
			k8sg.GET("/:id/serviceaccounts", K8sServiceAccounts)
			k8sg.POST("/:id/serviceaccounts/:namespace", middleware.RequireCap("k8s", "manage"), K8sCreateServiceAccount)
			k8sg.DELETE("/:id/serviceaccounts/:namespace/:name", middleware.RequireCap("k8s", "manage"), K8sDeleteServiceAccount)
			k8sg.GET("/:id/podlog", K8sPodLog)
			k8sg.DELETE("/:id/pods/:namespace/:name", K8sDeletePod)
			k8sg.POST("/:id/shell", K8sClusterShell)
			// Management page read-only views (summary + additional resource lists)
			k8sg.GET("/:id/summary", K8sSummary)
			k8sg.GET("/:id/daemonsets", K8sDaemonSets)
			k8sg.GET("/:id/statefulsets", K8sStatefulSets)
			k8sg.GET("/:id/jobs", K8sJobs)
			k8sg.GET("/:id/services", K8sServices)
			k8sg.GET("/:id/ingresses", K8sIngresses)
			k8sg.GET("/:id/pvcs", K8sPVCs)
			k8sg.GET("/:id/pvs", K8sPVs)
			k8sg.GET("/:id/storageclasses", K8sStorageClasses)
			// Phase 2: YAML viewer / scaling / resource usage / Helm releases
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
			k8sg.PUT("/:id/members", middleware.RequireCap("k8s", "manage"), SetClusterMembers)
		}

		mwin := auth.Group("/maintenance_windows", middleware.RequireRole())
		{
			mwin.GET("", GetMaintenances)
			mwin.GET("/status", MaintenanceStatus)
			mwin.GET("/logs", GetMaintenanceLogs)
			mwin.DELETE("/logs/:id", middleware.RequireRole(model.RoleAdmin), DeleteMaintenanceLog)
			mwin.DELETE("/logs", middleware.RequireRole(model.RoleAdmin), ClearMaintenanceLogs)
			mwin.PUT("", middleware.RequireCap("monitor", "manage"), UpdateMaintenances)
		}

		arule.GET("/cmd", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor), GetCmdLevels)
		arule.GET("/templates", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer, model.RoleAuditor), GetAlertTemplates)
		arule.POST("/templates/preview", PreviewAlertTemplates)
		arule.PUT("/templates", middleware.RequireCap("monitor", "manage"), UpdateAlertTemplates)
		arule.PUT("/cmd", middleware.RequireCap("monitor", "manage"), UpdateCmdLevels)

		ach := auth.Group("/alert_channels", middleware.RequireCapAny("monitor", "view_host", "view_app", "view_sec"))
		{
			ach.GET("", ListAlertChannels)
			ach.POST("", middleware.RequireCap("monitor", "manage"), CreateAlertChannel)
			ach.PUT("/:id", middleware.RequireCap("monitor", "manage"), UpdateAlertChannel)
			ach.DELETE("/:id", middleware.RequireCap("monitor", "manage"), DeleteAlertChannel)
			ach.POST("/:id/test", middleware.RequireCap("monitor", "manage"), TestAlertChannel)
		}

		mg := auth.Group("/monitoring", middleware.RequireCap("monitor", "view_host"))
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

		// User management (admin)
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

		// Host groups
		groups := auth.Group("/host_groups", middleware.RequireCap("hosts", "view"))
		{
			groups.GET("", ListGroups)
			groups.POST("", middleware.RequireCap("hosts", "create"), CreateGroup)
			groups.PUT("/:id", middleware.RequireCap("hosts", "edit"), UpdateGroup)
			groups.DELETE("/:id", middleware.RequireCap("hosts", "delete"), DeleteGroup)
		}

		// SSH keys (admin / ops)
		keys := auth.Group("/ssh_keys", middleware.RequireCap("keys", "manage"))
		{
			keys.GET("", ListKeys)
			keys.POST("", CreateKey)
			keys.DELETE("/:id", DeleteKey)
		}

		// Hosts
		hosts := auth.Group("/hosts", middleware.RequireCap("hosts", "view"))
		{
			hosts.GET("", ListHosts)
			hosts.POST("", middleware.RequireCap("hosts", "create"), CreateHost)
			hosts.PUT("/:id", middleware.RequireCap("hosts", "edit"), UpdateHost)
			hosts.DELETE("/:id", middleware.RequireCap("hosts", "delete"), DeleteHost)
			hosts.POST("/batch-delete", middleware.RequireCap("hosts", "delete"), BatchDeleteHosts)
			hosts.POST("/batch-group", middleware.RequireCap("hosts", "edit"), BatchUpdateHostGroup)
			hosts.POST("/:id/rdp-token", RDPConnectToken)
			hosts.POST("/import", ImportHosts)
			hosts.POST("/probe", ProbeHostsHandler)
			// OS account (credential) management
			hosts.GET("/:id/credentials", middleware.RequireCap("credentials", "view"), ListHostCredentials)
			hosts.POST("/:id/credentials", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), CreateHostCredential)
		}
		// Paired credentials list: visible to admins/ops
		auth.GET("/credentials/paired", middleware.RequireRole(), ListPairedCredentials)

		// Report module: access controlled by the role's "view reports" setting
		reports := auth.Group("/reports", middleware.RequireReportPerm())
		{
			reports.GET("/templates", ListReportTemplates)
			reports.GET("", ListReports)
			reports.GET("/:id", GetReport)
			reports.GET("/accounts-matrix/:id", ReportAccountsMatrix)
			reports.GET("/:id/export", ExportReport)
			reports.POST("", CreateReport)
			reports.DELETE("/:id", middleware.RequireRole(), DeleteReport)
		}

		// Cron jobs: manageable by admins/ops
		crons := auth.Group("/crons", middleware.RequireCap("crons", "view"))
		{
			crons.GET("", ListCrons)
			crons.POST("", middleware.RequireCap("crons", "manage"), CreateCron)
			crons.PUT("/:id", middleware.RequireCap("crons", "manage"), UpdateCron)
			crons.DELETE("/:id", middleware.RequireCap("crons", "manage"), DeleteCron)
			crons.POST("/:id/toggle", middleware.RequireCap("crons", "manage"), ToggleCron)
			crons.POST("/:id/run", middleware.RequireCap("crons", "manage"), RunCronNow)
			crons.GET("/:id/history", CronHistory)
		}

		creds := auth.Group("/credentials", middleware.RequireCap("credentials", "view"))
		{
			creds.GET("", ListAllCredentials)
			// Credential templates (referenced by add host / batch import, avoids re-entering passwords)
			creds.GET("/templates", ListCredentialTemplates)
			creds.POST("/templates", middleware.RequireCap("credentials", "manage"), SaveCredentialTemplate)
			creds.DELETE("/templates/:id", middleware.RequireCap("credentials", "manage"), DeleteCredentialTemplate)
			creds.GET("/usable", UsableCredentialsHandler)
			// OS account management: requires the role to enable "account management" permission
			creds.POST("/batch", middleware.RequireCap("credentials", "manage"), BatchAddCredentialsHandler)
			creds.POST("/batch-delete", middleware.RequireCap("credentials", "manage"), BatchDeleteCredentials)
			creds.PUT("/:id", middleware.RequireCap("credentials", "manage"), UpdateCredential)
			creds.POST("/:id/default", middleware.RequireCap("credentials", "manage"), SetDefaultCredential)
			creds.POST("/:id/rotate", middleware.RequireCap("credentials", "manage"), RotateCredentialNow)
			creds.POST("/rotate-batch", middleware.RequireCap("credentials", "manage"), RotateCredentialsBatch)
			creds.GET("/rotate-batch/:batch", RotateCredentialsBatchStatus)
			// Reveal password plaintext: system admin only (audited)
			creds.POST("/:id/reveal", middleware.RequireRole(), RevealCredentialPassword)
			// Password history (each rotation/change archived): system admin only (audited)
			creds.GET("/:id/password-history", middleware.RequireRole(), PasswordHistory)
			// Delete: system admin only
			creds.DELETE("/:id", middleware.RequireRole(), DeleteCredential)
		}

		// Web assets: PAM-style web app assets (URL + vaulted credentials, audited use)
		wa := auth.Group("/webassets", middleware.JWT())
		{
			wa.GET("", ListWebAssets)
			wa.GET("/:id", GetWebAsset)
			wa.POST("", middleware.RequireRole(model.RoleAdmin), CreateWebAsset)
			wa.PUT("/:id", middleware.RequireRole(model.RoleAdmin), UpdateWebAsset)
			wa.DELETE("/:id", middleware.RequireRole(model.RoleAdmin), DeleteWebAsset)
			wa.POST("/:id/open", OpenWebAsset)
		}

		// Database workbench: sources/accounts admin-managed, account visibility via
		// AllowedGroups; query execution audited
		dbw := auth.Group("/databases", middleware.RequireCap("databases", "use"))
		{
			dbw.POST("/sources", middleware.RequireRole(model.RoleAdmin), CreateDBSource)
			dbw.GET("/sources", DBSourceListForWorkbench)
			dbw.PUT("/sources/:id/guardrails", middleware.RequireRole(model.RoleAdmin), DBSourceGuardrails)
			dbw.GET("/:id/accounts", DBSourceAccounts)
			dbw.POST("/:id/accounts", middleware.RequireRole(model.RoleAdmin), CreateDBAccount)
			dbw.PUT("/accounts/:id", middleware.RequireRole(model.RoleAdmin), UpdateDBAccount)
			dbw.DELETE("/accounts/:id", middleware.RequireRole(model.RoleAdmin), DeleteDBAccount)
			dbw.POST("/:id/query", RunDBQueryHandler)
		}

		// Cloud accounts (CSP asset sync): admin-managed, sync requires hosts:create
		cloud := auth.Group("/cloudaccounts", middleware.JWT())
		{
			cloud.GET("", middleware.RequireCap("hosts", "view"), ListCloudAccounts)
			cloud.POST("", middleware.RequireRole(model.RoleAdmin), CreateCloudAccount)
			cloud.PUT("/:id", middleware.RequireRole(model.RoleAdmin), UpdateCloudAccount)
			cloud.DELETE("/:id", middleware.RequireRole(model.RoleAdmin), DeleteCloudAccount)
			cloud.POST("/:id/test", middleware.RequireRole(model.RoleAdmin), TestCloudAccount)
			cloud.POST("/:id/sync", middleware.RequireCap("hosts", "create"), SyncCloudAccountByID)
			cloud.GET("/:id/conflicts", middleware.RequireCap("hosts", "view"), CloudSyncConflicts)
			cloud.POST("/:id/merge", middleware.RequireCap("hosts", "edit"), MergeCloudConflict)
		}

		// Batch execution
		exec := auth.Group("/exec", middleware.RequireCap("exec", "exec"))
		{
			exec.POST("", StartExec)
		}
		// Task records: admins/auditors see all; ops/publishers only their own tasks (filtered in the handler)
		tasks := auth.Group("/tasks", middleware.RequireCap("tasks", "view"))
		{
			tasks.GET("", ListTasks)
			tasks.GET("/:id", GetTask)
			tasks.GET("/:id/export", middleware.RequireCap("tasks", "export"), ExportTask)
		}

		// File distribution
		files := auth.Group("/files", middleware.RequireCap("files", "distribute"))
		{
			files.POST("/upload", UploadFile)
			files.POST("/distribute", Distribute)
		}

		// Script center
		scripts := auth.Group("/scripts", middleware.RequireCap("scripts", "view"))
		{
			scripts.GET("", ListScripts)
			scripts.POST("", middleware.RequireCap("scripts", "manage"), CreateScript)
			scripts.PUT("/:id", middleware.RequireCap("scripts", "manage"), UpdateScript)
			scripts.DELETE("/:id", middleware.RequireCap("scripts", "manage"), DeleteScript)
			scripts.POST("/:id/exec", middleware.RequireCap("scripts", "exec"), ExecScript)
		}

		// Apps and releases
		apps := auth.Group("/apps", middleware.RequireCap("apps", "view"))
		{
			apps.GET("", ListApps)
			apps.POST("", middleware.RequireCap("apps", "create"), CreateApp)
			apps.PUT("/:id", middleware.RequireCap("apps", "edit"), UpdateApp)
			apps.DELETE("/:id", middleware.RequireCap("apps", "delete"), DeleteApp)
		}
		releases := auth.Group("/releases", middleware.RequireCap("releases", "view"))
		{
			releases.GET("", ListReleases)
			releases.GET("/:id", GetRelease)
			releases.POST("", middleware.RequireCap("releases", "create"), CreateReleaseHandler)
			releases.POST("/:id/rollback", middleware.RequireCap("releases", "rollback"), RollbackHandler)
		}

		// Audit logs: admins/auditors only
		audit := auth.Group("/audit", middleware.RequireRole(model.RoleAuditor))
		{
			audit.GET("", ListAudit)
		}

		// Dangerous command rules (admin)
		danger := auth.Group("/danger_rules", middleware.RequireRole())
		{
			danger.GET("", ListDangerRules)
			danger.POST("", CreateDangerRule)
			danger.PUT("/:id", UpdateDangerRule)
			danger.DELETE("/:id", DeleteDangerRule)
		}

		// Data source for permission grants
		auth.GET("/grants/options", GrantsOptions)

		// User group management (admin)
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

		// System config (admin)
		auth.GET("/system/roles", GetSystemRoles)
		auth.GET("/system/capabilities", GetSystemCapabilities)
		auth.POST("/system/roles", middleware.RequireRole(), CreateRole)
		auth.DELETE("/system/roles/:key", middleware.RequireRole(), DeleteRole)
		auth.GET("/system/roles/:key/users", RoleUsers) // readable by any logged-in user (frontend menu rendering depends on it)
		sysCfg := auth.Group("/system", middleware.RequireRole())
		{
			sysCfg.GET("/config", GetSystemConfig)
			sysCfg.PUT("/config", UpdateSystemConfig)
			sysCfg.POST("/ldap/test", TestLDAPConfig)
			sysCfg.POST("/smtp/test", TestSMTPConfig)
			sysCfg.GET("/rotation/accounts", RotationAccounts)
			sysCfg.POST("/rotation/run-now", RotationRunNow)
			sysCfg.POST("/ai/test", middleware.RequireRole(model.RoleAdmin), AITest)
			sysCfg.POST("/oo/test", middleware.RequireRole(model.RoleAdmin), OOConfigTest)
			sysCfg.GET("/oo/status", middleware.RequireRole(model.RoleAdmin), OOAdminStatus)
			sysCfg.POST("/oo/integrations", middleware.RequireRole(model.RoleAdmin), OOAdminToggle)
			sysCfg.POST("/oo/push", middleware.RequireRole(model.RoleAdmin), OOAdminPush)
			// AI alert diagnostics: config + cleanup catalog (admin)
			sysCfg.GET("/ai/diag/config", middleware.RequireRole(model.RoleAdmin), OODiagConfigGet)
			sysCfg.PUT("/ai/diag/config", middleware.RequireRole(model.RoleAdmin), OODiagConfigPut)
			sysCfg.GET("/ai/diag/cleanup", middleware.RequireRole(model.RoleAdmin), OODiagCleanupList)
			sysCfg.GET("/key-rotation", middleware.RequireRole(model.RoleAdmin), KeyRotationConfigGet)
			sysCfg.PUT("/key-rotation", middleware.RequireRole(model.RoleAdmin), KeyRotationConfigPut)
			sysCfg.POST("/key-rotation/run", middleware.RequireRole(model.RoleAdmin), KeyRotationRun)
			sysCfg.POST("/ai/diag/cleanup", middleware.RequireRole(model.RoleAdmin), OODiagCleanupSave)
			sysCfg.POST("/ai/diag/cleanup/:hostId/run", middleware.RequireRole(model.RoleAdmin), OODiagCleanupTest)
			sysCfg.GET("/oo/dbsources", middleware.RequireRole(model.RoleAdmin), OODbSourceList)
			sysCfg.POST("/oo/dbsources", middleware.RequireRole(model.RoleAdmin), OODbSourceSave)
			sysCfg.DELETE("/oo/dbsources/:id", middleware.RequireRole(model.RoleAdmin), OODbSourceDelete)
			sysCfg.POST("/oo/dbsources/:id/enabled", middleware.RequireRole(model.RoleAdmin), OODbSourceEnable)
			sysCfg.POST("/oo/dbsources/:id/run", middleware.RequireRole(model.RoleAdmin), OODbSourceRun)
			sysCfg.GET("/platform_key", GetPlatformKey)
			sysCfg.GET("/oo/retention", OORetentionGet)
			sysCfg.POST("/oo/retention", OORetentionApply)
			sysCfg.GET("/sec/watch", SecWatchGet)
			sysCfg.PUT("/sec/watch", SecWatchPut)
			sysCfg.GET("/ticketing", TicketingConfigGet)
			sysCfg.PUT("/ticketing", TicketingConfigPut)
			sysCfg.POST("/ticketing/test", TicketingTest)
			sysCfg.PUT("/roles", UpdateSystemRoles)
		}
	}

	// Serve frontend SPA (frontend/dist): supports local dev (relative path under backend/) and container (/app/frontend/dist)
	candidates := []string{
		filepath.Join("..", "frontend", "dist"), // local: started from backend/
		filepath.Join("frontend", "dist"),       // container: WORKDIR /app
		"/app/frontend/dist",                    // container: absolute path fallback
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
			// Unknown API paths return 404 JSON; they must not fall back to the SPA page
			if strings.HasPrefix(p, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
				return
			}
			// Hashed static assets: long cache
			if strings.HasPrefix(p, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
				c.File(filepath.Join(abs, strings.TrimPrefix(p, "/")))
				return
			}
			// Everything else falls back to index.html with caching disabled: browsers get the new entry right after a release
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.File(filepath.Join(abs, "index.html"))
		})
		break
	}

	return r
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
