#!/bin/bash
# 🏪 服务商服务器部署脚本
set -e
cd "$(dirname "$0")"

echo "================================================"
echo "  MU Framework · 服务商服务器部署"
echo "================================================"

if [ ! -f .env ]; then
  echo "⚠️  .env 不存在，请先创建："
  echo "  cp .env.example .env"
  echo "  vim .env  # 修改 DEVELOPER_API_URL 指向开发商服务器"
  exit 1
fi

# 加载并校验必需变量
set -a; source .env; set +a
if [ -z "$DEVELOPER_API_URL" ]; then
  echo "❌ .env 中 DEVELOPER_API_URL 未设置"
  exit 1
fi

mkdir -p ssl

ACTION=${1:-up}
case $ACTION in
  up|start)
    echo "[1/3] 构建镜像（API 地址：$DEVELOPER_API_URL）..."
    docker compose --env-file .env build
    echo "[2/3] 启动服务..."
    docker compose --env-file .env up -d
    sleep 5
    docker compose ps
    echo ""
    echo "✅ 服务商后台已启动"
    echo "   访问: http://$(hostname -I | awk '{print $1}')"
    echo "   后端 API 由 Nginx 反代到: $DEVELOPER_API_URL"
    ;;
  stop)   docker compose --env-file .env down ;;
  logs)   docker compose --env-file .env logs -f ;;
  restart) docker compose --env-file .env restart ;;
  *)      echo "用法: $0 [up|stop|logs|restart]"; exit 1 ;;
esac
