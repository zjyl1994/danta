# danta 蛋挞图床 — 技术方案

> 单人自用图床：上传→链接；后台倒序列表（预览+翻页）。两上传模式：原图直存 / 缩放至 2560 内转 WebP。统一存 R2，CDN 域名分离。无标签、无对外展示。
> 技术栈：Go 1.25 + Fiber v3 + GORM（SQLite）+ Cloudflare R2；前端 Vite + React + MUI；控制/数据平面分离；零 CGO。

## 1. 架构

```
控制平面（danta Go 服务，仅本机/反代内网访问）— 上传鉴权、原图直存/缩放+WebP、写 R2、SQLite 元数据、管理 API
        │ 写 (S3 API)
        ▼
数据平面（R2 公开桶 + 自定义域名 CDN）— 图片直读、CDN 缓存，读流量不经控制平面
```

- 仅用 R2 + CDN，不用 Image Resizing / Workers。
- **cdn_host 直连 R2 桶**：该域名下任何路径均按对象键处理（`/v/*` → 取对象 `v/...` → 404），**cdn_host 不挂 web 路由/SPA**；web 页面仅在控制平面域名。
- 查看/预览全走 CDN 直链（后台列表预览、浏览器直开），流量由数据平面承载。

## 2. ID 与 URL

- **ID = SQLite 自增主键（uint）**，仅作后端接口操作句柄（列表/删除/批量）。
- **ObjectKey = R2 对象键**（unique），URL = `https://{cdn_host}/{ObjectKey}`，零转发、不可变；导入时 ObjectKey 原样保留，链接不变。
- 新上传：ObjectKey = `{ulid}.{ext}`（扁平桶根，ULID 26 字符）：原图模式 `ext`=magic 识别的原始格式（jpg/png/gif/webp/bmp/avif）；压缩模式恒 `webp`。
- 导入：ObjectKey = 现有对象键原样（可任意命名/层级，如 `2023/08/foo.png`）。
- 上传时写 `Cache-Control: public, max-age={cache_max_age}, immutable` + 正确 `Content-Type`；`cache_max_age` 默认 86400（24h），可配。

## 3. 功能

- **上传**：`POST /api/upload`（multipart/form-data：`file` + `original` 布尔，省略=压缩模式）。每张图单独选模式，前端多文件并行上传（限并发）。鉴权：上传 Key（`Bearer <upload_key>`）**或**管理员 JWT；**不支持 `?key=`**。
  - **原图模式**（`original=true`）：magic 识别 + SHA-256 后**原字节直存 R2**，不解码/缩放/转码。
  - **压缩模式**（`original=false`）：解码 → 等比缩放至长边 ≤`resize_max_dim`（已达标保持尺寸，仅重编码）→ 转 WebP（动画→动画 WebP）→ 存 R2；无法解码（如 avif）**回退原图直存**（`original` 记 true）。
- **去重**：SHA-256(原始字节) 命中即返回已有链接，不转码、不写 R2、不新增记录（`original` 以原记录为准）；导入记录 `hash`=NULL 不参与去重。
- **流程**（单机上传锁内串行）：读+magic+SHA-256（锁外）→ 拿锁 → 查重（命中结束）→ 按模式处理 → 生成 ObjectKey（`{ulid}.{ext}`）→ 写 R2（失败直接报错）→ 写 DB（失败补偿删 R2 后报错）→ 释放锁。`Hash` 唯一索引作多实例兜底。
- **配置门控**：已设密码但未配 R2 或 `cdn_host` → `POST /api/upload` 拒绝（400 + `config_required`）；配置完成后自动恢复。
- **外链**：API 返回单个 `url`；五格式前端拼接——URL=`{url}`；Markdown=`![{name}]({url})`；Markdown 带链接=`[![{name}]({url})]({url})`；BBCode=`[img]{url}[/img]`；HTML=`<img src="{url}" alt="{name}">`（`name`=原始文件名）。
- **通用接口**：无私有协议兼容层；全走标准 REST + JSON + multipart + Bearer，README 给 ShareX / PicGo web-uploader / uPic 配置示例（JSONPath `url` / 正则）。
- **管理后台**：倒序列表（`created_at DESC`）+ 预览（`<img src=CDN 直链>`）+ 翻页（`?page=&size=`，size 上限 100）+ 单删 / 批量删除（同步 R2+DB，失败可重试）+ 批量复制外链。无标签/编辑/过滤。
- **统计**：图片总数 / 总存储（`SUM(size)`）/ 近 24h 新增 / 原图直存数（`COUNT(original=1)`）。
- **孤儿清理**：`POST /api/admin/cleanup`（设置页按钮）——`ListObjectsV2` 分页扫全桶，对象键与 DB 的 ObjectKey 精确比对，删除无主对象；先导入后清理。
- **迁移导入（R2→DB）**：只读 R2（List/Get），绝不 Put/Delete/改键。`ListObjectsV2` 分页（可限 `prefix`）→ 扩展名白名单（jpg/jpeg/png/gif/webp/bmp/avif）过滤 → 已存在跳过 → 写记录（ObjectKey=原键、size/LastModified 取 S3 元数据、mime 按扩展名映射、name=键 basename、original=true、hash 暂空）。可选 `fetch_dimensions`：下载解码取 width/height + 算存储字节 SHA-256 写 hash。先 `scan`（dry-run）后 `run`。
- **前端**：Vite+React+MUI。页面：Setup / Login / 仪表盘 / 上传（拖拽/粘贴/多文件/原图开关/结果面板预览+五格式复制）/ 管理（倒序列表+预览+翻页+单删/批量删+批量复制）/ 设置（迁移导入 scan→run、孤儿清理、R2/域名/缓存等）。SPA 路由仅前端代管，后端只处理 `/api/*`；dev 反代见 §7。

## 4. 数据模型（GORM / SQLite）

```go
type Image struct {
    ID        uint      `gorm:"primaryKey;autoIncrement"` // 操作句柄
    ObjectKey string    `gorm:"uniqueIndex"`              // R2 对象键（导入原键原样）；URL={cdn_host}/{ObjectKey}
    Name      string                                      // 文件名（上传=原始名；导入=键 basename；仅后台可见）
    Mime      string                                      // 实际存储 MIME
    Size      int64                                       // R2 存储字节数
    Width     int                                         // 压缩=缩放后；原图=原尺寸；导入默认 0（可 fetch）
    Height    int
    Hash      []byte    `gorm:"uniqueIndex"`              // SHA-256 原始字节，去重键；可空（导入未 fetch 为 NULL）
    Original  bool                                        // true=原图直存；false=缩放+WebP；导入默认 true
    CreatedAt time.Time `gorm:"index"`                    // 上传时间/导入=LastModified；倒序排序
}

type Setting struct {
    Key       string `gorm:"primaryKey"`
    Value     string                                      // JSON
    UpdatedAt time.Time
}
```

- R2 层公开，全量即后台列表，无对外门控。
- ObjectKey unique 承载 R2 键 → URL 与 CDN 直链一一对应；`hash`=NULL 不参与去重；ID 自增仅供接口操作。
- 单机上传锁串行化「查重→处理→写 R2→写 DB」；DB 存在即保证 R2 已写入。
- AutoMigrate 建表。

## 5. 鉴权

- **Setup**：`admin.password_hash` 空 = 未配置 → 302 `/setup`；`POST /api/setup` 仅初始化管理员密码（argon2id）+ 生成 `upload_key`（`master_secret` 首次建库生成）；完成后永久禁用；仅本机/内网。
- **管理员**：`POST /api/login` 仅密码 → JWT（HS256，`master_secret` 签名，exp 7 天）；Bearer 访问管理 API，前端存 `sessionStorage`；无 Cookie 无 CSRF。设置面板可改密码。
- **上传 Key**：独立于登录，后台生成/重置（1 个）；上传 Key 或管理员 JWT 二选一。
- CDN 直链免登录公开只读。

## 6. settings 键

| key | 默认 | 说明 |
|---|---|---|
| `admin.password_hash` | - | 空=setup 模式 |
| `master_secret` | 首次建库生成 | JWT 签名密钥源 |
| `upload_key` | 后台生成 | 上传 Key（1 个，可重置） |
| `cdn_host` | - | CDN 域名；迁移须与迁移前一致，否则旧链接失效 |
| `r2.endpoint`/`access_key_id`/`secret_access_key`/`bucket` | - | S3 凭证（明文存表） |
| `proxy_mode` | none | `none`/`local` |
| `max_upload_bytes`/`webp_quality` | 15728640/80 | 上传上限 / WebP 质量 |
| `resize_max_dim` | 2560 | 压缩模式缩放长边上限 |
| `cache_max_age` | 86400 | CDN 缓存秒数；滑块（1h/6h/1d/7d/30d/1y） |
| `security.login_fail_limit`/`login_fail_window`/`login_ban_seconds` | 5/900/900 | 登录失败封禁 |

setup 完成判定 = `admin.password_hash` 非空；配置缺 R2/`cdn_host` 时上传被门控。启动：`.env` → `DANTA_LISTEN`/`DANTA_DATA_DIR` → SQLite（首建生成 `master_secret`）→ settings 载入 → 启动。

## 7. API（控制平面）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/setup` | 仅 setup | 初始化管理员密码 |
| POST | `/api/login` | 无 | 密码登录→JWT（IP 封禁兜底） |
| POST | `/api/upload` | 上传 Key 或 JWT | multipart `file`+`original`；返回单个 `url` |
| GET | `/api/admin/images` | 会话 | 倒序分页 `?page=&size=` |
| GET | `/api/admin/stats` | 会话 | 统计 |
| POST | `/api/admin/images/delete` | 会话 | 删除 `{ids:[uint]}`（单删=单元素）；按 ID 查 ObjectKey → R2 DeleteObject + 删 DB |
| POST | `/api/admin/cleanup` | 会话 | 孤儿清理 |
| GET | `/api/admin/import/scan` | 会话 | 预扫描 dry-run |
| POST | `/api/admin/import/run` | 会话 | 导入 `{prefix?, fetch_dimensions?}`；只读 |
| GET | `/api/admin/settings` | 会话 | 读（secret 掩码） |
| POST | `/api/admin/settings` | 会话 | 写 |
| POST | `/api/admin/settings/test-r2` | 会话 | 测试 R2 连接 |
| GET | `/api/admin/upload-key` | 会话 | 查看上传 Key |
| POST | `/api/admin/upload-key` | 会话 | 重置上传 Key |

**路由**：后端仅处理 `/api/*`；`/`、静态资源（`/assets/*`、`/manifest.webmanifest`、`/sw.js`、`/icons/*`）直接返回，其余任意路径 SPA fallback 到 index.html。
**反代暴露面**：公网放行 `/`（SPA）+ `/api/*`；`/api/admin/*` 整段拦截或仅内网，JWT 第二道防线。

响应：
- 上传：`{ id, original, url, size, width, height, mime, created_at }`；≤`max_upload_bytes`，`BodyLimit` 同值；去重命中返回原记录 id/url。
- 统计：`{ images, total_size, uploads_24h, originals }`。
- 列表：`{ items:[{ id, objectkey, name, original, mime, size, width, height, created_at, url }], total, page, size }`（size≤100，`url` 供预览）。
- 导入 scan：`{ total, new, existing, ignored, items:[{ key, size, last_modified }] }`（前 ~100）。
- 导入 run：`{ imported, skipped, ignored, errors:[...] }`。

**一致性**：上传 R2 优先（DB 失败补偿删 R2）；删除同步（任一失败报错可重试）。
**通用性约定**：统一 JSON；**动词只保留 GET/POST——读/查询（幂等）用 GET，写操作（增删改/重置/清理/导入）用 POST**；错误 `{ code, message }`（`config_required`/`auth_failed`/`not_found` 等），状态码 400/401/404/413/429，成功 200；取图地址统一顶层 `url`。

```bash
curl -X POST https://<host>/api/upload -H "Authorization: Bearer <upload_key>" -F "file=@image.png"
curl -X POST https://<host>/api/upload -H "Authorization: Bearer <upload_key>" -F "file=@image.png" -F "original=true"
```

**Vite dev 反代**：后端 `DANTA_LISTEN=127.0.0.1:32682`，`vite.config.ts` 反代 `/api` → 后端；生产 `dist/` go:embed，同源。

## 8. IP 与登录防爆破

- `c.IP()`：`TrustProxy` 按 `proxy_mode`（`none`=RemoteAddr；`local`=信任 loopback + X-Forwarded-For）。
- 登录失败内存滑窗封禁（429）；setup 阶段 `/api/setup`、`/api/login` 挂 IP 封禁/限流。
- 中间件 `auth` / `rate` / `ipban`；rate 覆盖 `/api/upload`、`/api/login`、`/api/admin/cleanup`。

## 9. 目录结构

```
danta/
├── cmd/danta/main.go         # .env → env(端口/数据目录) → GORM+SQLite → settings → 启动
├── internal/
│   ├── settings/settings.go  # typed 配置：settings 表读写 + 内存缓存
│   ├── store/                # models(gorm) + images CRUD、分页
│   ├── storage/storage.go    # 对象存储接口（Put/Delete/List/Get/DeleteBatch）
│   ├── storage/r2.go         # R2 (S3 兼容) 实现
│   ├── imageproc/proc.go     # 识别 / 缩放 / WebP 转码（含动画）
│   ├── idgen/idgen.go        # ULID
│   └── server/               # Fiber v3 路由
│       ├── middleware/       # auth / rate / ipban
│       └── handlers/
├── web/                      # Vite + React + MUI（开发反代 /api → 后端；build 产物 go:embed）
│   ├── index.html  vite.config.ts  package.json
│   └── src/                  # main.tsx App.tsx api/ pages/ components/
├── .env.example              # DANTA_LISTEN / DANTA_DATA_DIR
└── Makefile                  # build / web(dev|build) / test
```

## 10. 环境变量与部署

```bash
DANTA_LISTEN=127.0.0.1:32682  # 默认 127.0.0.1:32682
DANTA_DATA_DIR=./data         # SQLite 所在
```

- 数据平面：R2 公开桶 + 绑定域名。
- 控制平面：设上面两个变量（或 `.env`）后跑 `./danta`，浏览器走 `/setup`。
- `proxy_mode`：直连 `none` / 本机反代 `local`（反代供 HTTPS）。
- 迁移：旧对象拷入 R2 桶 → `cdn_host` 设迁移前域名 → 后台导入 → 可选孤儿清理。
- 备份 = 拷 db 文件（R2 对象由桶内副本保证）。

## 11. 里程碑

- **M1 骨架**：`.env` + GORM/settings 初始化 + Setup 流程 + proxy_mode/c.IP() + 路由 + R2 连通性 + Vite/React/MUI 脚手架 + dev 反代
- **M2 上传链路**：上传 Key 鉴权 + magic 识别 + 去重（单机上传锁内）+ **原图直存 / 缩放转 WebP（含动画、解码失败回退）** + 写 R2 + 写 DB + 单 url 外链
- **M3 管理面**：JWT 会话 + 登录封禁、**倒序列表 + 预览 + 翻页**、单删/批量删除、批量复制、统计仪表盘、上传 Key、设置面板
- **M4 迁移与打磨**：扫描导入（R2→DB，scan/run、对象键只读保留、可选 fetch 尺寸）、孤儿清理、限流、缓存头校验、README/部署文档（含 ShareX/PicGo web-uploader/uPic 接入配置示例）

— 各里程碑交付以 `make test`（settings/store/handlers 单测）与 lint 通过为准。

## 12. 明确不做

多用户/开放上传、多上传 Key、标签/随机图 API/对外瀑布流、图片编辑/水印/鉴黄、多格式输出（压缩模式仅 WebP，原图模式按原始格式）、按需缩放/动态处理、非图片文件（视频等）、图床协议兼容层（sm.ms 等专用协议一律不做，走通用 REST）、**跨桶/多存储迁移**（导入仅面向当前 R2 桶）、Presigned URL 直传、**私有桶/直链鉴权**（保持公开桶，`{cdn_host}/{id}` 直链始终可访问）。
