# 🛠️ MU Framework 环境要求文档

> 适用于 Docker 部署 / 宝塔面板部署 / 裸机部署 / K8s 部署等各场景。

---

## 1️⃣ 服务器硬件规格

### 按部署角色差异化配置

| 角色 | 最低配置 | 推荐配置 | 建议用途 |
|------|---------|---------|---------|
| 🏢 开发商服务器 | 2C/4G/40G | 4C/8G/100G SSD | 后端+DB+Redis+开发商前端 |
| 🏪 服务商服务器 | 1C/2G/20G | 2C/4G/40G | 仅 admin-provider 前端+Nginx |
| 👪 终端客户服务器 | 1C/2G/20G | 2C/4G/40G | 仅 admin-customer 前端+Nginx |
| 🔁 单机一体化演示 | 2C/4G/40G | 4C/8G/100G | 开发测试 |

### 生产集群建议

| 规模 | 开发商服务器配置 |
|------|----------------|
| 小型（<100 服务商，<1 万客户） | 4C/8G + PG 单机 + Redis 单机 |
| 中型（100-1000 服务商，1-10 万客户） | 8C/16G + PG 主从 + Redis 哨兵 |
| 大型（>1000 服务商，>10 万客户） | K8s 集群 + PG 主从+读副本 + Redis 集群 |

---

## 2️⃣ 操作系统

| 发行版 | 版本 | 支持级别 |
|--------|------|---------|
| Ubuntu | **24.04 LTS（推荐）** / 22.04 LTS | ⭐⭐⭐ 全面支持 |
| Debian | 12 / 11 | ⭐⭐⭐ 全面支持 |
| CentOS Stream | 9 | ⭐⭐ 兼容 |
| Rocky Linux / Alma Linux | 9 / 8 | ⭐⭐ 兼容 |
| Windows Server | — | ❌ 不支持（需 WSL2） |

---

## 3️⃣ 后端运行时

### Go（开发商服务器必装）

| 项目 | 要求 |
|------|------|
| 版本 | **Go 1.23+**（生产最佳 Go 1.26.1） |
| 架构 | linux/amd64 或 linux/arm64 |
| 代理 | `GOPROXY=https://goproxy.cn,direct`（国内） |
| 编译选项 | `CGO_ENABLED=0 -ldflags="-s -w"` 生成静态二进制 |

### 安装（Ubuntu 24.04）

```bash
# 官方二进制
cd /usr/local
wget https://go.dev/dl/go1.23.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
source /etc/profile
go version
```

---

## 4️⃣ 数据库

### PostgreSQL（开发商服务器必装）

| 项目 | 要求 |
|------|------|
| 版本 | **PostgreSQL 18.3（推荐）** / 16+ / 15+（兼容） |
| 必需扩展 | `uuid-ossp`、`pgcrypto` |
| RLS 行级安全 | 必须启用（迁移脚本 004 自动开启） |
| 字符编码 | UTF-8 |
| 时区 | `Asia/Shanghai` |
| 连接池 | 生产建议 ≥ 100 连接 |

### 安装（Ubuntu 24.04）

```bash
sudo apt install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh
sudo apt install -y postgresql-18 postgresql-contrib-18

# 调整监听（仅本地）
sudo sed -i "s/^#listen_addresses =.*/listen_addresses = 'localhost'/" /etc/postgresql/18/main/postgresql.conf
sudo systemctl restart postgresql

# 创建库和用户
sudo -u postgres psql <<EOF
CREATE USER mu_admin WITH PASSWORD 'YOUR_STRONG_PASSWORD';
CREATE DATABASE mu_framework OWNER mu_admin;
GRANT ALL PRIVILEGES ON DATABASE mu_framework TO mu_admin;
\c mu_framework
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
EOF
```

---

## 5️⃣ 缓存

### Redis（开发商服务器必装）

| 项目 | 要求 |
|------|------|
| 版本 | **Redis 7.4.x** |
| 部署方式 | 单机（小型） / 集群（生产） / 哨兵 |
| AOF 持久化 | `appendonly yes` |
| 密码 | 必须设置 `requirepass` |
| 监听 | `bind 127.0.0.1`（生产不对外） |
| 内存 | 建议 ≥ 2GB |

### 用途

MU 使用 Redis 承载：
- **缓存**（权限 / 租户层级链）
- **会话**（JWT 黑名单）
- **接口限流**（固定窗口 Lua 脚本）
- **分布式锁**（SET NX PX + 原子释放）
- **分布式任务**（Stream 入队）

---

## 6️⃣ 前端构建环境

### Node.js（三种服务器都需要）

| 项目 | 要求 |
|------|------|
| 版本 | **Node.js 22 LTS（推荐）** / 20 LTS / 18 LTS |
| 包管理 | npm 10+ / pnpm 9+ / yarn 1.22+ |
| 源 | `https://registry.npmmirror.com`（国内） |

### 安装（Ubuntu 24.04）

```bash
# 使用 NodeSource 安装 Node.js 22
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo bash -
sudo apt install -y nodejs
node -v && npm -v
```

---

## 7️⃣ Web 服务器 / 反向代理

### Nginx（所有服务器必装）

| 项目 | 要求 |
|------|------|
| 版本 | **Nginx 1.24+**（推荐 1.26+） |
| 模块 | 需支持 `http_ssl_module`、`http_v2_module`、`http_gzip_module` |
| 最大 body | 建议 ≥ 100MB（支持老族谱扫描件上传） |
| Keepalive | 建议 `keepalive_timeout 65` |
| WebSocket | 必须支持（站内信） |

### 安装

```bash
sudo apt install -y nginx
sudo systemctl enable --now nginx
```

---

## 8️⃣ 容器运行时（Docker 部署场景）

| 项目 | 要求 |
|------|------|
| Docker | 27.x+ |
| Docker Compose | v2+（插件模式） |
| 存储驱动 | `overlay2`（默认） |
| 镜像源 | 国内可配置 `https://mirror.ccs.tencentyun.com` |

---

## 9️⃣ 进程管理（宝塔部署场景）

### PM2（推荐）

```bash
npm install -g pm2
pm2 --version  # 需 ≥ 5.0
```

### Supervisor（替代）

```bash
sudo apt install -y supervisor
supervisorctl --version  # 需 ≥ 4.x
```

---

## 🔟 SSL 证书

| 方案 | 使用场景 |
|------|---------|
| **Let's Encrypt + Certbot**（推荐） | 免费、自动续签 |
| 宝塔面板一键申请 | 集成 Let's Encrypt |
| 阿里云 / 腾讯云 / Cloudflare | 企业级 |

### 宝塔一键启用
网站 → SSL → Let's Encrypt → 勾选域名 → 申请（自动续签）

### Certbot 命令
```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d admin.mu-developer.com
```

---

## 1️⃣1️⃣ 网络与防火墙

### 端口开放清单

| 角色 | 对外必开 | 内部服务 | 禁止对外 |
|------|---------|---------|---------|
| 🏢 开发商 | 80、443 | 8080/8081/8082 | 5432（PG）、6379（Redis） |
| 🏪 服务商 | 80、443 | — | — |
| 👪 终端 | 80、443 | — | — |

### 推荐带宽

| 规模 | 带宽 |
|------|------|
| 小型（< 100 并发） | 2 Mbps |
| 中型（< 1000 并发） | 10 Mbps |
| 大型（> 1000 并发） | 100 Mbps / CDN |

### 安全组建议

- **仅开放必要端口**
- 宝塔面板管理口（默认 8888）建议绑定 IP 白名单或改端口
- SSH 建议 `key-only`，禁用密码登录

---

## 1️⃣2️⃣ 域名 & DNS

| 项 | 要求 |
|------|------|
| 开发商后台域名 | 如 `admin.mu-developer.com` |
| API 对外域名 | 建议 `api.mu-developer.com`（或与后台同域） |
| 服务商域名 | 如 `provider-a.example.com`（每个服务商独立） |
| 终端客户域名 | 如 `family-001.example.com` 或贴牌域名 |
| DNS 解析记录 | A / AAAA / CNAME 均可 |

---

## 1️⃣3️⃣ 第三方云服务（可选集成）

### 存储中台

| 厂商 | 所需参数 |
|------|---------|
| 阿里云 OSS | Endpoint / Bucket / AccessKeyId / AccessKeySecret |
| 腾讯云 COS | Region / Bucket / SecretId / SecretKey |
| 七牛云 | Zone / Bucket / AccessKey / SecretKey |
| 华为云 OBS | Endpoint / Bucket / AccessKey / SecretKey |
| MinIO | Endpoint / AccessKey / SecretKey |
| 本地 | 挂载磁盘 + CDN 域名 |

### AI 网关

| 厂商 | 所需参数 |
|------|---------|
| 豆包（火山方舟） | API Key |
| 通义千问（DashScope） | API Key |
| 文心一言（千帆） | API Key |
| DeepSeek | API Key |
| 企业私有部署 | OpenAI 兼容 Endpoint + API Key |

### 支付渠道

| 厂商 | 所需参数 |
|------|---------|
| 微信支付 V3 | AppId / MchId / API V3 Key / 私钥证书 |
| 支付宝 | AppId / 应用私钥 / 支付宝公钥 |
| 聚合支付 | 对应聚合方参数 |

### 通知渠道

| 通道 | 所需参数 |
|------|---------|
| 短信（阿里云/腾讯云） | AccessKey + SignName + TemplateCode |
| 邮件（SMTP） | Host / Port / Username / Password |
| 微信模板消息 | AppId + AppSecret + TemplateId |

---

## 1️⃣4️⃣ 监控告警（生产推荐）

| 组件 | 用途 | 版本 |
|------|------|------|
| Prometheus | 指标采集 | 2.50+ |
| Grafana | 可视化仪表板 | 10.4+ |
| Loki | 日志聚合 | 3.0+ |
| Alertmanager | 告警分发 | 0.27+ |

---

## 1️⃣5️⃣ 备份方案

### PostgreSQL 备份

```bash
# 全量备份（宝塔 → 计划任务 → 数据库备份）
pg_dump -h 127.0.0.1 -U mu_admin -d mu_framework -F c -f /backup/mu_$(date +%Y%m%d).dump

# 恢复
pg_restore -h 127.0.0.1 -U mu_admin -d mu_framework /backup/mu_20260509.dump
```

**策略**：每日全量 + WAL 归档 + 30 天保留 + 异地备份

### Redis 备份

```bash
# RDB 自动快照（redis.conf）
save 900 1       # 15分钟内至少1次修改
save 300 10      # 5分钟内至少10次修改
appendonly yes   # AOF 持久化
```

### 应用代码/前端产物

```bash
# 宝塔计划任务：每日 tar 到 /backup/
tar czf /backup/mu_$(date +%Y%m%d).tar.gz /www/wwwroot/mu-framework
```

---

## 🔒 安全加固清单

| 项目 | 必须 | 推荐 |
|------|------|------|
| 数据库外网端口 | ❌ 关闭 | 仅 localhost |
| Redis 密码 | ✅ 必须 | 32+ 字符强密码 |
| JWT 密钥 | ✅ 必须 | `openssl rand -hex 32` |
| HTTPS | ✅ 生产必须 | Let's Encrypt |
| 宝塔面板端口 | 改默认 | 绑定 IP + 2FA |
| SSH | key-only | 禁密码登录 |
| 定期备份 | 每日 | 异地存储 |
| 日志轮转 | logrotate | 30天保留 |
| Nginx 限流 | `limit_req` | 防刷防爬 |
| CORS 白名单 | `MU_CORS_ALLOW_ORIGINS` | 生产必须设置 |

---

## 📞 支持与升级

- 代码仓库：https://github.com/zhongjinmuai-lang/Kiro
- 当前分支：`MuAgent-zupu`
- 问题反馈：GitHub Issues
- 技术文档：`docs/` 目录下 10+ 份完整文档
