-- ============================================================
-- 族谱业务域建表脚本
-- 启用 RLS 行级安全（依赖 004 中定义的 current_tenant_id/level）
-- ============================================================

-- 分支
CREATE TABLE IF NOT EXISTS genealogy_branches (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(50),
    parent_id   UUID REFERENCES genealogy_branches(id) ON DELETE SET NULL,
    depth       INT NOT NULL DEFAULT 0,
    remark      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_branches_tenant ON genealogy_branches(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_branches_parent ON genealogy_branches(parent_id) WHERE deleted_at IS NULL;
COMMENT ON TABLE genealogy_branches IS '族谱分支（长房、二房等）';

-- 成员
CREATE TABLE IF NOT EXISTS genealogy_members (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    branch_id   UUID REFERENCES genealogy_branches(id) ON DELETE SET NULL,
    father_id   UUID REFERENCES genealogy_members(id) ON DELETE SET NULL,
    mother_id   UUID REFERENCES genealogy_members(id) ON DELETE SET NULL,
    generation  INT NOT NULL DEFAULT 0,
    name        VARCHAR(100) NOT NULL,
    alias_name  VARCHAR(200),
    gender      VARCHAR(10) NOT NULL DEFAULT 'unknown' CHECK (gender IN ('male','female','unknown')),
    birth_date  TIMESTAMPTZ,
    death_date  TIMESTAMPTZ,
    birthplace  VARCHAR(200),
    biography   TEXT,
    avatar      VARCHAR(500),
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_members_tenant     ON genealogy_members(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_members_branch     ON genealogy_members(branch_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_members_father     ON genealogy_members(father_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_members_mother     ON genealogy_members(mother_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_members_generation ON genealogy_members(tenant_id, generation) WHERE deleted_at IS NULL;
COMMENT ON TABLE genealogy_members IS '族谱成员';

-- 关系
CREATE TABLE IF NOT EXISTS genealogy_relations (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    from_id     UUID NOT NULL REFERENCES genealogy_members(id) ON DELETE CASCADE,
    to_id       UUID NOT NULL REFERENCES genealogy_members(id) ON DELETE CASCADE,
    type        VARCHAR(20) NOT NULL,
    remark      VARCHAR(500),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_relations_tenant ON genealogy_relations(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_relations_from   ON genealogy_relations(from_id, type);
CREATE INDEX IF NOT EXISTS idx_relations_to     ON genealogy_relations(to_id,   type);

-- 公告
CREATE TABLE IF NOT EXISTS genealogy_announces (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    title       VARCHAR(200) NOT NULL,
    content     TEXT NOT NULL,
    author      VARCHAR(50),
    pinned      BOOLEAN NOT NULL DEFAULT FALSE,
    publish_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_announces_tenant ON genealogy_announces(tenant_id, publish_at DESC) WHERE deleted_at IS NULL;

-- 启用 RLS
DO $$
DECLARE tbl TEXT;
BEGIN
  FOREACH tbl IN ARRAY ARRAY[
    'genealogy_branches', 'genealogy_members', 'genealogy_relations', 'genealogy_announces'
  ]
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', tbl);
    EXECUTE format($p$
      CREATE POLICY %I_tenant_isolation ON %I
      USING (
        current_tenant_level() = 'developer'
        OR tenant_id = current_tenant_id()
      )
      WITH CHECK (
        current_tenant_level() = 'developer'
        OR tenant_id = current_tenant_id()
      )
    $p$, tbl, tbl);
  END LOOP;
END $$;
