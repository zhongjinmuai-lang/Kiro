#!/bin/bash
# MU Framework 部署脚本
# 用法: ./deploy.sh [dev|staging|prod]

set -e

ENV=${1:-dev}
PROJECT_ROOT=$(cd "$(dirname "$0")/../.." && pwd)

echo "========================================"
echo "  MU Framework 部署"
echo "  环境: ${ENV}"
echo "========================================"

case $ENV in
  dev)
    echo "[1/3] 启动开发环境 (Docker Compose)..."
    cd "$PROJECT_ROOT/deploy/docker"
    docker-compose up -d --build
    echo "[2/3] 等待服务就绪..."
    sleep 10
    echo "[3/3] 检查服务状态..."
    curl -s http://localhost:8080/health | jq .
    echo ""
    echo "✅ 开发环境已启动"
    echo "  API Server: http://localhost:8080"
    echo "  Admin Server: http://localhost:8081"
    echo "  Agent Engine: http://localhost:8082"
    echo "  前端: http://localhost:80"
    ;;
  staging|prod)
    echo "[1/4] 构建Docker镜像..."
    cd "$PROJECT_ROOT"
    docker build -f deploy/docker/Dockerfile.backend -t mu-framework/backend:${ENV} .
    docker build -f deploy/docker/Dockerfile.frontend -t mu-framework/frontend:${ENV} .
    
    echo "[2/4] 推送镜像到Registry..."
    # docker push mu-framework/backend:${ENV}
    # docker push mu-framework/frontend:${ENV}
    
    echo "[3/4] 应用Kubernetes配置..."
    kubectl apply -f deploy/k8s/namespace.yaml
    kubectl apply -f deploy/k8s/configmap.yaml
    kubectl apply -f deploy/k8s/api-server.yaml
    kubectl apply -f deploy/k8s/agent-engine.yaml
    kubectl apply -f deploy/k8s/ingress.yaml
    
    echo "[4/4] 等待部署完成..."
    kubectl -n mu-framework rollout status deployment/mu-api-server
    kubectl -n mu-framework rollout status deployment/mu-agent-engine
    
    echo ""
    echo "✅ ${ENV} 环境部署完成"
    kubectl -n mu-framework get pods
    ;;
  *)
    echo "❌ 未知环境: $ENV"
    echo "用法: ./deploy.sh [dev|staging|prod]"
    exit 1
    ;;
esac
