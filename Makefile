# ============================================================
# MU Framework v2.3 Makefile - 中国部署环境优化
# 使用国内镜像加速，一键编译可部署二进制
# ============================================================

.PHONY: all build clean deps frontend help lint test deploy-baota

# 版本信息
VERSION := 2.3.0
BUILD_TIME := $(shell date +%Y%m%d%H%M%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go 编译参数
GOFLAGS := CGO_ENABLED=0 GOOS=linux GOARCH=amd64
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"
BIN_DIR := bin

# 国内 Go 代理
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=sum.golang.google.cn
export GONOSUMCHECK=*

help: ## 帮助
	@echo ""
	@echo "  MU Framework v$(VERSION) 编译命令"
	@echo "  ─────────────────────────────────"
	@echo "  make deps          - 下载 Go 依赖（国内加速）"
	@echo "  make build         - 编译后端三服务"
	@echo "  make frontend      - 构建三端前端"
	@echo "  make all           - 完整构建（后端+前端）"
	@echo "  make clean         - 清理编译产物"
	@echo "  make lint          - 代码检查"
	@echo "  make test          - 运行测试"
	@echo "  make docker        - Docker 镜像构建"
	@echo "  make deploy-baota  - 宝塔面板一键部署"
	@echo ""

deps: ## 下载依赖（国内加速）
	cd backend && go mod download
	@echo "✅ Go 依赖下载完成"

build: deps ## 编译后端三服务
	@mkdir -p $(BIN_DIR)
	cd backend && $(GOFLAGS) go build $(LDFLAGS) -o ../$(BIN_DIR)/api-server ./cmd/api-server
	cd backend && $(GOFLAGS) go build $(LDFLAGS) -o ../$(BIN_DIR)/admin-server ./cmd/admin-server
	cd backend && $(GOFLAGS) go build $(LDFLAGS) -o ../$(BIN_DIR)/agent-engine ./cmd/agent-engine
	@chmod +x $(BIN_DIR)/*
	@echo "✅ 编译完成：$(BIN_DIR)/{api-server, admin-server, agent-engine}"
	@echo "   版本: $(VERSION) | 构建时间: $(BUILD_TIME) | 提交: $(GIT_COMMIT)"

frontend: ## 构建三端前端（国内 npm 源）
	@echo "构建开发商前端..."
	cd frontend/admin-developer && npm config set registry https://registry.npmmirror.com && npm install && npm run build
	@echo "构建服务商前端..."
	cd frontend/admin-provider && npm config set registry https://registry.npmmirror.com && npm install && npm run build
	@echo "构建终端客户前端..."
	cd frontend/admin-customer && npm config set registry https://registry.npmmirror.com && npm install && npm run build
	@echo "✅ 三端前端构建完成"

all: build frontend ## 完整构建

clean: ## 清理编译产物
	rm -rf $(BIN_DIR)
	rm -rf frontend/admin-developer/dist frontend/admin-developer/node_modules
	rm -rf frontend/admin-provider/dist frontend/admin-provider/node_modules
	rm -rf frontend/admin-customer/dist frontend/admin-customer/node_modules
	@echo "✅ 清理完成"

lint: ## 代码检查
	cd backend && go vet ./...
	@echo "✅ 代码检查通过"

test: ## 运行测试
	cd backend && go test -v -race -count=1 ./...
	@echo "✅ 测试完成"

docker: ## Docker 构建（使用国内镜像）
	cd deploy/docker && docker compose build

deploy-baota: ## 宝塔面板部署（交互式）
	@bash deploy/baota/setup.sh

# 快速开发启动
dev: build
	@echo "启动开发环境..."
	@mkdir -p logs plugins storage
	$(BIN_DIR)/api-server --config configs/dev.yaml &
	$(BIN_DIR)/admin-server --config configs/dev.yaml &
	$(BIN_DIR)/agent-engine --config configs/dev.yaml &
	@echo "✅ 开发服务已启动"
	@echo "   API:   http://localhost:8080"
	@echo "   Admin: http://localhost:8081"
	@echo "   Agent: http://localhost:8082"

# 停止开发服务
dev-stop:
	@pkill -f "api-server" 2>/dev/null || true
	@pkill -f "admin-server" 2>/dev/null || true
	@pkill -f "agent-engine" 2>/dev/null || true
	@echo "✅ 开发服务已停止"

# 数据库迁移（宝塔环境）
migrate:
	@echo "执行数据库迁移..."
	@for sql in backend/migrations/001_init_schema.sql \
	            backend/migrations/002_platform_tables.sql \
	            backend/migrations/003_seed_data.sql \
	            backend/migrations/004_rls_policies.sql \
	            backend/migrations/005_genealogy_tables.sql \
	            backend/migrations/006_v15_enhancements.sql; do \
		if [ -f "$$sql" ]; then \
			echo "  → $$sql"; \
			sudo -u postgres psql -d mu_framework -f "$$sql" 2>/dev/null || echo "  ⚠️ $$sql 可能已执行"; \
		fi; \
	done
	@echo "✅ 数据库迁移完成"
