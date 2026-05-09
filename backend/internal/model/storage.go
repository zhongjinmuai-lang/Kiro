package model

// StorageProvider 存储供应商
type StorageProvider string

const (
	StorageLocal      StorageProvider = "local"       // 本地
	StorageMinIO      StorageProvider = "minio"       // MinIO
	StorageAliyunOSS  StorageProvider = "aliyun_oss"  // 阿里云 OSS
	StorageTencentCOS StorageProvider = "tencent_cos" // 腾讯云 COS
	StorageQiniu      StorageProvider = "qiniu"       // 七牛云
	StorageHuaweiOBS  StorageProvider = "huawei_obs"  // 华为云 OBS
	StorageS3         StorageProvider = "s3"          // AWS S3
)

// StorageSource 存储源配置（开发商准入，服务商绑定）
type StorageSource struct {
	BaseModel
	TenantID  string          `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Level     TenantLevel     `gorm:"column:level;type:varchar(20);not null" json:"level"`
	Provider  StorageProvider `gorm:"column:provider;type:varchar(20);not null" json:"provider"`
	Name      string          `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Bucket    string          `gorm:"column:bucket;type:varchar(100)" json:"bucket"`
	Region    string          `gorm:"column:region;type:varchar(50)" json:"region"`
	Endpoint  string          `gorm:"column:endpoint;type:varchar(500)" json:"endpoint"`
	AccessKey string          `gorm:"column:access_key;type:varchar(200)" json:"-"`
	SecretKey string          `gorm:"column:secret_key;type:text" json:"-"`
	CDNDomain string          `gorm:"column:cdn_domain;type:varchar(200)" json:"cdn_domain"`
	MaxSize   int64           `gorm:"column:max_size;type:bigint;default:0" json:"max_size"`
	Status    int             `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (StorageSource) TableName() string { return "storage_sources" }

// StorageFile 文件上传记录
type StorageFile struct {
	BaseModel
	TenantID string `gorm:"column:tenant_id;type:uuid;not null;index:idx_files_tenant,priority:1" json:"tenant_id"`
	SourceID string `gorm:"column:source_id;type:uuid;not null" json:"source_id"`
	FileName string `gorm:"column:file_name;type:varchar(255);not null" json:"file_name"`
	FileSize int64  `gorm:"column:file_size;type:bigint;not null;default:0" json:"file_size"`
	MimeType string `gorm:"column:mime_type;type:varchar(100)" json:"mime_type"`
	Path     string `gorm:"column:path;type:varchar(500);not null" json:"path"`
	URL      string `gorm:"column:url;type:varchar(1000)" json:"url"`
	Hash     string `gorm:"column:hash;type:varchar(64);index" json:"hash"`
}

func (StorageFile) TableName() string { return "storage_files" }

// StorageQuota 租户存储配额
type StorageQuota struct {
	TenantID  string `gorm:"column:tenant_id;type:uuid;primaryKey" json:"tenant_id"`
	MaxBytes  int64  `gorm:"column:max_bytes;type:bigint;not null;default:0" json:"max_bytes"`
	UsedBytes int64  `gorm:"column:used_bytes;type:bigint;not null;default:0" json:"used_bytes"`
	MaxFiles  int64  `gorm:"column:max_files;type:bigint;not null;default:0" json:"max_files"`
	UsedFiles int64  `gorm:"column:used_files;type:bigint;not null;default:0" json:"used_files"`
}

func (StorageQuota) TableName() string { return "storage_quotas" }
