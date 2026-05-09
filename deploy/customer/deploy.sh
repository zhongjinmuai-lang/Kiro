#!/bin/bash
# 👪 终端客户服务器部署脚本
set -e
cd "$(dirname "$0")"

echo "================================================"
echo "  MU Framework · 终端客户服务器部署"
echo "================================================"

if [ ! -f .env ]; then
  echo "⚠️  .env 不存在，请：cp .env.example .env && vim .env"
  exit 1
fi

set -a; source .env; set +a
if [ -z "$DEVELOPER_API_URL" ]; then
  echo "❌ .env 中 DEVELOPER_API_URL 未设置"
  exit 1
fi

mkdir -p ssl

ACTION=${1:-up}
case $ACTION in
  up|start)
    docker compose --env-file .env build
    docker compose --env-file .env up -d
    sleep 5
    docker compose ps
    echo ""
    echo "✅ 终端客户业务后台已启动"
    echo "   访问: http://$(hostname -I | awk '{print $1}')"
    ;;
  stop)   docker compose --env-file .env down ;;
  logs)   docker compose --env-file .env logs -f ;;
  restart) docker compose --env-file .env restart ;;
  *)      echo "用法: $0 [up|stop|logs|restart]"; exit 1 ;;
esac
