-- ============================================================
-- v1.5 数据库升级：性能索引 + 亲属关系缓存 + 监控审计
-- ============================================================

-- 1. 族谱查询性能优化索引
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_members_father_tenant
ON genealogy_members(father_id, tenant_id) WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_members_mother_tenant
ON genealogy_members(mother_id, tenant_id) WHERE deleted_at IS NULL;

-- 复合索引加速世系树递归 CTE
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_members_tree_lookup
ON genealogy_members(id, father_id, mother_id, tenant_id, generation)
WHERE deleted_at IS NULL;

-- 2. 亲属关系缓存表（预计算常用称谓，避免每次实时递归）
CREATE TABLE IF NOT EXISTS genealogy_kinship_cache (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    from_id     UUID NOT NULL REFERENCES genealogy_members(id) ON DELETE CASCADE,
    to_id       UUID NOT NULL REFERENCES genealogy_members(id) ON DELETE CASCADE,
    kinship     VARCHAR(50) NOT NULL,       -- 称谓（如"祖父"、"侄子"）
    gen_diff    INT NOT NULL DEFAULT 0,     -- 世代差距
    lineage     VARCHAR(20) NOT NULL,       -- lineal / collateral / self
    lca_id      UUID,                       -- 最近公共祖先
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, from_id, to_id)
);

CREATE INDEX IF NOT EXISTS idx_kinship_cache_tenant
ON genealogy_kinship_cache(tenant_id, from_id);

CREATE INDEX IF NOT EXISTS idx_kinship_cache_to
ON genealogy_kinship_cache(tenant_id, to_id);

-- 启用 RLS
ALTER TABLE genealogy_kinship_cache ENABLE ROW LEVEL SECURITY;
ALTER TABLE genealogy_kinship_cache FORCE ROW LEVEL SECURITY;
CREATE POLICY kinship_cache_isolation ON genealogy_kinship_cache
  USING (current_tenant_level() = 'developer' OR tenant_id = current_tenant_id())
  WITH CHECK (current_tenant_level() = 'developer' OR tenant_id = current_tenant_id());

COMMENT ON TABLE genealogy_kinship_cache IS '亲属称谓缓存（v1.5 图数据升级）';

-- 3. 进化事件审计表（持久化 MU 自进化历史）
CREATE TABLE IF NOT EXISTS mu_evolution_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    strategy    VARCHAR(20) NOT NULL,
    rule_name   VARCHAR(100) NOT NULL,
    trigger_info TEXT,
    result      TEXT,
    success     BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms BIGINT DEFAULT 0,
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evolution_events_time
ON mu_evolution_events(created_at DESC);

COMMENT ON TABLE mu_evolution_events IS 'MU自进化内核事件审计（v1.5）';

-- 4. API 请求审计表（监控埋点持久化）
CREATE TABLE IF NOT EXISTS mu_request_audit (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID,
    user_id     UUID,
    method      VARCHAR(10) NOT NULL,
    path        VARCHAR(500) NOT NULL,
    status      INT NOT NULL,
    duration_ms INT NOT NULL DEFAULT 0,
    ip          VARCHAR(50),
    user_agent  VARCHAR(500),
    trace_id    VARCHAR(64),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 按时间分区（每月），便于自动归档清理
CREATE INDEX IF NOT EXISTS idx_audit_time ON mu_request_audit(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_tenant ON mu_request_audit(tenant_id, created_at DESC);

COMMENT ON TABLE mu_request_audit IS 'API 请求审计日志（v1.5 监控埋点）';

-- 5. 补充支付订单索引（对账性能）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_orders_channel_status
ON payment_orders(channel_id, status) WHERE status IN (0, 1);

-- 6. 补充存储文件索引（配额统计加速）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_files_tenant_size
ON storage_files(tenant_id, file_size);
