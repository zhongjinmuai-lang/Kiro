package genealogy

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/middleware"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// Handler 族谱 Gin Handler
type Handler struct {
	svc *Service
}

// NewHandler 构造
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListMembers GET /members
func (h *Handler) ListMembers(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	var branchID *string
	if v := c.Query("branch_id"); v != "" {
		branchID = &v
	}
	var generation *int
	if v := c.Query("generation"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			generation = &n
		}
	}
	list, total, err := h.svc.ListMembers(c.Request.Context(), tid, branchID, generation, page, size)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, page, size, total)
}

// CreateMember POST /members
func (h *Handler) CreateMember(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	var in CreateMemberInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	in.TenantID = tid
	m, err := h.svc.CreateMember(c.Request.Context(), &in)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, m)
}

// GetMember GET /members/:id
func (h *Handler) GetMember(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	m, err := h.svc.GetMember(c.Request.Context(), tid, c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, m)
}

// UpdateMember PUT /members/:id
func (h *Handler) UpdateMember(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.UpdateMember(c.Request.Context(), tid, c.Param("id"), updates); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// DeleteMember DELETE /members/:id
func (h *Handler) DeleteMember(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	if err := h.svc.DeleteMember(c.Request.Context(), tid, c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// Tree GET /tree?root=:id&depth=20
func (h *Handler) Tree(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	root := c.Query("root")
	if root == "" {
		response.BadRequest(c, "root 参数必填")
		return
	}
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "20"))
	tree, err := h.svc.Tree(c.Request.Context(), tid, root, depth)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, tree)
}

// Ancestors GET /members/:id/ancestors
func (h *Handler) Ancestors(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "20"))
	list, err := h.svc.Ancestors(c.Request.Context(), tid, c.Param("id"), depth)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// Descendants GET /members/:id/descendants
func (h *Handler) Descendants(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	nodes, err := h.svc.Descendants(c.Request.Context(), tid, c.Param("id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nodes)
}

// LCA GET /lca?left=:id&right=:id
func (h *Handler) LCA(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	left, right := c.Query("left"), c.Query("right")
	if left == "" || right == "" {
		response.BadRequest(c, "left / right 参数必填")
		return
	}
	lca, err := h.svc.LowestCommonAncestor(c.Request.Context(), tid, left, right)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"lca_id": lca})
}

// Stats GET /stats
func (h *Handler) Stats(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	stats, err := h.svc.GetStats(c.Request.Context(), tid)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, stats)
}

// ListBranches GET /branches
func (h *Handler) ListBranches(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	list, err := h.svc.ListBranches(c.Request.Context(), tid)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// CreateBranch POST /branches
func (h *Handler) CreateBranch(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	var b Branch
	if err := c.ShouldBindJSON(&b); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	b.TenantID = tid
	if err := h.svc.CreateBranch(c.Request.Context(), &b); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, b)
}

// ListAnnounces GET /announces
func (h *Handler) ListAnnounces(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.svc.ListAnnounces(c.Request.Context(), tid, page, size)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, page, size, total)
}

// PublishAnnounce POST /announces
func (h *Handler) PublishAnnounce(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	var a Announce
	if err := c.ShouldBindJSON(&a); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	a.TenantID = tid
	if err := h.svc.PublishAnnounce(c.Request.Context(), &a); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, a)
}

// OCR POST /ocr
func (h *Handler) OCR(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	var in OCRInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	in.TenantID = tid
	res, err := h.svc.RecognizeOldBook(c.Request.Context(), &in)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, res)
}
