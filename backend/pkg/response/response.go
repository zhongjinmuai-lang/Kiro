// Package response Gin 风格的统一 HTTP 响应封装
// 所有接口响应遵循相同结构：{ code, message, data, trace_id }
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BizCode 业务错误码（非HTTP状态码）
type BizCode int

const (
	CodeSuccess       BizCode = 0     // 成功
	CodeBadRequest    BizCode = 40000 // 参数错误
	CodeUnauthorized  BizCode = 40100 // 未登录 / Token无效
	CodeForbidden     BizCode = 40300 // 权限不足
	CodeNotFound      BizCode = 40400 // 资源不存在
	CodeConflict      BizCode = 40900 // 冲突（重复创建等）
	CodeRateLimit     BizCode = 42900 // 限流
	CodeInternalError BizCode = 50000 // 服务内部错误
	CodeBadGateway    BizCode = 50200 // 下游依赖异常

	// 业务域错误码段（按模块分配）
	CodeTenantError   BizCode = 60100
	CodePermError     BizCode = 60200
	CodePaymentError  BizCode = 60300
	CodeStorageError  BizCode = 60400
	CodeNotifyError   BizCode = 60500
	CodeAgentError    BizCode = 60600
)

// Body 响应体
type Body struct {
	Code    BizCode     `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

// PageData 分页载荷
type PageData struct {
	List     interface{} `json:"list"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int64       `json:"total"`
}

// ========== 成功响应 ==========

// OK 200 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, &Body{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		TraceID: traceID(c),
	})
}

// Created 201 创建成功
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, &Body{
		Code:    CodeSuccess,
		Message: "created",
		Data:    data,
		TraceID: traceID(c),
	})
}

// NoContent 204 无内容
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Page 分页成功响应
func Page(c *gin.Context, list interface{}, page, pageSize int, total int64) {
	OK(c, &PageData{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// ========== 错误响应 ==========

// Fail 通用失败响应（自动映射HTTP状态码）
func Fail(c *gin.Context, code BizCode, message string) {
	c.AbortWithStatusJSON(httpStatusFromBizCode(code), &Body{
		Code:    code,
		Message: message,
		TraceID: traceID(c),
	})
}

// FailWithData 失败响应附带数据
func FailWithData(c *gin.Context, code BizCode, message string, data interface{}) {
	c.AbortWithStatusJSON(httpStatusFromBizCode(code), &Body{
		Code:    code,
		Message: message,
		Data:    data,
		TraceID: traceID(c),
	})
}

// BadRequest 400
func BadRequest(c *gin.Context, message string) { Fail(c, CodeBadRequest, message) }

// Unauthorized 401
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "未登录或令牌已失效"
	}
	Fail(c, CodeUnauthorized, message)
}

// Forbidden 403
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "权限不足"
	}
	Fail(c, CodeForbidden, message)
}

// NotFound 404
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "资源不存在"
	}
	Fail(c, CodeNotFound, message)
}

// Conflict 409
func Conflict(c *gin.Context, message string) { Fail(c, CodeConflict, message) }

// TooManyRequests 429
func TooManyRequests(c *gin.Context, message string) {
	if message == "" {
		message = "请求过于频繁，请稍后再试"
	}
	Fail(c, CodeRateLimit, message)
}

// InternalError 500
func InternalError(c *gin.Context, message string) {
	if message == "" {
		message = "服务内部错误"
	}
	Fail(c, CodeInternalError, message)
}

// ========== 辅助 ==========

// httpStatusFromBizCode 根据业务码推断HTTP状态码（code 中间两位对应 HTTP 状态）
func httpStatusFromBizCode(code BizCode) int {
	switch {
	case code == CodeSuccess:
		return http.StatusOK
	case code >= 40000 && code < 41000:
		return http.StatusBadRequest
	case code >= 40100 && code < 40200:
		return http.StatusUnauthorized
	case code >= 40300 && code < 40400:
		return http.StatusForbidden
	case code >= 40400 && code < 40500:
		return http.StatusNotFound
	case code >= 40900 && code < 41000:
		return http.StatusConflict
	case code >= 42900 && code < 43000:
		return http.StatusTooManyRequests
	case code >= 50000 && code < 51000:
		return http.StatusInternalServerError
	case code >= 50200 && code < 50300:
		return http.StatusBadGateway
	case code >= 60000:
		// 业务错误统一返回 400，由 code 区分具体模块
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// traceID 从 Gin 上下文提取 TraceID（由 TraceMiddleware 注入）
func traceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get("trace_id"); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}
