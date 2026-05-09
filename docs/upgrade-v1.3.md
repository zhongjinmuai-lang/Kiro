# v1.3 升级说明 · 三服务器独立部署模式

## 概览

本次升级支持**三后台跨服务器独立部署**：开发商、服务商、终端客户分别运行在独立物理/云服务器上。

## 架构变更

### Before（v1.2 · 单机一体化）
```
┌─────────────────────────────────┐
│ 单台服务器                       │
│  ├── 后端 + DB + Redis           │
│  ├── :80   开发商后台            │
│  ├── :8000 服务商后台            │
│  └── :8001 终端客户后台          │
└─────────────────────────────────┘
```

### After（v1.3 · 三服务器独立）
```
┌────────────────┐   HTTPS   ┌───────────────┐
│ 🏢 开发商服务器 │ ◀──────── │ 🏪 服务商A     │
│ 后端+DB+前端   │           │ 前端+Nginx反代 │
└────────┬───────┘           └───────────────┘
         │ HTTPS
         │          ┌───────────────┐
         └────────  │ 👪 终端客户    │
                    │ 前端+反代/贴牌 │
                    └───────────────┘
```

## 新增文件

### 部署目录
- `deploy/developer/` — 开发商服务器（全栈）
  - `docker-compose.yml` / `Dockerfile.web` / `nginx.conf`
  - `.env.example` / `deploy.sh`
- `deploy/provider/` — 服务商服务器（纯前端 + 反代）
  - `docker-compose.yml` / `Dockerfile.web` / `nginx.conf.template`
  - `.env.example` / `deploy.sh`
- `deploy/customer/` — 终端客户服务器（纯前端 + 反代）
  - `docker-compose.yml` / `Dockerfile.web` / `nginx.conf.template`
  - `.env.example` / `deploy.sh`
- `deploy/README.md` — 四种部署方案总览与安全建议
- `deploy/.gitignore` — 忽略 .env 和 SSL 证书

### 前端增强
- 三个前端项目各增加 `.env.example`，支持 `VITE_API_BASE_URL` 构建参数
- `utils/request.ts` 读取 `import.meta.env.VITE_API_BASE_URL` 作为 axios baseURL
- 默认空字符串 → 走 Nginx 同域反代
- 指定 URL → 跨域直连开发商 API（需 CORS 放行）

## 后端变更

### CORS 中间件（`internal/core/middleware/cors.go`）
支持通过环境变量配置白名单：
```env
MU_CORS_ALLOW_ORIGINS=https://provider-a.com,https://family-001.com
```

优先级：
1. 设置 `MU_CORS_ALLOW_ORIGINS` → 白名单模式 + AllowCredentials=true
2. 未设置 → 通配模式（调试方便，生产应收紧）

## 跨服务器调用方案

### 方案 A：Nginx 反代（推荐，无需 CORS）

**服务商后台工作流：**
1. 用户访问 `https://provider-a.com/login`
2. 页面调用 `/api/v1/auth/login`
3. 服务商服务器 Nginx 匹配 `/api/` 反代到 `https://api.mu-developer.com/api/`
4. 响应回流，浏览器视角全程同域

**优势：** 无需 CORS、无跨域 preflight、延迟低

### 方案 B：跨域直连（灵活但需 CORS）

前端 `.env.production`:
```env
VITE_API_BASE_URL=https://api.mu-developer.com
```

开发商 `.env`:
```env
CORS_ALLOW_ORIGINS=https://provider-a.com,https://family-001.com
```

## 部署步骤

### 1. 开发商（一次性部署）

```bash
# 开发商运维在自己的服务器上
git clone <repo>
cd Kiro/deploy/developer
cp .env.example .env
# 修改：POSTGRES_PASSWORD / REDIS_PASSWORD / JWT_SECRET
# 添加：CORS_ALLOW_ORIGINS=https://provider-a.com,https://family-001.com
./deploy.sh up

# 开发商后台: http://<开发商IP>/
# 对外 API:   http://<开发商IP>/api/
```

### 2. 服务商（每个服务商独立部署）

```bash
# 服务商运维在自己的服务器上
git clone <repo>
cd Kiro/deploy/provider
cp .env.example .env
# 修改：DEVELOPER_API_URL=https://api.mu-developer.com
./deploy.sh up

# 服务商后台: http://<服务商IP>/
```

### 3. 终端客户（可贴牌定制）

```bash
cd Kiro/deploy/customer
cp .env.example .env
# 修改：DEVELOPER_API_URL=https://api.mu-developer.com
./deploy.sh up

# 终端后台: http://<终端IP>/
```

## 数据流转示例

### 场景：服务商 A 的终端客户家族登录并查看族谱

```
1. 用户访问 https://family-001.com (终端服务器)
2. 加载 admin-customer SPA
3. SPA 调用 /api/v1/auth/login
4. 终端 Nginx 反代 → https://api.mu-developer.com/api/v1/auth/login
5. 开发商后端：
   - 查找 tenant (demo-family)
   - 验证密码
   - 签发 JWT（含 tenant_id, level=customer）
6. 返回令牌，SPA 写入 localStorage
7. SPA 请求 /api/v1/genealogy/tree
8. Nginx 反代 → 开发商 API
9. 中间件：
   - JWT 验证
   - 注入 app.current_tenant_id (RLS)
10. PG 返回仅该家族数据
11. 浏览器渲染世系树
```

## 防火墙建议

| 服务器 | 开放端口 | 拒绝端口 |
|--------|---------|---------|
| 开发商 | 80/443（HTTP/HTTPS）| 5432 (PG) / 6379 (Redis) 只内网 |
| 服务商 | 80/443 | 无特殊端口 |
| 终端 | 80/443 | 无特殊端口 |

## 已知限制

1. **JWT 密钥必须一致**：开发商签发的 token 要在服务商/终端的 Nginx 反代回开发商验证，所以只需开发商那一份 JWT_SECRET。（服务商/终端不直接验签，是纯反代）
2. **WebSocket 支持**：三端 Nginx 都配置了 `/ws` 路径的 WebSocket 反代（Upgrade 头）
3. **SSL 终止**：建议在每台服务器的 Nginx 上单独配置证书（或使用 Cloudflare 等 CDN 前置）

## 回滚方案

如需回到 v1.2 单机模式：
```bash
./deploy/scripts/deploy.sh dev
```
原 `deploy/docker/docker-compose.yml` 仍保留作为单机演示。
