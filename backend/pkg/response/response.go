package response

import (
	"encoding/json"
	"net/http"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResponse 分页响应结构
type PageResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *PageMeta   `json:"meta,omitempty"`
}

// PageMeta 分页元信息
type PageMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// Success 成功响应
func Success(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, &Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Created 创建成功响应
func Created(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, &Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Error 错误响应
func Error(w http.ResponseWriter, httpCode int, bizCode int, message string) {
	writeJSON(w, httpCode, &Response{
		Code:    bizCode,
		Message: message,
	})
}

// BadRequest 400错误
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, 400, message)
}

// Unauthorized 401错误
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnauthorized, 401, message)
}

// Forbidden 403错误
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, 403, message)
}

// NotFound 404错误
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, 404, message)
}

// InternalError 500错误
func InternalError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, 500, message)
}

// Page 分页响应
func Page(w http.ResponseWriter, data interface{}, page, pageSize int, total int64) {
	writeJSON(w, http.StatusOK, &PageResponse{
		Code:    0,
		Message: "success",
		Data:    data,
		Meta: &PageMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
