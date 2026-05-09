# MU 框架 · 部署运维手册

> 云原生生产级标准 · Ubuntu 24.04 LTS / Docker 27.x / K3s / Kubernetes 1.32+

## 一、最小化部署（Docker Compose）

适用：开发 / 测试 / 单机演示

```bash
cd deploy/docker
docker compose up -d --build

# 查看日志
docker compose logs -f mu-api

# 健康检查
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8082/health
```

启动的服务：
- `mu-postgres`（PostgreSQL 18.3）
- `mu-redis`（Redis 7.4）
- `mu-api`（API Server，:8080）
- `mu-admin`（Admin Server，:8081）
- `mu-agent`（Agent Engine，:8082）
- `mu-frontend`（Nginx + Vue 三端）

## 二、Kubernetes 生产部署

### 2.1 前置

```bash
# 创建命名空间
kubectl apply -f deploy/k8s/namespace.yaml

# 注入密钥（生产请使用 Secret 管理工具，如 SealedSecrets / Vault）
kubectl -n mu-framework create secret generic mu-db-secret \
  --from-literal=host=postgres.mu.svc.cluster.local \
  --from-literal=password='<strong-password>'

kubectl -n mu-framework create secret generic mu-jwt-secret \
  --from-literal=secret='<jwt-secret>'
```

### 2.2 部署

```bash
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/api-server.yaml
kubectl apply -f deploy/k8s/agent-engine.yaml
kubectl apply -f deploy/k8s/ingress.yaml
```

### 2.3 弹性伸缩（HPA）

`api-server.yaml` 已内置 HPA：
- 最小 2 副本 / 最大 10 副本
- CPU 阈值 70% / 内存阈值 80%

## 三、一键部署脚本

```bash
./deploy/scripts/deploy.sh dev       # 开发
./deploy/scripts/deploy.sh staging   # 预发布
./deploy/scripts/deploy.sh prod      # 生产
```

## 四、数据库迁移

```bash
# 按顺序执行
psql -U mu_admin -d mu_framework -f backend/migrations/001_init_schema.sql
psql -U mu_admin -d mu_framework -f backend/migrations/002_platform_tables.sql
psql -U mu_admin -d mu_framework -f backend/migrations/003_seed_data.sql
psql -U mu_admin -d mu_framework -f backend/migrations/004_rls_policies.sql
```

Docker Compose 会自动执行（通过 `init-db.sql`）。

## 五、SSL 证书

### 5.1 Let's Encrypt（cert-manager）

`ingress.yaml` 已配置 `cert-manager.io/cluster-issuer: letsencrypt-prod`，自动签发。

### 5.2 手动证书

```bash
kubectl -n mu-framework create secret tls mu-tls-secret \
  --cert=fullchain.pem --key=privkey.pem
```

## 六、监控与告警

推荐栈：
- **指标**：Prometheus + Grafana
- **日志**：Loki / ELK
- **链路**：Jaeger / Tempo（通过 TraceID）
- **告警**：Alertmanager → 钉钉 / 企业微信 / 短信

关键指标：
- `mu_http_request_duration_seconds` - 接口延迟
- `mu_agent_queue_depth` - 智能体队列积压
- `mu_db_slow_query_total` - 慢SQL总数
- `mu_evolution_events_total` - 自进化触发次数

## 七、日志收集

生产日志默认输出到：
- `stdout`（容器日志，由 K8s 收集）
- `/var/log/mu-framework/mu-framework.log`（按 100MB 切割、30天保留）

推荐使用 **Promtail** 采集到 Loki，通过 `trace_id` 字段聚合单次请求全链路。

## 八、灰度发布

### 8.1 插件灰度

```bash
curl -XPOST /agent/plugins/install \
  -H "Authorization: Bearer <dev-token>" \
  -d '{"plugin_id":"xxx","version":"1.2.0","canary":10}'  # 10% 流量
```

### 8.2 服务灰度

K8s `Deployment` 滚动更新：
```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
```

配合 Istio / Higress 实现流量按比例分发。

## 九、备份与恢复

- PG：`pg_dump` 每日全量 + WAL 连续归档
- Redis：`appendonly yes` + AOF 持久化
- 存储文件：通过中台绑定的 OSS/COS 等天然有 3 副本

## 十、常见运维操作

```bash
# 查看服务状态
kubectl -n mu-framework get pods,svc

# 查看某服务日志
kubectl -n mu-framework logs -f deployment/mu-api-server

# 进入容器排查
kubectl -n mu-framework exec -it deployment/mu-api-server -- sh

# 重启服务
kubectl -n mu-framework rollout restart deployment/mu-api-server

# 扩容
kubectl -n mu-framework scale deployment mu-api-server --replicas=5

# 查看 HPA
kubectl -n mu-framework get hpa
```
