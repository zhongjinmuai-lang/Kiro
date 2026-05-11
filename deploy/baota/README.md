# 🎋 MU Framework v2.3 · 宝塔面板部署指南

## 部署架构

```
┌──────────────────────────────────────────────────────────┐
│  开发商服务器（宝塔面板）                                   │
│  ┌─────────┐ ┌───────────┐ ┌──────────────┐              │
│  │api-server│ │admin-server│ │agent-engine  │ ← PM2 管理   │
│  │  :8080   │ │   :8081   │ │    :8082    │              │
│  └────┬─────┘ └─────┬─────┘ └──────┬──────┘              │
│       └──────────────┼──────────────┘                     │
│                      ▼                                    │
│  ┌────────────────────────────────┐                       │
│  │   Nginx（宝塔自动管理 + SSL）    │  ← 域名反代          │
│  └────────────────────────────────┘                       │
│  ┌──────────────┐  ┌──────────────┐                       │
│  │ PostgreSQL 16│  │  Redis 7.4   │  ← 宝塔商店安装       │
│  └──────────────┘  └──────────────┘                       │
└──────────────────────────────────────────────────────────┘

┌─────────────────────────────┐   ┌─────────────────────────┐
│  服务商服务器（宝塔）          │   │  终端客户服务器（宝塔）    │
│  Nginx + 前端静态文件         │   │  Nginx + 前端静态文件    │
│  API 反代 → 开发商服务器      │   │  API 反代 → 开发商      │
└─────────────────────────────┘   └─────────────────────────┘
```

## 环境要求

| 组件 | 版本 | 安装方式 |
|------|------|---------|
| 操作系统 | Ubuntu 24.04 LTS / CentOS 7.9+ | - |
| 宝塔面板 | 9.0+ | bt.cn |
| Go | 1.23+ | 宝塔 → 软件商店 → Go |
| Node.js | 22 LTS | 宝塔 → Node.js 版本管理 |
| PostgreSQL | 16+ | 宝塔 → 软件商店 |
| Redis | 7.4+ | 宝塔 → 软件商店 |
| Nginx | 1.24+ | 宝塔默认安装 |
| PM2 | 5.x | `npm install -g pm2` |

## 快速部署

### 开发商（全栈部署）

```bash
# 1. 克隆代码
cd /www/wwwroot
git clone https://github.com/zhongjinmuai-lang/Kiro.git mu-framework
cd mu-framework

# 2. 切换到最新分支
git checkout MU智能体族谱-v2

# 3. 运行部署脚本
bash deploy/baota/setup.sh developer
```

### 服务商 / 终端客户

```bash
cd /www/wwwroot
git clone https://github.com/zhongjinmuai-lang/Kiro.git mu-framework
cd mu-framework
git checkout MU智能体族谱-v2

# 服务商
bash deploy/baota/setup.sh provider

# 或终端客户
bash deploy/baota/setup.sh customer
```

## 目录结构

```
deploy/baota/
├── README.md                    # 本文件
├── setup.sh                     # 半自动部署脚本
└── nginx/                       # 运行 setup.sh 后生成
    ├── mu-developer.conf        # 开发商 Nginx 配置
    ├── mu-provider.conf         # 服务商 Nginx 配置
    └── mu-customer.conf         # 终端客户 Nginx 配置
```

## 安全说明

- 敏感配置（数据库密码、JWT Secret）写入 `.env.production`，权限 600
- `.env.production` 已加入 `.gitignore`，不会提交到仓库
- PM2 通过读取 `.env.production` 注入环境变量，生态配置文件无明文密码
- Nginx 配置包含 HTTPS 模板，申请证书后取消注释即可启用
- `/metrics` 端点仅允许内网访问

## 常用运维命令

```bash
# 查看服务状态
pm2 status

# 重启所有服务
pm2 restart all

# 查看日志
pm2 logs mu-api --lines 50
pm2 logs mu-agent --lines 50

# 重新部署（更新代码后）
git pull
cd backend && go build -ldflags="-s -w" -o ../bin/api-server ./cmd/api-server
cd backend && go build -ldflags="-s -w" -o ../bin/admin-server ./cmd/admin-server
cd backend && go build -ldflags="-s -w" -o ../bin/agent-engine ./cmd/agent-engine
cd frontend/admin-developer && npm run build
pm2 restart all
```

## 故障排查

| 问题 | 排查方式 |
|------|---------|
| 502 Bad Gateway | `pm2 status` 检查服务是否运行 |
| 数据库连接失败 | 检查 `.env.production` 中密码是否正确 |
| Redis 连接失败 | `redis-cli ping`，检查密码配置 |
| 前端白屏 | 检查 Nginx root 路径是否正确 |
| CORS 错误 | 检查 `MU_CORS_ALLOW_ORIGINS` 配置 |

## 相关文档

- [完整部署手册](../../docs/baota-deployment.md)
- [环境要求](../../docs/environment-requirements.md)
- [运维手册](../../docs/operations-manual.md)
