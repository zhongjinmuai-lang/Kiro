# MU 框架 · API 规范（OpenAPI 3.1）

## 一、访问入口

| 环境 | API Server | Admin Server | Agent Engine | Swagger UI |
|------|------------|--------------|--------------|-----------|
| 开发 | http://localhost:8080 | http://localhost:8081 | http://localhost:8082 | http://localhost:8080/swagger/index.html |
| 生产 | https://api.mu-framework.com | https://admin.mu-framework.com | internal only | https://api.mu-framework.com/swagger |

## 二、统一响应结构

所有接口返回统一包装体：

```json
{
  "code": 0,
  "message": "success",
  "data": { "...": "..." },
  "trace_id": "f8d1..."
}
```

- `code = 0` 表示成功；非 0 为业务错误码
- HTTP 状态码由业务码前两位自动映射（400xx→400，401xx→401 …）
- 所有响应都会附带 `trace_id`，客户端问题排查时请提供该 ID

## 三、认证机制

### 3.1 双令牌体系

| 令牌 | TTL | 用途 |
|------|-----|------|
| AccessToken | 2h | 接口鉴权 |
| RefreshToken | 7d | 刷新 AccessToken |

登录后拿到令牌对：

```http
POST /api/v1/auth/login
Content-Type: application/json

{ "username": "admin", "password": "mu_admin_2026" }
```

### 3.2 请求头

```http
Authorization: Bearer <AccessToken>
X-Trace-ID:    <可选，客户端自定义>
```

### 3.3 智能续签

当 AccessToken 剩余时间 < 15 分钟时，服务端会在响应头追加：

```http
X-New-Access-Token:  <新的 access>
X-New-Refresh-Token: <新的 refresh>
```

客户端收到后应替换本地存储的令牌对，实现无感续期。

## 四、三级路由约定

| 路径前缀 | 所属服务 | 可访问层级 |
|----------|----------|-----------|
| `/api/v1/*` | API Server | 已登录的任意层级 |
| `/admin/developer/*` | Admin Server | developer |
| `/admin/provider/*` | Admin Server | provider |
| `/admin/customer/*` | Admin Server | customer |
| `/agent/*` | Agent Engine | developer |

## 五、分页约定

```http
GET /api/v1/orders?page=1&page_size=20
```

- `page` 从 1 开始
- `page_size` 默认 20，上限 100

响应：

```json
{
  "code": 0,
  "data": {
    "list": [ /* ... */ ],
    "page": 1,
    "page_size": 20,
    "total": 123
  }
}
```

## 六、全链路追踪

- 服务端自动生成 `X-Trace-ID`（UUID v4），客户端也可自行传入
- 同一请求会贯穿所有日志、SQL、下游调用
- 通过 `trace_id` 可在 ELK / Loki / 云日志中聚合查看完整调用链

## 七、OpenAPI 3.1 规范源文件

- 静态规范：`backend/docs/openapi.yaml`
- 生成注释：使用 `swag init -g cmd/api-server/main.go -o backend/docs`
- Swagger UI：访问 `/swagger/index.html`

## 八、错误码速查

| 业务码 | HTTP | 含义 |
|--------|------|------|
| 0 | 200 | 成功 |
| 40000 | 400 | 参数错误 |
| 40100 | 401 | 未登录/令牌无效 |
| 40300 | 403 | 权限不足 |
| 40400 | 404 | 资源不存在 |
| 40900 | 409 | 冲突（重复创建等） |
| 42900 | 429 | 限流 |
| 50000 | 500 | 服务内部错误 |
| 50200 | 502 | 下游依赖异常 |
| 60100 | 400 | 租户相关业务错误 |
| 60200 | 400 | 权限相关业务错误 |
| 60300 | 400 | 支付相关业务错误 |
| 60400 | 400 | 存储相关业务错误 |
| 60500 | 400 | 通知相关业务错误 |
| 60600 | 400 | 智能体相关业务错误 |
