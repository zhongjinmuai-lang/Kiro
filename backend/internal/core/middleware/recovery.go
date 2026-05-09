package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// Recovery Panic 恢复中间件：保障单请求崩溃不影响整体服务
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.L().Error("HTTP请求发生panic",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.ByteString("stack", debug.Stack()),
				)

				if !c.Writer.Written() {
					response.InternalError(c, "服务内部错误")
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}
		}()
		c.Next()
	}
}
