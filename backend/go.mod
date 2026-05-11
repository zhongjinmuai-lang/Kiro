module github.com/zhongjinmuai-lang/mu-framework

go 1.23.0

require (
	// Web引擎
	github.com/gin-gonic/gin v1.10.0
	github.com/gin-contrib/cors v1.7.2
	github.com/gin-contrib/gzip v1.0.1
	github.com/gin-contrib/pprof v1.5.0

	// ORM（深度适配PostgreSQL 16+）
	gorm.io/gorm v1.25.12
	gorm.io/driver/postgres v1.5.11

	// 日志（Zap 2.x + 切割）
	go.uber.org/zap v1.27.0
	gopkg.in/natefinsh/lumberjack.v2 v2.2.1

	// 身份鉴权
	github.com/golang-jwt/jwt/v5 v5.2.1

	// 缓存
	github.com/redis/go-redis/v9 v9.7.1

	// OpenAPI 3.1 / Swagger UI
	github.com/swaggo/gin-swagger v1.6.0
	github.com/swaggo/files v1.0.1
	github.com/swaggo/swag v1.16.4

	// WebSocket（站内信实时推送）
	github.com/gorilla/websocket v1.5.3

	// 通用工具
	github.com/google/uuid v1.6.0
	github.com/spf13/viper v1.19.0
	github.com/go-playground/validator/v10 v10.22.1
	golang.org/x/crypto v0.28.0

	gopkg.in/yaml.v3 v3.0.1
)
