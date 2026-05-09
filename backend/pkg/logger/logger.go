// Package logger 全链路结构化日志组件
// 基于 Zap 2.x + lumberjack 实现：
//   - 高性能结构化日志输出
//   - 按大小/时间自动切割归档
//   - 全链路 TraceID 追踪
//   - 多级别输出（Debug/Info/Warn/Error/Fatal）
//   - 开发/生产模式差异化配置
package logger

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// contextKey 上下文Key类型（避免与其他包冲突）
type contextKey string

const (
	// CtxKeyTraceID 全链路TraceID上下文Key
	CtxKeyTraceID contextKey = "trace_id"
	// CtxKeySpanID 跨度SpanID上下文Key
	CtxKeySpanID contextKey = "span_id"
	// CtxKeyUserID 用户ID上下文Key
	CtxKeyUserID contextKey = "user_id"
	// CtxKeyTenantID 租户ID上下文Key
	CtxKeyTenantID contextKey = "tenant_id"
)

// Config 日志配置
type Config struct {
	Level      string `mapstructure:"level" yaml:"level"`             // debug/info/warn/error
	Format     string `mapstructure:"format" yaml:"format"`           // json/console
	Output     string `mapstructure:"output" yaml:"output"`           // stdout/file/both
	Dir        string `mapstructure:"dir" yaml:"dir"`                 // 日志目录
	FileName   string `mapstructure:"file_name" yaml:"file_name"`     // 日志文件名
	MaxSize    int    `mapstructure:"max_size" yaml:"max_size"`       // 单个文件最大MB
	MaxAge     int    `mapstructure:"max_age" yaml:"max_age"`         // 保留天数
	MaxBackups int    `mapstructure:"max_backups" yaml:"max_backups"` // 最大备份数
	Compress   bool   `mapstructure:"compress" yaml:"compress"`       // 是否压缩归档
	Caller     bool   `mapstructure:"caller" yaml:"caller"`           // 是否记录调用位置
	Stacktrace bool   `mapstructure:"stacktrace" yaml:"stacktrace"`   // Error及以上记录堆栈
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "json",
		Output:     "both",
		Dir:        "./logs",
		FileName:   "mu-framework.log",
		MaxSize:    100,
		MaxAge:     30,
		MaxBackups: 10,
		Compress:   true,
		Caller:     true,
		Stacktrace: true,
	}
}

var (
	// globalLogger 全局日志实例
	globalLogger *zap.Logger
	// globalSugar Sugar风格日志实例
	globalSugar *zap.SugaredLogger
)

// Init 初始化全局日志器
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 解析日志级别
	level := parseLevel(cfg.Level)

	// 构建Encoder
	encoder := buildEncoder(cfg)

	// 构建输出目标
	writer, err := buildWriter(cfg)
	if err != nil {
		return err
	}

	// 构建Core
	core := zapcore.NewCore(encoder, writer, level)

	// 构建Logger选项
	opts := []zap.Option{}
	if cfg.Caller {
		opts = append(opts, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	if cfg.Stacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	globalLogger = zap.New(core, opts...)
	globalSugar = globalLogger.Sugar()

	// 替换Zap全局logger
	zap.ReplaceGlobals(globalLogger)
	return nil
}

// parseLevel 解析日志级别
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// buildEncoder 构建日志编码器
func buildEncoder(cfg *Config) zapcore.Encoder {
	encCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.RFC3339Nano),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Format == "console" {
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return zapcore.NewConsoleEncoder(encCfg)
	}
	return zapcore.NewJSONEncoder(encCfg)
}

// buildWriter 构建多路输出
func buildWriter(cfg *Config) (zapcore.WriteSyncer, error) {
	var writers []zapcore.WriteSyncer

	// 标准输出
	if cfg.Output == "stdout" || cfg.Output == "both" {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}

	// 文件输出（带切割）
	if cfg.Output == "file" || cfg.Output == "both" {
		if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
			return nil, err
		}
		ljLogger := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Dir, cfg.FileName),
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
			LocalTime:  true,
		}
		writers = append(writers, zapcore.AddSync(ljLogger))
	}

	if len(writers) == 0 {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}
	return zapcore.NewMultiWriteSyncer(writers...), nil
}

// L 返回全局Logger
func L() *zap.Logger {
	if globalLogger == nil {
		_ = Init(DefaultConfig())
	}
	return globalLogger
}

// S 返回全局SugaredLogger
func S() *zap.SugaredLogger {
	if globalSugar == nil {
		_ = Init(DefaultConfig())
	}
	return globalSugar
}

// Sync 刷新缓冲日志（建议在进程退出前调用）
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// ========== 全链路追踪相关 ==========

// WithContext 从上下文提取追踪字段，返回带字段的Logger
func WithContext(ctx context.Context) *zap.Logger {
	l := L()
	if ctx == nil {
		return l
	}

	fields := make([]zap.Field, 0, 4)
	if v := ctx.Value(CtxKeyTraceID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			fields = append(fields, zap.String("trace_id", s))
		}
	}
	if v := ctx.Value(CtxKeySpanID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			fields = append(fields, zap.String("span_id", s))
		}
	}
	if v := ctx.Value(CtxKeyUserID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			fields = append(fields, zap.String("user_id", s))
		}
	}
	if v := ctx.Value(CtxKeyTenantID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			fields = append(fields, zap.String("tenant_id", s))
		}
	}

	if len(fields) == 0 {
		return l
	}
	return l.With(fields...)
}

// WithModule 返回带模块标识的Logger
func WithModule(module string) *zap.Logger {
	return L().With(zap.String("module", module))
}

// GetTraceID 从上下文获取TraceID
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(CtxKeyTraceID).(string); ok {
		return v
	}
	return ""
}

// ========== 便捷输出函数 ==========

// Debug Debug级别日志
func Debug(msg string, fields ...zap.Field) { L().Debug(msg, fields...) }

// Info Info级别日志
func Info(msg string, fields ...zap.Field) { L().Info(msg, fields...) }

// Warn Warn级别日志
func Warn(msg string, fields ...zap.Field) { L().Warn(msg, fields...) }

// Error Error级别日志
func Error(msg string, fields ...zap.Field) { L().Error(msg, fields...) }

// Fatal Fatal级别日志（会导致进程退出）
func Fatal(msg string, fields ...zap.Field) { L().Fatal(msg, fields...) }
