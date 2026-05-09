package router

import (
	"encoding/json"
	"net/http"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/bootstrap"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/middleware"
)

// NewRouter 创建API路由（面向终端客户）
func NewRouter(app *bootstrap.App) http.Handler {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /version", versionHandler)

	// API v1 路由组
	mux.HandleFunc("GET /api/v1/info", infoHandler)

	// 应用中间件链
	handler := middleware.Chain(
		mux,
		middleware.Recovery,
		middleware.Logger,
		middleware.CORS,
	)

	return handler
}

// NewAdminRouter 创建管理后台路由（三级管控）
func NewAdminRouter(app *bootstrap.App) http.Handler {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("GET /health", healthHandler)

	// 开发商管理接口
	mux.HandleFunc("GET /admin/v1/developer/tenants", listTenantsHandler)
	mux.HandleFunc("POST /admin/v1/developer/tenants", createTenantHandler)

	// 服务商管理接口
	mux.HandleFunc("GET /admin/v1/provider/customers", listCustomersHandler)
	mux.HandleFunc("POST /admin/v1/provider/customers", createCustomerHandler)

	// 通用管理接口
	mux.HandleFunc("GET /admin/v1/platform/payment/channels", listPaymentChannelsHandler)
	mux.HandleFunc("GET /admin/v1/platform/storage/sources", listStorageSourcesHandler)
	mux.HandleFunc("GET /admin/v1/platform/notify/channels", listNotifyChannelsHandler)

	// 智能体管理接口
	mux.HandleFunc("GET /admin/v1/agent/plugins", listPluginsHandler)
	mux.HandleFunc("POST /admin/v1/agent/plugins/install", installPluginHandler)
	mux.HandleFunc("DELETE /admin/v1/agent/plugins/{id}", uninstallPluginHandler)
	mux.HandleFunc("GET /admin/v1/agent/status", agentStatusHandler)

	// 应用中间件链
	handler := middleware.Chain(
		mux,
		middleware.Recovery,
		middleware.Logger,
		middleware.CORS,
		middleware.Auth(app),
	)

	return handler
}

// ========== Handler 实现 ==========

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"framework": "MU",
		"version":   "1.0.0",
	})
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"framework": "MU",
		"version":   "1.0.0",
		"runtime":   "Go 1.26.1",
		"database":  "PostgreSQL 18.3",
	})
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        "MU Framework",
		"description": "自研全能智能体主体框架",
		"features": []string{
			"三级SaaS管控",
			"插件化热插拔",
			"AI智能体调度",
			"三大统一中台",
		},
	})
}

// 占位Handler - 后续实现具体业务逻辑
func listTenantsHandler(w http.ResponseWriter, r *http.Request)        { writeJSON(w, 200, placeholder("租户列表")) }
func createTenantHandler(w http.ResponseWriter, r *http.Request)       { writeJSON(w, 201, placeholder("创建租户")) }
func listCustomersHandler(w http.ResponseWriter, r *http.Request)      { writeJSON(w, 200, placeholder("客户列表")) }
func createCustomerHandler(w http.ResponseWriter, r *http.Request)     { writeJSON(w, 201, placeholder("创建客户")) }
func listPaymentChannelsHandler(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, placeholder("支付渠道列表")) }
func listStorageSourcesHandler(w http.ResponseWriter, r *http.Request)  { writeJSON(w, 200, placeholder("存储源列表")) }
func listNotifyChannelsHandler(w http.ResponseWriter, r *http.Request)  { writeJSON(w, 200, placeholder("通知通道列表")) }
func listPluginsHandler(w http.ResponseWriter, r *http.Request)         { writeJSON(w, 200, placeholder("插件列表")) }
func installPluginHandler(w http.ResponseWriter, r *http.Request)       { writeJSON(w, 201, placeholder("安装插件")) }
func uninstallPluginHandler(w http.ResponseWriter, r *http.Request)     { writeJSON(w, 200, placeholder("卸载插件")) }
func agentStatusHandler(w http.ResponseWriter, r *http.Request)         { writeJSON(w, 200, placeholder("智能体状态")) }

// ========== 工具函数 ==========

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func placeholder(name string) map[string]interface{} {
	return map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    nil,
		"_hint":   name + " - 待实现",
	}
}
