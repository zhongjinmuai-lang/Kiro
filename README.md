# MU - 自研全能智能体主体框架

> 核心内核，可自进化、自升级、自修复

## 🏗️ 技术基线

| 组件 | 版本 |
|------|------|
| 后端 Runtime | Go 1.26.1 |
| 主数据库 | PostgreSQL 18.3（企业级） |
| 前端框架 | Vue 3.5 + UniApp X |
| 部署运维 | Docker 27.x + K3s/Kubernetes 1.32+ |

## 🎯 核心架构

**MU自研智能体框架** + **三级SaaS层级管控** + **插件化热插拔** + **AI智能体调度** + **全端多端统一**

### 三级SaaS层级管控

```
开发商总后台 → 服务商SaaS管理后台 → 终端客户业务后台
（权限自上而下递减，控制流向严格单向）
```

### 三大统一中台

- **支付中台** - 统一支付能力，三级权限管控
- **存储中台** - 统一文件存储，逐级授权
- **通知中台** - 统一消息通知，分层投递

## 📁 项目结构

```
mu-framework/
├── backend/                    # 后端服务（Go）
│   ├── cmd/                    # 启动入口
│   │   ├── api-server/         # API 网关服务
│   │   ├── admin-server/       # 管理后台服务
│   │   └── agent-engine/       # 智能体引擎服务
│   ├── internal/               # 内部模块（不对外暴露）
│   │   ├── core/               # 框架核心
│   │   │   ├── config/         # 配置管理
│   │   │   ├── middleware/     # 中间件
│   │   │   ├── router/         # 路由管理
│   │   │   └── bootstrap/      # 启动引导
│   │   ├── saas/               # 三级SaaS管控
│   │   │   ├── tenant/         # 租户管理
│   │   │   ├── permission/     # 权限体系
│   │   │   └── hierarchy/      # 层级管控
│   │   ├── platform/           # 三大统一中台
│   │   │   ├── payment/        # 支付中台
│   │   │   ├── storage/        # 存储中台
│   │   │   └── notify/         # 通知中台
│   │   ├── agent/              # MU智能体引擎
│   │   │   ├── engine/         # 调度引擎
│   │   │   ├── plugin/         # 插件系统
│   │   │   ├── evolution/      # 自进化模块
│   │   │   └── registry/       # 能力注册中心
│   │   └── model/              # 数据模型
│   ├── pkg/                    # 公共工具包
│   │   ├── utils/              # 通用工具
│   │   ├── logger/             # 日志组件
│   │   ├── cache/              # 缓存组件
│   │   └── crypto/             # 加密组件
│   └── migrations/             # 数据库迁移
├── frontend/                   # 前端项目
│   ├── admin-developer/        # 开发商总后台（Vue 3.5）
│   ├── admin-provider/         # 服务商管理后台（Vue 3.5）
│   ├── admin-customer/         # 终端客户后台（Vue 3.5）
│   └── app-uniapp/             # 多端应用（UniApp X）
├── deploy/                     # 部署配置
│   ├── docker/                 # Docker 配置
│   ├── k8s/                    # Kubernetes 部署清单
│   └── scripts/                # 部署脚本
├── docs/                       # 文档
│   └── architecture.md         # 架构设计文档
└── configs/                    # 环境配置文件
    ├── dev.yaml
    ├── staging.yaml
    └── prod.yaml
```

## 🚀 快速开始

```bash
# 1. 启动基础设施
docker-compose -f deploy/docker/docker-compose.yml up -d

# 2. 运行数据库迁移
go run backend/cmd/migrate/main.go

# 3. 启动API服务
go run backend/cmd/api-server/main.go

# 4. 启动管理后台
cd frontend/admin-developer && npm run dev
```

## 📋 核心特质

- ✅ 完全自研，无第三方SaaS框架依赖
- ✅ 工业级严谨代码，低耦合、高可扩展
- ✅ 支持私有化部署、贴牌定制、二次开发
- ✅ 内置三大统一中台，权限逐级管控与能力复用
- ✅ 插件化热插拔，按需加载
- ✅ AI智能体调度，自进化/自升级/自修复

## 📄 License

Proprietary - All Rights Reserved
