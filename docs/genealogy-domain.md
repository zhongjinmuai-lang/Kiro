# MU 框架 · 族谱业务域

## 一、领域模型

```
┌──────────┐ 1─N ┌────────────────┐
│  Branch  │─────│     Member     │
│ (分支)    │     │   (族谱成员)   │
└──────────┘     └─┬───────┬──────┘
                   │       │
               father_id  mother_id (Member 自关联)

┌──────────────┐  ┌──────────────┐
│  Relation    │  │  Announce    │
│ (额外关系)   │  │ (家族公告)   │
└──────────────┘  └──────────────┘
```

所有表均启用 **PG RLS 行级安全**，按 `tenant_id` 严格隔离。

## 二、核心 API

| 接口 | 说明 |
|------|------|
| `GET    /api/v1/genealogy/tree?root=:id&depth=20` | 世系树（后代展开） |
| `GET    /api/v1/genealogy/members/:id/ancestors`  | 祖先溯源（沿 father_id） |
| `GET    /api/v1/genealogy/members/:id/descendants`| 分支遍历（所有后裔） |
| `GET    /api/v1/genealogy/lca?left=:id&right=:id` | 最近公共祖先（判断亲属） |
| `GET    /api/v1/genealogy/members`                | 成员列表 |
| `POST   /api/v1/genealogy/members`                | 新增成员（自动推导世代） |
| `PUT    /api/v1/genealogy/members/:id`            | 更新 |
| `DELETE /api/v1/genealogy/members/:id`            | 软删除 |
| `GET    /api/v1/genealogy/branches`               | 分支列表 |
| `POST   /api/v1/genealogy/branches`               | 新建分支 |
| `GET    /api/v1/genealogy/announces`              | 公告列表 |
| `POST   /api/v1/genealogy/announces`              | 发布公告 |
| `POST   /api/v1/genealogy/ocr`                    | AI 识别老族谱建档 |
| `GET    /api/v1/genealogy/stats`                  | 统计（成员/分支/世代） |

## 三、PG 递归 CTE 替代图数据库

### 3.1 世系树（下行）
```sql
WITH RECURSIVE tree AS (
  SELECT id, father_id, mother_id, name, generation, 0 AS depth
  FROM genealogy_members WHERE id = ?
UNION ALL
  SELECT m.id, m.father_id, m.mother_id, m.name, m.generation, t.depth + 1
  FROM genealogy_members m
  JOIN tree t ON m.father_id = t.id OR m.mother_id = t.id
  WHERE t.depth < 20
)
SELECT * FROM tree;
```

### 3.2 最近公共祖先（LCA）
两条祖先链路求交集，取最小 depth。

## 四、AI OCR 建档流程

```
端侧拍照 → 上传存储中台 → 返回 URL
                ↓
  POST /ocr { image_url, hint }
                ↓
  AI 网关（多供应商降级）
                ↓
  返回 JSON { raw_text, members[] }
                ↓
  人工校对 → 批量 POST /members 建档
```

## 五、数据安全

- `tenant_id` + RLS 双重保障
- 接口必须通过 `TenantRequired` + `TenantRLS` 中间件
- `father_id / mother_id` 外键级联 `SET NULL`，避免误删祖先丢追溯

## 六、扩展计划

- [ ] SVG 族谱图在线生成 + PDF 打印
- [ ] 中英文双语编辑
- [ ] 两家族谱合并时的相似成员识别
- [ ] 按朝代/年份筛选时间线
