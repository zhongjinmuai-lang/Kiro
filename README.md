# MU - 自研全能智能体主体框架

> **MU** 不仅是 Web 开发框架，更是具备**自感知、自升级、自进化、自修复**的智能体底座。
> 适配 SaaS 多租户、插件化、AI 调度、族谱业务全场景。

[![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-18.3-336791?logo=postgresql)](https://www.postgresql.org)
[![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js)](https://vuejs.org)
[![UniApp X](https://img.shields.io/badge/UniApp_X-latest-2B2B2B)](https://uniapp.dcloud.net.cn)

---

## 🎯 部署方案（三服务器独立部署）

**🏢 开发商服务器** — 后端全栈 + 开发商总后台
```bash
cd deploy/developer && cp .env.example .env && vim .env && ./deploy.sh up
```

**🏪 服务商服务器** — 服务商 SaaS 后台独立部署（指向开发商 API）
```bash
cd deploy/provider && cp .env.example .env && vim .env && ./deploy.sh up
```

**👪 终端客户服务器** — 业务后台独立部署（支持贴牌）
```bash
cd deploy/customer && cp .env.example .env && vim .env && ./deploy.sh up
```

**🎋 宝塔面板手动部署**（无 Docker 场景）
```bash
cd /www/wwwroot/mu-framework
bash deploy/baota/setup.sh [developer|provider|customer]
```

**🔁 单机演示（一键启动全部）**
```bash
./deploy/scripts/deploy.sh dev
```

详见 [部署说明 (deploy/README.md)](deploy/README.md)。

---

## 🏗️ 技术栈（2026 官方最新稳定版）

| 层级 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.26.1 |
| Web | Gin | 1.10 |
| ORM | GORM v2 + postgres driver | 1.25+ |
| 数据库 | PostgreSQL | 18.3 |
| 缓存/队列 | Redis | 7.4.x（集群） |
| 日志 | Zap 2.x + lumberjack | v1.27 / v2.2 |
| 鉴权 | JWT v5（双令牌 + 智能续签） | v5.2 |
| 配置 | Viper（含环境变量覆盖） | v1.19 |
| API文档 | OpenAPI 3.1 + Swagger UI | v1.6 |
| 前端 | Vue 3.5 + Vite + Pinia | - |
| 移动端 | UniApp X（Vue3 一次编译多端） | latest |
| 部署 | Docker 27.x / K3s / Kubernetes 1.32+ | - |

---

## 🎨 核心架构

```
                 ┌─────────────────────────────┐
                 │  MU 自进化智能内核（自研）     │
                 │  ├── 状态自感知              │
                 │  ├── 版本自升级（热更新）      │
                 │  ├── 性能自优化              │
                 │  └── 故障自修复              │
                 └──────────────┬──────────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          │                     │                     │
    ┌─────▼─────┐         ┌─────▼─────┐         ┌─────▼─────┐
    │插件热插拔 │         │三大统一中台│         │AI调度网关 │
    │引擎       │         │支付/存储/  │         │豆包/通义/ │
    │           │         │通知       │         │文心/DS/私有│
    └───────────┘         └───────────┘         └───────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          │            三级SaaS管控体系               │
    ┌─────▼─────┐         ┌─────▼─────┐         ┌─────▼─────┐
    │开发商总后台│──授权──▶│服务商SaaS ├──授权──▶│终端客户业务│
    │顶层集权   │         │二级管控   │         │三级受限使用│
    └───────────┘         └───────────┘         └───────────┘
```

---

## 📁 项目结构（DDD 分层）

```
Kiro/
├── backend/                    # 后端（Go + DDD 分层）
│   ├── cmd/                    # 启动入口（单体/微服务无缝切换）
│   │   ├── api-server/         # 终端客户业务 API（:8080）
│   │   ├── admin-server/       # 三级管理后台 API（:8081）
│   │   └── agent-engine/       # 智能体引擎（:8082）
│   ├── internal/               # 内部业务域
│   │   ├── core/               # 框架核心（bootstrap/config/middleware/router）
│   │   ├── auth/               # 认证域
│   │   ├── saas/               # 三级SaaS管控（租户/层级/权限）
│   │   ├── platform/           # 三大统一中台（支付/存储/通知 + 适配器/驱动/发送器）
│   │   ├── agent/              # 智能体引擎（插件/调度/自进化/注册中心）
│   │   ├── ai/                 # AI 网关（豆包/通义/文心/DeepSeek/私有化）
│   │   ├── genealogy/          # 族谱业务域（世系树/亲属溯源/AI OCR）
│   │   └── model/              # GORM 实体模型
│   ├── pkg/                    # 公共包
│   │   ├── cache/              # Redis 集群 + 分布式锁 + 限流
│   │   ├── hierarchy/          # PG 递归 CTE 族谱工具
│   │   ├── jwt/                # JWT 双令牌 + 智能续签
│   │   ├── logger/             # Zap + 切割归档 + TraceID
│   │   └── response/           # Gin 统一响应
│   ├── migrations/             # PostgreSQL 迁移 SQL（001~005）
│   └── docs/                   # OpenAPI 3.1 规范
├── frontend/                   # 前端
│   ├── admin-developer/        # 开发商总后台（Vue 3.5）
│   ├── admin-provider/         # 服务商 SaaS 管理后台（Vue 3.5）
│   ├── admin-customer/         # 终端客户业务后台（Vue 3.5 + 族谱可视化）
│   └── app-uniapp/             # UniApp X 多端（微信/抖音/支付宝/小红书/鸿蒙/App/H5）
├── configs/                    # 多环境配置（dev/staging/prod）
├── deploy/                     # 部署编排
│   ├── docker/                 # Docker Compose + Nginx + 多阶段构建
│   ├── k8s/                    # K8s 清单（Deployment / HPA / Ingress）
│   └── scripts/                # 一键部署脚本
└── docs/                       # 配套文档（8 份）
```

---

## 🎯 核心能力

### 1. MU 自进化智能内核 🧠

- **状态自感知**：实时感知负载、性能、压力、异常
- **版本自升级**：框架与插件在线灰度升级，无需停机
- **性能自优化**：自动分析慢 SQL、优化索引、调整连接池
- **故障自修复**：常规异常自动重试、服务自愈

### 2. 插件化热插拔引擎 🧩

- 业务插件独立开发、在线上传、热加载/卸载
- 与主框架完全解耦，支持依赖管理、版本灰度
- 示例：`backend/internal/agent/plugin/examples/hello`

### 3. 三级 SaaS 管控体系 🏢

| 层级 | 主体 | 权限 |
|------|------|------|
| L1 顶层 | 开发商 | 全局渠道准入、模板定义、费率设定、配额规则 |
| L2 中层 | 服务商 | 二次绑定商户号、下属客户管理、分润/提现、品牌定制 |
| L3 底层 | 终端客户 | 配额内使用，无权配置渠道/密钥 |

**开关联动**：开发商关闭任一能力 → 服务商立即失效 → 终端客户同步受限

### 4. 三大统一中台 🏗️

- **聚合支付中台**：微信/支付宝/聚合支付，统一下单/退款/对账/分润
- **第三方存储中台**：阿里 OSS / 腾讯 COS / 七牛 / 华为 OBS / MinIO / 本地
- **消息通知中台**：短信 / 邮件 / 微信推送 / App 推送 / 站内信

### 5. AI 多供应商网关 🤖

- 豆包 / 通义千问 / 文心一言 / DeepSeek / 企业私有部署
- 按优先级自动降级、租户维度用量计量
- 全部兼容 OpenAI Chat Completion 协议

### 6. 族谱业务域 🌳

- 世系树可视化（PG 递归 CTE，替代图数据库）
- 亲属溯源、分支遍历、最近公共祖先（LCA）
- AI OCR 老族谱识别建档
- 分支管理、家族公告

### 7. 数据隔离 🔒

- **租户级**：PG Row Level Security（RLS）行级隔离
- **应用级**：`app.current_tenant_id` / `app.current_tenant_level` 会话变量
- **金融级**：即使应用层漏判，DB 层兜底拒绝

---

## 📚 配套文档

- [架构设计](docs/architecture.md)
- [三级管控矩阵](docs/control-matrix.md)
- [数据库设计](docs/database-design.md)
- [API 规范（OpenAPI 3.1）](docs/api-conventions.md)
- [部署运维手册](docs/deployment.md)
- [🎋 宝塔面板部署指南](docs/baota-deployment.md)
- [🛠️ 环境要求文档](docs/environment-requirements.md)
- [三级管控操作手册](docs/operations-manual.md)
- [插件开发指南](docs/plugin-development.md)
- [族谱业务域](docs/genealogy-domain.md)

---

## 🛠️ 本地开发

### 后端
```bash
# 启动依赖（PG + Redis）
docker compose -f deploy/docker/docker-compose.yml up -d postgres redis

# 启动 API Server
cd backend
go run ./cmd/api-server --config ../configs/dev.yaml
```

### 前端
```bash
# 开发商后台
cd frontend/admin-developer && npm install && npm run dev   # :3000

# 服务商后台
cd frontend/admin-provider && npm install && npm run dev    # :3001

# 终端客户后台
cd frontend/admin-customer && npm install && npm run dev    # :3002

# UniApp X 多端
cd frontend/app-uniapp && npm install && npm run dev:h5
```

---

## 📄 License

Proprietary - 自研框架，商业授权，禁止未经许可的复制传播。
