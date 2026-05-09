// Package genealogy 族谱业务域 —— 核心业务（终端客户主阵地）
package genealogy

import (
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// Gender 性别
type Gender string

const (
	GenderMale    Gender = "male"
	GenderFemale  Gender = "female"
	GenderUnknown Gender = "unknown"
)

// Branch 分支（如"长房"、"二房"）
type Branch struct {
	model.BaseModel
	TenantID string  `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Name     string  `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Code     string  `gorm:"column:code;type:varchar(50)" json:"code"`
	ParentID *string `gorm:"column:parent_id;type:uuid;index" json:"parent_id,omitempty"`
	Depth    int     `gorm:"column:depth;type:int;not null;default:0" json:"depth"`
	Remark   string  `gorm:"column:remark;type:text" json:"remark"`
}

func (Branch) TableName() string { return "genealogy_branches" }

// Member 族谱成员
type Member struct {
	model.BaseModel
	TenantID   string     `gorm:"column:tenant_id;type:uuid;not null;index:idx_member_tenant" json:"tenant_id"`
	BranchID   *string    `gorm:"column:branch_id;type:uuid;index" json:"branch_id,omitempty"`
	FatherID   *string    `gorm:"column:father_id;type:uuid;index" json:"father_id,omitempty"`
	MotherID   *string    `gorm:"column:mother_id;type:uuid;index" json:"mother_id,omitempty"`
	Generation int        `gorm:"column:generation;type:int;not null;default:0;index" json:"generation"`
	Name       string     `gorm:"column:name;type:varchar(100);not null" json:"name"`
	AliasName  string     `gorm:"column:alias_name;type:varchar(200)" json:"alias_name"`
	Gender     Gender     `gorm:"column:gender;type:varchar(10);not null;default:'unknown'" json:"gender"`
	BirthDate  *time.Time `gorm:"column:birth_date" json:"birth_date,omitempty"`
	DeathDate  *time.Time `gorm:"column:death_date" json:"death_date,omitempty"`
	Birthplace string     `gorm:"column:birthplace;type:varchar(200)" json:"birthplace"`
	Biography  string     `gorm:"column:biography;type:text" json:"biography"`
	Avatar     string     `gorm:"column:avatar;type:varchar(500)" json:"avatar"`
	Status     int        `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (Member) TableName() string { return "genealogy_members" }

// RelationType 关系类型
type RelationType string

const (
	RelSpouse  RelationType = "spouse"  // 配偶
	RelSibling RelationType = "sibling" // 兄弟姐妹
	RelAdopted RelationType = "adopted" // 收养
	RelInLaw   RelationType = "in_law"  // 姻亲
)

// Relation 额外关系（父母已固化在 Member）
type Relation struct {
	model.BaseModel
	TenantID string       `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	FromID   string       `gorm:"column:from_id;type:uuid;not null" json:"from_id"`
	ToID     string       `gorm:"column:to_id;type:uuid;not null" json:"to_id"`
	Type     RelationType `gorm:"column:type;type:varchar(20);not null" json:"type"`
	Remark   string       `gorm:"column:remark;type:varchar(500)" json:"remark"`
}

func (Relation) TableName() string { return "genealogy_relations" }

// Announce 家族公告
type Announce struct {
	model.BaseModel
	TenantID  string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Title     string    `gorm:"column:title;type:varchar(200);not null" json:"title"`
	Content   string    `gorm:"column:content;type:text;not null" json:"content"`
	Author    string    `gorm:"column:author;type:varchar(50)" json:"author"`
	Pinned    bool      `gorm:"column:pinned;type:boolean;not null;default:false" json:"pinned"`
	PublishAt time.Time `gorm:"column:publish_at;not null;default:NOW()" json:"publish_at"`
}

func (Announce) TableName() string { return "genealogy_announces" }

// AllModels 族谱域所有模型
func AllModels() []any {
	return []any{&Branch{}, &Member{}, &Relation{}, &Announce{}}
}
