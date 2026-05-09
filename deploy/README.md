# MU Framework 部署方案

支持**三种部署模式**，按业务规模自由选择：

## 📐 部署拓扑

```
                         🌐 Internet
                              │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
    ┌────▼────────┐    ┌────▼────────┐    ┌────▼──────────┐
    │ 🏢 开发商     │    │ 🏪 服务商 A  │    │ 👪 终端 A     │
    │ 服务器       │    │ 服务器       │    │ 服务器（贴牌） │
    │              │    │              │    │                │
    │ PG + Redis   │    │ Nginx 反代   │    │ Nginx 反代     │
    │ 后端 3服务   │◀───│ admin-provider◀───│ admin-customer │
    │ admin-developer  │ 前端(SPA)    │    │ 前端(SPA)      │
    │              │    └─────────────┘    └────────────────┘
    │              │
    │              │    ┌─────────────┐
    │              │◀───│ 🏪 服务商 B  │
    │              │    │ 服务器       │ ...（可横向扩展多服务商）
    └──────────────┘    └─────────────┘
```

**核心原则：**
- **开发商服务器**：承载全部业务逻辑（后端 + 数据库）+ 自家前端
- **服务商服务器**：只部署前端 SPA + Nginx 反代，无业务代码
- **终端客户服务器**：同服务商，纯 SPA + 反代（支持贴牌定制）
- 所有业务请求通过 HTTPS 跨服务器调用开发商 API
- 数据完全集中，通过 PG RLS 做多租户隔离

---

## 🏢 方案一：开发商服务器部署

**目录：** `deploy/developer/`

**部署内容：**
- PostgreSQL 18.3 + Redis 7.4
- 后端三服务（API/Admin/Agent Server）
- 开发商前端（admin-developer）
- Nginx（对外暴露 /api、/admin、/agent，含 CORS）

**步骤：**
```bash
cd deploy/developer
cp .env.example .env
vim .env                          # 修改密码和 CORS 白名单
./deploy.sh up
```

**访问：**
- 开发商后台：http://<你的服务器IP>/
- 对外 API：http://<你的服务器IP>/api/ （供服务商、终端调用）

---

## 🏪 方案二：服务商服务器独立部署

**目录：** `deploy/provider/`

**部署内容：**
- 仅 admin-provider 前端 SPA
- Nginx 反代到开发商 API

**步骤：**
```bash
# 在服务商自己的服务器上
git clone https://github.com/zhongjinmuai-lang/Kiro.git
cd Kiro/deploy/provider
cp .env.example .env
vim .env
#   DEVELOPER_API_URL=https://api.mu-developer.com   👈 填开发商的 API 地址
./deploy.sh up
```

**访问：**
- 服务商后台：http://<服务商IP>/
- 所有 API 请求由 Nginx 反代到开发商服务器

**配置模式（二选一）：**
- **Nginx 反代（推荐）**：`VITE_API_BASE_URL=` 留空，走同域反代，无需跨域
- **跨域直连**：`VITE_API_BASE_URL=https://api.mu-developer.com`，需开发商 CORS 放行

---

## 👪 方案三：终端客户服务器（或贴牌）部署

**目录：** `deploy/customer/`

**部署内容：** 与服务商相同，只是前端换成 admin-customer。

**步骤：**
```bash
cd Kiro/deploy/customer
cp .env.example .env
vim .env
#   DEVELOPER_API_URL=https://api.mu-developer.com
./deploy.sh up
```

---

## 🎋 方案四：宝塔面板手动部署（无 Docker）

**目录：** `deploy/baota/`

适合不使用 Docker、习惯宝塔面板管理的用户。

**步骤：**
```bash
# 在宝塔所在服务器上
cd /www/wwwroot/mu-framework
bash deploy/baota/setup.sh [developer|provider|customer]
```

脚本会交互式完成：
- 环境检查（Go 1.23+ / Node 22 / PG 18 / Redis 7.4 / Nginx）
- 数据库迁移（仅 developer 角色）
- 编译后端 + 构建前端
- 生成 PM2 ecosystem.config.js + Nginx 站点配置

**详细手册：**
- 📗 [`docs/baota-deployment.md`](../docs/baota-deployment.md) - 完整部署步骤
- 📘 [`docs/environment-requirements.md`](../docs/environment-requirements.md) - 环境要求清单

---

## 🔁 方案五：单机一体化（演示/开发）

**目录：** `deploy/docker/`

**部署内容：** 三个前端 + 后端全部部署在同一台机器上，80/8000/8001 三端口分别对应。

```bash
cd deploy/docker
docker compose up -d
```

**访问：**
- http://localhost       → 开发商后台
- http://localhost:8000  → 服务商后台
- http://localhost:8001  → 终端客户后台

---

## 🔒 安全建议

### 1. JWT 密钥（`JWT_SECRET`）
所有三个 `.env` 文件中的 `JWT_SECRET` **必须一致**（因为 token 由开发商签发后，跨服务器验证）。
生成命令：
```bash
openssl rand -hex 32
```

### 2. CORS 白名单
生产环境务必设置 `CORS_ALLOW_ORIGINS`：
```env
CORS_ALLOW_ORIGINS=https://provider-a.com,https://provider-b.com,https://family-001.com
```

### 3. HTTPS
三种方案的 Nginx 都预留了 443 端口和 `ssl/` 目录，推荐：
- Let's Encrypt 自动签发（cert-manager 或 certbot）
- 将 `fullchain.pem` 与 `privkey.pem` 放入 `ssl/` 目录
- 取消 nginx.conf 中 HTTPS server 块的注释

### 4. 数据库加固
- 生产环境 `POSTGRES_PASSWORD` 至少 16 位随机字符
- 将 5432 端口**不暴露**到公网（只在 docker 内部网络）

### 5. Redis 加固
- 设置强密码 `REDIS_PASSWORD`
- 不对外暴露 6379 端口

---

## 🧰 运维命令速查

每个方案目录下的 `deploy.sh` 都支持：
```bash
./deploy.sh up          # 启动
./deploy.sh stop        # 停止
./deploy.sh logs [svc]  # 查看日志
./deploy.sh restart     # 重启
./deploy.sh clean       # 清理（含数据卷，慎用）
```

---

## ☸️ Kubernetes 部署

见 `deploy/k8s/` 目录，生产集群部署参考。
