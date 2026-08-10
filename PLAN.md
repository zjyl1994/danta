# danta 蛋挞图床 — 技术方案

> 单人自用图床：快速上传 + 一键复制外链、收集展示、按标签随机图 API。
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
- 技术栈：Fiber v3（`gofiber/fiber/v3`，路由/中间件/limiter/ProxyHeader）、**GORM**（`gorm.io/gorm` + `gorm.io/driver/sqlite`，数据访问层）、`aws-sdk-go-v2`（S3 兼容端点）、`deepteams/webp`（纯 Go WebP 编解码+动画，零 CGO）、`oklog/ulid/v2`、`golang-jwt/jwt/v5`、`joho/godotenv`、`x/crypto/argon2`。

## 2. ID 与 URL

- ID = **ULID**，26 字符，作 TEXT 主键（`WITHOUT ROWID`）与 R2 对象键；时间有序、随机位防枚举。
- 对象键 = `{id}.webp`，桶根扁平；URL = `https://{cdn_host}/{id}.webp`，零转发、不可变。
- 所有上传**统一转 WebP**；对象带 `Content-Type: image/webp`、`Cache-Control: public, max-age=31536000, immutable`。

## 3. 功能

- **上传**：Web 拖拽/粘贴/选择 + `POST /api/upload`（multipart/form-data，文件字段 + 可选 `tags`/`remark` 字段）；鉴权 `Authorization` **二选一**：上传 Key（`Bearer <upload_key>`）**或管理员 JWT**——脚本/外部工具用 Key，管理页直传用 JWT；**不支持 `?key=`**（避免密钥进 URL/日志/历史）。流程：读取 + magic 识别（选解码器 + 拒绝非图片）+ 算原始字节 SHA-256（纯计算，锁外）→ **获取单机上传锁（全局互斥，串行化后续流程）** → 查重（命中已有 hash → 返回相同 URL，tags/remark 合并进原记录，**不转码不写 R2**，直接结束）→ 未命中 → 解码 → 统一转 WebP（GIF/动画 → 动画 WebP，异常退首帧）→ 生成 ULID → **写 R2（PutObject {id}.webp）** → R2 失败则直接返回错误 → **写 DB** → DB 失败则**补偿删除 R2**后返回错误 → 释放锁。**不保留原图**。
- **配置完备性门控**：已设置密码但未配置 R2（endpoint/ak/sk/bucket）或 `cdn_host` 时，`POST /api/upload`、`GET /api/v/{id}`、`GET /api/random/{tag}`、`GET /api/gallery` 与 `GET /api/gallery/{tag}` 直接拒绝（400 + `config_required` 提示"请先在设置面板配置 R2 与 CDN 域名"）；配置完成后自动恢复，无需重启。
- **外链**：响应返回 URL / Markdown / BBcode / HTML 四格式，一键复制；域名取 `cdn_host`。
- **管理**：仪表盘统计（见下）、列表、标签/日期过滤、删除（**同步删除**：R2 DeleteObject + 物理删 DB 记录，失败返回错误可重试；**删除后顺带清理孤立标签**）、批量复制、**标签 public 开关**（决定对外展示）；设置页（`cdn_host`、上传 Key、S3/R2 配置 + 测试连接、proxy_mode、**改管理员密码**）。`cdn_host` 变更会使所有已分发外链失效（URL 由 `cdn_host` 动态推导），前端输入框带**红字警告**提示，保存前弹**二次确认**对话框。
- **统计**：管理页仪表盘四个数字指标——**图片总数**、**总存储占用**、**标签数**、**近 24h 新增**；全部来自 SQLite 聚合（`COUNT`/`SUM(size)`/`COUNT(tags)`/`created_at >= now-24h`），无访问量统计。
- **对外展示**：公开瀑布流 `GET /api/gallery`（免登录，全部 public 标签图）与 `GET /api/gallery/{tag}`（按标签筛选），以及按标签随机 `GET /api/random/{tag}`。**标签可标记 `public`**（默认关闭，管理页开关）；图片**任一标签 public 即对外可见**，无标签图永不对外。`/api/gallery/{tag}`、`/api/random/{tag}` 仅对 public 标签生效（非 public/不存在统一 404）。**公开接口响应中的 `tags` 字段只返回该图的 public 标签**（private 标签名不出现，防止经公开接口泄露未公开标签名；`/api/random/{tag}` 为 302 无 body，无此问题）。**门控仅作用于控制平面接口**：R2 是公开桶，`{cdn_host}/{id}.webp` 直链与 `/api/v/{id}` 不按 public 过滤，仍可访问。
- **前端**：Vite + React + Material UI（`@mui/material`）；页面：Setup / Login / 仪表盘（统计卡片）/ 上传（拖拽/粘贴/外链复制）/ 管理（列表/过滤/编辑/设置）/ **公开页面 `/v/{id}`（查看）与 `/gallery`、`/gallery/{tag}`（瀑布流，数据来自 `/api/v/{id}`、`/api/gallery[/{tag}]`，免登录，仅 public 标签图）**；**页面路由全部由前端（SPA）代管，仅 `/api/*` 由后端处理**。PWA（manifest + sw.js，外壳缓存优先、API 网络优先）；HTTP 直连仅失去安装/离线外壳，其余正常。开发期 `vite dev` 反代 `/api` → 后端（见 §7），构建产物 go:embed。
- **Setup/登录**：见 §5。

## 4. 数据模型（GORM / SQLite）

```go
type Image struct {
    ID        string    `gorm:"primaryKey;size:26"`        // ULID
    Name      string    `gorm:"index"`                     // 原始文件名（来自上传，非用户编辑）
    Remark    string                                        // 用户备注/描述（管理页可编辑，可空）
    Mime      string                                        // 转码后的 MIME（统一 WebP，恒为 image/webp，与 R2 对象一致）
    Size      int64                                        // WebP 字节数（R2 实际占用）
    Width     int
    Height    int
    Hash      []byte    `gorm:"uniqueIndex"`               // SHA-256(BLOB)，原始字节哈希，去重键
    CreatedAt time.Time `gorm:"index"`                     // created_at DESC
    Tags      []*Tag    `gorm:"many2many:image_tags"`      // 关联标签
}

type Tag struct {
    ID     uint     `gorm:"primaryKey"`
    Name   string   `gorm:"uniqueIndex;collate:NOCASE"`   // trim 后入库，不区分大小写
    Public bool                                            // public 标签 → 参与对外展示（瀑布流/随机）；图片任一 public 标签即公开
    Images []*Image `gorm:"many2many:image_tags"`
}

type Setting struct {
    Key       string `gorm:"primaryKey"`
    Value     string                                       // value JSON
    UpdatedAt time.Time
}
```

- `gorm:"many2many:image_tags"` 自动建连接表 `image_tags`（`image_id`/`tag_id` 复合主键 + tag_id 索引），DBCollation 处理大小写。
- 标签写：trim 后**过滤空串**再按名 upsert `tags` + 关联 `image_tags`；改名一处生效。**孤立标签清理**：删除图片时在事务内顺带执行 `DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM image_tags)`（标签下无图即删）。标签 `public` 由管理接口开关（默认 false，见 §7），不影响标签名 upsert 的合并语义（同名标签 public 以首次 upsert 为准，管理页显式开关）。
- 图片在 R2 层公开（公开桶直链可访问），**对外展示面由标签 `public` 门控**（见 §3），无 `is_public` 字段。
- `Hash` = 上传**原始字节**的 SHA-256（32 字节），存 **BLOB**（GORM `[]byte` → SQLite BLOB），字节精确等值比较。**前置去重**：查重在解码/转码之前——命中已有 hash → 返回相同 URL，tags 并入（取并集）、remark 非空追加，不新增记录、不转码、不写 R2；未命中才转码入库。代价：hash 与 R2 存的对象字节（WebP）不一致，仅字节级去重、不做跨源格式去重。
- **单机上传锁 + 唯一索引兜底**：上传流程为**R2 优先**，且查重 → 转码 → 写 R2 → 写 DB **整段在全局互斥锁内串行执行**——同 hash 并发请求排队，后到者锁内查重命中即返回，不转码不写 R2，无竞态、无败方浪费、无孤儿对象。锁外仅做读取、magic 识别与 SHA-256 计算（纯计算，无共享状态）。`Hash` 列唯一索引保留作最终保险（仅多实例等锁覆盖不到的场景才可能触发）。流程：查重（hash 在 DB？）→ 命中则返回（DB 记录存在即保证 R2 对象已写入）→ 未命中则转码 → 生成 ULID → PutObject({id}.webp) → R2 失败则直接返回错误 → 插入 DB 记录 → DB 失败则补偿 DeleteObject({id}.webp) 后返回错误。DB 记录存在即保证 R2 对象存在，不存在返回不存在的 URL 的情况。
- AutoMigrate 建表；`WITHOUT ROWID` 由 SQLite driver 按需支持，不强制。

### 按标签随机
单人场景单标签最多几千张图，直接 `ORDER BY RANDOM()`（`idx_tag` 索引定位 + join + 内存临时 B-tree 排序）。实测：单标签 5k 张 avg 1.6ms / p99 3ms，10k 张 avg 5ms，50k 张 avg 33ms，100k 张 avg 64ms——瓶颈在十万级，几千张场景无性能问题。

```go
db.Joins("JOIN image_tags t ON t.image_id = images.id").
    Where("t.tag_id = ?", tagID).
    Order("RANDOM()").First(&img)
```

## 5. 鉴权

- **Setup（一次性）**：`admin.password_hash` 为空 = 未配置 → 任意访问 302 `/setup`；`POST /api/setup` **仅初始化管理员密码**（argon2id），并 `crypto/rand` 生成 `upload_key`（`master_secret` 已在首次初始化 DB 时生成，见 §6）；其余配置（`cdn_host`、R2/S3、`proxy_mode`、上传上限、质量等）**不在此处填写，登录后进设置面板配置**。完成后 `/api/setup` 永久禁用（忘密码只能改 SQLite）；**setup 完成判定以 `admin.password_hash` 非空为唯一信号**（无独立 `setup_done` 键）。setup 仅本机/内网可访问，IP 封禁兜底防爆破（见 §8）。
- **管理员**：`POST /api/login` 仅密码（无用户名，单人无多用户）成功后 → JWT（HS256，`master_secret` 签名，exp 默认 **7 天**）；Bearer 访问管理 API，前端存 `sessionStorage`。因凭据非浏览器自动携带（不用 Cookie），**无 CSRF 风险**，删除类接口同理。登录防爆破由 IP 封禁承担（见 §8）。**`/api/admin/logout` 无状态**：仅前端删除 token 即失效（无服务端黑名单，token 到期前仍有效）。登录后可在设置面板**修改管理员密码**（旧密码校验 + 新密码 argon2id 重写 `admin.password_hash`）。
- **上传 Key**：独立于登录，后台生成/重置（首版 1 个）；上传接口 `Authorization: Bearer <upload_key>` **或管理员 JWT 二选一**。
- 数据平面公开只读。对外展示面（`/api/gallery`、`/api/random/{tag}`、`/api/v/{id}`、CDN 直链）免登录；标签 `public` 门控仅作用于控制平面接口（见 §3）。

## 6. settings 键

| key | 默认 | 配置入口 | 说明 |
|---|---|---|---|
| `admin.password_hash` | - | setup/设置面板 | 空 = 进入 setup 模式（仅密码，无用户名）；登录后可在面板改密码 |
| `master_secret` | 首次初始化 DB 生成 | DB 初始化 | JWT 签名等一切密钥的数据源（重启稳定） |
| `upload_key` | 后台生成 | setup/后台 | 上传 Key（1 个，可重置） |
| `cdn_host` | - | 设置面板 | 图片 CDN 域名（外链、随机 302 前缀），登录后配置 |
| `r2.endpoint` / `access_key_id` / `secret_access_key` / `bucket` | - | 设置面板 | S3 凭证（密钥明文存表），登录后配置 |
| `proxy_mode` | none | 设置面板 | 反代预设：`none`/`local` |
| `max_upload_bytes` / `webp_quality` | 15728640 / 80 | 设置面板 | 上传上限（默认 15MB）/ WebP 质量 |
| `security.login_fail_limit` / `login_fail_window` / `login_ban_seconds` | 5 / 900 / 900 | 设置面板 | 登录失败封禁 |

setup 完成判定即 `admin.password_hash` 非空（无 `setup_done` 键）；`/api/setup` 在 setup 完成后返回 404/409 永久禁用。

**setup 只初始化**管理员密码 + 生成 `upload_key`（`master_secret` 已在 DB 初始化时生成），其余键由登录后的**设置面板**写入；带默认值的可选配置无需 setup 参与，读取时缺键用默认值兜底，设置面板可覆盖。`cdn_host`/R2 配置缺失时上传、查看与随机接口被门控拒绝（见 §3），设置面板正常可操作，配置完成后即恢复。

启动：`godotenv` 加载 `.env` → 读 `DANTA_LISTEN`/`DANTA_DATA_DIR` → 打开 SQLite（**首次建库时 `crypto/rand` 生成并持久化 `master_secret`**）→ settings 载入内存 → 启动。运行期配置经 `PUT /api/admin/settings` 修改即时生效。

## 7. API（控制平面）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/setup` | 仅 setup | 首启写入（仅初始化管理员密码） |
| POST | `/api/login` | 无 | 仅密码登录（IP 封禁兜底）→JWT |
| POST | `/api/upload` | 上传 Key 或管理员 JWT | multipart/form-data，返回四格式外链 |
| GET | `/api/v/{id}` | 无 | 查看页数据（公开字段，见下）：不受 public 门控，不存在 404；配置门控见 §3 |
| GET | `/api/random/{tag}` | 无 | 302 随机图，响应带 `Cache-Control: no-store`（防浏览器/CDN 缓存导致"随机"失效）；仅 public 标签（非 public/不存在统一 404）；配置门控见 §3 |
| GET | `/api/gallery` `/api/gallery/{tag}` | 无 | 公开瀑布流 `?page=&size=`：全部 public 标签图 / 按标签筛选（`{tag}` 仅 public 标签，非 public/不存在 404）；一张图挂多个 public 标签需 `SELECT DISTINCT` 去重；返回公开字段（结构见下）；配置门控见 §3 |
| POST | `/api/admin/logout` | 会话 | 注销（前端弃 token，无状态） |
| GET | `/api/admin/me` | 会话 | 当前管理员信息（会话校验） |
| GET | `/api/admin/images` | 会话 | 分页 `?tag=&page=&size=`（结构见下） |
| GET | `/api/admin/stats` | 会话 | 仪表盘统计：图片总数 / 总尺寸 / 标签数 / 近 24h 新增 |
| DELETE/PATCH | `/api/admin/image/{id}` | 会话 | 同步删除（R2 DeleteObject + 物理删 DB 记录，失败返回错误）/ 改 tags·name·remark |
| GET | `/api/admin/tags` | 会话 | 标签与计数（含 `public` 标记） |
| PATCH | `/api/admin/tag/{id}` | 会话 | 开关标签 `public`（决定对外展示） |
| GET/PUT | `/api/admin/settings` | 会话 | 读（secret 掩码）/ 写 |
| POST | `/api/admin/settings/test-r2` | 会话 | 测试 S3/R2 连接 |
| GET/POST | `/api/admin/upload-key` | 会话 | 查看 / 重置上传 Key |

**路由规则**：后端**仅处理 `/api/*`**；其余一切路径统一返回 SPA——`/` 与真实存在的静态文件（`/assets/*`、`/manifest.webmanifest`、`/sw.js`、`/icons/*`，go:embed 产物，hash 文件名在 `/assets/*`，sw.js 根路径注册）直接返回，其余任意路径（`/setup`、`/v/{id}`、`/gallery`、`/gallery/{tag}` 等前端路由）走 **SPA fallback 到 index.html**，由前端路由接管并调用对应 `/api/*`。

**反代暴露面**：`/api/*` 与 `/api/admin/*` 分组的意义——公网反代只需放行 `/`（SPA 静态 + 前端路由 fallback）与 `/api/*`（公开+上传，即 `/api` 下除 `/api/admin/*` 外的全部路径）；**`/api/admin/*` 整段拦截或仅内网开放**，管理面不暴露公网，JWT 鉴权仍作为第二道防线。

上传响应：`{ id, urls:{url,markdown,bbcode,html}, size, width, height, mime }`。单文件默认 ≤15MB（`max_upload_bytes`），Fiber `BodyLimit` 同值；上传接口 `limiter` 限流；**命中去重**时返回原记录同 id/url，不产生新对象/新行。
统计响应：`{ images, total_size, tags, uploads_24h }`（图片总数 / 总尺寸字节 / 标签数 / 近 24h 新增），均为 SQLite 聚合，无访问量指标。
列表响应：`{ items:[{ id, name, remark, mime, size, width, height, created_at, tags:[...], urls:{url,markdown,bbcode,html} }], total, page, size }`；`size` 上限默认 100。
瀑布流响应：`{ items:[{ id, url, width, height, tags:[...], created_at }], total, page, size }`（公开字段，不含 remark/name 等私密信息，**`tags` 仅含 public 标签**）；`size` 上限默认 100；**`total` 须与 items 同口径去重**（`COUNT(DISTINCT images.id)`），否则多 public 标签图会导致分页错位。
查看页数据响应（`GET /api/v/{id}`）：`{ id, url, width, height, mime, size, tags:[...], created_at }`（公开字段，**`tags` 仅含 public 标签**）。
**一致性**：上传采用 **R2 优先**策略（见 §4）——先写 R2 再写 DB，DB 失败时补偿删除 R2。DB 记录存在即保证 R2 对象存在，不存在返回不存在的 URL 的情况。**单机上传锁**串行化整段上传流程（见 §4），同文件并发去重由锁保证，`Hash` 唯一索引仅作多实例等边界场景的最终保险。删除**同步执行**：R2 DeleteObject 成功后物理删 DB 记录；任一失败返回错误，用户重试即可。

### Vite dev server 反代（调试）
开发时后端跑 `DANTA_LISTEN=127.0.0.1:32682`，前端 `npm run dev`，`vite.config.ts` 反代到后端，无需 CORS：

```ts
export default defineConfig({
  server: {
    proxy: {
      '/api':  'http://127.0.0.1:32682',   // 全部 API（公开+上传+admin）
    },
  },
});
```

生产：`vite build` → `dist/` 由 Go `web` 包 `go:embed` 托管；生产环境后端同源服务，无需反代，前端路由走 SPA fallback。公网反代时 `/api/admin/*` 可整段拦截（见上"反代暴露面"）。

## 8. IP 与登录防爆破

- **c.IP()**（Fiber v3）：`TrustProxy=true` 且直连在信任列表才解析 `ProxyHeader`，从右向左跳过受信任代理、取首个非信任 IP（防伪造）。
- **proxy_mode**：
  - `none`：TrustProxy=false，`c.IP()`=RemoteAddr（直连）
  - `local`：信任 loopback（`127.0.0.1/8`、`::1`）+ X-Forwarded-For（本机 Caddy/Nginx 反代）
- **登录封禁**：内存滑窗（IP→失败时间戳），窗口内 ≥limit 封禁 ban_seconds 返回 429，成功清零；封禁状态内存存（重启即清）。setup 阶段可被外部访问的 API 仅 `/api/setup` 与 `/api/login`，两者均挂 IP 封禁/限流兜底防爆破。
- 中间件 `server/middleware/`：auth / rate / ipban。**rate 覆盖**：`/api/upload`、`/api/login`（与登录封禁叠加）。

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

- **数据平面**：R2 公开桶 + 绑定域名 `img.example.com`。
- **控制平面**：设上面两个变量（或 `.env`）后跑 `./danta`；浏览器打开自动跳 `/setup` 一次性配置，之后密码登录。
- **两种形态**（对应 `proxy_mode`）：直连 `:32682`+`none`；本机反代 `127.0.0.1:32682`+`local`（反代供 HTTPS 满足 PWA）。
- 备份 = 拷数据目录 db 文件。

## 11. 里程碑

- **M1 骨架**：`.env` + GORM/settings 初始化 + Setup 流程（302/`/api/setup`/argon2id，仅初始化管理员密码）+ proxy_mode/c.IP() + 路由 + R2 连通性 + Vite/React/MUI 脚手架 + dev 反代
- **M2 上传链路**：上传 Key 鉴权 + 识别 → 查重（单机上传锁内）→ 转 WebP（含动画）→ 写 R2 → 写 DB → 短链
- **M3 外链与管理**：JWT 会话 + 登录封禁、`/v/{id}`、四格式复制、列表/同步删除/改标签、**仪表盘统计（`/api/admin/stats`）**、上传 Key、PWA 外壳
- **M4 对外展示**：标签 `public` 标记 + `PATCH /api/admin/tag/{id}` + `/api/random/{tag}`（ORDER BY RANDOM()，仅 public 标签）+ 公开瀑布流 `/api/gallery[/{tag}]`（免登录）
- **M5 打磨**：限流、缓存头校验、README/部署文档

— 各里程碑交付以 `make test`（settings/store/handlers 单测）与 lint 通过为准。

## 12. 明确不做

多用户/开放上传、多上传 Key、图片编辑/水印/鉴黄、保留原图/多格式输出、按需缩放/动态处理、图床协议兼容层（客户端直连原生 API 即可）、多存储后端、Presigned URL 直传、**私有桶/直链鉴权**（保持公开桶，`{cdn_host}/{id}.webp` 直链始终可访问；对外可见仅由控制平面接口按标签 `public` 门控）。
