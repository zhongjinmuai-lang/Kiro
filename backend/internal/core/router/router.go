// Package router Gin 路由注册中心
// 按三级管控体系进行路由分组：
//   /api/v1/*            - 终端客户业务接口（API Server）
//   /admin/developer/*   - 开发商总后台（Admin Server）
//   /admin/provider/*    - 服务商管理后台（Admin Server）
//   /admin/customer/*    - 终端客户业务后台（Admin Server）
//   /agent/*             - 智能体引擎（Agent Engine）
package router

import (
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// 引入 docs 包以注册 Swagger 规范
	_ "github.com/zhongjinmuai-lang/mu-framework/docs"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/bootstrap"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/middleware"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

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

	// 健康探针（无需鉴权）
	r.GET("/health", healthHandler(app))
	r.GET("/ready", readyHandler(app))
	r.GET("/version", versionHandler(app))

	// Swagger UI
	if app.Config.Swagger.Enabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return r
}

// NewAPIServer API Server（面向终端客户的业务接口）
func NewAPIServer(app *bootstrap.App) *gin.Engine {
	r := baseEngine(app)

	// ===== 公开接口（无需登录）=====
	public := r.Group("/api/v1")
	{
		// 接口信息
		public.GET("/info", infoHandler(app))

		// 认证：登录 / 刷新令牌（高频，限流保护）
		auth := public.Group("/auth")
		auth.Use(middleware.RateLimit(app.Redis, 30, 1*time.Minute, nil))
		{
			auth.POST("/login", notImplemented("登录"))
			auth.POST("/refresh", notImplemented("刷新令牌"))
			auth.POST("/logout", notImplemented("登出"))
		}
	}

	// ===== 需登录接口 =====
	authed := r.Group("/api/v1")
	authed.Use(
		middleware.Auth(app.JWT),
		middleware.TenantRequired(),
		middleware.TenantRLS(app.DB),
	)
	{
		// 用户信息
		authed.GET("/me", notImplemented("当前用户"))
		authed.PUT("/me", notImplemented("更新资料"))

		// 族谱（终端客户核心业务）
		genealogy := authed.Group("/genealogy")
		{
			genealogy.GET("/tree", notImplemented("世系树"))
			genealogy.GET("/members/:id/ancestors", notImplemented("亲属溯源"))
			genealogy.GET("/members/:id/descendants", notImplemented("分支遍历"))
			genealogy.POST("/members", notImplemented("新增成员"))
			genealogy.POST("/ocr", notImplemented("老族谱AI识别建档"))
		}

		// 支付（仅使用服务商授权的渠道）
		pay := authed.Group("/pay")
		{
			pay.GET("/channels", notImplemented("可用支付渠道"))
			pay.POST("/orders", notImplemented("创建订单"))
			pay.GET("/orders", notImplemented("我的订单"))
			pay.GET("/orders/:id", notImplemented("订单详情"))
		}

		// 存储（配额内上传下载）
		storage := authed.Group("/storage")
		{
			storage.POST("/upload", notImplemented("上传文件"))
			storage.GET("/files", notImplemented("文件列表"))
			storage.DELETE("/files/:id", notImplemented("删除文件"))
			storage.GET("/quota", notImplemented("配额查询"))
		}

		// 消息通知
		msg := authed.Group("/messages")
		{
			msg.GET("", notImplemented("站内信列表"))
			msg.PUT("/:id/read", notImplemented("标记已读"))
			msg.PUT("/subscriptions", notImplemented("订阅设置"))
		}
	}

	return r
}

// NewAdminServer Admin Server（三级管理后台）
func NewAdminServer(app *bootstrap.App) *gin.Engine {
	r := baseEngine(app)

	// 所有管理接口都需要登录
	admin := r.Group("/admin")
	admin.Use(
		middleware.Auth(app.JWT),
		middleware.TenantRequired(),
		middleware.TenantRLS(app.DB),
		middleware.RateLimitByUser(app.Redis, 300, 1*time.Minute),
	)

	registerDeveloperRoutes(admin, app)
	registerProviderRoutes(admin, app)
	registerCustomerRoutes(admin, app)

	return r
}

// NewAgentEngine Agent Engine（智能体引擎）
func NewAgentEngine(app *bootstrap.App) *gin.Engine {
	r := baseEngine(app)

	agent := r.Group("/agent")
	agent.Use(
		middleware.Auth(app.JWT),
		middleware.RequireLevel("developer"), // 仅开发商可直接访问引擎
	)
	{
		// 引擎状态
		agent.GET("/status", notImplemented("引擎状态"))
		agent.GET("/stats", notImplemented("运行统计"))

		// 插件管理（热插拔）
		plugins := agent.Group("/plugins")
		{
			plugins.GET("", notImplemented("插件列表"))
			plugins.POST("/install", notImplemented("安装插件"))
			plugins.POST("/:id/start", notImplemented("启动插件"))
			plugins.POST("/:id/stop", notImplemented("停止插件"))
			plugins.DELETE("/:id", notImplemented("卸载插件"))
		}

		// 任务调度
		tasks := agent.Group("/tasks")
		{
			tasks.POST("", notImplemented("提交任务"))
			tasks.GET("/:id", notImplemented("任务状态"))
		}

		// 能力注册中心
		caps := agent.Group("/capabilities")
		{
			caps.GET("", notImplemented("能力列表"))
			caps.GET("/by-category/:category", notImplemented("按分类查找"))
		}

		// 自进化
		evo := agent.Group("/evolution")
		{
			evo.GET("/events", notImplemented("进化事件历史"))
			evo.POST("/metrics", notImplemented("上报指标"))
		}

		// AI 网关
		ai := agent.Group("/ai")
		{
			ai.POST("/chat", notImplemented("AI对话（多供应商降级）"))
			ai.GET("/providers", notImplemented("供应商列表"))
		}
	}

	return r
}

// ========== 三级后台路由 ==========

func registerDeveloperRoutes(root *gin.RouterGroup, app *bootstrap.App) {
	g := root.Group("/developer")
	g.Use(middleware.RequireLevel("developer"))

	// 服务商管理
	providers := g.Group("/providers")
	{
		providers.GET("", notImplemented("服务商列表"))
		providers.POST("", notImplemented("入驻审核"))
		providers.PUT("/:id", notImplemented("编辑服务商"))
		providers.PUT("/:id/status", notImplemented("启用/禁用"))
		providers.PUT("/:id/profit-rule", notImplemented("分润规则配置"))
	}

	// 支付中台（顶层集权）
	payment := g.Group("/payment")
	{
		payment.GET("/channels", notImplemented("全局支付渠道"))
		payment.POST("/channels", notImplemented("渠道准入"))
		payment.PUT("/channels/:id", notImplemented("渠道配置"))
		payment.DELETE("/channels/:id", notImplemented("渠道下架"))
		payment.POST("/channels/:id/grant", notImplemented("授予服务商"))
		payment.GET("/orders", notImplemented("全局订单"))
		payment.GET("/reconcile", notImplemented("对账中心"))
	}

	// 存储中台（顶层集权）
	storage := g.Group("/storage")
	{
		storage.GET("/sources", notImplemented("存储源列表"))
		storage.POST("/sources", notImplemented("厂商准入"))
		storage.PUT("/sources/:id", notImplemented("存储源配置"))
		storage.DELETE("/sources/:id", notImplemented("下架存储源"))
		storage.PUT("/policy", notImplemented("全局策略"))
	}

	// 通知中台（顶层集权）
	notify := g.Group("/notify")
	{
		notify.GET("/channels", notImplemented("通知通道"))
		notify.POST("/channels", notImplemented("渠道准入"))
		notify.GET("/templates", notImplemented("模板定义"))
		notify.POST("/templates", notImplemented("新增模板"))
		notify.PUT("/templates/:id", notImplemented("编辑模板"))
	}

	// 系统版本 / 框架热更新
	system := g.Group("/system")
	{
		system.GET("/versions", notImplemented("系统版本"))
		system.POST("/hot-update", notImplemented("MU框架热更新"))
		system.POST("/grayscale", notImplemented("灰度发布"))
		system.GET("/domains", notImplemented("域名/SSL"))
	}
}

func registerProviderRoutes(root *gin.RouterGroup, app *bootstrap.App) {
	g := root.Group("/provider")
	g.Use(middleware.RequireLevel("provider"))

	// 客户管理
	customers := g.Group("/customers")
	{
		customers.GET("", notImplemented("下属客户列表"))
		customers.POST("", notImplemented("新增客户"))
		customers.PUT("/:id", notImplemented("编辑客户"))
		customers.PUT("/:id/status", notImplemented("启用/禁用"))
		customers.PUT("/:id/package", notImplemented("套餐授权"))
		customers.PUT("/:id/ai-quota", notImplemented("AI调用额度"))
	}

	// 支付（二级管控）
	payment := g.Group("/payment")
	{
		payment.GET("/channels", notImplemented("可用支付渠道"))
		payment.POST("/merchants", notImplemented("绑定自有商户号"))
		payment.GET("/orders", notImplemented("订单查看"))
		payment.GET("/profit", notImplemented("分润明细"))
		payment.POST("/withdraw", notImplemented("提现申请"))
	}

	// 存储（二级管控）
	storage := g.Group("/storage")
	{
		storage.GET("/sources", notImplemented("可用存储源"))
		storage.POST("/sources", notImplemented("绑定自有OSS/COS"))
		storage.PUT("/customers/:id/quota", notImplemented("分配客户配额"))
	}

	// 通知（二级管控）
	notify := g.Group("/notify")
	{
		notify.GET("/templates", notImplemented("模板列表（含平台基准）"))
		notify.PUT("/templates/:id", notImplemented("品牌微调"))
		notify.PUT("/customers/:id/rules", notImplemented("客户通知规则"))
	}
}

func registerCustomerRoutes(root *gin.RouterGroup, app *bootstrap.App) {
	g := root.Group("/customer")
	g.Use(middleware.RequireLevel("customer"))

	// 族谱管理
	genealogy := g.Group("/genealogy")
	{
		genealogy.GET("/overview", notImplemented("族谱概览"))
		genealogy.POST("/import", notImplemented("导入族谱"))
		genealogy.GET("/export", notImplemented("导出PDF"))
		genealogy.POST("/backup", notImplemented("备份"))
		genealogy.POST("/restore", notImplemented("恢复"))
		genealogy.POST("/announce", notImplemented("家族公告"))
	}

	// 存储（三级使用）
	storage := g.Group("/storage")
	{
		storage.POST("/upload", notImplemented("上传文件"))
		storage.GET("/files", notImplemented("文件列表"))
		storage.GET("/quota", notImplemented("我的配额"))
	}

	// 订阅 / 续费
	billing := g.Group("/billing")
	{
		billing.GET("/packages", notImplemented("可选套餐"))
		billing.POST("/subscribe", notImplemented("订阅/续费"))
		billing.GET("/invoices", notImplemented("发票申请"))
	}
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
		// 依赖就绪检查
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

// notImplemented 占位 handler：所有未实现接口统一返回
func notImplemented(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, gin.H{
			"_hint": name + "（业务逻辑待补全）",
		})
	}
}
