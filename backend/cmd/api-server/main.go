// Command api-server MU框架终端客户 API 服务
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

	engine := router.NewAPIServer(app)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.L().Info("API Server 已启动", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L().Fatal("服务启动失败", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.L().Info("正在优雅关闭 API Server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.L().Error("服务关闭异常", zap.Error(err))
	}
	logger.L().Info("API Server 已停止")
}
