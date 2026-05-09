// Package hierarchy 基于 PostgreSQL 递归 CTE 的族谱/层级关系查询工具
// 适用场景：世系树、亲属关系溯源、分支遍历、三级SaaS层级链
// 设计目标：用 PG 18.3 原生递归 CTE 替代重型图数据库（Neo4j等），零额外依赖
package hierarchy

import (
	"context"

	"gorm.io/gorm"
)

// Node 通用层级节点
type Node struct {
	ID       string `json:"id" gorm:"column:id"`
	ParentID string `json:"parent_id" gorm:"column:parent_id"`
	Depth    int    `json:"depth" gorm:"column:depth"`        // 相对根的深度
	Path     string `json:"path" gorm:"column:path"`          // 根→当前的完整路径（id列表，逗号分隔）
	Code     string `json:"code,omitempty" gorm:"column:code"`
	Name     string `json:"name,omitempty" gorm:"column:name"`
}

// Descendants 查询指定节点的所有后代（含自身可选）
// table：表名，需包含 id / parent_id 字段
// rootID：起始节点ID；includeSelf：是否包含 rootID 本身
func Descendants(ctx context.Context, db *gorm.DB, table, rootID string, includeSelf bool) ([]Node, error) {
	baseCond := "parent_id = ?"
	baseArgs := []any{rootID}
	if includeSelf {
		baseCond = "id = ?"
	}

	sql := `
WITH RECURSIVE tree AS (
    SELECT id::text AS id, COALESCE(parent_id::text, '') AS parent_id,
           COALESCE(code, '') AS code, COALESCE(name, '') AS name,
           0 AS depth,
           id::text AS path
    FROM ` + table + `
    WHERE ` + baseCond + ` AND deleted_at IS NULL
  UNION ALL
    SELECT t.id::text, COALESCE(t.parent_id::text, ''),
           COALESCE(t.code, ''), COALESCE(t.name, ''),
           tr.depth + 1,
           tr.path || ',' || t.id::text
    FROM ` + table + ` t
    INNER JOIN tree tr ON t.parent_id = tr.id::uuid
    WHERE t.deleted_at IS NULL
)
SELECT id, parent_id, code, name, depth, path FROM tree ORDER BY depth, id`

	var out []Node
	if err := db.WithContext(ctx).Raw(sql, baseArgs...).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Ancestors 查询指定节点的所有祖先（按深度从近到远）
// 典型用例：族谱溯源、三级租户链路校验
func Ancestors(ctx context.Context, db *gorm.DB, table, nodeID string, includeSelf bool) ([]Node, error) {
	sql := `
WITH RECURSIVE tree AS (
    SELECT id::text AS id, COALESCE(parent_id::text, '') AS parent_id,
           COALESCE(code, '') AS code, COALESCE(name, '') AS name,
           0 AS depth,
           id::text AS path
    FROM ` + table + `
    WHERE id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT t.id::text, COALESCE(t.parent_id::text, ''),
           COALESCE(t.code, ''), COALESCE(t.name, ''),
           tr.depth + 1,
           t.id::text || ',' || tr.path
    FROM ` + table + ` t
    INNER JOIN tree tr ON tr.parent_id = t.id::text
    WHERE t.deleted_at IS NULL
)
SELECT id, parent_id, code, name, depth, path FROM tree
` + includeSelfFilter(includeSelf) + `
ORDER BY depth ASC`

	var out []Node
	if err := db.WithContext(ctx).Raw(sql, nodeID).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// IsAncestor 判断 ancestorID 是否为 descendantID 的祖先（严格：不包含自身）
func IsAncestor(ctx context.Context, db *gorm.DB, table, ancestorID, descendantID string) (bool, error) {
	sql := `
WITH RECURSIVE up AS (
    SELECT id, parent_id FROM ` + table + `
    WHERE id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT t.id, t.parent_id FROM ` + table + ` t
    INNER JOIN up u ON t.id = u.parent_id
    WHERE t.deleted_at IS NULL
)
SELECT EXISTS(SELECT 1 FROM up WHERE id = ? AND id <> ?) AS matched`

	var matched bool
	if err := db.WithContext(ctx).Raw(sql, descendantID, ancestorID, descendantID).Scan(&matched).Error; err != nil {
		return false, err
	}
	return matched, nil
}

// LowestCommonAncestor 查询两个节点的最近公共祖先（族谱亲属关系核心）
// 返回 LCA 节点ID；若无公共祖先返回空字符串
func LowestCommonAncestor(ctx context.Context, db *gorm.DB, table, leftID, rightID string) (string, error) {
	sql := `
WITH RECURSIVE
 l AS (
    SELECT id, parent_id, 0 AS depth FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT t.id, t.parent_id, l.depth + 1 FROM ` + table + ` t
    INNER JOIN l ON t.id = l.parent_id WHERE t.deleted_at IS NULL
 ),
 r AS (
    SELECT id, parent_id, 0 AS depth FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT t.id, t.parent_id, r.depth + 1 FROM ` + table + ` t
    INNER JOIN r ON t.id = r.parent_id WHERE t.deleted_at IS NULL
 )
SELECT l.id::text FROM l
INNER JOIN r ON l.id = r.id
ORDER BY l.depth ASC
LIMIT 1`

	var lca string
	if err := db.WithContext(ctx).Raw(sql, leftID, rightID).Scan(&lca).Error; err != nil {
		return "", err
	}
	return lca, nil
}

// BranchCount 统计某节点的分支数量（直接子节点数）
func BranchCount(ctx context.Context, db *gorm.DB, table, nodeID string) (int64, error) {
	var n int64
	err := db.WithContext(ctx).
		Table(table).
		Where("parent_id = ? AND deleted_at IS NULL", nodeID).
		Count(&n).Error
	return n, err
}

func includeSelfFilter(includeSelf bool) string {
	if includeSelf {
		return ""
	}
	return "WHERE depth > 0"
}
