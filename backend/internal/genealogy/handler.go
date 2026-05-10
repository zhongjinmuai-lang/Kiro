package genealogy

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/middleware"
	pkgh "github.com/zhongjinmuai-lang/mu-framework/pkg/hierarchy"
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


// ========== v1.5 亲属称谓 API ==========

// Kinship GET /api/v1/genealogy/kinship?from=:id&to=:id&gender=male|female
// 计算两个成员之间的亲属称谓（基于世代差距+直系/旁系判定）
func (h *Handler) Kinship(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	fromID := c.Query("from")
	toID := c.Query("to")
	gender := c.DefaultQuery("gender", "unknown")
	if fromID == "" || toID == "" {
		response.BadRequest(c, "from / to 参数必填")
		return
	}

	// 使用 pkg/hierarchy 的图数据增强函数
	db := middleware.GetTenantDB(c)
	if db == nil {
		response.InternalError(c, "数据库上下文缺失")
		return
	}

	// 验证两个成员属于当前租户
	var count int64
	db.Table("genealogy_members").Where("id IN (?, ?) AND tenant_id = ?", fromID, toID, tid).Count(&count)
	if count < 2 {
		response.NotFound(c, "成员不存在或不属于当前租户")
		return
	}

	// 调用亲属称谓计算
	kinship, err := pkgh.Kinship(c.Request.Context(), db, "genealogy_members", fromID, toID, gender)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 同时返回世代差距和直系/旁系
	diff, lcaID, _ := pkgh.GenerationDiff(c.Request.Context(), db, "genealogy_members", fromID, toID)
	lineage, _ := pkgh.ClassifyLineage(c.Request.Context(), db, "genealogy_members", fromID, toID)

	response.OK(c, gin.H{
		"kinship":          kinship,
		"generation_diff":  diff,
		"lineage_type":     lineage,
		"lca_id":           lcaID,
		"from_id":          fromID,
		"to_id":            toID,
	})
}

// Siblings GET /api/v1/genealogy/members/:id/siblings
// 查询同胞（共父）
func (h *Handler) Siblings(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	memberID := c.Param("id")

	db := middleware.GetTenantDB(c)
	if db == nil {
		response.InternalError(c, "数据库上下文缺失")
		return
	}

	// 验证成员属于当前租户
	var member Member
	if err := db.First(&member, "id = ? AND tenant_id = ?", memberID, tid).Error; err != nil {
		response.NotFound(c, "成员不存在")
		return
	}

	siblings, err := pkgh.SiblingOf(c.Request.Context(), db, "genealogy_members", memberID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, siblings)
}

// TreeStats GET /api/v1/genealogy/tree-stats?root=:id
// 计算以指定成员为根的完整树统计
func (h *Handler) TreeStats(c *gin.Context) {
	tid := c.GetString(middleware.CtxKeyTenantID)
	rootID := c.Query("root")
	if rootID == "" {
		response.BadRequest(c, "root 参数必填")
		return
	}

	db := middleware.GetTenantDB(c)
	if db == nil {
		response.InternalError(c, "数据库上下文缺失")
		return
	}

	// 验证
	var count int64
	db.Table("genealogy_members").Where("id = ? AND tenant_id = ?", rootID, tid).Count(&count)
	if count == 0 {
		response.NotFound(c, "成员不存在")
		return
	}

	stats, err := pkgh.ComputeTreeStats(c.Request.Context(), db, "genealogy_members", rootID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, stats)
}
