// Package router Gin 路由注册中心
// 按三级管控体系进行路由分组：
//
//	/api/v1/*            - 终端客户业务接口（API Server）
//	/admin/developer/*   - 开发商总后台（Admin Server）
//	/admin/provider/*    - 服务商管理后台（Admin Server）
//	/admin/customer/*    - 终端客户业务后台（Admin Server）
//	/agent/*             - 智能体引擎（Agent Engine）
package router

import (
	"strconv"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// 引入 docs 包以注册 Swagger 规范
	_ "github.com/zhongjinmuai-lang/mu-framework/docs"

	"github.com/zhongjinmuai-lang/mu-framework/internal/ai"
	"github.com/zhongjinmuai-lang/mu-framework/internal/auth"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/bootstrap"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/middleware"
	"github.com/zhongjinmuai-lang/mu-framework/internal/genealogy"
	"github.com/zhongjinmuai-lang/mu-framework/internal/platform"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// Services 业务服务集合（供三个 Server 复用）
type Services struct {
	Auth      *auth.Service
	Genealogy *genealogy.Service
	SaaS      *saas.Manager
	Platform  *platform.Manager
	AI        *ai.Gateway
}

// NewServices 一次性构造所有服务
func NewServices(app *bootstrap.App) *Services {
	aiGw := ai.NewGateway(nil)
	saasMgr := saas.NewManager(app.DB, app.Redis)
	return &Services{
		Auth:      auth.NewService(app.DB, app.Redis, app.JWT),
		Genealogy: genealogy.NewService(app.DB, aiGw),
		SaaS:      saasMgr,
		Platform:  platform.NewManager(app.DB, saasMgr.Hierarchy),
		AI:        aiGw,
	}
}

// baseEngine 构建带通用中间件的 Gin 引擎
func baseEngine(app *bootstrap.App) *gin.Engine {
	gin.SetMode(app.Config.Server.Mode)

	r := gin.New()
	r.Use(
		middleware.Recovery(),
		middleware.Trace(),
		middleware.AccessLog(),
		middleware.CORS(),
		gzip.Gzip(gzip.DefaultCompression),
	)

	r.GET("/health", healthHandler(app))
	r.GET("/ready", readyHandler(app))
	r.GET("/version", versionHandler(app))

	if app.Config.Swagger.Enabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return r
}

// NewAPIServer 终端客户业务 API
func NewAPIServer(app *bootstrap.App) *gin.Engine {
	r := baseEngine(app)
	svc := NewServices(app)
	authH := auth.NewHandler(svc.Auth)
	genH := genealogy.NewHandler(svc.Genealogy)

	// 公开接口
	public := r.Group("/api/v1")
	{
		public.GET("/info", infoHandler(app))
		ag := public.Group("/auth")
		ag.Use(middleware.RateLimit(app.Redis, 30, time.Minute, nil))
		{
			ag.POST("/login", authH.Login)
			ag.POST("/refresh", authH.Refresh)
		}
	}

	// 需登录接口
	authed := r.Group("/api/v1")
	authed.Use(
		middleware.Auth(app.JWT),
		middleware.TenantRequired(),
		middleware.TenantRLS(app.DB),
	)
	{
		authed.GET("/me", authH.Me)
		authed.POST("/auth/logout", authH.Logout)
		authed.PUT("/auth/password", authH.ChangePassword)

		// 族谱
		gg := authed.Group("/genealogy")
		{
			gg.GET("/stats", genH.Stats)
			gg.GET("/tree", genH.Tree)
			gg.GET("/lca", genH.LCA)
			gg.GET("/members", genH.ListMembers)
			gg.POST("/members", genH.CreateMember)
			gg.GET("/members/:id", genH.GetMember)
			gg.PUT("/members/:id", genH.UpdateMember)
			gg.DELETE("/members/:id", genH.DeleteMember)
			gg.GET("/members/:id/ancestors", genH.Ancestors)
			gg.GET("/members/:id/descendants", genH.Descendants)
			gg.GET("/branches", genH.ListBranches)
			gg.POST("/branches", genH.CreateBranch)
			gg.GET("/announces", genH.ListAnnounces)
			gg.POST("/announces", genH.PublishAnnounce)
			gg.POST("/ocr", genH.OCR)
		}

		// 支付
		pay := authed.Group("/pay")
		{
			pay.GET("/channels", notImplemented("可用支付渠道"))
			pay.POST("/orders", notImplemented("创建订单"))
			pay.GET("/orders", notImplemented("我的订单"))
			pay.GET("/orders/:id", notImplemented("订单详情"))
		}

		// 存储
		storage := authed.Group("/storage")
		{
			storage.POST("/upload", notImplemented("上传文件"))
			storage.GET("/files", notImplemented("文件列表"))
			storage.DELETE("/files/:id", notImplemented("删除文件"))
			storage.GET("/quota", notImplemented("配额查询"))
		}

		// 消息
		msg := authed.Group("/messages")
		{
			msg.GET("", notImplemented("站内信列表"))
			msg.PUT("/:id/read", notImplemented("标记已读"))
			msg.PUT("/subscriptions", notImplemented("订阅设置"))
		}
	}

	return r
}

// NewAdminServer 三级管理后台
func NewAdminServer(app *bootstrap.App) *gin.Engine {
	r := baseEngine(app)
	svc := NewServices(app)
	authH := auth.NewHandler(svc.Auth)

	// 管理后台登录（独立于 API Server，便于隔离）
	pub := r.Group("/admin/v1")
	pub.Use(middleware.RateLimit(app.Redis, 30, time.Minute, nil))
	{
		pub.POST("/auth/login", authH.Login)
		pub.POST("/auth/refresh", authH.Refresh)
	}

	// 需登录
	admin := r.Group("/admin")
	admin.Use(
		middleware.Auth(app.JWT),
		middleware.TenantRequired(),
		middleware.TenantRLS(app.DB),
		middleware.RateLimitByUser(app.Redis, 300, time.Minute),
	)
	{
		admin.GET("/v1/me", authH.Me)
		admin.POST("/v1/auth/logout", authH.Logout)
		admin.PUT("/v1/auth/password", authH.ChangePassword)
	}

	registerDeveloperRoutes(admin, app, svc, authH)
	registerProviderRoutes(admin, app, svc)
	registerCustomerRoutes(admin, app, svc)
	return r
}

// NewAgentEngine 智能体引擎
func NewAgentEngine(app *bootstrap.App) *gin.Engine {
	r := baseEngine(app)

	agent := r.Group("/agent")
	agent.Use(
		middleware.Auth(app.JWT),
		middleware.RequireLevel("developer"),
	)
	{
		agent.GET("/status", notImplemented("引擎状态"))
		agent.GET("/stats", notImplemented("运行统计"))
		plugins := agent.Group("/plugins")
		{
			plugins.GET("", notImplemented("插件列表"))
			plugins.POST("/install", notImplemented("安装插件"))
			plugins.POST("/:id/start", notImplemented("启动插件"))
			plugins.POST("/:id/stop", notImplemented("停止插件"))
			plugins.DELETE("/:id", notImplemented("卸载插件"))
		}
		tasks := agent.Group("/tasks")
		{
			tasks.POST("", notImplemented("提交任务"))
			tasks.GET("/:id", notImplemented("任务状态"))
		}
		caps := agent.Group("/capabilities")
		{
			caps.GET("", notImplemented("能力列表"))
			caps.GET("/by-category/:category", notImplemented("按分类查找"))
		}
		evo := agent.Group("/evolution")
		{
			evo.GET("/events", notImplemented("进化事件历史"))
			evo.POST("/metrics", notImplemented("上报指标"))
		}
		aiGrp := agent.Group("/ai")
		{
			aiGrp.POST("/chat", notImplemented("AI对话（多供应商降级）"))
			aiGrp.GET("/providers", notImplemented("供应商列表"))
		}
	}
	return r
}

// ========== 三级后台路由 ==========

func registerDeveloperRoutes(root *gin.RouterGroup, app *bootstrap.App, svc *Services, authH *auth.Handler) {
	g := root.Group("/developer")
	g.Use(middleware.RequireLevel("developer"))

	// 控制台统计
	g.GET("/dashboard/stats", func(c *gin.Context) {
		var providers, customers int64
		app.DB.Table("tenants").Where("level = ? AND deleted_at IS NULL", "provider").Count(&providers)
		app.DB.Table("tenants").Where("level = ? AND deleted_at IS NULL", "customer").Count(&customers)
		response.OK(c, gin.H{
			"tenants":   providers + customers,
			"providers": providers,
			"customers": customers,
			"plugins":   0,
		})
	})

	// 用户管理（注册）
	g.POST("/users", authH.Register)

	// 租户列表（服务商）
	providers := g.Group("/providers")
	{
		providers.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
			// 开发商直属的所有服务商
			tid := c.GetString(middleware.CtxKeyTenantID)
			list, total, err := svc.SaaS.Tenant.ListByParent(c.Request.Context(), tid, page, size)
			if err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.Page(c, list, page, size, total)
		})
		providers.POST("", func(c *gin.Context) {
			var in saas.CreateProviderInput
			if err := c.ShouldBindJSON(&in); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			tid := c.GetString(middleware.CtxKeyTenantID)
			t, err := svc.SaaS.CreateProvider(c.Request.Context(), tid, &in)
			if err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.Created(c, t)
		})
		providers.PUT("/:id/status", func(c *gin.Context) {
			var body struct {
				Status int `json:"status"`
			}
			_ = c.ShouldBindJSON(&body)
			if err := svc.SaaS.Tenant.UpdateStatus(c.Request.Context(), c.Param("id"), body.Status); err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
		providers.DELETE("/:id", func(c *gin.Context) {
			if err := svc.SaaS.Tenant.Delete(c.Request.Context(), c.Param("id")); err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
		// 开发商重置服务商管理员密码
		providers.POST("/reset-password", func(c *gin.Context) {
			var in saas.ResetTenantPasswordInput
			if err := c.ShouldBindJSON(&in); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			tid := c.GetString(middleware.CtxKeyTenantID)
			if err := svc.SaaS.ResetTenantPassword(c.Request.Context(), tid, &in); err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
	}

	// 支付中台
	payment := g.Group("/payment")
	{
		payment.GET("/channels", notImplemented("全局支付渠道"))
		payment.POST("/channels", notImplemented("渠道准入"))
		payment.PUT("/channels/:id", notImplemented("渠道配置"))
		payment.DELETE("/channels/:id", notImplemented("渠道下架"))
		payment.POST("/channels/:id/grant", notImplemented("授予服务商"))
		payment.GET("/orders", notImplemented("全局订单"))
	}
	storage := g.Group("/storage")
	{
		storage.GET("/sources", notImplemented("存储源列表"))
		storage.POST("/sources", notImplemented("厂商准入"))
	}
	notify := g.Group("/notify")
	{
		notify.GET("/channels", notImplemented("通知通道"))
		notify.POST("/channels", notImplemented("渠道准入"))
		notify.GET("/templates", notImplemented("模板定义"))
	}
}

func registerProviderRoutes(root *gin.RouterGroup, app *bootstrap.App, svc *Services) {
	g := root.Group("/provider")
	g.Use(middleware.RequireLevel("provider"))

	g.GET("/dashboard/stats", func(c *gin.Context) {
		tid := c.GetString(middleware.CtxKeyTenantID)
		var customers int64
		app.DB.Table("tenants").Where("parent_id = ? AND deleted_at IS NULL", tid).Count(&customers)
		response.OK(c, gin.H{
			"customers": customers,
			"revenue":   0.0,
			"orders":    0,
			"messages":  0,
		})
	})

	// 终端客户管理
	customers := g.Group("/customers")
	{
		customers.GET("", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
			tid := c.GetString(middleware.CtxKeyTenantID)
			list, total, err := svc.SaaS.Tenant.ListByParent(c.Request.Context(), tid, page, size)
			if err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.Page(c, list, page, size, total)
		})
		customers.POST("", func(c *gin.Context) {
			var in saas.CreateCustomerInput
			if err := c.ShouldBindJSON(&in); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			tid := c.GetString(middleware.CtxKeyTenantID)
			t, err := svc.SaaS.CreateCustomer(c.Request.Context(), tid, &in)
			if err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.Created(c, t)
		})
		customers.PUT("/:id/status", func(c *gin.Context) {
			var body struct {
				Status int `json:"status"`
			}
			_ = c.ShouldBindJSON(&body)
			if err := svc.SaaS.Tenant.UpdateStatus(c.Request.Context(), c.Param("id"), body.Status); err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
		// 服务商重置终端客户管理员密码
		customers.POST("/reset-password", func(c *gin.Context) {
			var in saas.ResetTenantPasswordInput
			if err := c.ShouldBindJSON(&in); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			tid := c.GetString(middleware.CtxKeyTenantID)
			if err := svc.SaaS.ResetTenantPassword(c.Request.Context(), tid, &in); err != nil {
				response.InternalError(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
	}

	// 支付、存储、通知（管理用）
	g.GET("/payment/channels", notImplemented("可用支付渠道"))
	g.GET("/storage/quotas", notImplemented("客户配额列表"))
	g.GET("/notify/templates", notImplemented("模板列表"))
	g.GET("/permissions", notImplemented("本租户权限"))
}

func registerCustomerRoutes(root *gin.RouterGroup, app *bootstrap.App, svc *Services) {
	g := root.Group("/customer")
	g.Use(middleware.RequireLevel("customer"))

	g.GET("/dashboard/stats", func(c *gin.Context) {
		tid := c.GetString(middleware.CtxKeyTenantID)
		stats, _ := svc.Genealogy.GetStats(c.Request.Context(), tid)
		response.OK(c, stats)
	})
}

// ========== 通用 Handler ==========

func healthHandler(app *bootstrap.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, gin.H{
			"status":  "healthy",
			"app":     app.Config.App.Name,
			"version": app.Config.App.Version,
		})
	}
}

func readyHandler(app *bootstrap.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := app.DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			response.Fail(c, response.CodeBadGateway, "数据库未就绪")
			return
		}
		if err := app.Redis.Ping(c.Request.Context()).Err(); err != nil {
			response.Fail(c, response.CodeBadGateway, "Redis 未就绪")
			return
		}
		response.OK(c, gin.H{"ready": true})
	}
}

func versionHandler(app *bootstrap.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, gin.H{
			"app":       app.Config.App.Name,
			"version":   app.Config.App.Version,
			"env":       app.Config.App.Env,
			"runtime":   "Go 1.26.1",
			"database":  "PostgreSQL 18.3",
			"framework": "MU Framework",
		})
	}
}

func infoHandler(app *bootstrap.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, gin.H{
			"name":        "MU Framework",
			"description": "自研全能智能体主体框架",
			"features": []string{
				"三级SaaS管控",
				"插件化热插拔",
				"AI智能体调度",
				"三大统一中台",
				"MU自进化智能内核",
			},
		})
	}
}

func notImplemented(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, gin.H{"_hint": name + "（业务逻辑待补全）"})
	}
}
