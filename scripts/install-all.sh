#!/bin/bash
# MU Framework 三端安装脚本（中国环境·宝塔直接部署）
# 用法: bash scripts/install-all.sh [developer|provider|customer]
set -e
ROLE=${1:-developer}
DIR=$(cd "$(dirname "$0")/.." && pwd)
cd "$DIR"
log() { echo -e "\033[0;32m[OK]\033[0m $*"; }
warn() { echo -e "\033[0;33m[注意]\033[0m $*"; }

echo "========================================"
echo "  MU Framework 安装 [$ROLE] (中国环境)"
echo "========================================"

# 通用检查
command -v nginx &>/dev/null || { echo "请先在宝塔安装Nginx"; exit 1; }
command -v node &>/dev/null || { echo "请先在宝塔安装Node.js 22"; exit 1; }
npm config set registry https://registry.npmmirror.com

case $ROLE in
developer)
  # Go 环境
  if ! command -v go &>/dev/null; then
    warn "安装Go..."
    wget -q https://go.dev/dl/go1.23.5.linux-amd64.tar.gz -O /tmp/go.tar.gz
    tar -C /usr/local -xzf /tmp/go.tar.gz && rm /tmp/go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    export PATH=$PATH:/usr/local/go/bin
  fi
  go env -w GOPROXY=https://goproxy.cn,direct
  go env -w GOSUMDB=off

  # 编译后端
  log "编译后端..."
  cd backend && go mod download
  mkdir -p ../bin
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/api-server ./cmd/api-server
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/admin-server ./cmd/admin-server
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/agent-engine ./cmd/agent-engine
  chmod +x ../bin/*
  cd "$DIR"
  log "编译完成: bin/api-server | admin-server | agent-engine"

  # 数据库
  log "初始化数据库..."
  sudo -u postgres psql -c "CREATE DATABASE mu_framework;" 2>/dev/null || true
  sudo -u postgres psql mu_framework -c 'CREATE EXTENSION IF NOT EXISTS "uuid-ossp"; CREATE EXTENSION IF NOT EXISTS "pgcrypto";' 2>/dev/null
  for f in 001_init_schema 002_platform_tables 004_rls_policies 005_genealogy_tables 006_v15_enhancements 003_seed_data; do
    sudo -u postgres psql mu_framework -f "backend/migrations/${f}.sql" 2>/dev/null || true
  done
  log "数据库初始化完成"

  # 前端
  log "构建开发商前端..."
  cd frontend/admin-developer && npm install --no-audit --no-fund 2>/dev/null
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build 2>/dev/null || warn "前端构建失败"
  cd "$DIR"

  # PM2启动
  cat > ecosystem.config.js <<'EOF'
module.exports = { apps: [
  { name:'mu-api', script:'./bin/api-server', args:'--config configs/prod.yaml', autorestart:true },
  { name:'mu-admin', script:'./bin/admin-server', args:'--config configs/prod.yaml', autorestart:true },
  { name:'mu-agent', script:'./bin/agent-engine', args:'--config configs/prod.yaml', autorestart:true },
]}
EOF
  pm2 start ecosystem.config.js && pm2 save

  log "✅ 开发商安装完成！"
  echo "  后台: 宝塔添加站点→根目录 $DIR/frontend/admin-developer/dist"
  echo "  Nginx反代: /api/→127.0.0.1:8080 /admin/→127.0.0.1:8081"
  echo "  账号: mu-platform / admin / mu_admin_2026"
  ;;

provider)
  log "构建服务商前端..."
  cd frontend/admin-provider && npm install --no-audit --no-fund 2>/dev/null
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build 2>/dev/null || warn "构建失败"
  cd "$DIR"
  log "✅ 服务商安装完成！"
  echo "  后台: 宝塔添加站点→根目录 $DIR/frontend/admin-provider/dist"
  echo "  Nginx反代: /api/→开发商IP:8080 /admin/→开发商IP:8081"
  echo "  账号: 由开发商在后台创建"
  ;;

customer)
  log "构建终端客户前端..."
  cd frontend/admin-customer && npm install --no-audit --no-fund 2>/dev/null
  echo "VITE_API_BASE_URL=" > .env.production
  npm run build 2>/dev/null || warn "构建失败"
  cd "$DIR"
  log "✅ 终端客户安装完成！"
  echo "  后台: 宝塔添加站点→根目录 $DIR/frontend/admin-customer/dist"
  echo "  Nginx反代: /api/→开发商IP:8080"
  echo "  账号: 由服务商在后台创建"
  ;;

*) echo "用法: bash scripts/install-all.sh [developer|provider|customer]"; exit 1 ;;
esac
