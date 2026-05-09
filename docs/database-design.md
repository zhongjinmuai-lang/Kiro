# MU 框架 · 数据库设计文档

> PostgreSQL 18.3（企业级）· 启用 RLS 行级安全实现金融级多租户隔离

## 一、设计原则

1. **UUID 主键**：全局唯一，避免自增ID在分布式下的冲突
2. **软删除**：统一 `deleted_at` 字段，soft_delete 插件兼容 PG 索引
3. **自动时间戳**：`created_at` / `updated_at` 由 GORM Hook 自动维护
4. **行级安全**：通过 `app.current_tenant_id` / `app.current_tenant_level` 会话变量激活 RLS，业务代码零侵入
5. **递归 CTE**：族谱/层级关系使用 PG 原生递归，替代重型图数据库

## 二、表清单

### 2.1 三级SaaS核心（`001_init_schema.sql`）

| 表 | 说明 | 索引 |
|----|------|------|
| `tenants` | 租户（三级层级） | `(parent_id)` `(level)` `(code UNIQUE)` |
| `users` | 用户 | `(tenant_id, username) UNIQUE` |
| `roles` | 角色 | `(tenant_id, code) UNIQUE` |
| `permissions` | 权限定义 | `(module)` `(code UNIQUE)` |
| `tenant_permissions` | 租户权限授予 | `(tenant_id, permission_code) UNIQUE` |

### 2.2 三大中台（`002_platform_tables.sql`）

| 表 | 说明 |
|----|------|
| `payment_channels` | 支付渠道（开发商配置） |
| `tenant_payment_auth` | 支付渠道授权（三级管控） |
| `payment_orders` | 订单（订单号唯一，支持退款） |
| `storage_sources` | 存储源（7种供应商） |
| `storage_files` | 文件记录 |
| `storage_quotas` | 配额（字节数+文件数双限） |
| `notify_channels` | 通知通道（5种） |
| `notify_templates` | 模板（支持 `{{变量}}` 占位） |
| `notify_messages` | 发送记录（重试计数） |

### 2.3 初始数据（`003_seed_data.sql`）

- 默认开发商租户 `mu-platform`
- 超级管理员：`admin` / `mu_admin_2026`（bcrypt 加密）
- 预置 20+ 权限定义（支付/存储/通知/智能体/租户）

### 2.4 行级安全（`004_rls_policies.sql`）

```sql
-- 会话变量注入
SELECT set_config('app.current_tenant_id', '<uuid>', true);
SELECT set_config('app.current_tenant_level', 'provider', true);

-- RLS 策略示例
CREATE POLICY xxx_tenant_isolation ON xxx
  USING (current_tenant_level() = 'developer' OR tenant_id = current_tenant_id());
```

策略规则：
- `developer` 可看全部
- `provider` / `customer` 只能看自己 `tenant_id = current_tenant_id()` 的数据

## 三、族谱查询（递归 CTE）

### 3.1 后代遍历

```sql
WITH RECURSIVE tree AS (
    SELECT id, parent_id, 0 AS depth FROM tenants WHERE parent_id = ?
  UNION ALL
    SELECT t.id, t.parent_id, tr.depth + 1 FROM tenants t
    INNER JOIN tree tr ON t.parent_id = tr.id
)
SELECT * FROM tree ORDER BY depth;
```

### 3.2 祖先溯源

见 `pkg/hierarchy/cte.go` `Ancestors()`

### 3.3 最近公共祖先（LCA）

见 `pkg/hierarchy/cte.go` `LowestCommonAncestor()`

## 四、性能优化策略

| 策略 | 说明 |
|------|------|
| 预编译缓存 | GORM `PrepareStmt: true` |
| 连接池 | 生产 `max_open_conns: 100`，`max_idle_conns: 20` |
| 慢SQL告警 | `slow_threshold: 500ms`，超过记录 WARN |
| 覆盖索引 | 常用 WHERE 字段建立复合索引 |
| Redis缓存 | 权限检查 5 分钟缓存；热点租户层级链路 |

## 五、备份与恢复

```bash
# 备份
pg_dump -h $MU_DATABASE_HOST -U mu_admin -d mu_framework -F c -f backup_$(date +%Y%m%d).dump

# 恢复
pg_restore -h $MU_DATABASE_HOST -U mu_admin -d mu_framework backup_20260509.dump
```

生产建议：
- 每日全量备份 + 30天保留
- 连续归档 WAL（PITR）
- 异地灾备（S3/OSS 双地域）
