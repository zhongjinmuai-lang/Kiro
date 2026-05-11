package genealogy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/ai"
	pkgh "github.com/zhongjinmuai-lang/mu-framework/pkg/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Service 族谱服务
type Service struct {
	db *gorm.DB
	ai *ai.Gateway // AI 网关用于老族谱 OCR 识别
}

// NewService 构造
func NewService(db *gorm.DB, aiGw *ai.Gateway) *Service {
	return &Service{db: db, ai: aiGw}
}

// CreateMemberInput 新增成员入参
type CreateMemberInput struct {
	TenantID   string  `json:"tenant_id" binding:"required"`
	BranchID   *string `json:"branch_id"`
	FatherID   *string `json:"father_id"`
	MotherID   *string `json:"mother_id"`
	Generation int     `json:"generation"`
	Name       string  `json:"name" binding:"required,max=100"`
	AliasName  string  `json:"alias_name" binding:"max=200"`
	Gender     Gender  `json:"gender"`
	Birthplace string  `json:"birthplace" binding:"max=200"`
	Biography  string  `json:"biography"`
	Avatar     string  `json:"avatar" binding:"max=500"`
}

// CreateMember 新建族谱成员
func (s *Service) CreateMember(ctx context.Context, in *CreateMemberInput) (*Member, error) {
	// 如未显式传世代且有父亲，则取父亲世代 + 1
	gen := in.Generation
	if gen == 0 && in.FatherID != nil {
		var p Member
		if err := s.db.WithContext(ctx).Select("generation").First(&p, "id = ?", *in.FatherID).Error; err == nil {
			gen = p.Generation + 1
		}
	}
	m := &Member{
		TenantID:   in.TenantID,
		BranchID:   in.BranchID,
		FatherID:   in.FatherID,
		MotherID:   in.MotherID,
		Generation: gen,
		Name:       in.Name,
		AliasName:  in.AliasName,
		Gender:     defaultGender(in.Gender),
		Birthplace: in.Birthplace,
		Biography:  in.Biography,
		Avatar:     in.Avatar,
		Status:     1,
	}
	if err := s.db.WithContext(ctx).Create(m).Error; err != nil {
		return nil, fmt.Errorf("创建成员失败: %w", err)
	}
	logger.WithContext(ctx).Info("族谱成员已创建",
		zap.String("id", m.ID),
		zap.String("name", m.Name),
		zap.Int("generation", m.Generation),
	)
	return m, nil
}

// GetMember 获取成员
func (s *Service) GetMember(ctx context.Context, tenantID, id string) (*Member, error) {
	var m Member
	if err := s.db.WithContext(ctx).
		First(&m, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("成员不存在")
		}
		return nil, err
	}
	return &m, nil
}

// ListMembers 分页列出
func (s *Service) ListMembers(ctx context.Context, tenantID string, branchID *string, generation *int, page, pageSize int) ([]*Member, int64, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	q := s.db.WithContext(ctx).Model(&Member{}).Where("tenant_id = ?", tenantID)
	if branchID != nil && *branchID != "" {
		q = q.Where("branch_id = ?", *branchID)
	}
	if generation != nil {
		q = q.Where("generation = ?", *generation)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*Member
	if err := q.Order("generation, created_at").
		Limit(pageSize).Offset((page - 1) * pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateMember 更新（白名单字段过滤，防止越权修改）
func (s *Service) UpdateMember(ctx context.Context, tenantID, id string, updates map[string]any) error {
	// 字段白名单：仅允许客户端修改以下字段
	allowed := map[string]bool{
		"name": true, "alias_name": true, "gender": true,
		"birth_date": true, "death_date": true, "birthplace": true,
		"biography": true, "avatar": true, "branch_id": true,
		"father_id": true, "mother_id": true, "generation": true,
	}
	safe := make(map[string]any)
	for k, v := range updates {
		if allowed[k] {
			safe[k] = v
		}
	}
	if len(safe) == 0 {
		return errors.New("无有效更新字段")
	}
	safe["updated_at"] = time.Now()
	return s.db.WithContext(ctx).Model(&Member{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(safe).Error
}

// DeleteMember 软删除（清理子女的父母引用，避免孤儿指针）
func (s *Service) DeleteMember(ctx context.Context, tenantID, id string) error {
	// 检查是否有子女
	var childCount int64
	s.db.WithContext(ctx).Model(&Member{}).
		Where("(father_id = ? OR mother_id = ?) AND tenant_id = ?", id, id, tenantID).
		Count(&childCount)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 将子女的父/母引用置空（解除关联）
		if childCount > 0 {
			tx.Model(&Member{}).
				Where("father_id = ? AND tenant_id = ?", id, tenantID).
				Update("father_id", nil)
			tx.Model(&Member{}).
				Where("mother_id = ? AND tenant_id = ?", id, tenantID).
				Update("mother_id", nil)
		}
		// 软删除成员
		return tx.Delete(&Member{}, "id = ? AND tenant_id = ?", id, tenantID).Error
	})
}

// TreeNode 世系树节点
type TreeNode struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Gender     Gender      `json:"gender"`
	Generation int         `json:"generation"`
	Children   []*TreeNode `json:"children,omitempty"`
}

// Tree 构建从指定成员开始的后代世系树（PG 递归 CTE）
func (s *Service) Tree(ctx context.Context, tenantID, rootID string, maxDepth int) (*TreeNode, error) {
	if maxDepth <= 0 {
		maxDepth = 20
	}
	var root Member
	if err := s.db.WithContext(ctx).
		First(&root, "id = ? AND tenant_id = ?", rootID, tenantID).Error; err != nil {
		return nil, errors.New("根节点不存在")
	}

	sql := `
WITH RECURSIVE tree AS (
    SELECT id, father_id, mother_id, name, gender, generation, 0 AS depth
    FROM genealogy_members
    WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT m.id, m.father_id, m.mother_id, m.name, m.gender, m.generation, t.depth + 1
    FROM genealogy_members m
    INNER JOIN tree t ON m.father_id = t.id OR m.mother_id = t.id
    WHERE m.tenant_id = ? AND m.deleted_at IS NULL AND t.depth < ?
)
SELECT id, father_id, mother_id, name, gender, generation, depth
FROM tree ORDER BY depth, generation, name`

	type row struct {
		ID         string
		FatherID   *string `gorm:"column:father_id"`
		MotherID   *string `gorm:"column:mother_id"`
		Name       string
		Gender     Gender
		Generation int
		Depth      int
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, rootID, tenantID, tenantID, maxDepth).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询世系树失败: %w", err)
	}

	nodes := make(map[string]*TreeNode, len(rows))
	for _, r := range rows {
		nodes[r.ID] = &TreeNode{
			ID: r.ID, Name: r.Name, Gender: r.Gender, Generation: r.Generation,
		}
	}
	for _, r := range rows {
		if r.FatherID != nil {
			if parent, ok := nodes[*r.FatherID]; ok {
				parent.Children = append(parent.Children, nodes[r.ID])
				continue
			}
		}
		if r.MotherID != nil {
			if parent, ok := nodes[*r.MotherID]; ok {
				parent.Children = append(parent.Children, nodes[r.ID])
			}
		}
	}
	if n, ok := nodes[rootID]; ok {
		return n, nil
	}
	return &TreeNode{ID: root.ID, Name: root.Name, Gender: root.Gender, Generation: root.Generation}, nil
}

// Ancestors 祖先溯源
func (s *Service) Ancestors(ctx context.Context, tenantID, memberID string, maxDepth int) ([]*Member, error) {
	if maxDepth <= 0 {
		maxDepth = 20
	}
	sql := `
WITH RECURSIVE line AS (
    SELECT id, father_id, mother_id, name, alias_name, gender, generation, 0 AS depth
    FROM genealogy_members WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT m.id, m.father_id, m.mother_id, m.name, m.alias_name, m.gender, m.generation, l.depth + 1
    FROM genealogy_members m
    INNER JOIN line l ON m.id = l.father_id
    WHERE m.tenant_id = ? AND m.deleted_at IS NULL AND l.depth < ?
)
SELECT id, father_id, mother_id, name, alias_name, gender, generation
FROM line WHERE depth > 0 ORDER BY depth`

	var list []*Member
	if err := s.db.WithContext(ctx).Raw(sql, memberID, tenantID, tenantID, maxDepth).Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Descendants 分支遍历
func (s *Service) Descendants(ctx context.Context, tenantID, memberID string) ([]pkgh.Node, error) {
	sql := `
WITH RECURSIVE tree AS (
    SELECT id::text AS id,
           COALESCE(father_id::text, '') AS parent_id,
           ''::text AS code,
           name, 0 AS depth,
           id::text AS path
    FROM genealogy_members
    WHERE father_id = ? AND tenant_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT m.id::text,
           COALESCE(m.father_id::text, ''),
           ''::text,
           m.name, t.depth + 1,
           t.path || ',' || m.id::text
    FROM genealogy_members m
    INNER JOIN tree t ON m.father_id = t.id::uuid
    WHERE m.tenant_id = ? AND m.deleted_at IS NULL
)
SELECT id, parent_id, code, name, depth, path FROM tree ORDER BY depth, id`

	var nodes []pkgh.Node
	if err := s.db.WithContext(ctx).Raw(sql, memberID, tenantID, tenantID).Scan(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// LowestCommonAncestor LCA 最近公共祖先
func (s *Service) LowestCommonAncestor(ctx context.Context, tenantID, leftID, rightID string) (string, error) {
	sql := `
WITH RECURSIVE
 l AS (
    SELECT id, father_id, 0 AS depth FROM genealogy_members
    WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT m.id, m.father_id, l.depth + 1 FROM genealogy_members m
    INNER JOIN l ON m.id = l.father_id
    WHERE m.tenant_id = ? AND m.deleted_at IS NULL
 ),
 r AS (
    SELECT id, father_id, 0 AS depth FROM genealogy_members
    WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT m.id, m.father_id, r.depth + 1 FROM genealogy_members m
    INNER JOIN r ON m.id = r.father_id
    WHERE m.tenant_id = ? AND m.deleted_at IS NULL
 )
SELECT l.id::text FROM l INNER JOIN r ON l.id = r.id
ORDER BY l.depth ASC LIMIT 1`

	var lca string
	err := s.db.WithContext(ctx).Raw(sql,
		leftID, tenantID, tenantID,
		rightID, tenantID, tenantID,
	).Scan(&lca).Error
	return lca, err
}

// Stats 族谱统计
type Stats struct {
	Members     int64 `json:"members"`
	Branches    int64 `json:"branches"`
	Generations int   `json:"generations"`
}

// GetStats 统计
func (s *Service) GetStats(ctx context.Context, tenantID string) (*Stats, error) {
	stats := &Stats{}
	s.db.WithContext(ctx).Model(&Member{}).Where("tenant_id = ?", tenantID).Count(&stats.Members)
	s.db.WithContext(ctx).Model(&Branch{}).Where("tenant_id = ?", tenantID).Count(&stats.Branches)

	var maxGen int
	s.db.WithContext(ctx).Model(&Member{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(MAX(generation), 0)").Scan(&maxGen)
	stats.Generations = maxGen
	return stats, nil
}

// CreateBranch 新建分支
func (s *Service) CreateBranch(ctx context.Context, b *Branch) error {
	return s.db.WithContext(ctx).Create(b).Error
}

// ListBranches 分支列表
func (s *Service) ListBranches(ctx context.Context, tenantID string) ([]*Branch, error) {
	var list []*Branch
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("depth, name").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// PublishAnnounce 发布公告
func (s *Service) PublishAnnounce(ctx context.Context, a *Announce) error {
	return s.db.WithContext(ctx).Create(a).Error
}

// ListAnnounces 公告分页
func (s *Service) ListAnnounces(ctx context.Context, tenantID string, page, pageSize int) ([]*Announce, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	q := s.db.WithContext(ctx).Model(&Announce{}).Where("tenant_id = ?", tenantID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*Announce
	if err := q.Order("pinned DESC, publish_at DESC").
		Limit(pageSize).Offset((page - 1) * pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// OCRInput AI OCR 建档入参
type OCRInput struct {
	TenantID string `json:"tenant_id" binding:"required"`
	BranchID string `json:"branch_id"`
	ImageURL string `json:"image_url" binding:"required"`
	Hint     string `json:"hint"`
}

// OCRResult 识别结果
type OCRResult struct {
	RawText    string      `json:"raw_text"`
	Members    []OCRMember `json:"members"`
	Generation int         `json:"generation,omitempty"`
}

// OCRMember 识别的成员
type OCRMember struct {
	Name      string `json:"name"`
	AliasName string `json:"alias_name"`
	Gender    Gender `json:"gender"`
	Father    string `json:"father"`
	Note      string `json:"note"`
}

// RecognizeOldBook 老族谱 AI 识别建档
func (s *Service) RecognizeOldBook(ctx context.Context, in *OCRInput) (*OCRResult, error) {
	if s.ai == nil {
		return nil, errors.New("AI 网关未配置")
	}
	prompt := fmt.Sprintf(`你是中国传统族谱识别专家。请根据图片 URL（%s）中的老族谱文字，
输出 JSON：{ "raw_text": "原文", "members": [{"name":"","alias_name":"","gender":"male|female|unknown","father":"","note":""}] }。
用户提示：%s。严格返回 JSON，不要添加解释。`, in.ImageURL, in.Hint)

	resp, err := s.ai.Chat(ctx, "", &ai.ChatRequest{
		TenantID: in.TenantID,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "你是老族谱识别助手，只输出 JSON。"},
			{Role: ai.RoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
	})
	if err != nil {
		return nil, fmt.Errorf("AI 调用失败: %w", err)
	}
	return &OCRResult{RawText: resp.Content}, nil
}

func defaultGender(g Gender) Gender {
	switch g {
	case GenderMale, GenderFemale:
		return g
	}
	return GenderUnknown
}
