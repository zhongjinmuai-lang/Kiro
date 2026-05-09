# v1.1 升级说明 · 全栈技术栈切换

> 将后端从"标准库 + 手写HTTP/ORM"升级为 Gin + GORM + Zap + JWT + OpenAPI 3.1 工业级技术栈

## 一、升级摘要

| 维度 | v1.0（初始骨架） | v1.1（本次升级） |
|------|----------------|-----------------|
| HTTP | `net/http` | **Gin 1.10** |
| ORM | 原生 `pgx` + 手写SQL | **GORM v2** + `postgres driver` |
| 日志 | `log/slog` | **Zap 2.x** + lumberjack切割 + 全链路TraceID |
| 鉴权 | 占位 | **JWT 双令牌**（Access + Refresh）+ 智能续签 + Redis黑名单 |
| 配置 | yaml 手动解析 | **Viper**（支持环境变量覆盖） |
| Redis | `go-redis` 单机 | **UniversalClient**（单机/集群/哨兵自适应） |
| 参数校验 | 无 | **go-playground/validator**（binding tag） |
| API文档 | 无 | **OpenAPI 3.1** + Swagger UI |
| 限流 | 无 | Redis 分布式固定窗口（IP/用户/租户三维） |
| 族谱/层级 | 多次SQL | **PG 递归 CTE** 单次 |
| RLS | 仅SQL定义 | **应用层 set_config 自动注入** |

## 二、目录变化

### 新增
```
backend/
├── pkg/
│   ├── cache/redis.go          # Redis 7.4 UniversalClient + 分布式锁 + 限流
│   ├── hierarchy/cte.go        # PG 递归 CTE 族谱工具
│   └── jwt/jwt.go              # JWT 双令牌 + 智能续签
├── internal/
│   ├── ai/gateway.go           # AI 调度网关（5家供应商）
│   └── core/middleware/        # 中间件按职责拆分为 7 个文件
└── docs/                        # OpenAPI 规范 + Swagger 入口

docs/
├── api-conventions.md           # API 规范
├── control-matrix.md            # 三级管控矩阵
├── database-design.md           # 数据库设计
├── deployment.md                # 部署手册
├── operations-manual.md         # 三级管控操作手册
└── upgrade-v1.1.md              # 本文档

.github/workflows/ci.yml         # GitHub Actions CI/CD
backend/migrations/004_rls_policies.sql  # 行级安全策略
configs/staging.yaml             # 预发布环境配置
```

### 重写
- `cmd/*/main.go`：全部切换 Gin + 命令行 `--config` 参数 + Zap 日志
- `internal/core/bootstrap/app.go`：GORM v2 + PrepareStmt + WithTenantSession
- `internal/core/config/config.go`：Viper + 环境变量覆盖
- `internal/core/router/router.go`：三套 Gin 引擎（API/Admin/Agent）
- `internal/core/middleware/*.go`：拆分 trace/recovery/logger/cors/auth/ratelimit/tenant
- `internal/model/*.go`：GORM 标签 + 软删除 + 自动时间戳
- `internal/saas/**/*.go`：所有 Service 全部 GORM 化
- `internal/platform/**/*.go`：支付/存储/通知 中台 GORM 化
- `pkg/logger/logger.go`：Zap + lumberjack
- `pkg/response/response.go`：Gin 风格统一响应
- `configs/dev.yaml` / `configs/prod.yaml`：匹配新 Config 结构

### 删除
- `backend/internal/core/middleware/middleware.go`（被多文件替代）
- `backend/internal/core/config/fmt.go`（临时垫片）
- `backend/internal/platform/storage/providers.go`（供应商常量迁至 model）

## 三、迁移指引

### 3.1 编译环境
```bash
go version   # 需要 Go 1.26.1
cd backend
go mod tidy
```

### 3.2 数据库
新增 `004_rls_policies.sql` 需手动执行：
```bash
psql -U mu_admin -d mu_framework -f backend/migrations/004_rls_policies.sql
```

### 3.3 配置迁移
旧配置：扁平结构  
新配置：分层结构（`app`/`server`/`database`/`redis`/`logger`/`jwt`/`agent`/`platform`/`swagger`）

请参考最新 `configs/dev.yaml` 重新编写。

### 3.4 环境变量
生产建议通过环境变量覆盖敏感信息：
```bash
export MU_DATABASE_PASSWORD=xxx
export MU_JWT_SECRET=xxx
export MU_REDIS_PASSWORD=xxx
```

### 3.5 Swagger 文档生成（可选）
```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/api-server/main.go -o backend/docs
```

## 四、破坏性变更

1. **中间件 API 变化**：`middleware.Chain` 移除，请直接使用 `engine.Use(...)`
2. **响应结构**：新增 `trace_id` 字段，客户端解析兼容即可
3. **鉴权头**：统一为 `Authorization: Bearer <AccessToken>`，不再支持 query 参数（除下载场景）
4. **Config 结构**：扁平 → 分层，现有配置文件需重写
5. **Model**：所有 ID 改为 UUID 字符串；时间戳由 GORM 自动管理

## 五、性能提升预期

| 指标 | 预期 |
|------|------|
| 路由查找 | Gin radix tree，比标准库 mux 快约 3~5x |
| ORM | PrepareStmt 预编译，重复查询快 20~30% |
| 日志 | Zap 零分配，比 slog 快 2~3x |
| 限流 | Redis Lua 原子操作，单机 > 50k QPS |
| 族谱查询 | 单次递归 CTE 替代多次查询，N 层降为 O(1) 往返 |

## 六、下一步计划

- [ ] 接入 OpenTelemetry 实现完整分布式追踪
- [ ] 支付中台：微信支付 / 支付宝 / 聚合支付 实际适配器
- [ ] 存储中台：阿里OSS / 腾讯COS / 七牛 / 华为OBS 实际适配器
- [ ] 通知中台：短信 / 邮件 / 微信模板消息 实际适配器
- [ ] AI 网关：豆包 / 通义 / 文心 / DeepSeek 实际客户端
- [ ] 前端 Vue 3.5 三端页面补全
- [ ] UniApp X 族谱可视化、OCR 建档
