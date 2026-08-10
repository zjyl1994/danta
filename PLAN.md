# danta 蛋挞图床 — 技术方案

> 单人自用图床：快速上传 + 一键复制外链（论坛发帖）、收集展示、按标签随机图 API。
> 技术栈：Go 1.25 + Fiber v3 + GORM（SQLite driver）+ Cloudflare R2；前端 Vite + React + Material UI；控制/数据平面分离；全链路零 CGO。

## 1. 架构

```
控制平面（danta Go 服务，仅本机/反代内网访问）
  上传鉴权(上传Key)、WebP 转码、写 R2、SQLite 元数据、管理/随机 API
        │ 写 (S3 API)
        ▼
数据平面（R2 公开桶 + 自定义域名 CDN）—— 图片直读、CDN 缓存，读流量不经过控制平面
```

- 只用 CF 的 **R2 + CDN**，不用 Image Resizing / Workers 等其它产品。
- 查看/随机接口 302 到 CDN 域名，流量由数据平面承载。
- 技术栈：Fiber v3（`gofiber/fiber/v3`，路由/中间件/limiter/ProxyHeader）、**GORM**（`gorm.io/gorm` + `gorm.io/driver/sqlite`，数据访问层）、`aws-sdk-go-v2`（S3 兼容端点）、`deepteams/webp`（纯 Go WebP 编解码+动画，零 CGO）、`oklog/ulid/v2`、`golang-jwt/jwt/v5`、`joho/godotenv`、`x/crypto/argon2`、`github.com/zjyl1994/cap-go`（Cap.js 后端，setup/login 的 PoW 保护）。

## 2. ID 与 URL

- ID = **ULID**，26 字符，作 TEXT 主键（`WITHOUT ROWID`）与 R2 对象键；时间有序（B-tree 局部性好），随机位防枚举。
- 对象键 = `{id}.webp`，桶根扁平；URL = `https://{cdn_host}/{id}.webp`，零转发、不可变。
- 所有上传**统一转 WebP**；对象带 `Content-Type: image/webp`、`Cache-Control: public, max-age=31536000, immutable`。

## 3. 功能

- **上传**：Web 拖拽/粘贴/选择 + `POST /api/upload`（multipart 或裸流）；需上传 Key（仅 `Authorization: Bearer <key>`，**不支持 `?key=`**，避免密钥进 URL/日志/历史）。可带 `tags`（可选）与 `remark`（可选，可写可不写）。流程：magic 识别（选解码器 + 拒绝非图片）→ 算原始字节 SHA-256 → **查重**（命中已有 hash → 返回相同 URL，tags/remark 合并进原记录，**不转码不写 R2**，直接结束）→ 未命中 → 解码 → 统一转 WebP（GIF/动画 → 动画 WebP，异常退首帧）→ 写 R2 → 入库。**不保留原图**。
- **配置完备性门控**：仅初始化账号、未配置 R2（endpoint/ak/sk/bucket）或 `cdn_host` 时，`POST /api/upload` 与 `GET /random/{tag}` 直接拒绝（如 400 + `config_required` 提示"请先在设置面板配置 R2 与 CDN 域名"），防止产生无法生成外链/无法落盘的脏数据；配置完成后自动恢复，无需重启。
- **外链**：响应返回 URL / Markdown / BBcode / HTML 四格式，一键复制；域名取 `cdn_host`。
- **管理**：仪表盘统计（见下）、列表、标签/日期过滤、删除（级联 R2 对象 + 表）、批量复制；设置页（`cdn_host`、上传 Key、S3/R2 配置 + 测试连接、proxy_mode、**改管理员密码**）。
- **统计**：管理页仪表盘四个数字指标——**图片总数**、**总存储占用**、**标签数**、**近 24h 新增**。全部来自 SQLite 聚合查询（`COUNT`/`SUM(size)`/`COUNT(tags)`/`created_at >= now-24h`），不依赖 CDN 访问日志；访问量不可统计（图片走 CDN 直读，不经过控制平面）。
- **随机**：`GET /random/{tag}`（公开）302 到 CDN；只含带标签图，无标签不参与；标签不存在 404；`random.enabled=false` 时统一 404。
- **前端**：Vite + React + Material UI（`@mui/material`）；页面：Setup / Login / 仪表盘（统计卡片）/ 上传（拖拽/粘贴/外链复制）/ 管理（列表/过滤/编辑/设置）；PWA（manifest + sw.js，外壳缓存优先、API 网络优先）；HTTP 直连仅失去安装/离线外壳，其余正常。开发期 `vite dev` 反代后端（见 §7），构建产物 go:embed。Setup 与 Login 表单内嵌 **Cap.js 组件** `<cap-widget data-cap-api-endpoint="/cap/">`，`solve` 事件拿到一次性 redeem token 后随请求提交（`X-Cap-Token`）。
- **Setup/登录**：见 §5。

## 4. 数据模型（GORM / SQLite）

```go
type Image struct {
    ID        string    `gorm:"primaryKey;size:26"`        // ULID
    Name      string    `gorm:"index"`                     // 原始文件名（来自上传，非用户编辑）
    Remark    string                                        // 用户备注/描述（管理页可编辑，可空）
    Mime      string
    Size      int64                                        // WebP 字节数（R2 实际占用）
    Width     int
    Height    int
    Hash      []byte    `gorm:"uniqueIndex"`               // SHA-256(BLOB)，原始字节哈希，去重键
    RandID    float64   `gorm:"column:rand_id"`            // [0,1) 随机，插入时生成，按标签随机排序用
    CreatedAt time.Time `gorm:"index"`                     // created_at DESC
    Tags      []*Tag    `gorm:"many2many:image_tags"`      // 关联标签
}

type Tag struct {
    ID     uint     `gorm:"primaryKey"`
    Name   string   `gorm:"uniqueIndex;collate:NOCASE"`   // trim 后入库，不区分大小写
    Images []*Image `gorm:"many2many:image_tags"`
}

type Setting struct {
    Key       string `gorm:"primaryKey"`
    Value     string                                       // value JSON
    UpdatedAt time.Time
}
```

- `gorm:"many2many:image_tags"` 自动建连接表 `image_tags`（`image_id`/`tag_id` 复合主键 + tag_id 索引），DBCollation 处理大小写。
- 标签写：trim 后**过滤空串**再按名 upsert `tags` + 关联 `image_tags`；改名一处生效；孤立标签 `DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM image_tags)`。
- 所有图片公开，无 `is_public`。
- `Hash` = 上传**原始字节**的 SHA-256（32 字节），存 **BLOB**（GORM `[]byte` → SQLite BLOB）：字节精确等值比较，不受 TEXT collation 干扰，唯一索引查重性能好。**前置去重**：查重在解码/转码之前，SHA-256 仅几十毫秒（≫ 快于 WebP 转码），重复上传几乎零成本——命中已有 hash → 返回相同 URL，tags 并入（取并集）、remark 非空追加，不新增记录、不转码、不写 R2。代价：hash 与 R2 存的对象字节（WebP）不一致、不做跨源格式去重，仅字节级去重（单人重复传同文件的场景最优）。
- **并发去重竞态**：并发上传同一文件会同时查重未命中，入库时撞 `hash` 唯一索引——捕获 GORM 唯一约束错误（`gorm.ErrDuplicatedKey`），转查重返回已有记录，不返回 500。
- AutoMigrate 建表；`WITHOUT ROWID` 由 SQLite driver 按需支持，不强制。

### 按标签随机（rand_id 法）
主查询（命中 idx_tag，绝大多数请求）；无结果（r 大于该标签全部 rand_id）走 `ORDER BY RANDOM()` 均匀回绕，避免固定返回同一张：

```go
db.Joins("JOIN image_tags t ON t.image_id = images.id").
    Where("t.tag_id = ? AND images.rand_id > ?", tagID, r).
    Order("images.rand_id").First(&img)
// 回绕：
db.Joins("JOIN image_tags t ON t.image_id = images.id").
    Where("t.tag_id = ?", tagID).
    Order("RANDOM()").First(&img)
```

## 5. 鉴权

- **Setup（一次性）**：`admin.password_hash` 为空 = 未配置 → 任意访问 302 `/setup`；`POST /api/setup` **需先完成 Cap.js PoW**（cap-go 签发 challenge，浏览器解出后提交一次性 `X-Cap-Token`，服务端校验并消费，兼作 CSRF 防护——PoW token 跨站无法伪造）；**仅初始化管理员账号**（用户名 + 密码 argon2id），并 `crypto/rand` 生成 `master_secret` 与 `upload_key`；其余配置（`cdn_host`、R2/S3、`proxy_mode`、上传上限、质量等）**不在此处填写，登录后进设置面板配置**。完成后 `/api/setup` 永久禁用（忘密码只能改 SQLite）；**setup 完成判定以 `admin.password_hash` 非空为唯一信号**（不设独立的 `setup_done` 键，避免双信号不一致）。
- **管理员**：`POST /api/login` **同样需 PoW**（cap-go，防爆破，与登录失败封禁叠加）；成功后 → JWT（HS256，`master_secret` 签名，sub=用户名，exp 默认 **7 天**）；Bearer 访问管理 API，前端存 `sessionStorage`。因凭据非浏览器自动携带（不用 Cookie），**无 CSRF 风险**，删除类接口同理。**`/api/logout` 无状态**：仅前端删除 token 即失效（无服务端黑名单，token 到期前仍有效）。登录后可在设置面板**修改管理员密码**（旧密码校验 + 新密码 argon2id 重写 `admin.password_hash`）。
- **上传 Key**：独立于登录，后台生成/重置（首版 1 个）；上传接口 `Authorization: Bearer <key>`。
- 数据平面公开只读。

## 6. settings 键

| key | 默认 | 配置入口 | 说明 |
|---|---|---|---|
| `admin.username` / `admin.password_hash` | admin / - | setup/设置面板 | 空密码哈希 = 进入 setup 模式；登录后可在面板改密码 |
| `master_secret` | setup 生成 | setup | JWT 签名等一切密钥的数据源 |
| `upload_key` | 后台生成 | setup/后台 | 上传 Key（1 个，可重置） |
| `cdn_host` | - | 设置面板 | 图片 CDN 域名（外链、随机 302 前缀），登录后配置 |
| `r2.endpoint` / `access_key_id` / `secret_access_key` / `bucket` | - | 设置面板 | S3 凭证（密钥明文存表，单人可接受），登录后配置 |
| `proxy_mode` | none | 设置面板 | 反代预设：`none`/`local`/`cloudflare` |
| `max_upload_bytes` / `webp_quality` | 15728640 / 80 | 设置面板 | 上传上限（默认 15MB）/ WebP 质量 |
| `random.enabled` | true | 设置面板 | 随机图开关 |
| `trusted_proxies` / `client_ip_header` | [] / X-Forwarded-For | 设置面板 | 高级覆盖（优先于预设） |
| `security.login_fail_limit` / `login_fail_window` / `login_ban_seconds` | 5 / 900 / 900 | 设置面板 | 登录失败封禁 |

setup 完成判定即 `admin.password_hash` 非空（无 `setup_done` 键）；`/api/setup` 在 setup 完成后返回 404/409 永久禁用。

**setup 只初始化**管理员账号 + 生成 `master_secret`/`upload_key`，其余键由登录后的**设置面板**写入；带默认值的可选配置无需 setup 参与，读取时缺键用默认值兜底，设置面板可覆盖。`cdn_host`/R2 配置缺失时上传与随机接口被门控拒绝（见 §3），设置面板正常可操作，配置完成后即恢复。

启动：`godotenv` 加载 `.env` → 读 `DANTA_LISTEN`/`DANTA_DATA_DIR` → 打开 SQLite → settings 载入内存 → 启动。运行期配置经 `PUT /api/settings` 修改即时生效。

## 7. API（控制平面）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/cap/challenge` `/cap/redeem` | 无 | Cap.js PoW：签发 challenge / 校验解算结果并签发一次性 redeem token |
| GET/POST | `/setup` `/api/setup` | 仅 setup+PoW | 首启配置页 / 写入（需 `X-Cap-Token`） |
| POST | `/api/login` `/api/logout` | PoW / 会话 | 登录（需 `X-Cap-Token`）→JWT / 注销（前端弃 token） |
| GET | `/api/me` | 会话 | 当前管理员信息（会话校验） |
| POST | `/api/upload` | 上传 Key | multipart/裸流，返回四格式外链 |
| GET | `/api/images` | 会话 | 分页 `?tag=&page=&size=`（结构见下） |
| GET | `/api/stats` | 会话 | 仪表盘统计：图片总数 / 总尺寸 / 标签数 / 近 24h 新增 |
| DELETE/PATCH | `/api/image/{id}` | 会话 | 删除（级联）/ 改 tags·name·remark（同步表） |
| GET | `/api/tags` | 会话 | 标签与计数 |
| GET/PUT | `/api/settings` | 会话 | 读（secret 掩码）/ 写 |
| POST | `/api/settings/test-r2` | 会话 | 测试 S3/R2 连接 |
| GET/POST | `/api/upload-key` | 会话 | 查看 / 重置上传 Key |
| GET | `/v/{id}` `/random/{tag}` | 无 | 查看页 / 302 随机图 |
| GET | `/` `/assets/*` `/manifest.webmanifest` `/sw.js` `/icons/*` | 无 | Vite 构建产物（go:embed，hash 文件名在 `/assets/*`），sw.js 根路径注册 |

上传响应：`{ id, urls:{url,markdown,bbcode,html}, size, width, height, mime }`。单文件默认 ≤15MB（`max_upload_bytes`），Fiber `BodyLimit` 同值；上传接口 `limiter` 限流；**命中去重**时返回原记录同 id/url，不产生新对象/新行。
统计响应：`{ images, total_size, tags, uploads_24h }`（图片总数 / 总尺寸字节 / 标签数 / 近 24h 新增），均为 SQLite 聚合，无访问量指标。
列表响应：`{ items:[{ id, name, remark, mime, size, width, height, created_at, tags:[...], urls:{url,markdown,bbcode,html} }], total, page, size }`；`size` 上限默认 100。
`random.enabled=false` 时 `GET /random/{tag}` 返回 404。
**一致性**：上传先写 R2 再入库，R2 成功但入库失败则尽力补偿删除该对象（失败记日志）；删除先删 R2 再删 DB，R2 删除失败返回错误且不删 DB 记录，避免孤儿对象/悬空引用。并发去重撞 `hash` 唯一索引（`gorm.ErrDuplicatedKey`）时捕获并转查重返回已有记录（见 §4）。

### Cap.js PoW 流程（cap-go）
- 密钥 = `master_secret`（≥16B，重启稳定）；scope = `danta` 绑定站点。
- 前端 `<cap-widget data-cap-api-endpoint="/cap/">` → `POST /cap/challenge`（c=50/s=32/d=4，TTL 10min）→ 浏览器解算 → `POST /cap/redeem` 校验（签名 nonce 一次性防重放）→ 返回 redeem token（TTL 20min，`scope:id:secret`）。
- 受保护接口（`POST /api/setup`、`POST /api/login`）要求请求头 `X-Cap-Token: <redeem>`，服务端 `GetDel` 一次性消费后放行。
- `/cap/challenge`、`/cap/redeem` **挂 rate 限流**，与 `/api/login` 行为一致（见 §8），防恶意刷 challenge / 暴力试 redeem。
- cap-go 自身无存储，nonce/已发 token 用**内存 store**（`SetNX`/`GetDel`，重启即清，单实例适用）；token 被消费后无论登录成功失败都需重新解 PoW。
- 前端组件版本：`@cap.js/widget` v0.1.56 + cap-go v1.0.0（capjs-core v0.1.2）。

### Vite dev server 反代（调试）
开发时后端跑 `DANTA_LISTEN=127.0.0.1:32682`，前端 `npm run dev`，`vite.config.ts` 反代到后端，无需 CORS：

```ts
export default defineConfig({
  server: {
    proxy: {
      '/api':  'http://127.0.0.1:32682',   // 所有 API 含 /api/upload 上传
      '/cap':  'http://127.0.0.1:32682',   // Cap.js PoW 端点
      '/setup': 'http://127.0.0.1:32682',
      '/v':    'http://127.0.0.1:32682',
      '/random': 'http://127.0.0.1:32682',
    },
  },
});
```

生产：`vite build` → `dist/` 由 Go `web` 包 `go:embed` 托管；生产环境后端同源服务，无需反代。

## 8. IP 与登录防爆破

- **c.IP()**（Fiber v3）：`TrustProxy=true` 且直连在信任列表才解析 `ProxyHeader`，从右向左跳过受信任代理、取首个非信任 IP（防伪造）。
- **proxy_mode**：
  - `none`：TrustProxy=false，`c.IP()`=RemoteAddr（直连）
  - `local`：信任 loopback（`127.0.0.1/8`、`::1`）+ X-Forwarded-For（本机 Caddy/Nginx 反代）
  - `cloudflare`：信任 CF IP 段 + CF-Connecting-IP；段启动时从 cloudflare.com/ips-v4|v6 拉取缓存，失败用内置快照
  - 高级覆盖 `trusted_proxies`/`client_ip_header` 用于多跳（CF→本机反代→app），信任链须含每个代理
- **登录封禁**：内存滑窗（IP→失败时间戳），窗口内 ≥limit 封禁 ban_seconds 返回 429，成功清零；封禁状态内存存（重启即清）。
- 中间件 `server/middleware/`：auth / rate / ipban。**rate 覆盖**：`/api/upload`、`/cap/challenge`、`/cap/redeem`、`/api/login`（与登录封禁叠加）。

## 9. 目录结构

```
danta/
├── cmd/danta/main.go         # .env → env(端口/数据目录) → GORM+SQLite → settings → 启动
├── internal/
│   ├── settings/settings.go  # typed 配置：settings 表读写 + 内存缓存
│   ├── store/                # models(gorm) + images/tags CRUD、随机查询
│   ├── storage/storage.go    # 对象存储接口（Put/Delete/Stat）
│   ├── storage/r2.go         # R2 (S3 兼容) 实现
│   ├── imageproc/proc.go     # 识别 / WebP 转码（含动画）
│   ├── idgen/idgen.go        # ULID
│   ├── capgo/                # Cap.js PoW（cap-go 封装：challenge/redeem 路由 + Require 中间件 + 内存 store）
│   └── server/               # Fiber v3 路由
│       ├── middleware/       # auth / rate / ipban
│       └── handlers/
├── web/                      # Vite + React + MUI（开发反代 /api → 后端；build 产物 go:embed）
│   ├── index.html  vite.config.ts  package.json
│   ├── public/               # manifest.webmanifest sw.js icons/
│   └── src/                  # main.tsx App.tsx api/ pages/ components/
├── .env.example              # DANTA_LISTEN / DANTA_DATA_DIR
└── Makefile                  # build / web(dev|build) / test
```

## 10. 环境变量与部署

```bash
DANTA_LISTEN=127.0.0.1:32682  # 默认 127.0.0.1:32682
DANTA_DATA_DIR=./data         # SQLite 所在
```

- **数据平面**：R2 公开桶 + 绑定域名 `img.example.com`（自动 CDN 缓存）。
- **控制平面**：设上面两个变量（或 `.env`）后跑 `./danta`；浏览器打开自动跳 `/setup` 一次性配置，之后账号密码登录。
- **三种形态**（对应 `proxy_mode`）：直连 `:32682`+`none`；本机反代 `127.0.0.1:32682`+`local`（反代供 HTTPS 满足 PWA）；CF 反代 `cloudflare`（多跳用覆盖项）。
- 备份 = 拷数据目录 db 文件；R2 有 CF 冗余。

## 11. 里程碑

- **M1 骨架**：`.env` + GORM/settings 初始化 + Setup 流程（302/`/api/setup`/**cap-go PoW**/argon2id，仅初始化管理员账号）+ **cap-go 集成（challenge/redeem + `X-Cap-Token` 中间件）** + proxy_mode/c.IP() + 路由 + R2 连通性 + Vite/React/MUI 脚手架 + dev 反代
- **M2 上传链路**：上传 Key 鉴权 + 识别 → 转 WebP（含动画）→ 写 R2 → 入库 → 短链
- **M3 外链与管理**：JWT 会话 + 登录封禁、`/v/{id}`、四格式复制、列表/删除/改标签、**仪表盘统计（`/api/stats`）**、上传 Key、PWA 外壳
- **M4 按标签随机**：`/random/{tag}`（rand_id 关联排序）
- **M5 打磨**：限流、缓存头校验、README/部署文档

— 各里程碑交付以 `make test`（settings/store/handlers 单测）与 lint 通过为准。

## 12. 明确不做

多用户/开放上传、多上传 Key、图片编辑/水印/鉴黄、保留原图/多格式输出、按需缩放/动态处理、图床协议兼容层（客户端直连原生 API 即可）、多存储后端、Presigned URL 直传。
