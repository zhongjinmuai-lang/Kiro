# 📦 MU Framework 安装指南

> 快速安装、部署、验证 MU 自研全能智能体主体框架

---

## 一、环境要求

| 组件 | 最低版本 | 用途 |
|------|---------|------|
| Go | 1.23+ | 后端编译 |
| PostgreSQL | 16+（推荐 18.3） | 主数据库 |
| Redis | 7.0+（推荐 7.4） | 缓存/队列/锁/限流 |
| Node.js | 20+（推荐 22 LTS） | 前端构建 |
| Docker | 27.x+ | 容器化部署（可选） |
| Nginx | 1.24+ | 反向代理（宝塔部署需要） |

---

## 二、快速安装（Docker 一键启动）

```bash
# 1. 克隆代码
git clone -b MU智能体族谱 https://github.com/zhongjinmuai-lang/Kiro.git
cd Kiro

# 2. 一键启动全部服务
cd deploy/docker
docker compose up -d

# 3. 等待 ~30s 后验证
curl http://localhost:8080/health

# 4. 访问
# 开发商:   http://localhost
# 服务商:   http://localhost:8000
# 终端客户: http://localhost:8001
# Swagger:  http://localhost:8080/swagger/index.html
```

**默认登录账号：**

| 后台 | 编码 | 用户名 | 密码 |
|------|------|--------|------|
| 开发商总后台 | `mu-platform` | `admin` | `mu_admin_2026` |
| 服务商后台 | `demo-provider` | `admin` | `mu_admin_2026` |
| 终端客户后台 | `demo-family` | `admin` | `mu_admin_2026` |

---

## 三、手动安装（宝塔面板 / 裸机）

### 3.1 安装后端

```bash
cd Kiro/backend
go env -w GOPROXY=https://goproxy.cn,direct
go mod download

mkdir -p ../bin
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/api-server   ./cmd/api-server
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/admin-server ./cmd/admin-server
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/agent-engine ./cmd/agent-engine
```

### 3.2 初始化数据库

```bash
sudo -u postgres createdb mu_framework
sudo -u postgres psql mu_framework -c 'CREATE EXTENSION IF NOT EXISTS "uuid-ossp"; CREATE EXTENSION IF NOT EXISTS "pgcrypto";'

# 按顺序执行迁移
for f in 001_init_schema 002_platform_tables 004_rls_policies 005_genealogy_tables 006_v15_enhancements 003_seed_data; do
  sudo -u postgres psql mu_framework -f backend/migrations/${f}.sql
done
```

### 3.3 启动后端

```bash
# PM2（推荐生产）
pm2 start bin/api-server --name mu-api -- --config configs/prod.yaml
pm2 start bin/admin-server --name mu-admin -- --config configs/prod.yaml
pm2 start bin/agent-engine --name mu-agent -- --config configs/prod.yaml
pm2 save && pm2 startup
```

### 3.4 构建前端

```bash
cd frontend/admin-developer && npm install && npm run build
cd ../admin-provider && npm install && npm run build
cd ../admin-customer && npm install && npm run build
```

### 3.5 Nginx 反代

详见 [`docs/baota-deployment.md`](docs/baota-deployment.md)

---

## 四、三服务器独立部署

```bash
# 开发商服务器（全栈）
cd deploy/developer && cp .env.example .env && vim .env && ./deploy.sh up

# 服务商服务器（纯前端+反代）
cd deploy/provider && cp .env.example .env && vim .env && ./deploy.sh up

# 终端客户服务器（贴牌）
cd deploy/customer && cp .env.example .env && vim .env && ./deploy.sh up
```

---

## 五、宝塔面板半自动安装

```bash
bash deploy/baota/setup.sh [developer|provider|customer]
```

---

## 六、环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MU_DATABASE_HOST` | PG 主机 | `localhost` |
| `MU_DATABASE_PASSWORD` | PG 密码 | `mu_secret_dev` |
| `MU_REDIS_ADDR` | Redis | `localhost:6379` |
| `MU_JWT_SECRET` | JWT 密钥 | 开发密钥 |
| `MU_CORS_ALLOW_ORIGINS` | CORS 白名单 | 空 |

---

## 七、验证

```bash
curl http://localhost:8080/health    # 健康
curl http://localhost:8080/ready     # 就绪
curl http://localhost:8080/version   # 版本
curl http://localhost:8080/metrics   # 监控指标
```

---

## 八、升级

```bash
git pull origin MU智能体族谱
cd backend && go build -o ../bin/api-server ./cmd/api-server
pm2 restart mu-api
```

---

## 相关文档

- [部署手册](docs/deployment.md) | [宝塔部署](docs/baota-deployment.md) | [环境要求](docs/environment-requirements.md)
- [API 规范](docs/api-conventions.md) | [架构设计](docs/architecture.md) | [插件开发](docs/plugin-development.md)
