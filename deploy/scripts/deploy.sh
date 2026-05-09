#!/bin/bash
# MU Framework 一键部署脚本
# 用法：./deploy.sh [dev|staging|prod]
# 环境要求：Docker 27.x+、Docker Compose v2+
set -e

ENV=${1:-dev}
PROJECT_ROOT=$(cd "$(dirname "$0")/../.." && pwd)

echo "========================================"
echo "  MU Framework 部署  · 环境: ${ENV}"
echo "========================================"

cd "$PROJECT_ROOT/deploy/docker"

case $ENV in
  dev|staging|prod)
    echo "[1/4] 构建所有镜像..."
    docker compose build

    echo "[2/4] 启动服务..."
    docker compose up -d

    echo "[3/4] 等待服务就绪..."
    sleep 15

    echo "[4/4] 健康检查..."
    for svc in api admin agent; do
      port=$(case $svc in api) echo 8080;; admin) echo 8081;; agent) echo 8082;; esac)
      if curl -fs "http://localhost:$port/health" > /dev/null 2>&1; then
        echo "  ✅ mu-$svc (端口 $port) 健康"
      else
        echo "  ⚠️  mu-$svc (端口 $port) 尚未就绪"
      fi
    done

    echo ""
    echo "========================================"
    echo "  🎉 MU Framework 部署完成"
    echo "========================================"
    echo "  开发商后台:   http://localhost"
    echo "  服务商后台:   http://localhost:8000"
    echo "  终端客户后台: http://localhost:8001"
    echo "  API 服务:    http://localhost:8080"
    echo "  管理服务:    http://localhost:8081"
    echo "  智能体引擎:  http://localhost:8082"
    echo "  Swagger:    http://localhost:8080/swagger/index.html"
    echo ""
    echo "  默认账号（三个后台通用，修改后请更新数据库）"
    echo "  - 开发商总后台:   mu-platform / admin / mu_admin_2026"
    echo "  - 服务商管理后台: demo-provider / admin / mu_admin_2026"
    echo "  - 终端客户后台:   demo-family / admin / mu_admin_2026"
    echo "========================================"
    ;;
  stop)
    docker compose down
    echo "✅ 服务已停止"
    ;;
  logs)
    docker compose logs -f
    ;;
  clean)
    docker compose down -v
    echo "✅ 服务已停止，数据卷已清理"
    ;;
  *)
    echo "❌ 未知环境: $ENV"
    echo "用法: ./deploy.sh [dev|staging|prod|stop|logs|clean]"
    exit 1
    ;;
esac
