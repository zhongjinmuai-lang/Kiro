-- ============================================================
-- MU Framework 初始数据
-- 创建默认开发商租户、管理员用户、基础权限
-- ============================================================

-- 1. 创建默认开发商租户
INSERT INTO tenants (id, name, code, level, parent_id, status, config) VALUES
(uuid_generate_v4(), 'MU平台', 'mu-platform', 'developer', NULL, 1, '{"description": "MU框架默认开发商"}');

-- 获取刚创建的租户ID
DO $$
DECLARE
    dev_tenant_id UUID;
    admin_role_id UUID;
BEGIN
    SELECT id INTO dev_tenant_id FROM tenants WHERE code = 'mu-platform';
    
    -- 2. 创建管理员角色
    INSERT INTO roles (id, tenant_id, name, code, level, permissions, status) VALUES
    (uuid_generate_v4(), dev_tenant_id, '超级管理员', 'super_admin', 'developer', '["*"]', 1)
    RETURNING id INTO admin_role_id;
    
    -- 3. 创建默认管理员用户 (密码: mu_admin_2026)
    INSERT INTO users (id, tenant_id, username, password, nickname, role_id, status) VALUES
    (uuid_generate_v4(), dev_tenant_id, 'admin', crypt('mu_admin_2026', gen_salt('bf')), 'MU管理员', admin_role_id, 1);
    
    -- 4. 初始化权限定义
    INSERT INTO permissions (module, name, code, level) VALUES
    -- 租户管理权限
    ('tenant', '查看租户', 'tenant:list:read', 'developer'),
    ('tenant', '创建租户', 'tenant:item:create', 'developer'),
    ('tenant', '编辑租户', 'tenant:item:update', 'developer'),
    ('tenant', '删除租户', 'tenant:item:delete', 'developer'),
    -- 支付中台权限
    ('payment', '查看支付渠道', 'payment:channel:read', 'developer'),
    ('payment', '创建支付渠道', 'payment:channel:create', 'developer'),
    ('payment', '授权支付渠道', 'payment:channel:grant', 'developer'),
    ('payment', '查看订单', 'payment:order:read', 'provider'),
    ('payment', '发起退款', 'payment:order:refund', 'provider'),
    -- 存储中台权限
    ('storage', '查看存储源', 'storage:source:read', 'developer'),
    ('storage', '创建存储源', 'storage:source:create', 'developer'),
    ('storage', '上传文件', 'storage:file:upload', 'customer'),
    ('storage', '删除文件', 'storage:file:delete', 'customer'),
    ('storage', '管理配额', 'storage:quota:manage', 'provider'),
    -- 通知中台权限
    ('notify', '查看通知通道', 'notify:channel:read', 'developer'),
    ('notify', '创建通知通道', 'notify:channel:create', 'developer'),
    ('notify', '管理模板', 'notify:template:manage', 'provider'),
    ('notify', '发送消息', 'notify:message:send', 'customer'),
    -- 智能体引擎权限
    ('agent', '查看插件', 'agent:plugin:read', 'developer'),
    ('agent', '安装插件', 'agent:plugin:install', 'developer'),
    ('agent', '卸载插件', 'agent:plugin:uninstall', 'developer'),
    ('agent', '查看引擎状态', 'agent:engine:status', 'developer'),
    ('agent', '提交任务', 'agent:task:submit', 'provider');
    
    -- 5. 为开发商租户授予所有权限
    INSERT INTO tenant_permissions (tenant_id, permission_code, granted_by, status)
    SELECT dev_tenant_id, code, dev_tenant_id, 1 FROM permissions;
    
END $$;
