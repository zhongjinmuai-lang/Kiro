# v1.2 升级说明 · 真正可部署运行版

## 概览

本次升级聚焦"开箱即用、一键部署"，补齐了可运行所需的所有关键缺口。

## 核心变更

### ✅ 修复的关键 Bug
1. **soft_delete 类型不匹配**：`BaseModel.DeletedAt` 从 `soft_delete.DeletedAt`（INT 语义）改为 `gorm.DeletedAt`（TIMESTAMPTZ NULL），与 SQL 迁移脚本完全对齐
2. **tenant.Delete 级联 SQL 错误**：修复 `EXTRACT(EPOCH)::bigint` 与 TIMESTAMPTZ 列冲突
3. **Docker Compose 环境变量不一致**：统一用 `MU_DATABASE_*` 匹配 Viper env prefix
4. **前端登录字段不匹配**：统一为 `tenant_code + username + password`，匹配后端 `/api/v1/auth/login` 入参

### 🆕 新增业务域
- `internal/auth`：登录 / 注册 / 刷新 / 登出 / 改密 完整实现
- `internal/genealogy`：族谱业务完整域（Branch / Member / Relation / Announce）
  - 世系树（PG 递归 CTE）
  - 祖先溯源 / 分支遍历 / LCA
  - AI OCR 建档
  - 统计接口

### 🆕 新增适配器骨架
- `internal/ai/clients.go`：豆包 / 通义 / 文心 / DeepSeek / 私有化（OpenAI 兼容协议）
- `internal/platform/storage/drivers/`：本地 + 阿里 OSS / 腾讯 COS / 七牛 / 华为 OBS 骨架
- `internal/platform/notify/senders/`：Email (SMTP) / WebSocket / SMS 发送器
- `internal/platform/payment/adapters/`：微信 / 支付宝 / 银联 适配器骨架
- `internal/agent/plugin/examples/hello`：示例插件

### 🆕 部署增强
- **docker-compose.yml** 修正：
  - 挂载 `migrations/` 目录到 PG 容器
  - 环境变量 `MU_DATABASE_HOST / MU_REDIS_ADDR / MU_JWT_SECRET` 正确注入
  - 增加 `storage_data` 卷
  - 开放 80 / 8000 / 8001 三端口（对应三个前端后台）
- **Dockerfile.frontend** 一次性构建三个 Vue 后台，缺失时写入占位页
- **Dockerfile.backend** 使用 `golang:1.23-alpine`（公共镜像可用），多阶段构建
- **nginx.conf** 三个 server 块分别托管三个前端，含 WebSocket 代理
- **init-db.sql** 执行顺序修正：001 → 002 → 004 → 005 → 003(seed)
- **003_seed_data.sql** 创建三级租户 + 各级默认 admin 账号（密码统一 `admin123`）
- **005_genealogy_tables.sql** 新增族谱域建表 + RLS 策略
- **deploy.sh** 改为 docker compose v2 + 健康检查 + 友好输出

### 🆕 前端完整性
- **admin-developer**：补齐 `Dashboard / Login / Settings / Payment / Storage / Notify / Plugins / Engine / Tenants` 等所有页面
- **admin-provider**：补齐全部核心页面（`Customers / Payment / Storage / Notify / Permissions / Settings / Login / Dashboard`）
- **admin-customer**：补齐含族谱可视化树的 `Genealogy / Files / Messages / Account / Dashboard / Login`
- 三端均实现 Bearer Token + 智能续签自动捕获

### 🎯 默认账号（一键部署后可直接登录）
| 后台 | tenantCode | username | password |
|------|-----------|----------|----------|
| 开发商总后台 | `mu-platform` | `admin` | `admin123` |
| 服务商 SaaS 后台 | `demo-provider` | `admin` | `admin123` |
| 终端客户业务后台 | `demo-family` | `admin` | `admin123` |

### 📦 示例数据
开箱包含 1 个分支 + 3 代族谱成员 + 1 条家族公告，方便立即体验族谱可视化。

## 部署步骤

```bash
# 1. 克隆代码
git clone https://github.com/zhongjinmuai-lang/Kiro.git
cd Kiro

# 2. 切换到本版本分支
git checkout MuAgent-zupu

# 3. 一键启动
./deploy/scripts/deploy.sh dev

# 4. 访问
# 开发商后台: http://localhost
# 服务商后台: http://localhost:8000
# 终端后台:   http://localhost:8001
```

## 下一步

- [ ] 接入真实支付/存储/短信 SDK
- [ ] 完整 E2E 测试套件
- [ ] Grafana + Prometheus 监控模板
- [ ] K8s Helm Chart
