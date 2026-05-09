-- ============================================================
-- MU Framework 数据库初始化迁移
-- PostgreSQL 18.3
-- 创建时间：2026-05-09
-- ============================================================

-- 启用UUID扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- 一、三级SaaS管控核心表
-- ============================================================

-- 租户表（三级层级）
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(50) NOT NULL UNIQUE,
    level       VARCHAR(20) NOT NULL CHECK (level IN ('developer', 'provider', 'customer')),
    parent_id   UUID REFERENCES tenants(id) ON DELETE SET NULL,
    status      SMALLINT NOT NULL DEFAULT 1,  -- 1:启用 0:禁用
    config      JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_tenants_parent_id ON tenants(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_level ON tenants(level) WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_code ON tenants(code) WHERE deleted_at IS NULL;

COMMENT ON TABLE tenants IS '租户表 - 三级SaaS层级管控核心';
COMMENT ON COLUMN tenants.level IS '层级：developer(开发商)/provider(服务商)/customer(终端客户)';

-- 用户表
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    username    VARCHAR(50) NOT NULL,
    password    VARCHAR(255) NOT NULL,
    nickname    VARCHAR(100),
    email       VARCHAR(100),
    phone       VARCHAR(20),
    avatar      VARCHAR(500),
    role_id     UUID,
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_users_tenant_username ON users(tenant_id, username) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_tenant_id ON users(tenant_id) WHERE deleted_at IS NULL;

COMMENT ON TABLE users IS '用户表';

-- 角色表
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(50) NOT NULL,
    code        VARCHAR(50) NOT NULL,
    level       VARCHAR(20) NOT NULL,
    permissions JSONB DEFAULT '[]',
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_roles_tenant_code ON roles(tenant_id, code) WHERE deleted_at IS NULL;

COMMENT ON TABLE roles IS '角色表';

-- 权限定义表
CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    module      VARCHAR(50) NOT NULL,
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(100) NOT NULL UNIQUE,
    level       VARCHAR(20) NOT NULL,
    parent_id   UUID REFERENCES permissions(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_permissions_module ON permissions(module) WHERE deleted_at IS NULL;
CREATE INDEX idx_permissions_level ON permissions(level) WHERE deleted_at IS NULL;

COMMENT ON TABLE permissions IS '权限定义表';
COMMENT ON COLUMN permissions.code IS '权限编码，格式：module:resource:action';

-- 租户权限授予表
CREATE TABLE tenant_permissions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    permission_code VARCHAR(100) NOT NULL,
    granted_by      UUID REFERENCES tenants(id),
    status          SMALLINT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tenant_perm_unique ON tenant_permissions(tenant_id, permission_code);
CREATE INDEX idx_tenant_perm_tenant ON tenant_permissions(tenant_id) WHERE status = 1;

COMMENT ON TABLE tenant_permissions IS '租户权限授予表 - 三级管控核心';
