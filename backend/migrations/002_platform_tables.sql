-- ============================================================
-- MU Framework 三大中台数据表
-- ============================================================

-- ============================================================
-- 二、支付中台
-- ============================================================

-- 支付渠道配置表
CREATE TABLE payment_channels (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    level       VARCHAR(20) NOT NULL,
    type        VARCHAR(20) NOT NULL CHECK (type IN ('wechat', 'alipay', 'union', 'stripe')),
    name        VARCHAR(100) NOT NULL,
    app_id      VARCHAR(100),
    merchant_id VARCHAR(100),
    secret_key  TEXT,
    notify_url  VARCHAR(500),
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_channels_tenant ON payment_channels(tenant_id) WHERE status = 1;

COMMENT ON TABLE payment_channels IS '支付渠道配置 - 开发商配置，逐级授权';

-- 支付渠道授权表（三级管控）
CREATE TABLE tenant_payment_auth (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    channel_id  UUID NOT NULL REFERENCES payment_channels(id),
    granted_by  UUID REFERENCES tenants(id),
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_payment_auth_unique ON tenant_payment_auth(tenant_id, channel_id);

COMMENT ON TABLE tenant_payment_auth IS '支付渠道授权 - 上级授予下级可用渠道';

-- 支付订单表
CREATE TABLE payment_orders (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id),
    channel_id   UUID NOT NULL REFERENCES payment_channels(id),
    channel_type VARCHAR(20),
    order_no     VARCHAR(64) NOT NULL,
    trade_no     VARCHAR(64),
    amount       BIGINT NOT NULL,
    currency     VARCHAR(10) NOT NULL DEFAULT 'CNY',
    subject      VARCHAR(200),
    status       SMALLINT NOT NULL DEFAULT 0,
    paid_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_payment_orders_no ON payment_orders(order_no);
CREATE INDEX idx_payment_orders_tenant ON payment_orders(tenant_id, created_at DESC);
CREATE INDEX idx_payment_orders_status ON payment_orders(status) WHERE status = 0;

COMMENT ON TABLE payment_orders IS '支付订单表';
COMMENT ON COLUMN payment_orders.amount IS '金额，单位：分';
COMMENT ON COLUMN payment_orders.status IS '0:待支付 1:已支付 2:退款中 3:已退款 4:已关闭';

-- ============================================================
-- 三、存储中台
-- ============================================================

-- 存储源配置表
CREATE TABLE storage_sources (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    level       VARCHAR(20) NOT NULL,
    provider    VARCHAR(20) NOT NULL CHECK (provider IN ('local', 'oss', 'cos', 's3', 'minio')),
    name        VARCHAR(100) NOT NULL,
    bucket      VARCHAR(100),
    region      VARCHAR(50),
    endpoint    VARCHAR(500),
    access_key  VARCHAR(200),
    secret_key  TEXT,
    cdn_domain  VARCHAR(200),
    max_size    BIGINT DEFAULT 0,
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_storage_sources_tenant ON storage_sources(tenant_id) WHERE status = 1;

COMMENT ON TABLE storage_sources IS '存储源配置 - 开发商配置，逐级绑定';

-- 存储文件记录表
CREATE TABLE storage_files (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    source_id   UUID NOT NULL REFERENCES storage_sources(id),
    file_name   VARCHAR(255) NOT NULL,
    file_size   BIGINT NOT NULL DEFAULT 0,
    mime_type   VARCHAR(100),
    path        VARCHAR(500) NOT NULL,
    url         VARCHAR(1000),
    hash        VARCHAR(64),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_storage_files_tenant ON storage_files(tenant_id, created_at DESC);
CREATE INDEX idx_storage_files_hash ON storage_files(hash);

COMMENT ON TABLE storage_files IS '文件上传记录';

-- 存储配额表
CREATE TABLE storage_quotas (
    tenant_id   UUID PRIMARY KEY REFERENCES tenants(id),
    max_bytes   BIGINT NOT NULL DEFAULT 0,
    used_bytes  BIGINT NOT NULL DEFAULT 0,
    max_files   BIGINT NOT NULL DEFAULT 0,
    used_files  BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE storage_quotas IS '存储配额管理';

-- ============================================================
-- 四、通知中台
-- ============================================================

-- 通知通道配置表
CREATE TABLE notify_channels (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    level       VARCHAR(20) NOT NULL,
    type        VARCHAR(20) NOT NULL CHECK (type IN ('sms', 'email', 'push', 'wechat', 'websocket')),
    name        VARCHAR(100) NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}',
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notify_channels_tenant ON notify_channels(tenant_id, type) WHERE status = 1;

COMMENT ON TABLE notify_channels IS '通知通道配置 - 开发商配置，逐级启用';

-- 通知模板表
CREATE TABLE notify_templates (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    channel     VARCHAR(20) NOT NULL,
    code        VARCHAR(50) NOT NULL,
    name        VARCHAR(100) NOT NULL,
    content     TEXT NOT NULL,
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_notify_templates_code ON notify_templates(tenant_id, code);

COMMENT ON TABLE notify_templates IS '通知消息模板';
COMMENT ON COLUMN notify_templates.content IS '模板内容，支持 {{变量}} 占位符';

-- 通知消息记录表
CREATE TABLE notify_messages (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    channel_id  UUID NOT NULL REFERENCES notify_channels(id),
    template_id UUID REFERENCES notify_templates(id),
    receiver    VARCHAR(200) NOT NULL,
    content     TEXT NOT NULL,
    status      SMALLINT NOT NULL DEFAULT 0,
    retry_count SMALLINT NOT NULL DEFAULT 0,
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notify_messages_tenant ON notify_messages(tenant_id, created_at DESC);
CREATE INDEX idx_notify_messages_status ON notify_messages(status) WHERE status IN (0, 1, 4);

COMMENT ON TABLE notify_messages IS '消息发送记录';
COMMENT ON COLUMN notify_messages.status IS '0:待发送 1:发送中 2:已发送 3:失败 4:重试中';
