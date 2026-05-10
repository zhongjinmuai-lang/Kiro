// Package hierarchy 基于 PostgreSQL 递归 CTE 的族谱/层级关系查询工具
//
// 【v1.5 图数据升级】新增：
//   - GenerationDiff：世代差距计算
//   - ClassifyLineage：直系/旁系判定
//   - Kinship：亲属称谓计算（中国汉族通用）
//   - CommonAncestors：所有公共祖先
//   - SiblingOf：同胞查询
//   - ComputeTreeStats：树统计
//
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
	Depth    int    `json:"depth" gorm:"column:depth"` // 相对根的深度
	Path     string `json:"path" gorm:"column:path"`   // 根→当前的完整路径（id列表，逗号分隔）
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


// ========== v1.5 图数据增强 ==========

import "fmt"

// GenerationDiff 两节点世代差距
// 返回 diff（A 比 B 长辈则 > 0）和 LCA 节点 ID
func GenerationDiff(ctx context.Context, db *gorm.DB, table, aID, bID string) (diff int, lcaID string, err error) {
	sql := `
WITH RECURSIVE
 a AS (
    SELECT id, parent_id, 0 AS d FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT t.id, t.parent_id, a.d+1 FROM ` + table + ` t INNER JOIN a ON t.id = a.parent_id WHERE t.deleted_at IS NULL
 ),
 b AS (
    SELECT id, parent_id, 0 AS d FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT t.id, t.parent_id, b.d+1 FROM ` + table + ` t INNER JOIN b ON t.id = b.parent_id WHERE t.deleted_at IS NULL
 ),
 meet AS (
    SELECT a.id::text AS lca, (b.d - a.d) AS diff FROM a INNER JOIN b ON a.id = b.id ORDER BY a.d + b.d ASC LIMIT 1
 )
SELECT COALESCE(lca,''), COALESCE(diff, 0) FROM meet`
	row := db.WithContext(ctx).Raw(sql, aID, bID).Row()
	if err = row.Scan(&lcaID, &diff); err != nil {
		return 0, "", fmt.Errorf("计算世代差距失败: %w", err)
	}
	if lcaID == "" {
		return 0, "", fmt.Errorf("两节点无公共祖先")
	}
	return diff, lcaID, nil
}

// LineageRelation 亲属关系类型
type LineageRelation string

const (
	RelSelf       LineageRelation = "self"       // 本人
	RelLineal     LineageRelation = "lineal"     // 直系
	RelCollateral LineageRelation = "collateral" // 旁系
	RelUnknown    LineageRelation = "unknown"    // 无关系
)

// ClassifyLineage 判定直系/旁系
func ClassifyLineage(ctx context.Context, db *gorm.DB, table, aID, bID string) (LineageRelation, error) {
	if aID == bID {
		return RelSelf, nil
	}
	isA2B, err := IsAncestor(ctx, db, table, aID, bID)
	if err != nil {
		return RelUnknown, err
	}
	if isA2B {
		return RelLineal, nil
	}
	isB2A, err := IsAncestor(ctx, db, table, bID, aID)
	if err != nil {
		return RelUnknown, err
	}
	if isB2A {
		return RelLineal, nil
	}
	lca, err := LowestCommonAncestor(ctx, db, table, aID, bID)
	if err != nil {
		return RelUnknown, err
	}
	if lca != "" {
		return RelCollateral, nil
	}
	return RelUnknown, nil
}

// Kinship 亲属称谓（基于世代差距 + 性别）
// genderB: 被称呼人性别 (male/female/unknown)
func Kinship(ctx context.Context, db *gorm.DB, table, aID, bID, genderB string) (string, error) {
	if aID == bID {
		return "本人", nil
	}
	diff, _, err := GenerationDiff(ctx, db, table, aID, bID)
	if err != nil {
		return "", err
	}
	lineage, err := ClassifyLineage(ctx, db, table, aID, bID)
	if err != nil {
		return "", err
	}
	male := genderB == "male"
	if lineage == RelLineal {
		switch diff {
		case 1:
			if male { return "父亲", nil }
			return "母亲", nil
		case -1:
			if male { return "儿子", nil }
			return "女儿", nil
		case 2:
			if male { return "祖父", nil }
			return "祖母", nil
		case -2:
			if male { return "孙子", nil }
			return "孙女", nil
		case 3:
			if male { return "曾祖父", nil }
			return "曾祖母", nil
		case -3:
			if male { return "曾孙", nil }
			return "曾孙女", nil
		default:
			if diff > 0 { return fmt.Sprintf("%d世祖", diff), nil }
			return fmt.Sprintf("%d世孙", -diff), nil
		}
	}
	if lineage == RelCollateral {
		switch diff {
		case 0:
			if male { return "兄弟（同宗）", nil }
			return "姐妹（同宗）", nil
		case 1:
			if male { return "伯叔辈", nil }
			return "姑母辈", nil
		case -1:
			if male { return "侄子", nil }
			return "侄女", nil
		default:
			if diff > 0 { return fmt.Sprintf("%d辈旁系长辈", diff), nil }
			return fmt.Sprintf("%d辈旁系晚辈", -diff), nil
		}
	}
	return "亲属关系未知", nil
}

// CommonAncestors 所有公共祖先（按深度从近到远）
func CommonAncestors(ctx context.Context, db *gorm.DB, table, aID, bID string) ([]string, error) {
	sql := `
WITH RECURSIVE
 a AS (SELECT id, parent_id, 0 AS d FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
       UNION ALL SELECT t.id, t.parent_id, a.d+1 FROM ` + table + ` t INNER JOIN a ON t.id = a.parent_id WHERE t.deleted_at IS NULL),
 b AS (SELECT id, parent_id, 0 AS d FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
       UNION ALL SELECT t.id, t.parent_id, b.d+1 FROM ` + table + ` t INNER JOIN b ON t.id = b.parent_id WHERE t.deleted_at IS NULL)
SELECT a.id::text FROM a INNER JOIN b ON a.id = b.id ORDER BY (a.d + b.d) ASC`
	var ids []string
	err := db.WithContext(ctx).Raw(sql, aID, bID).Scan(&ids).Error
	return ids, err
}

// SiblingOf 查询同胞（共父）
func SiblingOf(ctx context.Context, db *gorm.DB, table, nodeID string) ([]Node, error) {
	sql := `SELECT s.id::text AS id, COALESCE(s.parent_id::text,'') AS parent_id,
       COALESCE(s.code,'') AS code, COALESCE(s.name,'') AS name, 0 AS depth, s.id::text AS path
FROM ` + table + ` s
WHERE s.parent_id = (SELECT parent_id FROM ` + table + ` WHERE id = ?)
  AND s.id <> ? AND s.deleted_at IS NULL ORDER BY s.created_at`
	var nodes []Node
	err := db.WithContext(ctx).Raw(sql, nodeID, nodeID).Scan(&nodes).Error
	return nodes, err
}

// TreeStats 树统计
type TreeStats struct {
	TotalCount   int64 `json:"total_count"`
	MaxDepth     int   `json:"max_depth"`
	LeafCount    int64 `json:"leaf_count"`
	DirectBranch int64 `json:"direct_branch"`
}

// ComputeTreeStats 计算树统计
func ComputeTreeStats(ctx context.Context, db *gorm.DB, table, rootID string) (*TreeStats, error) {
	sql := `
WITH RECURSIVE tree AS (
    SELECT id, parent_id, 0 AS depth FROM ` + table + ` WHERE id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT t.id, t.parent_id, tr.depth+1 FROM ` + table + ` t INNER JOIN tree tr ON t.parent_id = tr.id WHERE t.deleted_at IS NULL
)
SELECT COUNT(*), COALESCE(MAX(depth),0),
  (SELECT COUNT(*) FROM tree t2 WHERE NOT EXISTS(SELECT 1 FROM tree c WHERE c.parent_id = t2.id)),
  (SELECT COUNT(*) FROM ` + table + ` WHERE parent_id = ? AND deleted_at IS NULL)
FROM tree`
	s := &TreeStats{}
	row := db.WithContext(ctx).Raw(sql, rootID, rootID).Row()
	if err := row.Scan(&s.TotalCount, &s.MaxDepth, &s.LeafCount, &s.DirectBranch); err != nil {
		return nil, fmt.Errorf("树统计失败: %w", err)
	}
	return s, nil
}
