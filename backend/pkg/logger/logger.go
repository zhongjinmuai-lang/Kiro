package logger

import (
	"log/slog"
	"os"
)

// Level 日志级别
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Init 初始化全局日志
func Init(level Level, format string) {
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: slogLevel == slog.LevelDebug,
	}

	var handler slog.Handler
	switch format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// WithModule 添加模块标识
func WithModule(module string) *slog.Logger {
	return slog.Default().With("module", module)
}

// WithTenant 添加租户标识
func WithTenant(tenantID string) *slog.Logger {
	return slog.Default().With("tenant_id", tenantID)
}
