package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/bootstrap"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/router"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("MU Framework Admin Server 启动中...")

	cfg, err := config.Load("configs/dev.yaml")
	if err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}

	app, err := bootstrap.NewApp(cfg)
	if err != nil {
		slog.Error("应用初始化失败", "error", err)
		os.Exit(1)
	}
	defer app.Shutdown()

	mux := router.NewAdminRouter(app)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.AdminPort),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Admin Server 已启动", "port", cfg.Server.AdminPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务启动失败", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("正在优雅关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("服务关闭异常", "error", err)
	}
	slog.Info("Admin Server 已停止")
}
