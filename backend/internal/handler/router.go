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
			hosts.POST("", CreateHost)
			hosts.PUT("/:id", UpdateHost)
			hosts.DELETE("/:id", DeleteHost)
			hosts.POST("/import", ImportHosts)
			hosts.POST("/probe", ProbeHostsHandler)
		}

		// 批量执行
		exec := auth.Group("/exec", middleware.RequireRole(model.RoleOps, model.RolePublisher))
		{
			exec.POST("", StartExec)
		}
		tasks := auth.Group("/tasks", middleware.RequireRole(model.RoleOps, model.RolePublisher, model.RoleViewer))
		{
			tasks.GET("", ListTasks)
			tasks.GET("/:id", GetTask)
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

		// 审计日志（admin 全量，其他人可看自己的）
		auth.GET("/audit", ListAudit)

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
	}

	// 托管前端 SPA（frontend/dist）
	frontDist := filepath.Join("..", "frontend", "dist")
	if abs, err := filepath.Abs(frontDist); err == nil {
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			r.StaticFile("/favicon.ico", filepath.Join(abs, "favicon.ico"))
			r.NoRoute(func(c *gin.Context) {
				p := c.Request.URL.Path
				if p != "/" && !strings.HasPrefix(p, "/assets") {
					c.File(filepath.Join(abs, "index.html"))
					return
				}
				c.File(filepath.Join(abs, strings.TrimPrefix(p, "/")))
			})
		}
	}

	return r
}
