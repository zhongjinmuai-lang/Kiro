# 🎋 MU Framework 宝塔面板手动部署指南

> 适合不使用 Docker 的传统部署场景 —— 在宝塔面板（BT Panel）上手动安装配置。
> 三个后台（开发商 / 服务商 / 终端客户）均支持独立部署到不同服务器。

## 📋 环境要求

### 服务器基础环境

| 组件 | 版本要求 | 说明 |
|------|---------|------|
| 操作系统 | Ubuntu 22.04/24.04 LTS、CentOS 7.9+、Debian 11+、Alma Linux 9+ | 推荐 Ubuntu 24.04 |
| CPU | ≥ 2 核（开发商服务器建议 ≥ 4 核） | 服务商/终端仅前端，≥ 1 核即可 |
| 内存 | ≥ 4 GB（开发商建议 ≥ 8 GB） | 服务商/终端 ≥ 2 GB 即可 |
| 磁盘 | ≥ 40 GB SSD（开发商建议 ≥ 100 GB） | 含系统 + 数据库 + 日志 |
| 宝塔面板 | 9.x+ 或开心版 9.x+ | https://www.bt.cn/ |
| 公网带宽 | ≥ 2 Mbps | HTTPS 推荐 ≥ 5 Mbps |

### 宝塔面板需安装的软件

| 服务器角色 | 必装 | 可选 |
|-----------|------|------|
| 🏢 开发商服务器 | Nginx 1.24+、PostgreSQL 18.3、Redis 7.4+、Go 1.23+、Node.js 22 LTS、PM2 | Supervisor |
| 🏪 服务商服务器 | Nginx 1.24+、Node.js 22 LTS | — |
| 👪 终端客户服务器 | Nginx 1.24+、Node.js 22 LTS | — |

---

## 🏢 方案一：开发商服务器部署（全栈）

### 第一步：宝塔面板软件安装

在宝塔面板"**软件商店**"中依次安装：

| 软件 | 推荐版本 | 备注 |
|------|---------|------|
| Nginx | 1.24+ | 反向代理 + 托管前端 |
| PostgreSQL | 18.3（若宝塔暂无，用 16.x 兼容） | 数据库 |
| Redis | 7.4+ | 缓存/会话/限流/分布式锁 |
| PM2 管理器 | 最新 | 管理 Go 后端进程 |
| Node.js 版本管理器 | 最新 | 安装 Node.js 22 LTS |

> **Go 环境手动安装**（宝塔面板无 Go）：
> ```bash
> cd /usr/local
> wget https://go.dev/dl/go1.23.5.linux-amd64.tar.gz
> tar -C /usr/local -xzf go1.23.5.linux-amd64.tar.gz
> echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
> source /etc/profile
> go version   # 验证 go1.23.5
> ```

### 第二步：PostgreSQL 配置

1. 宝塔 → PostgreSQL → 创建数据库：
   - 数据库名：`mu_framework`
   - 用户名：`mu_admin`
   - 密码：强密码（记住）
   - 访问权限：本地

2. 启用必需扩展：
   ```bash
   sudo -u postgres psql mu_framework <<EOF
   CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
   CREATE EXTENSION IF NOT EXISTS "pgcrypto";
   EOF
   ```

3. 导入迁移脚本（按顺序执行）：
   ```bash
   cd /www/wwwroot/mu-framework/backend/migrations
   sudo -u postgres psql mu_framework -f 001_init_schema.sql
   sudo -u postgres psql mu_framework -f 002_platform_tables.sql
   sudo -u postgres psql mu_framework -f 004_rls_policies.sql
   sudo -u postgres psql mu_framework -f 005_genealogy_tables.sql
   sudo -u postgres psql mu_framework -f 003_seed_data.sql
   ```

### 第三步：Redis 配置

宝塔 → Redis → 配置修改：
```conf
requirepass YOUR_STRONG_REDIS_PASSWORD
bind 127.0.0.1
appendonly yes
```
保存并重启 Redis。

### 第四步：上传代码 + 编译后端

```bash
cd /www/wwwroot
git clone -b MuAgent-zupu https://github.com/zhongjinmuai-lang/Kiro.git mu-framework
cd mu-framework/backend

# 国内代理加速
go env -w GOPROXY=https://goproxy.cn,direct
go mod download

# 编译三个服务
mkdir -p ../bin
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/api-server     ./cmd/api-server
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/admin-server   ./cmd/admin-server
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/agent-engine   ./cmd/agent-engine
chmod +x ../bin/*
```

### 第五步：PM2 启动后端三服务

推荐使用 `ecosystem.config.js`（见下方"高级配置"），或简易启动：

```bash
cd /www/wwwroot/mu-framework

# 创建环境变量文件
cat > .env.prod <<'EOF'
MU_APP_ENV=prod
MU_DATABASE_HOST=127.0.0.1
MU_DATABASE_USER=mu_admin
MU_DATABASE_PASSWORD=YOUR_DB_PASSWORD
MU_DATABASE_DBNAME=mu_framework
MU_REDIS_ADDR=127.0.0.1:6379
MU_REDIS_PASSWORD=YOUR_REDIS_PASSWORD
MU_JWT_SECRET=PLACEHOLDER_CHANGE_ME
MU_CORS_ALLOW_ORIGINS=https://provider-a.example.com,https://customer.example.com
EOF

# 生成真实的 JWT 密钥
sed -i "s|PLACEHOLDER_CHANGE_ME|$(openssl rand -hex 32)|" .env.prod

# 安装 PM2 并启动
npm i -g pm2
pm2 start bin/api-server     --name mu-api    -- --config configs/prod.yaml
pm2 start bin/admin-server   --name mu-admin  -- --config configs/prod.yaml
pm2 start bin/agent-engine   --name mu-agent  -- --config configs/prod.yaml

# 开机自启
pm2 save && pm2 startup
```

### 第六步：构建开发商前端

```bash
cd /www/wwwroot/mu-framework/frontend/admin-developer

npm config set registry https://registry.npmmirror.com
npm install

# 同机部署 API_BASE_URL 留空（走 Nginx 反代）
echo 'VITE_API_BASE_URL=' > .env.production
npm run build
# 产物在 dist/
```

### 第七步：宝塔创建网站并配置 Nginx

1. 宝塔 → 网站 → 添加站点：
   - 域名：`admin.mu-developer.com`
   - 根目录：`/www/wwwroot/mu-framework/frontend/admin-developer/dist`

2. 配置文件：

```nginx
server {
    listen 80;
    server_name admin.mu-developer.com;

    root /www/wwwroot/mu-framework/frontend/admin-developer/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # 反代后端 API
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # CORS（开放给服务商/终端跨域调用）
        add_header Access-Control-Allow-Origin $http_origin always;
        add_header Access-Control-Allow-Methods "GET,POST,PUT,PATCH,DELETE,OPTIONS" always;
        add_header Access-Control-Allow-Headers "Origin,Content-Type,Accept,Authorization,X-Tenant-ID,X-Trace-ID" always;
        add_header Access-Control-Expose-Headers "X-New-Access-Token,X-New-Refresh-Token,X-Trace-ID" always;
        if ($request_method = OPTIONS) { return 204; }
    }

    location /admin/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /agent/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_set_header Host $host;
    }

    # WebSocket（站内信）
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
    }

    access_log /www/wwwlogs/mu-developer.access.log;
    error_log  /www/wwwlogs/mu-developer.error.log;
}
```

3. 站点 → SSL → **Let's Encrypt** 免费证书 → 自动续签。

### 第八步：放行防火墙端口

宝塔 → 安全 → 开放端口：
- `80` HTTP、`443` HTTPS

**禁止对外开放**：
- `5432` PostgreSQL
- `6379` Redis
- `8080/8081/8082` 后端端口（由 Nginx 内部反代）

---

## 🏪 方案二：服务商服务器部署（纯前端 + 反代）

### 第一步：宝塔软件安装
```
Nginx 1.24+     （必装）
Node.js 22 LTS  （必装，构建用）
```

### 第二步：上传代码并构建

```bash
cd /www/wwwroot
git clone -b MuAgent-zupu https://github.com/zhongjinmuai-lang/Kiro.git mu-provider
cd mu-provider/frontend/admin-provider

# 关键：空值走 Nginx 反代（推荐，无跨域）
echo 'VITE_API_BASE_URL=' > .env.production

npm config set registry https://registry.npmmirror.com
npm install
npm run build
```

### 第三步：宝塔网站 + Nginx 配置

1. 宝塔 → 网站 → 添加站点：
   - 域名：`provider-a.example.com`
   - 根目录：`/www/wwwroot/mu-provider/frontend/admin-provider/dist`

2. Nginx 配置（反代到开发商 API）：

```nginx
server {
    listen 80;
    server_name provider-a.example.com;

    root /www/wwwroot/mu-provider/frontend/admin-provider/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # 👇 改为真实开发商 API 地址
    set $dev_api "https://api.mu-developer.com";

    location /api/ {
        proxy_pass $dev_api;
        proxy_http_version 1.1;
        proxy_set_header Host api.mu-developer.com;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_ssl_server_name on;
        proxy_pass_header X-New-Access-Token;
        proxy_pass_header X-New-Refresh-Token;
        proxy_pass_header X-Trace-ID;
    }

    location /admin/ {
        proxy_pass $dev_api;
        proxy_http_version 1.1;
        proxy_set_header Host api.mu-developer.com;
        proxy_ssl_server_name on;
        proxy_pass_header X-New-Access-Token;
    }

    # WebSocket
    location /ws {
        proxy_pass $dev_api;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_ssl_server_name on;
        proxy_read_timeout 3600s;
    }

    access_log /www/wwwlogs/mu-provider.access.log;
    error_log  /www/wwwlogs/mu-provider.error.log;
}
```

3. 一键开启 SSL（Let's Encrypt）。

### 第四步：测试登录

访问 `https://provider-a.example.com/`，使用开发商开通的服务商账号登录：
- 服务商编码：由开发商通过"新增服务商"分配
- 用户名/密码：开发商开通时设置

---

## 👪 方案三：终端客户服务器部署（可贴牌）

**步骤与服务商完全相同**，仅需替换：
- 仓库路径：`frontend/admin-customer`
- 域名：`family-001.example.com` 或贴牌域名 `zupu.zhangshi.com`
- 站点根目录：`/www/wwwroot/mu-customer/frontend/admin-customer/dist`

Nginx 配置改为 `mu-customer` 路径，其他完全一致。

---

## 🔧 高级配置

### PM2 生态配置（推荐）

`/www/wwwroot/mu-framework/ecosystem.config.js`：

```javascript
module.exports = {
  apps: [
    {
      name: 'mu-api',
      script: './bin/api-server',
      args: '--config configs/prod.yaml',
      cwd: '/www/wwwroot/mu-framework',
      env: {
        MU_APP_ENV: 'prod',
        MU_DATABASE_HOST: '127.0.0.1',
        MU_DATABASE_USER: 'mu_admin',
        MU_DATABASE_PASSWORD: 'YOUR_DB_PASSWORD',
        MU_DATABASE_DBNAME: 'mu_framework',
        MU_REDIS_ADDR: '127.0.0.1:6379',
        MU_REDIS_PASSWORD: 'YOUR_REDIS_PASSWORD',
        MU_JWT_SECRET: 'YOUR_32_CHAR_RANDOM_SECRET',
        MU_CORS_ALLOW_ORIGINS: 'https://provider-a.example.com,https://family-001.example.com',
      },
      autorestart: true,
      max_restarts: 10,
      log_date_format: 'YYYY-MM-DD HH:mm:ss',
    },
    {
      name: 'mu-admin',
      script: './bin/admin-server',
      args: '--config configs/prod.yaml',
      cwd: '/www/wwwroot/mu-framework',
      env: { /* 同上 */ },
      autorestart: true,
    },
    {
      name: 'mu-agent',
      script: './bin/agent-engine',
      args: '--config configs/prod.yaml',
      cwd: '/www/wwwroot/mu-framework',
      env: { /* 同上 */ },
      autorestart: true,
    },
  ],
}
```

启动：
```bash
pm2 start ecosystem.config.js
pm2 save && pm2 startup
```

### Supervisor（替代 PM2）

宝塔 → 软件商店 → **Supervisor 管理器**：

```ini
[program:mu-api]
command=/www/wwwroot/mu-framework/bin/api-server --config /www/wwwroot/mu-framework/configs/prod.yaml
directory=/www/wwwroot/mu-framework
autostart=true
autorestart=true
user=www
environment=MU_JWT_SECRET="xxx",MU_DATABASE_PASSWORD="xxx",MU_REDIS_PASSWORD="xxx"
stdout_logfile=/www/wwwlogs/mu-api.log
stderr_logfile=/www/wwwlogs/mu-api.err.log
```

### 日志轮转（logrotate）

`/etc/logrotate.d/mu-framework`：
```
/www/wwwlogs/mu-*.log {
    daily
    rotate 30
    compress
    missingok
    notifempty
    create 640 www www
}
```

---

## 🔒 安全加固清单

| 项目 | 动作 |
|------|------|
| 修改默认密码 | 数据库、Redis、admin 账号 |
| 启用 HTTPS | 宝塔 Let's Encrypt 一键 |
| 禁用 PG 外网 | `postgresql.conf`: `listen_addresses='localhost'` |
| 禁用 Redis 外网 | `redis.conf`: `bind 127.0.0.1` |
| Nginx 限流 | 添加 `limit_req_zone` 指令 |
| 宝塔面板加固 | 改默认端口、IP 白名单、2FA |
| JWT 密钥 | `openssl rand -hex 32` |
| 日志轮转 | logrotate 30 天保留 |
| 定时备份 | 宝塔计划任务：PG 全量 + 代码快照 |

---

## 🛠️ 常见问题 FAQ

### Q1：宝塔找不到 PostgreSQL 18
手动安装：
```bash
sudo apt install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh
sudo apt install -y postgresql-18 postgresql-contrib-18
```

### Q2：Go 编译慢或报错
```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=off
```

### Q3：npm install 卡住
```bash
npm config set registry https://registry.npmmirror.com
# 或用 pnpm
npm i -g pnpm && pnpm install
```

### Q4：PG RLS 策略不生效
- 确认迁移 `004_rls_policies.sql` 已执行
- 应用代码通过 `set_config('app.current_tenant_id', ...)` 注入租户

### Q5：前端刷新 404
Nginx 必须有 `try_files $uri $uri/ /index.html;` 以支持 SPA History 模式。

### Q6：跨服务器调用报 401/CORS
1. 检查开发商 `.env` 中 `MU_CORS_ALLOW_ORIGINS` 是否包含服务商域名
2. 推荐使用 Nginx 反代模式（`VITE_API_BASE_URL=` 留空）

### Q7：PM2 启动后无法访问 8080 端口
- `netstat -tlnp | grep 8080` 检查监听
- `pm2 logs mu-api` 查日志
- 检查 `configs/prod.yaml` 中 `server.port: 8080`

---

## 📜 部署后验证

```bash
# 1. 后端三服务 online
pm2 list
#  mu-api / mu-admin / mu-agent  均应为 online

# 2. 依赖就绪
curl http://127.0.0.1:8080/ready
# {"code":0,"data":{"ready":true},...}

# 3. 版本信息
curl http://127.0.0.1:8080/version

# 4. 登录测试
curl -X POST http://127.0.0.1:8081/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"tenant_code":"mu-platform","username":"admin","password":"mu_admin_2026"}'
# 应返回 access_token
```

成功后：
- 🏢 开发商后台：https://admin.mu-developer.com
- 🏪 服务商后台：https://provider-a.example.com
- 👪 终端客户后台：https://family-001.example.com

**⚠️ 修改所有默认密码！**

---

## 📦 配合脚本工具

`scripts/baota-setup.sh` 提供交互式半自动安装：

```bash
cd /www/wwwroot/mu-framework
bash deploy/baota/setup.sh [developer|provider|customer]
```

脚本会：
- 校验环境（Go / Node / PG / Redis）
- 询问参数（域名、密码、开发商 API）
- 自动构建 + 生成 Nginx 配置 + PM2 启动
