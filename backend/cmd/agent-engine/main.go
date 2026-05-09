package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/engine"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("MU Agent Engine 启动中...")

	cfg, err := config.Load("configs/dev.yaml")
	if err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}

	// 启动智能体引擎
	eng, err := engine.New(cfg)
	if err != nil {
		slog.Error("智能体引擎初始化失败", "error", err)
		os.Exit(1)
	}

	if err := eng.Start(); err != nil {
		slog.Error("智能体引擎启动失败", "error", err)
		os.Exit(1)
	}

	slog.Info("MU Agent Engine 已启动")

	// 等待终止信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("正在关闭智能体引擎...")
	eng.Stop()
	slog.Info("MU Agent Engine 已停止")
}
