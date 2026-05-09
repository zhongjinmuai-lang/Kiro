-- ============================================================
-- MU Framework 行级安全（Row Level Security）
-- 基于 PostgreSQL 18.3 RLS 实现金融级多租户数据隔离
-- 运行时通过 SET LOCAL app.current_tenant_id = '<uuid>' 注入当前租户
-- ============================================================

-- 公共策略函数：当前请求的租户ID
CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS UUID
LANGUAGE SQL STABLE AS $$
  SELECT NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
$$;

-- 公共策略函数：当前请求的层级（developer/provider/customer）
CREATE OR REPLACE FUNCTION current_tenant_level() RETURNS TEXT
LANGUAGE SQL STABLE AS $$
  SELECT current_setting('app.current_tenant_level', true)
$$;

-- ============================================================
-- 为核心表启用 RLS
-- ============================================================
DO $$
DECLARE tbl TEXT;
BEGIN
  FOREACH tbl IN ARRAY ARRAY[
    'users', 'roles',
    'payment_channels', 'tenant_payment_auth', 'payment_orders',
    'storage_sources', 'storage_files', 'storage_quotas',
    'notify_channels', 'notify_templates', 'notify_messages',
    'tenant_permissions'
  ]
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', tbl);
  END LOOP;
END $$;

-- ============================================================
-- 租户隔离策略：
--   - developer 层级可看全部（平台运营需要）
--   - provider / customer 只能看自己的数据
-- ============================================================
DO $$
DECLARE tbl TEXT;
BEGIN
  FOREACH tbl IN ARRAY ARRAY[
    'users', 'roles',
    'payment_orders',
    'storage_files', 'storage_quotas',
    'notify_templates', 'notify_messages',
    'tenant_permissions'
  ]
  LOOP
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

-- ============================================================
-- 支付/存储/通知渠道配置：仅开发商可读写
--   服务商/终端通过 tenant_*_auth 关联表获得可用渠道
-- ============================================================
CREATE POLICY payment_channels_dev_only ON payment_channels
  USING (current_tenant_level() = 'developer')
  WITH CHECK (current_tenant_level() = 'developer');

CREATE POLICY storage_sources_dev_only ON storage_sources
  USING (
    current_tenant_level() = 'developer'
    OR tenant_id = current_tenant_id()  -- 服务商可绑定自有存储
  )
  WITH CHECK (
    current_tenant_level() = 'developer'
    OR (current_tenant_level() = 'provider' AND tenant_id = current_tenant_id())
  );

CREATE POLICY notify_channels_read_down ON notify_channels
  USING (
    current_tenant_level() = 'developer'
    OR tenant_id = current_tenant_id()
  )
  WITH CHECK (current_tenant_level() = 'developer');

CREATE POLICY tenant_payment_auth_policy ON tenant_payment_auth
  USING (
    current_tenant_level() = 'developer'
    OR tenant_id = current_tenant_id()
    OR granted_by = current_tenant_id()
  )
  WITH CHECK (current_tenant_level() IN ('developer', 'provider'));
