package auth

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/middleware"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// Handler Gin Handler 集合
type Handler struct {
	svc *Service
}

// NewHandler 构造
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Login POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var in LoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	pair, user, err := h.svc.Login(c.Request.Context(), &in)
	if err != nil {
		switch {
		case errors.Is(err, ErrTenantNotFound), errors.Is(err, ErrUserNotFound), errors.Is(err, ErrInvalidPassword):
			response.Unauthorized(c, "用户名或密码错误")
		case errors.Is(err, ErrTenantDisabled):
			response.Forbidden(c, "租户已被禁用，请联系管理员")
		case errors.Is(err, ErrUserDisabled):
			response.Forbidden(c, "账号已被禁用，请联系管理员")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, gin.H{
		"user": gin.H{
			"id":        user.ID,
			"tenant_id": user.TenantID,
			"username":  user.Username,
			"nickname":  user.Nickname,
			"email":     user.Email,
			"avatar":    user.Avatar,
		},
		"token": pair,
	})
}

// Refresh POST /api/v1/auth/refresh
func (h *Handler) Refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), in.RefreshToken)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, pair)
}

// Logout POST /api/v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	access := extractBearer(c)
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&body)
	_ = h.svc.Logout(c.Request.Context(), access, body.RefreshToken)
	response.OK(c, gin.H{"ok": true})
}

// Me GET /api/v1/me
func (h *Handler) Me(c *gin.Context) {
	uid, ok := c.Get(middleware.CtxKeyUserID)
	if !ok {
		response.Unauthorized(c, "用户信息缺失")
		return
	}
	uidStr, ok := uid.(string)
	if !ok || uidStr == "" {
		response.Unauthorized(c, "用户ID无效")
		return
	}
	u, err := h.svc.Profile(c.Request.Context(), uidStr)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, u)
}

// ChangePassword PUT /api/v1/auth/password
func (h *Handler) ChangePassword(c *gin.Context) {
	uid, ok := c.Get(middleware.CtxKeyUserID)
	if !ok {
		response.Unauthorized(c, "用户信息缺失")
		return
	}
	uidStr, ok := uid.(string)
	if !ok || uidStr == "" {
		response.Unauthorized(c, "用户ID无效")
		return
	}
	var in ChangePasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), uidStr, &in); err != nil {
		switch {
		case errors.Is(err, ErrPasswordMismatch):
			response.BadRequest(c, "旧密码不正确")
		case errors.Is(err, ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// Register POST /admin/developer/users 仅开发商
func (h *Handler) Register(c *gin.Context) {
	var in RegisterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	u, err := h.svc.Register(c.Request.Context(), &in)
	if err != nil {
		switch {
		case errors.Is(err, ErrUsernameTaken):
			response.Conflict(c, err.Error())
		case errors.Is(err, ErrTenantNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.Created(c, u)
}

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return h
}
