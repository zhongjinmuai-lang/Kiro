// Package storage 存储中台 - 多云适配器（v2.7）
//
// 统一接口对接：
//   - 腾讯云 COS（cos-go-sdk-v5）
//   - 阿里云 OSS（aliyun-oss-go-sdk）
//   - MinIO（兼容 S3 协议）
//   - 本地文件系统（开发/测试）
//
// 设计原则：
//   - Provider 接口统一 Upload/Delete/GenURL
//   - 按 StorageSource 配置动态选择适配器
//   - 支持预签名 URL（客户端直传）
//   - CDN 域名自动拼接
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// ========== 腾讯云 COS 适配器 ==========

// COSProvider 腾讯云 COS 存储适配器
type COSProvider struct {
	source     *model.StorageSource
	httpClient *http.Client
}

// NewCOSProvider 创建 COS 适配器
func NewCOSProvider(source *model.StorageSource) *COSProvider {
	return &COSProvider{
		source:     source,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Upload 上传文件到 COS
// COS REST API: PUT Object
// https://cloud.tencent.com/document/product/436/7749
func (p *COSProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	endpoint := p.source.Endpoint
	if endpoint == "" {
		// 默认格式: https://{bucket}.cos.{region}.myqcloud.com
		endpoint = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", p.source.Bucket, p.source.Region)
	}
	objectURL := fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, reader)
	if err != nil {
		return fmt.Errorf("COS 构建请求失败: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")

	// COS 签名（简化版，生产应使用 cos-go-sdk-v5）
	sign := p.cosSign(http.MethodPut, "/"+key, time.Now())
	req.Header.Set("Authorization", sign)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("COS 上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("COS 上传返回错误 HTTP %d: %s", resp.StatusCode, string(body))
	}

	logger.L().Info("COS 文件上传成功", zap.String("key", key), zap.Int64("size", size))
	return nil
}

// Delete 删除 COS 对象
func (p *COSProvider) Delete(ctx context.Context, key string) error {
	endpoint := p.source.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", p.source.Bucket, p.source.Region)
	}
	objectURL := fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), key)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, objectURL, nil)
	if err != nil {
		return err
	}
	sign := p.cosSign(http.MethodDelete, "/"+key, time.Now())
	req.Header.Set("Authorization", sign)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("COS 删除失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		return fmt.Errorf("COS 删除返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

// GenURL 生成访问 URL
func (p *COSProvider) GenURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	// 优先使用 CDN 域名
	if p.source.CDNDomain != "" {
		return fmt.Sprintf("https://%s/%s", p.source.CDNDomain, key), nil
	}
	// 生成预签名 URL
	endpoint := p.source.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", p.source.Bucket, p.source.Region)
	}
	// 简化预签名（生产应使用 SDK）
	expireTime := time.Now().Add(expires).Unix()
	signKey := p.cosHMACSHA1(p.source.SecretKey, fmt.Sprintf("%d;%d", time.Now().Unix(), expireTime))
	return fmt.Sprintf("%s/%s?sign=%s&expire=%d", endpoint, key, url.QueryEscape(signKey), expireTime), nil
}

// cosSign COS 签名（简化版本，生产环境建议使用官方 SDK）
func (p *COSProvider) cosSign(method, path string, t time.Time) string {
	startTime := t.Unix()
	endTime := t.Add(time.Hour).Unix()
	keyTime := fmt.Sprintf("%d;%d", startTime, endTime)

	signKey := p.cosHMACSHA1(p.source.SecretKey, keyTime)
	httpString := fmt.Sprintf("%s\n%s\n\n\n", strings.ToLower(method), path)
	sha1Hash := fmt.Sprintf("%x", sha1Sum([]byte(httpString)))
	stringToSign := fmt.Sprintf("sha1\n%s\n%s\n", keyTime, sha1Hash)
	signature := p.cosHMACSHA1(signKey, stringToSign)

	return fmt.Sprintf("q-sign-algorithm=sha1&q-ak=%s&q-sign-time=%s&q-key-time=%s&q-header-list=&q-url-param-list=&q-signature=%s",
		p.source.AccessKey, keyTime, keyTime, signature)
}

func (p *COSProvider) cosHMACSHA1(key, data string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func sha1Sum(data []byte) []byte {
	h := sha1.New()
	h.Write(data)
	return h.Sum(nil)
}

// ========== 阿里云 OSS 适配器 ==========

// OSSProvider 阿里云 OSS 存储适配器
type OSSProvider struct {
	source     *model.StorageSource
	httpClient *http.Client
}

// NewOSSProvider 创建 OSS 适配器
func NewOSSProvider(source *model.StorageSource) *OSSProvider {
	return &OSSProvider{
		source:     source,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Upload 上传文件到 OSS
// OSS REST API: PUT Object
// https://help.aliyun.com/document_detail/31978.html
func (p *OSSProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	endpoint := p.source.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.oss-%s.aliyuncs.com", p.source.Bucket, p.source.Region)
	}
	objectURL := fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, reader)
	if err != nil {
		return fmt.Errorf("OSS 构建请求失败: %w", err)
	}
	req.ContentLength = size
	contentType := "application/octet-stream"
	req.Header.Set("Content-Type", contentType)

	// OSS V1 签名
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	sign := p.ossSign(http.MethodPut, "/"+p.source.Bucket+"/"+key, contentType, date)
	req.Header.Set("Authorization", sign)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("OSS 上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OSS 上传返回错误 HTTP %d: %s", resp.StatusCode, string(body))
	}

	logger.L().Info("OSS 文件上传成功", zap.String("key", key), zap.Int64("size", size))
	return nil
}

// Delete 删除 OSS 对象
func (p *OSSProvider) Delete(ctx context.Context, key string) error {
	endpoint := p.source.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.oss-%s.aliyuncs.com", p.source.Bucket, p.source.Region)
	}
	objectURL := fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), key)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, objectURL, nil)
	if err != nil {
		return err
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	sign := p.ossSign(http.MethodDelete, "/"+p.source.Bucket+"/"+key, "", date)
	req.Header.Set("Authorization", sign)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("OSS 删除失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		return fmt.Errorf("OSS 删除返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

// GenURL 生成访问 URL（预签名）
func (p *OSSProvider) GenURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	// 优先使用 CDN 域名
	if p.source.CDNDomain != "" {
		return fmt.Sprintf("https://%s/%s", p.source.CDNDomain, key), nil
	}

	// OSS 预签名 URL
	endpoint := p.source.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.oss-%s.aliyuncs.com", p.source.Bucket, p.source.Region)
	}

	expireUnix := time.Now().Add(expires).Unix()
	resource := "/" + p.source.Bucket + "/" + key
	stringToSign := fmt.Sprintf("GET\n\n\n%d\n%s", expireUnix, resource)

	mac := hmac.New(sha1.New, []byte(p.source.SecretKey))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s/%s?OSSAccessKeyId=%s&Expires=%d&Signature=%s",
		endpoint, key,
		url.QueryEscape(p.source.AccessKey),
		expireUnix,
		url.QueryEscape(signature),
	), nil
}

// ossSign OSS V1 签名
func (p *OSSProvider) ossSign(method, canonicalResource, contentType, date string) string {
	stringToSign := fmt.Sprintf("%s\n\n%s\n%s\n%s", method, contentType, date, canonicalResource)
	mac := hmac.New(sha1.New, []byte(p.source.SecretKey))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("OSS %s:%s", p.source.AccessKey, signature)
}

// ========== MinIO 适配器（兼容 S3 协议） ==========

// MinIOProvider MinIO/S3 兼容存储适配器
type MinIOProvider struct {
	source     *model.StorageSource
	httpClient *http.Client
}

// NewMinIOProvider 创建 MinIO 适配器
func NewMinIOProvider(source *model.StorageSource) *MinIOProvider {
	return &MinIOProvider{
		source:     source,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Upload MinIO 上传（S3 兼容 PUT）
func (p *MinIOProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(p.source.Endpoint, "/"), p.source.Bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, reader)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	// MinIO 基本认证（简化，生产用 AWS SigV4）
	req.SetBasicAuth(p.source.AccessKey, p.source.SecretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("MinIO 上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("MinIO 上传 HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Delete MinIO 删除
func (p *MinIOProvider) Delete(ctx context.Context, key string) error {
	objectURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(p.source.Endpoint, "/"), p.source.Bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, objectURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.source.AccessKey, p.source.SecretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GenURL MinIO 访问 URL
func (p *MinIOProvider) GenURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if p.source.CDNDomain != "" {
		return fmt.Sprintf("https://%s/%s/%s", p.source.CDNDomain, p.source.Bucket, key), nil
	}
	return fmt.Sprintf("%s/%s/%s", p.source.Endpoint, p.source.Bucket, key), nil
}

// ========== 适配器工厂 ==========

// NewProvider 根据存储源配置创建对应的适配器
func NewProvider(source *model.StorageSource) Provider {
	switch source.Provider {
	case model.StorageTencentCOS:
		return NewCOSProvider(source)
	case model.StorageAliyunOSS:
		return NewOSSProvider(source)
	case model.StorageMinIO, model.StorageS3:
		return NewMinIOProvider(source)
	default:
		// 本地存储返回 nil，由 Service 层处理
		return nil
	}
}
