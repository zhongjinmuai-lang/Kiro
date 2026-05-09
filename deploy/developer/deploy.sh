#!/bin/bash
# 🏢 开发商服务器一键部署脚本
set -e
cd "$(dirname "$0")"

echo "================================================"
echo "  MU Framework · 开发商服务器部署"
echo "================================================"

# 检查 .env
if [ ! -f .env ]; then
  echo "⚠️  .env 不存在，从 .env.example 复制..."
  cp .env.example .env
  echo "❗ 请编辑 .env 修改密码后再运行"
  exit 1
fi

# SSL 证书目录
mkdir -p ssl

ACTION=${1:-up}
case $ACTION in
  up|start)
    echo "[1/3] 构建镜像..."
    docker compose --env-file .env build
    echo "[2/3] 启动服务..."
    docker compose --env-file .env up -d
    echo "[3/3] 等待就绪..."
    sleep 15
    docker compose ps
    echo ""
    echo "✅ 开发商服务器已启动"
    echo "   开发商总后台: http://$(hostname -I | awk '{print $1}')"
    echo "   API 对外:     http://$(hostname -I | awk '{print $1}')/api/"
    echo ""
    echo "📌 服务商/终端客户后台在各自服务器部署时，VITE_API_BASE_URL 填写为本机对外地址"
    ;;
  stop)
    docker compose --env-file .env down
    ;;
  clean)
    docker compose --env-file .env down -v
    echo "✅ 数据卷已清理"
    ;;
  logs)
    docker compose --env-file .env logs -f ${2:-}
    ;;
  restart)
    docker compose --env-file .env restart ${2:-}
    ;;
  *)
    echo "用法: $0 [up|stop|clean|logs|restart]"
    exit 1
    ;;
esac
