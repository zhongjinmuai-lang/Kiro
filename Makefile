# ============================================================
# MU Framework Makefile - 中国部署环境优化
# 使用国内镜像加速，一键编译可部署二进制
# ============================================================

.PHONY: all build clean deps frontend help

# Go 编译参数
GOFLAGS := CGO_ENABLED=0 GOOS=linux GOARCH=amd64
LDFLAGS := -ldflags="-s -w -X main.version=1.8.0 -X main.buildTime=$(shell date +%Y%m%d%H%M%S)"
BIN_DIR := bin

# 国内 Go 代理
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off

help: ## 帮助
	@echo "MU Framework 编译命令："
	@echo "  make deps      - 下载 Go 依赖（国内加速）"
	@echo "  make build     - 编译后端三服务"
	@echo "  make frontend  - 构建三端前端"
	@echo "  make all       - 完整构建（后端+前端）"
	@echo "  make clean     - 清理编译产物"
	@echo "  make docker    - Docker 镜像构建"

deps: ## 下载依赖（国内加速）
	cd backend && go mod download

build: deps ## 编译后端三服务
	@mkdir -p $(BIN_DIR)
	cd backend && $(GOFLAGS) go build $(LDFLAGS) -o ../$(BIN_DIR)/api-server ./cmd/api-server
	cd backend && $(GOFLAGS) go build $(LDFLAGS) -o ../$(BIN_DIR)/admin-server ./cmd/admin-server
	cd backend && $(GOFLAGS) go build $(LDFLAGS) -o ../$(BIN_DIR)/agent-engine ./cmd/agent-engine
	@chmod +x $(BIN_DIR)/*
	@echo "✅ 编译完成：$(BIN_DIR)/api-server | admin-server | agent-engine"

frontend: ## 构建三端前端（国内 npm 源）
	cd frontend/admin-developer && npm config set registry https://registry.npmmirror.com && npm install && npm run build
	cd frontend/admin-provider && npm config set registry https://registry.npmmirror.com && npm install && npm run build
	cd frontend/admin-customer && npm config set registry https://registry.npmmirror.com && npm install && npm run build
	@echo "✅ 前端构建完成"

all: build frontend ## 完整构建

clean: ## 清理
	rm -rf $(BIN_DIR) frontend/*/dist frontend/*/node_modules

docker: ## Docker 构建
	cd deploy/docker && docker compose build

# 快速部署（编译+启动）
deploy-dev: build
	@echo "启动开发环境..."
	cd deploy/docker && docker compose up -d postgres redis
	$(BIN_DIR)/api-server --config configs/dev.yaml &
	$(BIN_DIR)/admin-server --config configs/dev.yaml &
	$(BIN_DIR)/agent-engine --config configs/dev.yaml &
	@echo "✅ 服务已启动"
