# danta

一个基于 Go、React 和 Cloudflare R2 的轻量图片托管服务。

## 特性

- 图片上传、去重和管理
- 普通模式自动转换为 WebP，并限制图片尺寸
- 原图直存模式
- Cloudflare R2 / S3 兼容对象存储
- 管理员登录、登录会话管理和上传令牌
- 登录失败限制和请求限流
- 管理页面支持图片删除、批量操作、R2 连通性测试和失效记录清理
- 单个 Go 二进制运行，前端生产构建产物会嵌入二进制

> 项目目前处于早期阶段。生产环境使用前，请先完成备份、HTTPS 和对象存储权限配置。

## 技术栈

- 后端：Go 1.25、Fiber v3、GORM、SQLite
- 前端：React、TypeScript、Vite、MUI
- 对象存储：Cloudflare R2 或其他 S3 兼容服务
- 许可证：AGPL-3.0，见 [LICENSE](LICENSE)

## 快速开始

### 环境要求

- Make
- Go 1.25+
- Node.js 20+
- pnpm 9+

### 准备配置和依赖

```bash
cp .env.example .env
make web-install
```

`.env` 默认配置为：

```dotenv
DANTA_LISTEN=127.0.0.1:8080
DANTA_DATA_DIR=./data
```

不要公开 `.env`、`data/` 或生产数据库。R2 凭据在首次登录后的管理页面配置，不需要写入 `.env`。

### 构建并运行

```bash
make build
make run
```

浏览器打开 <http://127.0.0.1:8080>。首次启动时访问 `/setup` 设置管理员密码，密码至少 8 位。

### 本地开发

终端一启动后端：

```bash
make run
```

终端二启动 Vite：

```bash
make web-dev
```

访问 Vite 输出的地址。开发服务器会把 `/api` 请求转发到 `127.0.0.1:8080`。

常用检查命令：

```bash
make test
make vet
```

## 首次配置对象存储

登录管理页面后，在“设置”中配置“存储与域名”：

1. 在 R2 或 S3 兼容服务中创建存储桶和访问密钥。
2. 为存储桶配置公开访问域名或 CDN 自定义域名。
3. 填写以下字段：
   - `访问域名`：只填写域名，例如 `img.example.com`，不要带 `https://` 或路径。
   - `存储服务地址`：例如 `https://<account-id>.r2.cloudflarestorage.com`。
   - `Access Key ID`
   - `访问密钥`
   - `存储桶`
4. 点击“保存”，再点击“测试连接”。

danta 返回的图片地址是 `https://<访问域名>/<object-key>`。图片访问由对象存储/CDN 负责，danta 后端不会代理图片内容，因此访问域名必须能够公开读取对象。

R2 凭据会保存在 SQLite 数据库中。请严格保护数据目录权限；不要给无关用户读取数据库的权限。

## 生产部署

下面以 Debian/Ubuntu、Nginx 和 systemd 为例。域名、证书和 R2 账号请替换为自己的配置。

### 1. 创建运行用户和目录

```bash
sudo useradd --system --home /var/lib/danta --shell /usr/sbin/nologin danta
sudo install -d -o danta -g danta -m 750 /var/lib/danta
sudo install -d -o danta -g danta -m 700 /var/lib/danta/data
```

### 2. 构建二进制

在项目目录执行：

```bash
make web-install
make build
```

安装二进制：

```bash
sudo install -o root -g root -m 755 bin/danta /usr/local/bin/danta
```

### 3. 创建运行配置

```bash
sudo tee /var/lib/danta/.env >/dev/null <<'EOF'
DANTA_LISTEN=127.0.0.1:8080
DANTA_DATA_DIR=/var/lib/danta/data
EOF

sudo chown danta:danta /var/lib/danta/.env
sudo chmod 600 /var/lib/danta/.env
```

该文件只包含监听地址和数据库目录，不要在其中写入 R2 密钥。R2 配置在管理页面保存到数据库。

### 4. 安装并启动 systemd 服务

```bash
sudo install -o root -g root -m 644 deploy/danta.service /etc/systemd/system/danta.service
sudo systemctl daemon-reload
sudo systemctl enable --now danta
sudo systemctl status danta
```

查看日志：

```bash
sudo journalctl -u danta -f
```

服务默认只监听本机 `127.0.0.1:8080`，公网访问应通过反向代理提供 HTTPS。

### 5. 配置 Nginx 反向代理

安装 Nginx 后创建站点配置，例如 `/etc/nginx/sites-available/danta`：

```nginx
server {
    listen 80;
    server_name img.example.com;

    client_max_body_size 64m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

启用配置并检查：

```bash
sudo ln -s /etc/nginx/sites-available/danta /etc/nginx/sites-enabled/danta
sudo nginx -t
sudo systemctl reload nginx
```

然后使用 Certbot 或其他方式为域名配置 HTTPS。启用反向代理后，登录管理页面进入“设置 → 安全”，把“代理模式”改为“本机反向代理”，保存后重新登录。

### 6. 首次生产初始化

1. 打开 `https://img.example.com/setup`。
2. 设置管理员密码。
3. 登录管理页面。
4. 配置 R2、公开访问域名和上传限制。
5. 在“上传令牌”中创建令牌。令牌明文只在创建时显示一次，请立即保存。
6. 上传一张测试图片，确认返回的 CDN 地址可以访问。

## API 示例

创建上传令牌后，可以使用令牌上传图片：

```bash
curl -X POST https://img.example.com/api/upload \
  -H 'Authorization: Bearer <upload-token>' \
  -F 'file=@image.png'
```

默认会压缩为 WebP。需要保存原图时：

```bash
curl -X POST https://img.example.com/api/upload \
  -H 'Authorization: Bearer <upload-token>' \
  -F 'file=@image.png' \
  -F 'original=true'
```

接口返回 JSON，其中 `url` 是图片公开访问地址。完整路由可参考 [internal/server/server.go](internal/server/server.go)。

## 数据与备份

默认数据库路径为 `./data/danta.db`，生产部署中推荐使用 `/var/lib/danta/data`。SQLite 使用 WAL 模式，备份前建议先停止服务：

```bash
sudo systemctl stop danta
sudo tar -C /var/lib/danta -czf /var/backups/danta-$(date +%F).tar.gz data
sudo systemctl start danta
```

数据库只保存图片元数据和配置，图片对象保存在 R2。恢复时必须同时恢复数据库和对应的 R2 对象。

## 安全注意事项

- 不要公开 `data/`、`.env`、R2 密钥或上传令牌。
- 生产环境使用 HTTPS，不要直接把 8080 暴露到公网。
- 数据目录中包含管理员密码哈希、JWT 签名密钥和 R2 凭据，应限制为运行用户可读。
- R2 存储桶需要能够通过配置的访问域名公开读取；不要把管理接口和对象存储凭据暴露给访客。
- 上传令牌支持过期和吊销，建议按用途创建短期令牌。
- 升级前先备份数据库和 R2 对象。

## 许可证

danta 使用 GNU Affero General Public License v3.0，详见 [LICENSE](LICENSE)。
