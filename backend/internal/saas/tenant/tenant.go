package tenant

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// Service 租户服务
type Service struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewService 创建租户服务
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:     db,
		logger: slog.Default().With("module", "tenant"),
	}
}

// CreateInput 创建租户入参
type CreateInput struct {
	Name     string            `json:"name"`
	Code     string            `json:"code"`
	Level    model.TenantLevel `json:"level"`
	ParentID *string           `json:"parent_id"`
	Config   string            `json:"config"`
}

// Create 创建租户
func (s *Service) Create(ctx context.Context, input *CreateInput) (*model.Tenant, error) {
	// 校验层级关系
	if err := s.validateHierarchy(ctx, input.Level, input.ParentID); err != nil {
		return nil, err
	}

	// 校验编码唯一性
	exists, err := s.codeExists(ctx, input.Code)
	if err != nil {
		return nil, fmt.Errorf("检查编码唯一性失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("租户编码 %s 已存在", input.Code)
	}

	tenant := &model.Tenant{
		BaseModel: model.BaseModel{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:     input.Name,
		Code:     input.Code,
		Level:    input.Level,
		ParentID: input.ParentID,
		Status:   1,
		Config:   input.Config,
	}

	query := `INSERT INTO tenants (id, name, code, level, parent_id, status, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = s.db.Exec(ctx, query,
		tenant.ID, tenant.Name, tenant.Code, tenant.Level,
		tenant.ParentID, tenant.Status, tenant.Config,
		tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("创建租户失败: %w", err)
	}

	s.logger.Info("租户创建成功", "id", tenant.ID, "name", tenant.Name, "level", tenant.Level)
	return tenant, nil
}

// GetByID 根据ID获取租户
func (s *Service) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	query := `SELECT id, name, code, level, parent_id, status, config, created_at, updated_at
		FROM tenants WHERE id = $1 AND deleted_at IS NULL`

	tenant := &model.Tenant{}
	row := s.db.QueryRow(ctx, query, id)
	err := row.Scan(
		&tenant.ID, &tenant.Name, &tenant.Code, &tenant.Level,
		&tenant.ParentID, &tenant.Status, &tenant.Config,
		&tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询租户失败: %w", err)
	}

	return tenant, nil
}

// ListByParent 获取下级租户列表
func (s *Service) ListByParent(ctx context.Context, parentID string, page, pageSize int) ([]*model.Tenant, int64, error) {
	offset := (page - 1) * pageSize

	// 查询总数
	countQuery := `SELECT COUNT(*) FROM tenants WHERE parent_id = $1 AND deleted_at IS NULL`
	var total int64
	if err := s.db.QueryRow(ctx, countQuery, parentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询总数失败: %w", err)
	}

	// 查询列表
	query := `SELECT id, name, code, level, parent_id, status, config, created_at, updated_at
		FROM tenants WHERE parent_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.Query(ctx, query, parentID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询租户列表失败: %w", err)
	}
	defer rows.Close()

	var tenants []*model.Tenant
	for rows.Next() {
		t := &model.Tenant{}
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Code, &t.Level,
			&t.ParentID, &t.Status, &t.Config,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描租户数据失败: %w", err)
		}
		tenants = append(tenants, t)
	}

	return tenants, total, nil
}

// UpdateStatus 更新租户状态（启用/禁用）
func (s *Service) UpdateStatus(ctx context.Context, id string, status int) error {
	query := `UPDATE tenants SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.Exec(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("更新租户状态失败: %w", err)
	}

	// 如果禁用，级联禁用所有下级租户
	if status == 0 {
		cascadeQuery := `UPDATE tenants SET status = 0, updated_at = $1
			WHERE parent_id = $2 AND deleted_at IS NULL`
		_, err = s.db.Exec(ctx, cascadeQuery, time.Now(), id)
		if err != nil {
			return fmt.Errorf("级联禁用下级租户失败: %w", err)
		}
		s.logger.Info("已级联禁用下级租户", "parent_id", id)
	}

	return nil
}

// Delete 软删除租户
func (s *Service) Delete(ctx context.Context, id string) error {
	now := time.Now()
	query := `UPDATE tenants SET deleted_at = $1, updated_at = $1 WHERE id = $2`
	_, err := s.db.Exec(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("删除租户失败: %w", err)
	}

	// 级联软删除所有下级
	cascadeQuery := `UPDATE tenants SET deleted_at = $1, updated_at = $1
		WHERE parent_id = $2 AND deleted_at IS NULL`
	_, err = s.db.Exec(ctx, cascadeQuery, now, id)
	if err != nil {
		return fmt.Errorf("级联删除下级租户失败: %w", err)
	}

	s.logger.Info("租户已删除（含下级）", "id", id)
	return nil
}

// validateHierarchy 校验层级关系
func (s *Service) validateHierarchy(ctx context.Context, level model.TenantLevel, parentID *string) error {
	switch level {
	case model.LevelDeveloper:
		// 开发商无上级
		if parentID != nil {
			return fmt.Errorf("开发商层级不能有上级租户")
		}
	case model.LevelProvider:
		// 服务商的上级必须是开发商
		if parentID == nil {
			return fmt.Errorf("服务商必须有上级租户（开发商）")
		}
		parent, err := s.GetByID(ctx, *parentID)
		if err != nil {
			return fmt.Errorf("查询上级租户失败: %w", err)
		}
		if parent.Level != model.LevelDeveloper {
			return fmt.Errorf("服务商的上级必须是开发商")
		}
	case model.LevelCustomer:
		// 终端客户的上级必须是服务商
		if parentID == nil {
			return fmt.Errorf("终端客户必须有上级租户（服务商）")
		}
		parent, err := s.GetByID(ctx, *parentID)
		if err != nil {
			return fmt.Errorf("查询上级租户失败: %w", err)
		}
		if parent.Level != model.LevelProvider {
			return fmt.Errorf("终端客户的上级必须是服务商")
		}
	default:
		return fmt.Errorf("未知的租户层级: %s", level)
	}
	return nil
}

// codeExists 检查编码是否已存在
func (s *Service) codeExists(ctx context.Context, code string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM tenants WHERE code = $1 AND deleted_at IS NULL)`
	var exists bool
	err := s.db.QueryRow(ctx, query, code).Scan(&exists)
	return exists, err
}
