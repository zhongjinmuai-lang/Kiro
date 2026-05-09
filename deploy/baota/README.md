# 🎋 宝塔面板部署目录

## 结构

```
deploy/baota/
├── README.md       # 本文件
├── setup.sh        # 半自动部署脚本
└── nginx/          # 自动生成的 Nginx 配置（运行 setup.sh 后产生）
    ├── mu-developer.conf  （开发商角色部署后生成）
    ├── mu-provider.conf   （服务商角色部署后生成）
    └── mu-customer.conf   （终端客户角色部署后生成）
```

## 使用方法

在宝塔面板所在的服务器上运行：

```bash
cd /www/wwwroot/mu-framework
bash deploy/baota/setup.sh [developer|provider|customer]
```

脚本会：
1. 检查环境（Go/Node/PG/Redis/Nginx）
2. 交互询问密码、域名、开发商 API 地址等
3. 自动执行数据库迁移（仅 developer 角色）
4. 编译后端 + 构建前端
5. 生成 PM2 ecosystem 配置
6. 生成 Nginx 站点配置到 `deploy/baota/nginx/`

运行完成后按提示：
- `pm2 start ecosystem.config.js && pm2 save`（仅 developer）
- 将 `deploy/baota/nginx/*.conf` 复制到宝塔站点配置中
- 宝塔申请 Let's Encrypt SSL

## 更多

完整手册：[`docs/baota-deployment.md`](../../docs/baota-deployment.md)
环境要求：[`docs/environment-requirements.md`](../../docs/environment-requirements.md)
