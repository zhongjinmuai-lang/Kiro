-- ============================================================
-- MU Framework 初始数据（种子）
-- 创建开发商/服务商/终端客户三级租户 + 各级默认管理员
-- 密码统一：admin123（bcrypt 加密，与 Go bcrypt 库兼容）
-- ============================================================

-- 开发商租户
INSERT INTO tenants (id, name, code, level, parent_id, status, config)
VALUES ('11111111-1111-1111-1111-111111111111', 'MU平台', 'mu-platform', 'developer', NULL, 1, '{"description":"MU框架默认开发商"}')
ON CONFLICT (code) DO NOTHING;

-- 服务商租户（示例服务商）
INSERT INTO tenants (id, name, code, level, parent_id, status, config)
VALUES ('22222222-2222-2222-2222-222222222222', '示例服务商', 'demo-provider',
        'provider', '11111111-1111-1111-1111-111111111111', 1, '{}')
ON CONFLICT (code) DO NOTHING;

-- 终端客户租户（示例家族）
INSERT INTO tenants (id, name, code, level, parent_id, status, config)
VALUES ('33333333-3333-3333-3333-333333333333', '示例家族', 'demo-family',
        'customer', '22222222-2222-2222-2222-222222222222', 1, '{}')
ON CONFLICT (code) DO NOTHING;

-- 角色
DO $$
DECLARE
    dev_tid UUID := '11111111-1111-1111-1111-111111111111';
    pro_tid UUID := '22222222-2222-2222-2222-222222222222';
    cus_tid UUID := '33333333-3333-3333-3333-333333333333';
    dev_role_id UUID;
    pro_role_id UUID;
    cus_role_id UUID;
BEGIN
    -- 超级管理员角色（开发商）
    INSERT INTO roles (id, tenant_id, name, code, level, permissions, status)
    VALUES (uuid_generate_v4(), dev_tid, '超级管理员', 'super_admin', 'developer', '["*"]', 1)
    RETURNING id INTO dev_role_id;

    -- 服务商管理员
    INSERT INTO roles (id, tenant_id, name, code, level, permissions, status)
    VALUES (uuid_generate_v4(), pro_tid, '服务商管理员', 'provider_admin', 'provider', '["*"]', 1)
    RETURNING id INTO pro_role_id;

    -- 终端客户家主
    INSERT INTO roles (id, tenant_id, name, code, level, permissions, status)
    VALUES (uuid_generate_v4(), cus_tid, '家族族长', 'family_head', 'customer', '["*"]', 1)
    RETURNING id INTO cus_role_id;

    -- 默认管理员用户（密码：admin123）
    -- 开发商
    INSERT INTO users (id, tenant_id, username, password, nickname, role_id, status)
    VALUES (uuid_generate_v4(), dev_tid, 'admin',
            crypt('admin123', gen_salt('bf')),
            'MU开发商管理员', dev_role_id, 1);

    -- 服务商
    INSERT INTO users (id, tenant_id, username, password, nickname, role_id, status)
    VALUES (uuid_generate_v4(), pro_tid, 'admin',
            crypt('admin123', gen_salt('bf')),
            '示例服务商', pro_role_id, 1);

    -- 终端客户
    INSERT INTO users (id, tenant_id, username, password, nickname, role_id, status)
    VALUES (uuid_generate_v4(), cus_tid, 'admin',
            crypt('admin123', gen_salt('bf')),
            '示例家族族长', cus_role_id, 1);

    -- 初始化权限定义
    INSERT INTO permissions (module, name, code, level) VALUES
    ('tenant', '查看租户', 'tenant:list:read', 'developer'),
    ('tenant', '创建租户', 'tenant:item:create', 'developer'),
    ('tenant', '编辑租户', 'tenant:item:update', 'developer'),
    ('tenant', '删除租户', 'tenant:item:delete', 'developer'),
    ('payment', '查看支付渠道', 'payment:channel:read', 'developer'),
    ('payment', '创建支付渠道', 'payment:channel:create', 'developer'),
    ('payment', '授权支付渠道', 'payment:channel:grant', 'developer'),
    ('payment', '查看订单', 'payment:order:read', 'provider'),
    ('payment', '发起退款', 'payment:order:refund', 'provider'),
    ('storage', '查看存储源', 'storage:source:read', 'developer'),
    ('storage', '创建存储源', 'storage:source:create', 'developer'),
    ('storage', '上传文件', 'storage:file:upload', 'customer'),
    ('storage', '删除文件', 'storage:file:delete', 'customer'),
    ('storage', '管理配额', 'storage:quota:manage', 'provider'),
    ('notify', '查看通知通道', 'notify:channel:read', 'developer'),
    ('notify', '创建通知通道', 'notify:channel:create', 'developer'),
    ('notify', '管理模板', 'notify:template:manage', 'provider'),
    ('notify', '发送消息', 'notify:message:send', 'customer'),
    ('agent', '查看插件', 'agent:plugin:read', 'developer'),
    ('agent', '安装插件', 'agent:plugin:install', 'developer'),
    ('agent', '卸载插件', 'agent:plugin:uninstall', 'developer'),
    ('agent', '查看引擎状态', 'agent:engine:status', 'developer'),
    ('agent', '提交任务', 'agent:task:submit', 'provider'),
    ('genealogy', '成员管理', 'genealogy:member:manage', 'customer'),
    ('genealogy', '分支管理', 'genealogy:branch:manage', 'customer'),
    ('genealogy', '公告发布', 'genealogy:announce:publish', 'customer'),
    ('genealogy', 'OCR建档', 'genealogy:ocr:use', 'customer')
    ON CONFLICT (code) DO NOTHING;

    -- 开发商授予所有权限
    INSERT INTO tenant_permissions (tenant_id, permission_code, granted_by, status)
    SELECT dev_tid, code, dev_tid, 1 FROM permissions
    ON CONFLICT DO NOTHING;

    -- 服务商授予 provider/customer 级权限
    INSERT INTO tenant_permissions (tenant_id, permission_code, granted_by, status)
    SELECT pro_tid, code, dev_tid, 1 FROM permissions WHERE level IN ('provider', 'customer')
    ON CONFLICT DO NOTHING;

    -- 终端客户授予 customer 级权限
    INSERT INTO tenant_permissions (tenant_id, permission_code, granted_by, status)
    SELECT cus_tid, code, pro_tid, 1 FROM permissions WHERE level = 'customer'
    ON CONFLICT DO NOTHING;

    -- 示例族谱数据（让 customer 登录后有初始内容）
    DECLARE
        branch_id UUID := uuid_generate_v4();
        m1_id UUID := uuid_generate_v4();
        m2_id UUID := uuid_generate_v4();
        m3_id UUID := uuid_generate_v4();
    BEGIN
        INSERT INTO genealogy_branches (id, tenant_id, name, code, depth, remark)
        VALUES (branch_id, cus_tid, '长房', 'zhang', 0, '示例分支');

        INSERT INTO genealogy_members (id, tenant_id, branch_id, father_id, generation, name, alias_name, gender, birthplace, biography)
        VALUES
            (m1_id, cus_tid, branch_id, NULL, 1, '始祖公', '讳 · 示', 'male', '中原', '家族始祖，开基立业'),
            (m2_id, cus_tid, branch_id, m1_id, 2, '长子', '字 · 仁', 'male', '中原', '始祖长子'),
            (m3_id, cus_tid, branch_id, m2_id, 3, '长孙', '字 · 智', 'male', '中原', '长子之子');

        INSERT INTO genealogy_announces (id, tenant_id, title, content, author, pinned, publish_at)
        VALUES
            (uuid_generate_v4(), cus_tid, '欢迎使用 MU 族谱平台',
             '本平台支持世系可视化、AI 识别老族谱、亲属溯源等能力，欢迎族人共同维护。',
             '家族理事会', true, NOW());
    END;
END $$;
