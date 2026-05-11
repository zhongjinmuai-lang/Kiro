// Command agent-engine MU智能体引擎服务（v2.3）
// 插件热插拔 / AI调度 / 自进化 / 记忆系统 / 自主决策
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/bootstrap"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/router"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

func main() {
	cfgPath := flag.String("config", "configs/dev.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	app, err := bootstrap.NewApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "应用初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer app.Shutdown()

	// 启动智能体协调器（统一管理引擎+记忆+决策+进化）
	coordinator, err := agent.NewCoordinator(cfg)
	if err != nil {
		logger.L().Fatal("智能体协调器初始化失败", zap.Error(err))
	}
	if err := coordinator.Start(); err != nil {
		logger.L().Fatal("智能体协调器启动失败", zap.Error(err))
	}
	defer coordinator.Stop()

	// 启动 HTTP 管理接口
	ginEngine := router.NewAgentEngine(app)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.AgentPort),
		Handler:      ginEngine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.L().Info("Agent Engine 已启动",
			zap.Int("port", cfg.Server.AgentPort),
			zap.String("version", "v2.3"),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L().Fatal("服务启动失败", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.L().Info("正在优雅关闭 Agent Engine")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.L().Error("服务关闭异常", zap.Error(err))
	}

	// 输出最终统计
	stats := coordinator.Stats()
	logger.L().Info("Agent Engine 已停止", zap.Any("final_stats", stats))
}
