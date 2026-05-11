#!/bin/bash
# ============================================================
# MU Framework 中国环境一键编译脚本
# 解决国内网络问题：Go 代理、npm 镜像、Docker 镜像加速
# ============================================================
set -e

echo "========================================"
echo "  MU Framework 中国环境编译"
echo "========================================"

PROJECT_ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$PROJECT_ROOT"

# ===== Go 环境配置（国内加速）=====
echo "[1/5] 配置 Go 国内代理..."
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=off
go env -w GONOSUMCHECK=*

# ===== 下载依赖 =====
echo "[2/5] 下载 Go 依赖..."
cd backend
go mod download
cd ..

# ===== 编译后端 =====
echo "[3/5] 编译后端三服务（linux/amd64）..."
mkdir -p bin
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/api-server ./cmd/api-server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/admin-server ./cmd/admin-server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/agent-engine ./cmd/agent-engine
cd ..
chmod +x bin/*
echo "  ✓ bin/api-server ($(du -h bin/api-server | cut -f1))"
echo "  ✓ bin/admin-server ($(du -h bin/admin-server | cut -f1))"
echo "  ✓ bin/agent-engine ($(du -h bin/agent-engine | cut -f1))"

# ===== 构建前端（可选，需要 Node.js）=====
if command -v node &>/dev/null; then
  echo "[4/5] 构建前端（国内 npm 源）..."
  npm config set registry https://registry.npmmirror.com

  for dir in admin-developer admin-provider admin-customer; do
    echo "  构建 $dir..."
    cd frontend/$dir
    npm install --no-audit --no-fund --loglevel=error 2>/dev/null
    npm run build 2>/dev/null || echo "  ⚠️ $dir 构建失败（可后续手动处理）"
    cd ../..
  done
else
  echo "[4/5] 跳过前端构建（未检测到 Node.js，请手动构建）"
fi

# ===== 打包部署包 =====
echo "[5/5] 打包部署文件..."
PACKAGE="mu-framework-$(date +%Y%m%d).tar.gz"
tar czf "$PACKAGE" \
  bin/ \
  configs/ \
  backend/migrations/ \
  deploy/docker/init-db.sql \
  deploy/baota/ \
  deploy/scripts/ \
  frontend/admin-developer/dist/ 2>/dev/null \
  frontend/admin-provider/dist/ 2>/dev/null \
  frontend/admin-customer/dist/ 2>/dev/null \
  INSTALL.md \
  Makefile \
  scripts/

echo ""
echo "========================================"
echo "  ✅ 编译完成！"
echo "========================================"
echo ""
echo "  部署包: $PACKAGE"
echo "  大小: $(du -h $PACKAGE | cut -f1)"
echo ""
echo "  部署步骤："
echo "  1. 上传 $PACKAGE 到服务器"
echo "  2. tar xzf $PACKAGE"
echo "  3. 参考 INSTALL.md 完成部署"
echo ""
echo "  或直接在服务器执行："
echo "  bash deploy/baota/setup.sh developer"
