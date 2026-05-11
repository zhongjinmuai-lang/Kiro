#!/bin/bash
# ============================================================
# 🎋 MU Framework v2.3 · 宝塔面板部署脚本（中国环境优化）
# 用法：bash setup.sh [developer|provider|customer]
#
# v2.3 更新：
#   - 修复 migration 执行顺序（严格按编号 001→006）
#   - 敏感信息写入 .env 文件而非明文 ecosystem.config.js
#   - 新增 HTTPS/SSL 自动化 Nginx 模板
#   - 国内镜像加速全覆盖（Go/npm/Docker）
#   - 新增 v2.3 agent-engine 部署支持
#   - 宝塔面板兼容性优化（Node 22 LTS / Go 1.23+）
# ============================================================
set -e

ROLE=${1:-}
SCRIPT_VERSION="2.3.0"
COLOR_G='\033[0;32m'
COLOR_Y='\033[0;33m'
COLOR_R='\033[0;31m'
COLOR_B='\033[0;34m'
COLOR_N='\033[0m'

log()   { echo -e "${COLOR_G}[INFO ]${COLOR_N} $*"; }
warn()  { echo -e "${COLOR_Y}[WARN ]${COLOR_N} $*"; }
error() { echo -e "${COLOR_R}[ERROR]${COLOR_N} $*" >&2; }
title() { echo -e "${COLOR_B}[=====]${COLOR_N} $*"; }
ask()   { read -p "$(echo -e "${COLOR_Y}[ASK]${COLOR_N} $1")" -r REPLY; echo "$REPLY"; }

PROJECT_ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$PROJECT_ROOT"

title "MU Framework v${SCRIPT_VERSION} · 宝塔面板部署"
echo ""

# -------------------- 角色选择 --------------------
if [ -z "$ROLE" ]; then
  echo "请选择部署角色："
  echo "  1) 🏢 developer  - 开发商服务器（全栈：后端+DB+Redis+前端）"
  echo "  2) 🏪 provider   - 服务商服务器（前端+反代到开发商API）"
  echo "  3) 👪 customer   - 终端客户服务器（前端+反代，可贴牌）"
  echo ""
  CHOICE=$(ask "输入序号 [1/2/3]: ")
  case $CHOICE in
    1) ROLE=developer ;;
    2) ROLE=provider ;;
    3) ROLE=customer ;;
    *) error "无效选择"; exit 1 ;;
  esac
fi

log "部署角色: $ROLE"
log "项目根目录: $PROJECT_ROOT"
echo ""

# -------------------- 通用环境检查 --------------------
check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    warn "未找到命令: $1 $2"
    return 1
  fi
  log "✓ $1 已安装: $($1 --version 2>/dev/null | head -1 || echo 'unknown')"
  return 0
}

title "[1] 环境检查"
check_cmd node "（建议 22 LTS，宝塔 → 软件商店 → Node.js 版本管理器）" || true
check_cmd npm  "" || true
check_cmd nginx "（宝塔已内置）" || true

if [ "$ROLE" = "developer" ]; then
  check_cmd go   "（需 1.23+，宝塔 → 软件商店 → Go 语言）" || true
  check_cmd psql "（PostgreSQL 客户端）" || true
  check_cmd redis-cli "（Redis 7.4+）" || true
fi

# 国内镜像加速配置
log "配置国内镜像加速..."
npm config set registry https://registry.npmmirror.com 2>/dev/null || true
if command -v go &>/dev/null; then
  go env -w GOPROXY=https://goproxy.cn,direct 2>/dev/null || true
  go env -w GOSUMDB=sum.golang.google.cn 2>/dev/null || true
fi
log "✓ npm 源: npmmirror | Go 代理: goproxy.cn"
echo ""

# ==================== 开发商角色 ====================
if [ "$ROLE" = "developer" ]; then

  title "[2] 收集部署参数"
  DOMAIN=$(ask "开发商后台域名（如 admin.example.com）: ")
  DB_NAME=$(ask "数据库名称（默认 mu_framework）: ")
  DB_NAME=${DB_NAME:-mu_framework}
  DB_USER=$(ask "数据库用户名（默认 mu_admin）: ")
  DB_USER=${DB_USER:-mu_admin}
  DB_PASSWORD=$(ask "PostgreSQL 密码: ")
  REDIS_PASSWORD=$(ask "Redis 密码（留空表示无密码）: ")
  CORS_ORIGINS=$(ask "CORS 白名单（逗号分隔，留空允许所有）: ")

  # 自动生成 JWT Secret
  JWT_SECRET=$(openssl rand -hex 32)
  log "已自动生成 JWT_SECRET"

  # 写入 .env 文件（不纳入版本控制）
  title "[3] 生成环境配置"
  ENV_FILE="$PROJECT_ROOT/.env.production"
  cat > "$ENV_FILE" <<EOF
# MU Framework 生产环境配置（由 setup.sh 自动生成）
# ⚠️ 请勿提交到 Git！已在 .gitignore 中排除

# 数据库
MU_DATABASE_HOST=127.0.0.1
MU_DATABASE_PORT=5432
MU_DATABASE_USER=${DB_USER}
MU_DATABASE_PASSWORD=${DB_PASSWORD}
MU_DATABASE_DBNAME=${DB_NAME}
MU_DATABASE_SSLMODE=disable

# Redis
MU_REDIS_ADDR=127.0.0.1:6379
MU_REDIS_PASSWORD=${REDIS_PASSWORD}

# JWT
MU_JWT_SECRET=${JWT_SECRET}

# CORS
MU_CORS_ALLOW_ORIGINS=${CORS_ORIGINS}

# 应用
MU_APP_ENV=prod
MU_SERVER_PORT=8080
MU_SERVER_ADMIN_PORT=8081
MU_SERVER_AGENT_PORT=8082
EOF
  chmod 600 "$ENV_FILE"
  log "✓ 环境配置已写入 $ENV_FILE（权限 600）"

  # 确保 .gitignore 排除 .env
  if ! grep -q ".env.production" .gitignore 2>/dev/null; then
    echo ".env.production" >> .gitignore
  fi

  title "[4] 数据库初始化"
  ask "将执行 migrations 到 ${DB_NAME} 库，按 Enter 继续（Ctrl+C 跳过）..."

  # 创建数据库和用户（如果不存在）
  sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME};" 2>/dev/null || warn "数据库可能已存在"
  sudo -u postgres psql -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASSWORD}';" 2>/dev/null || warn "用户可能已存在"
  sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};" 2>/dev/null || true
  sudo -u postgres psql -d "${DB_NAME}" -c 'CREATE EXTENSION IF NOT EXISTS "uuid-ossp";' 2>/dev/null || true
  sudo -u postgres psql -d "${DB_NAME}" -c 'CREATE EXTENSION IF NOT EXISTS "pgcrypto";' 2>/dev/null || true

  # 严格按编号顺序执行 migrations
  log "按顺序执行数据库迁移..."
  for sql in backend/migrations/001_init_schema.sql \
             backend/migrations/002_platform_tables.sql \
             backend/migrations/003_seed_data.sql \
             backend/migrations/004_rls_policies.sql \
             backend/migrations/005_genealogy_tables.sql \
             backend/migrations/006_v15_enhancements.sql; do
    if [ -f "$sql" ]; then
      log "  执行 $sql"
      sudo -u postgres psql -d "${DB_NAME}" -f "$sql" 2>/dev/null || warn "  $sql 执行失败（可能已初始化）"
    else
      warn "  跳过不存在的文件: $sql"
    fi
  done
  log "✓ 数据库迁移完成"

  title "[5] 编译后端服务"
  cd backend
  log "下载依赖（使用 goproxy.cn 加速）..."
  go mod download
  mkdir -p ../bin

  log "编译 api-server..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=${SCRIPT_VERSION}" -o ../bin/api-server ./cmd/api-server

  log "编译 admin-server..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=${SCRIPT_VERSION}" -o ../bin/admin-server ./cmd/admin-server

  log "编译 agent-engine..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=${SCRIPT_VERSION}" -o ../bin/agent-engine ./cmd/agent-engine

  chmod +x ../bin/*
  cd ..
  log "✓ 后端编译完成: bin/{api-server, admin-server, agent-engine}"

  title "[6] 构建开发商前端"
  cd frontend/admin-developer
  npm install
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build
  cd "$PROJECT_ROOT"
  log "✓ 前端构建完成: frontend/admin-developer/dist/"

  title "[7] 生成 PM2 配置"
  cat > ecosystem.config.js <<'EOF'
// MU Framework PM2 配置
// 环境变量从 .env.production 加载
const fs = require('fs');
const path = require('path');

// 读取 .env.production
function loadEnv() {
  const envFile = path.join(__dirname, '.env.production');
  const env = {};
  if (fs.existsSync(envFile)) {
    fs.readFileSync(envFile, 'utf8').split('\n').forEach(line => {
      line = line.trim();
      if (line && !line.startsWith('#')) {
        const [key, ...val] = line.split('=');
        env[key.trim()] = val.join('=').trim();
      }
    });
  }
  return env;
}

const env = loadEnv();

module.exports = {
  apps: [
    {
      name: 'mu-api',
      script: './bin/api-server',
      args: '--config configs/prod.yaml',
      cwd: __dirname,
      autorestart: true,
      max_restarts: 10,
      restart_delay: 5000,
      env: env,
    },
    {
      name: 'mu-admin',
      script: './bin/admin-server',
      args: '--config configs/prod.yaml',
      cwd: __dirname,
      autorestart: true,
      max_restarts: 10,
      restart_delay: 5000,
      env: env,
    },
    {
      name: 'mu-agent',
      script: './bin/agent-engine',
      args: '--config configs/prod.yaml',
      cwd: __dirname,
      autorestart: true,
      max_restarts: 10,
      restart_delay: 5000,
      env: env,
    },
  ],
};
EOF
  log "✓ PM2 配置已生成（从 .env.production 读取密钥，无明文泄露）"

  title "[8] 生成 Nginx 配置（含 HTTPS 模板）"
  mkdir -p deploy/baota/nginx

  # HTTP 配置（宝塔申请 SSL 后自动添加 HTTPS）
  cat > deploy/baota/nginx/mu-developer.conf <<EOF
# MU Framework 开发商 Nginx 配置
# 宝塔面板 → 网站 → 设置 → 配置文件 → 粘贴以下内容

# HTTP → HTTPS 强制跳转（申请 SSL 证书后取消注释）
# server {
#     listen 80;
#     server_name ${DOMAIN};
#     return 301 https://\$host\$request_uri;
# }

server {
    listen 80;
    # listen 443 ssl http2;  # 申请 SSL 后取消注释
    server_name ${DOMAIN};

    # SSL 证书（宝塔 Let's Encrypt 自动管理）
    # ssl_certificate    /www/server/panel/vhost/cert/${DOMAIN}/fullchain.pem;
    # ssl_certificate_key /www/server/panel/vhost/cert/${DOMAIN}/privkey.pem;
    # ssl_protocols TLSv1.2 TLSv1.3;
    # ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:HIGH:!aNULL:!MD5:!RC4:!DHE;
    # ssl_prefer_server_ciphers on;
    # ssl_session_cache shared:SSL:10m;
    # ssl_session_timeout 10m;

    root ${PROJECT_ROOT}/frontend/admin-developer/dist;
    index index.html;

    access_log /www/wwwlogs/${DOMAIN}.access.log;
    error_log  /www/wwwlogs/${DOMAIN}.error.log;

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Gzip 压缩
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml;
    gzip_min_length 1024;
    gzip_vary on;

    # 前端 SPA 路由
    location / {
        try_files \$uri \$uri/ /index.html;
        expires 7d;
        add_header Cache-Control "public, immutable";
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }

    # API 代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 30s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        # CORS 头
        add_header Access-Control-Allow-Origin \$http_origin always;
        add_header Access-Control-Allow-Methods "GET,POST,PUT,PATCH,DELETE,OPTIONS" always;
        add_header Access-Control-Allow-Headers "Origin,Content-Type,Accept,Authorization,X-Tenant-ID,X-Trace-ID" always;
        add_header Access-Control-Expose-Headers "X-New-Access-Token,X-New-Refresh-Token,X-Trace-ID" always;
        add_header Access-Control-Allow-Credentials "true" always;
        if (\$request_method = OPTIONS) { return 204; }
    }

    # Admin API 代理
    location /admin/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # Agent API 代理
    location /agent/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    }

    # Metrics 端点（仅内网访问）
    location /metrics {
        proxy_pass http://127.0.0.1:8080;
        allow 127.0.0.1;
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
    }

    # WebSocket 代理
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    # 禁止访问隐藏文件
    location ~ /\. { deny all; }
}
EOF

  log "✓ Nginx 配置已生成: deploy/baota/nginx/mu-developer.conf"

  title "✅ 开发商部署就绪"
  echo ""
  log "下一步操作："
  log "  1. 启动后端服务:"
  log "     pm2 start ecosystem.config.js && pm2 save && pm2 startup"
  log ""
  log "  2. 配置 Nginx:"
  log "     宝塔 → 网站 → 添加站点 → 域名: ${DOMAIN}"
  log "     将 deploy/baota/nginx/mu-developer.conf 粘贴到站点配置"
  log ""
  log "  3. 申请 SSL 证书:"
  log "     宝塔 → 网站 → ${DOMAIN} → SSL → Let's Encrypt → 申请"
  log "     然后取消注释 Nginx 配置中的 SSL 部分"
  log ""
  log "  4. 访问后台:"
  log "     http://${DOMAIN}"
  log "     默认账号: admin / admin123"
  log "     ⚠️ 首次登录后立即修改密码！"
  echo ""

# ==================== 服务商/终端客户角色 ====================
elif [ "$ROLE" = "provider" ] || [ "$ROLE" = "customer" ]; then

  if [ "$ROLE" = "provider" ]; then
    FRONTEND_DIR="frontend/admin-provider"
    ROLE_NAME="服务商"
    DEFAULT_DOMAIN="provider.example.com"
  else
    FRONTEND_DIR="frontend/admin-customer"
    ROLE_NAME="终端客户"
    DEFAULT_DOMAIN="customer.example.com"
  fi

  title "[2] 收集 ${ROLE_NAME} 部署参数"
  DEVELOPER_API=$(ask "开发商 API 地址（含协议，如 https://api.mu-developer.com）: ")
  DOMAIN=$(ask "本站域名（默认 ${DEFAULT_DOMAIN}）: ")
  DOMAIN=${DOMAIN:-$DEFAULT_DOMAIN}

  # 从 URL 提取 host（去除协议和端口用于 proxy_set_header）
  API_HOST=$(echo "$DEVELOPER_API" | sed -E 's|^https?://||' | sed -E 's|:[0-9]+$||' | sed -E 's|/$||')

  title "[3] 构建 ${ROLE_NAME} 前端"
  cd "$FRONTEND_DIR"
  npm install
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build
  cd "$PROJECT_ROOT"
  log "✓ 前端构建完成: ${FRONTEND_DIR}/dist/"

  title "[4] 生成 Nginx 配置"
  mkdir -p deploy/baota/nginx
  CONF_FILE="deploy/baota/nginx/mu-${ROLE}.conf"

  cat > "$CONF_FILE" <<EOF
# MU Framework ${ROLE_NAME} Nginx 配置
# 宝塔面板 → 网站 → 设置 → 配置文件 → 粘贴以下内容

# HTTP → HTTPS 强制跳转（申请 SSL 后取消注释）
# server {
#     listen 80;
#     server_name ${DOMAIN};
#     return 301 https://\$host\$request_uri;
# }

server {
    listen 80;
    # listen 443 ssl http2;
    server_name ${DOMAIN};

    # SSL 证书（宝塔 Let's Encrypt）
    # ssl_certificate    /www/server/panel/vhost/cert/${DOMAIN}/fullchain.pem;
    # ssl_certificate_key /www/server/panel/vhost/cert/${DOMAIN}/privkey.pem;
    # ssl_protocols TLSv1.2 TLSv1.3;
    # ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:HIGH:!aNULL:!MD5:!RC4:!DHE;

    root ${PROJECT_ROOT}/${FRONTEND_DIR}/dist;
    index index.html;

    access_log /www/wwwlogs/${DOMAIN}.access.log;
    error_log  /www/wwwlogs/${DOMAIN}.error.log;

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;

    # Gzip
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml;
    gzip_min_length 1024;

    # 前端 SPA 路由
    location / {
        try_files \$uri \$uri/ /index.html;
    }

    # API 反代到开发商服务器
    location /api/ {
        proxy_pass ${DEVELOPER_API};
        proxy_http_version 1.1;
        proxy_set_header Host ${API_HOST};
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_ssl_server_name on;
        proxy_connect_timeout 30s;
        proxy_read_timeout 60s;

        # 透传响应头
        proxy_pass_header X-New-Access-Token;
        proxy_pass_header X-New-Refresh-Token;
        proxy_pass_header X-Trace-ID;
    }

    # Admin API 反代
    location /admin/ {
        proxy_pass ${DEVELOPER_API};
        proxy_http_version 1.1;
        proxy_set_header Host ${API_HOST};
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_ssl_server_name on;
    }

    # WebSocket 反代
    location /ws {
        proxy_pass ${DEVELOPER_API};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host ${API_HOST};
        proxy_ssl_server_name on;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    # 禁止访问隐藏文件
    location ~ /\. { deny all; }
}
EOF

  log "✓ Nginx 配置已生成: $CONF_FILE"

  title "✅ ${ROLE_NAME} 部署就绪"
  echo ""
  log "下一步操作："
  log "  1. 宝塔 → 网站 → 添加站点"
  log "     域名: ${DOMAIN}"
  log "     根目录: ${PROJECT_ROOT}/${FRONTEND_DIR}/dist"
  log ""
  log "  2. 将 $CONF_FILE 内容粘贴到站点 Nginx 配置"
  log ""
  log "  3. 宝塔 → 网站 → ${DOMAIN} → SSL → Let's Encrypt"
  log "     申请证书后取消注释配置中的 SSL 部分"
  log ""
  log "  4. 访问 http://${DOMAIN}"
  echo ""

else
  error "未知角色: $ROLE（支持: developer / provider / customer）"
  exit 1
fi

title "部署完成 🎉 (v${SCRIPT_VERSION})"
