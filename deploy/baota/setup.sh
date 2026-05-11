#!/bin/bash
# ============================================================
# 🎋 MU Framework · 宝塔面板半自动部署脚本
# 用法：bash setup.sh [developer|provider|customer]
#
# 脚本功能：
#   1. 校验环境（Go/Node/PG/Redis 是否安装）
#   2. 交互式询问参数（域名/密码/开发商 API 等）
#   3. 自动构建前后端
#   4. 生成 Nginx 配置 + PM2 ecosystem 文件
# ============================================================
set -e

ROLE=${1:-}
COLOR_G='\033[0;32m'
COLOR_Y='\033[0;33m'
COLOR_R='\033[0;31m'
COLOR_N='\033[0m'

log()   { echo -e "${COLOR_G}[INFO ]${COLOR_N} $*"; }
warn()  { echo -e "${COLOR_Y}[WARN ]${COLOR_N} $*"; }
error() { echo -e "${COLOR_R}[ERROR]${COLOR_N} $*" >&2; }
ask()   { read -p "$(echo -e "${COLOR_Y}[ASK]${COLOR_N} $1")" -r REPLY; echo "$REPLY"; }

PROJECT_ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$PROJECT_ROOT"

if [ -z "$ROLE" ]; then
  echo "请选择部署角色："
  echo "  1) 🏢 developer  - 开发商服务器（全栈：后端+DB+Redis+开发商前端）"
  echo "  2) 🏪 provider   - 服务商服务器（仅前端+反代）"
  echo "  3) 👪 customer   - 终端客户服务器（仅前端+反代，可贴牌）"
  CHOICE=$(ask "输入序号 [1/2/3]: ")
  case $CHOICE in
    1) ROLE=developer ;;
    2) ROLE=provider ;;
    3) ROLE=customer ;;
    *) error "无效选择"; exit 1 ;;
  esac
fi

log "========================================"
log "  MU Framework · 宝塔部署 [$ROLE]"
log "========================================"

# -------------------- 通用环境检查 --------------------
check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    warn "未找到命令: $1 $2"
    return 1
  fi
  log "✓ $1 $($1 --version 2>/dev/null | head -1 || true)"
}

log "[1/6] 环境检查"
check_cmd node "（建议 22 LTS）"
check_cmd npm  ""
check_cmd nginx "（宝塔已安装）"

if [ "$ROLE" = "developer" ]; then
  check_cmd go   "（需 1.23+）"
  check_cmd psql "（PostgreSQL 客户端）"
  check_cmd redis-cli ""
fi

# -------------------- 开发商角色 --------------------
if [ "$ROLE" = "developer" ]; then
  log "[2/6] 收集参数"
  DB_PASSWORD=$(ask "PostgreSQL 数据库 mu_admin 用户密码: ")
  REDIS_PASSWORD=$(ask "Redis 密码（留空表示无密码）: ")
  JWT_SECRET=$(openssl rand -hex 32)
  log "已生成 JWT_SECRET: ${JWT_SECRET:0:16}..."
  CORS_ORIGINS=$(ask "CORS 白名单（逗号分隔服务商/终端域名，留空允许所有）: ")
  DOMAIN=$(ask "开发商后台域名（如 admin.mu-developer.com）: ")

  log "[3/6] 数据库初始化（跳过请按 Ctrl+C）"
  ask "将执行 migrations 001~005 到 mu_framework 库，继续？[Enter 继续]"
  sudo -u postgres psql -d mu_framework -c 'CREATE EXTENSION IF NOT EXISTS "uuid-ossp";' || warn "扩展可能已存在"
  sudo -u postgres psql -d mu_framework -c 'CREATE EXTENSION IF NOT EXISTS "pgcrypto";' || true
  for sql in backend/migrations/001_init_schema.sql \
             backend/migrations/002_platform_tables.sql \
             backend/migrations/004_rls_policies.sql \
             backend/migrations/005_genealogy_tables.sql \
             backend/migrations/003_seed_data.sql; do
    log "  执行 $sql"
    sudo -u postgres psql -d mu_framework -f "$sql" || warn "$sql 执行失败（可能已初始化）"
  done

  log "[4/6] 编译后端三服务"
  cd backend
  go env -w GOPROXY=https://goproxy.cn,direct
  go mod download
  mkdir -p ../bin
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/api-server   ./cmd/api-server
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/admin-server ./cmd/admin-server
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/agent-engine ./cmd/agent-engine
  chmod +x ../bin/*
  cd ..

  log "[5/6] 构建开发商前端"
  cd frontend/admin-developer
  npm config set registry https://registry.npmmirror.com
  npm install
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build
  cd ../..

  log "[6/6] 生成 PM2 + Nginx 配置"
  cat > ecosystem.config.js <<EOF
module.exports = {
  apps: [
    {
      name: 'mu-api', script: './bin/api-server', args: '--config configs/prod.yaml',
      cwd: '$PROJECT_ROOT', autorestart: true,
      env: {
        MU_APP_ENV: 'prod',
        MU_DATABASE_HOST: '127.0.0.1',
        MU_DATABASE_USER: 'mu_admin',
        MU_DATABASE_PASSWORD: '$DB_PASSWORD',
        MU_DATABASE_DBNAME: 'mu_framework',
        MU_REDIS_ADDR: '127.0.0.1:6379',
        MU_REDIS_PASSWORD: '$REDIS_PASSWORD',
        MU_JWT_SECRET: '$JWT_SECRET',
        MU_CORS_ALLOW_ORIGINS: '$CORS_ORIGINS',
      },
    },
    {
      name: 'mu-admin', script: './bin/admin-server', args: '--config configs/prod.yaml',
      cwd: '$PROJECT_ROOT', autorestart: true,
      env: {
        MU_APP_ENV: 'prod',
        MU_DATABASE_HOST: '127.0.0.1', MU_DATABASE_USER: 'mu_admin',
        MU_DATABASE_PASSWORD: '$DB_PASSWORD', MU_DATABASE_DBNAME: 'mu_framework',
        MU_REDIS_ADDR: '127.0.0.1:6379', MU_REDIS_PASSWORD: '$REDIS_PASSWORD',
        MU_JWT_SECRET: '$JWT_SECRET',
        MU_CORS_ALLOW_ORIGINS: '$CORS_ORIGINS',
      },
    },
    {
      name: 'mu-agent', script: './bin/agent-engine', args: '--config configs/prod.yaml',
      cwd: '$PROJECT_ROOT', autorestart: true,
      env: {
        MU_APP_ENV: 'prod',
        MU_DATABASE_HOST: '127.0.0.1', MU_DATABASE_USER: 'mu_admin',
        MU_DATABASE_PASSWORD: '$DB_PASSWORD', MU_DATABASE_DBNAME: 'mu_framework',
        MU_REDIS_ADDR: '127.0.0.1:6379', MU_REDIS_PASSWORD: '$REDIS_PASSWORD',
        MU_JWT_SECRET: '$JWT_SECRET',
      },
    },
  ],
}
EOF

  mkdir -p deploy/baota/nginx
  cat > deploy/baota/nginx/mu-developer.conf <<EOF
server {
    listen 80;
    server_name $DOMAIN;
    root $PROJECT_ROOT/frontend/admin-developer/dist;
    index index.html;
    access_log /www/wwwlogs/mu-developer.access.log;
    error_log  /www/wwwlogs/mu-developer.error.log;

    location / { try_files \$uri \$uri/ /index.html; }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        add_header Access-Control-Allow-Origin \$http_origin always;
        add_header Access-Control-Allow-Methods "GET,POST,PUT,PATCH,DELETE,OPTIONS" always;
        add_header Access-Control-Allow-Headers "Origin,Content-Type,Accept,Authorization,X-Tenant-ID,X-Trace-ID" always;
        add_header Access-Control-Expose-Headers "X-New-Access-Token,X-New-Refresh-Token,X-Trace-ID" always;
        if (\$request_method = OPTIONS) { return 204; }
    }
    location /admin/ { proxy_pass http://127.0.0.1:8081; proxy_set_header Host \$host; }
    location /agent/ { proxy_pass http://127.0.0.1:8082; proxy_set_header Host \$host; }
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
    }
}
EOF

  log "✅ 开发商部署就绪"
  log "  下一步手动操作："
  log "  1. 启动后端:  pm2 start ecosystem.config.js && pm2 save"
  log "  2. 配置 Nginx: 将 deploy/baota/nginx/mu-developer.conf 复制到宝塔站点配置"
  log "  3. 宝塔面板申请 Let's Encrypt SSL 证书"
  log "  4. 访问 http://$DOMAIN （默认账号 mu-platform/admin/admin123）"
  log "  ⚠️ 立即修改默认密码！"

# -------------------- 服务商/终端角色 --------------------
elif [ "$ROLE" = "provider" ] || [ "$ROLE" = "customer" ]; then
  if [ "$ROLE" = "provider" ]; then
    FRONTEND_DIR="frontend/admin-provider"
    DEFAULT_DOMAIN="provider-a.example.com"
  else
    FRONTEND_DIR="frontend/admin-customer"
    DEFAULT_DOMAIN="family-001.example.com"
  fi

  log "[2/4] 收集参数"
  DEVELOPER_API=$(ask "开发商 API 地址（含协议，如 https://api.mu-developer.com）: ")
  DOMAIN=$(ask "本站域名（默认 $DEFAULT_DOMAIN）: ")
  DOMAIN=${DOMAIN:-$DEFAULT_DOMAIN}

  log "[3/4] 构建前端"
  cd "$FRONTEND_DIR"
  npm config set registry https://registry.npmmirror.com
  npm install
  # Nginx 反代模式：VITE_API_BASE_URL 留空
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build
  cd "$PROJECT_ROOT"

  log "[4/4] 生成 Nginx 配置"
  mkdir -p deploy/baota/nginx
  CONF_FILE="deploy/baota/nginx/mu-$ROLE.conf"
  cat > "$CONF_FILE" <<EOF
server {
    listen 80;
    server_name $DOMAIN;
    root $PROJECT_ROOT/$FRONTEND_DIR/dist;
    index index.html;
    access_log /www/wwwlogs/mu-$ROLE.access.log;
    error_log  /www/wwwlogs/mu-$ROLE.error.log;

    location / { try_files \$uri \$uri/ /index.html; }

    location /api/ {
        proxy_pass $DEVELOPER_API;
        proxy_http_version 1.1;
        proxy_set_header Host ${DEVELOPER_API#*://};
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_ssl_server_name on;
        proxy_pass_header X-New-Access-Token;
        proxy_pass_header X-New-Refresh-Token;
        proxy_pass_header X-Trace-ID;
    }
    location /admin/ {
        proxy_pass $DEVELOPER_API;
        proxy_http_version 1.1;
        proxy_set_header Host ${DEVELOPER_API#*://};
        proxy_ssl_server_name on;
        proxy_pass_header X-New-Access-Token;
    }
    location /ws {
        proxy_pass $DEVELOPER_API;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_ssl_server_name on;
        proxy_read_timeout 3600s;
    }
}
EOF

  log "✅ $ROLE 前端部署就绪"
  log "  产物目录: $PROJECT_ROOT/$FRONTEND_DIR/dist"
  log "  Nginx 配置: $CONF_FILE"
  log "  下一步："
  log "  1. 宝塔 → 网站 → 添加站点 → 根目录填 $PROJECT_ROOT/$FRONTEND_DIR/dist"
  log "  2. 将 $CONF_FILE 内容复制到站点 Nginx 配置"
  log "  3. 申请 Let's Encrypt SSL"
  log "  4. 访问 http://$DOMAIN"

else
  error "未知角色: $ROLE"
  exit 1
fi

log "========================================"
log "  部署完成 🎉"
log "========================================"
