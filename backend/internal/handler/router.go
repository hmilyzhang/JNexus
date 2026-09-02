// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"autoops/internal/config"
	"autoops/internal/middleware"
	"autoops/internal/model"
	"autoops/internal/ws"
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
		api.GET("/system/info", SystemInfo)
		// Web 终端（自带 token 鉴权，不走 hub）
		api.GET("/ws/term/:hostId", WebTerminal)
		api.GET("/ws/task/:id", ws.Handler(func(c *gin.Context) string {
			return "task-" + c.Param("id")
		}))
		api.GET("/ws/release/:id", ws.Handler(func(c *gin.Context) string {
			return "release-" + c.Param("id")
		}))
	}

	auth := api.Group("", middleware.JWT())
	{
		auth.GET("/me", Me)
		auth.POST("/change_password", ChangePassword)

		// 用户管理（admin）
		users := auth.Group("/users", middleware.RequireRole())
		{
			users.GET("", ListUsers)
			users.POST("", CreateUser)
			users.PUT("/:id", UpdateUser)
			users.DELETE("/:id", DeleteUser)
			users.PUT("/:id/grants", SetUserGrants)
			users.GET("/:id/grants", GetUserGrants)
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
			hosts.POST("/import", ImportHosts)
			hosts.POST("/probe", ProbeHostsHandler)
			// OS 账号（凭据）管理
			hosts.GET("/:id/credentials", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer), ListHostCredentials)
			hosts.POST("/:id/credentials", middleware.RequireRole(model.RoleOps), middleware.RequireCredPerm(), CreateHostCredential)
		}
		// 配对密钥列表：管理员/运维可见
		auth.GET("/credentials/paired", middleware.RequireRole(), ListPairedCredentials)

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
			creds.GET("/usable", UsableCredentialsHandler)
			// OS 账号管理：需角色开启「账号管理」权限
			creds.POST("/batch", middleware.RequireRole(model.RoleOps), middleware.RequireHostPerm("edit"), middleware.RequireCredPerm(), BatchAddCredentialsHandler)
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
			apps.POST("", CreateApp)
			apps.PUT("/:id", UpdateApp)
			apps.DELETE("/:id", DeleteApp)
		}
		releases := auth.Group("/releases", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer))
		{
			releases.GET("", ListReleases)
			releases.GET("/:id", GetRelease)
			releases.POST("", CreateReleaseHandler)
			releases.POST("/:id/rollback", RollbackHandler)
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
