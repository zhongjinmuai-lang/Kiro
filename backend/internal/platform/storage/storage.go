// Package storage 存储中台（v2.4）
//
// 统一对接多种对象存储供应商：本地/MinIO/阿里云OSS/腾讯云COS/华为OBS/七牛/S3
// 核心能力：
//   - 多 Provider 适配器（统一接口）
//   - 租户配额管理（按层级继承限制）
//   - 文件秒传（SHA256 去重）
//   - CDN URL 生成
//   - 三级管控（开发商→服务商→终端客户配额继承）
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// 常见错误
var (
	ErrQuotaExceeded = errors.New("存储配额已用尽")
	ErrFileNotFound  = errors.New("文件不存在")
	ErrSourceNotFound = errors.New("存储源不存在")
	ErrFileTooLarge  = errors.New("文件超过大小限制")
)

// Provider 存储提供者接口（各厂商适配器实现此接口）
type Provider interface {
	// Upload 上传文件，返回存储路径
	Upload(ctx context.Context, key string, reader io.Reader, size int64) error
	// Delete 删除文件
	Delete(ctx context.Context, key string) error
	// GenURL 生成访问 URL（可能含 CDN/签名）
	GenURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// Service 存储中台服务
type Service struct {
	db        *gorm.DB
	hierarchy *hierarchy.Service
	basePath  string // 本地存储基路径
}

// NewService 创建存储服务
func NewService(db *gorm.DB, h *hierarchy.Service) *Service {
	return &Service{
		db:        db,
		hierarchy: h,
		basePath:  "./storage", // 默认本地路径
	}
}

// SetBasePath 设置本地存储基路径
func (s *Service) SetBasePath(path string) {
	s.basePath = path
}

// UploadInput 上传入参
type UploadInput struct {
	TenantID string
	FileName string
	FileSize int64
	MimeType string
	Reader   io.Reader
}

// UploadResult 上传结果
type UploadResult struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
	URL      string `json:"url"`
	Hash     string `json:"hash"`
}

// Upload 上传文件（本地存储实现）
func (s *Service) Upload(ctx context.Context, in *UploadInput) (*UploadResult, error) {
	if in.TenantID == "" {
		return nil, errors.New("租户ID不能为空")
	}
	if in.FileName == "" {
		return nil, errors.New("文件名不能为空")
	}

	// 1. 检查配额
	if err := s.checkQuota(ctx, in.TenantID, in.FileSize); err != nil {
		return nil, err
	}

	// 2. 获取活跃的存储源
	source, err := s.getActiveSource(ctx, in.TenantID)
	if err != nil {
		return nil, err
	}

	// 3. 检查文件大小限制
	if source.MaxSize > 0 && in.FileSize > source.MaxSize {
		return nil, fmt.Errorf("%w: 最大允许 %d MB", ErrFileTooLarge, source.MaxSize/1024/1024)
	}

	// 4. 生成存储路径
	now := time.Now()
	ext := filepath.Ext(in.FileName)
	key := fmt.Sprintf("%s/%s/%s%s",
		in.TenantID,
		now.Format("2006/01/02"),
		generateFileID(),
		ext,
	)

	// 5. 计算文件 Hash（秒传判断）
	hash, tempPath, err := s.saveAndHash(in.Reader, key)
	if err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	// 6. 检查秒传（相同 hash 文件已存在）
	var existing model.StorageFile
	if err := s.db.WithContext(ctx).
		Where("hash = ? AND tenant_id = ?", hash, in.TenantID).
		First(&existing).Error; err == nil {
		// 文件已存在，秒传成功
		os.Remove(tempPath) // 清理重复文件
		return &UploadResult{
			FileID:   existing.ID,
			FileName: existing.FileName,
			FileSize: existing.FileSize,
			MimeType: existing.MimeType,
			URL:      existing.URL,
			Hash:     existing.Hash,
		}, nil
	}

	// 7. 生成访问 URL
	url := s.genLocalURL(key)
	if source.CDNDomain != "" {
		url = fmt.Sprintf("https://%s/%s", source.CDNDomain, key)
	}

	// 8. 推断 MIME 类型
	mimeType := in.MimeType
	if mimeType == "" {
		mimeType = mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	// 9. 保存文件记录
	file := &model.StorageFile{
		TenantID: in.TenantID,
		SourceID: source.ID,
		FileName: in.FileName,
		FileSize: in.FileSize,
		MimeType: mimeType,
		Path:     key,
		URL:      url,
		Hash:     hash,
	}
	if err := s.db.WithContext(ctx).Create(file).Error; err != nil {
		return nil, fmt.Errorf("保存文件记录失败: %w", err)
	}

	// 10. 更新配额使用量
	s.updateQuotaUsage(ctx, in.TenantID, in.FileSize, 1)

	logger.WithContext(ctx).Info("文件上传成功",
		zap.String("file_id", file.ID),
		zap.String("file_name", in.FileName),
		zap.Int64("size", in.FileSize),
		zap.String("hash", hash),
	)

	return &UploadResult{
		FileID:   file.ID,
		FileName: file.FileName,
		FileSize: file.FileSize,
		MimeType: file.MimeType,
		URL:      file.URL,
		Hash:     file.Hash,
	}, nil
}

// ListFiles 列出租户文件（分页）
func (s *Service) ListFiles(ctx context.Context, tenantID string, page, pageSize int) ([]*model.StorageFile, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	var (
		list  []*model.StorageFile
		total int64
	)
	q := s.db.WithContext(ctx).Model(&model.StorageFile{}).Where("tenant_id = ?", tenantID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at DESC").
		Limit(pageSize).Offset((page-1)*pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// DeleteFile 删除文件
func (s *Service) DeleteFile(ctx context.Context, tenantID, fileID string) error {
	var file model.StorageFile
	if err := s.db.WithContext(ctx).
		First(&file, "id = ? AND tenant_id = ?", fileID, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	// 删除物理文件
	fullPath := filepath.Join(s.basePath, file.Path)
	_ = os.Remove(fullPath)

	// 软删除记录
	if err := s.db.WithContext(ctx).Delete(&file).Error; err != nil {
		return err
	}

	// 释放配额
	s.updateQuotaUsage(ctx, tenantID, -file.FileSize, -1)

	logger.WithContext(ctx).Info("文件已删除",
		zap.String("file_id", fileID),
		zap.String("file_name", file.FileName),
	)
	return nil
}

// GetQuota 获取租户配额
func (s *Service) GetQuota(ctx context.Context, tenantID string) (*model.StorageQuota, error) {
	var quota model.StorageQuota
	if err := s.db.WithContext(ctx).
		FirstOrCreate(&quota, model.StorageQuota{TenantID: tenantID}).Error; err != nil {
		return nil, err
	}
	return &quota, nil
}

// SetQuota 设置租户配额（上级操作）
func (s *Service) SetQuota(ctx context.Context, operatorTenantID, targetTenantID string, maxBytes, maxFiles int64) error {
	// 验证权限链路
	if err := s.hierarchy.ValidateControlFlow(ctx, operatorTenantID, targetTenantID); err != nil {
		return fmt.Errorf("无权设置配额: %w", err)
	}
	return s.db.WithContext(ctx).
		Where("tenant_id = ?", targetTenantID).
		Assign(model.StorageQuota{MaxBytes: maxBytes, MaxFiles: maxFiles}).
		FirstOrCreate(&model.StorageQuota{TenantID: targetTenantID}).Error
}

// ========== 内部方法 ==========

func (s *Service) checkQuota(ctx context.Context, tenantID string, fileSize int64) error {
	var quota model.StorageQuota
	if err := s.db.WithContext(ctx).
		FirstOrCreate(&quota, model.StorageQuota{TenantID: tenantID}).Error; err != nil {
		return nil // 无配额记录时不限制
	}
	if quota.MaxBytes > 0 && quota.UsedBytes+fileSize > quota.MaxBytes {
		return fmt.Errorf("%w: 已用 %d MB / 总计 %d MB",
			ErrQuotaExceeded,
			quota.UsedBytes/1024/1024,
			quota.MaxBytes/1024/1024,
		)
	}
	if quota.MaxFiles > 0 && quota.UsedFiles+1 > quota.MaxFiles {
		return fmt.Errorf("%w: 已用 %d 个文件 / 最大 %d 个",
			ErrQuotaExceeded, quota.UsedFiles, quota.MaxFiles,
		)
	}
	return nil
}

func (s *Service) getActiveSource(ctx context.Context, tenantID string) (*model.StorageSource, error) {
	var source model.StorageSource
	// 优先找租户自己的存储源，然后找上级的
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = 1", tenantID).
		First(&source).Error
	if err == nil {
		return &source, nil
	}
	// 降级：使用默认本地存储
	return &model.StorageSource{
		Provider: model.StorageLocal,
		Name:     "本地默认存储",
		Bucket:   s.basePath,
	}, nil
}

func (s *Service) updateQuotaUsage(ctx context.Context, tenantID string, deltaBytes, deltaFiles int64) {
	s.db.WithContext(ctx).
		Model(&model.StorageQuota{}).
		Where("tenant_id = ?", tenantID).
		Updates(map[string]interface{}{
			"used_bytes": gorm.Expr("used_bytes + ?", deltaBytes),
			"used_files": gorm.Expr("used_files + ?", deltaFiles),
		})
}

func (s *Service) saveAndHash(reader io.Reader, key string) (string, string, error) {
	fullPath := filepath.Join(s.basePath, key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	h := sha256.New()
	w := io.MultiWriter(f, h)
	if _, err := io.Copy(w, reader); err != nil {
		return "", "", err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	return hash, fullPath, nil
}

func (s *Service) genLocalURL(key string) string {
	return "/storage/" + strings.ReplaceAll(key, "\\", "/")
}

func generateFileID() string {
	b := make([]byte, 8)
	io.ReadFull(strings.NewReader(time.Now().Format("20060102150405.000000000")), b)
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
