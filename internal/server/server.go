package server

import (
	"embed"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/server/handlers"
	"github.com/zjyl1994/danta/internal/server/middleware"
	"github.com/zjyl1994/danta/internal/settings"
	"github.com/zjyl1994/danta/internal/storage"
	"github.com/zjyl1994/danta/internal/store"
)

//go:embed dist
var webDist embed.FS

// New 构建 Fiber v3 应用（API 路由 + SPA fallback）
func New(s *store.Store, st *settings.Manager) *fiber.App {
	return NewWithDeps(s, st, &storage.Provider{})
}

// NewWithDeps 供测试注入自定义存储 provider
func NewWithDeps(s *store.Store, st *settings.Manager, sp handlers.StorageProvider) *fiber.App {
	h := &handlers.Handler{
		Store:    s,
		Settings: st,
		Storage:  sp,
		Ban:      middleware.NewBan(5, 900, 900),
	}
	s0 := st.Get()
	h.Ban.Configure(s0.LoginFailLimit, s0.LoginFailWindow, s0.LoginBanSeconds)
	// 限流器每次请求动态解析当前 proxy_mode（设置变更后无需重建）
	mode := func() string { return st.Get().ProxyMode }

	app := fiber.New(fiber.Config{
		BodyLimit: 64 * 1024 * 1024, // 硬上限，实际大小由 handler 按配置校验
		AppName:   "danta",
	})

	// 公开
	app.Get("/api/status", h.Status)
	app.Post("/api/login", middleware.BanGuard(h.Ban, st), middleware.NewRate(30, time.Minute, mode).Handler, h.Login)
	app.Post("/api/setup", middleware.BanGuard(h.Ban, st), middleware.NewRate(30, time.Minute, mode).Handler, h.Setup)

	// 上传：上传 Key 或管理员 JWT
	app.Post("/api/upload",
		middleware.UploadAuth(st),
		middleware.NewRate(120, time.Minute, mode).Handler,
		h.Upload,
	)

	// 管理（管理员 JWT）
	admin := app.Group("/api/admin", middleware.AdminAuth(st))
	admin.Get("/images", h.ListImages)
	admin.Get("/stats", h.Stats)
	admin.Post("/images/delete", h.DeleteImages)
	admin.Post("/cleanup", middleware.NewRate(10, time.Minute, mode).Handler, h.Cleanup)
	admin.Get("/import/scan", h.ImportScan)
	admin.Post("/import/run", h.ImportRun)
	admin.Get("/settings", h.GetSettings)
	admin.Post("/settings", h.UpdateSettings)
	admin.Post("/settings/test-r2", h.TestR2)
	admin.Get("/upload-key", h.GetUploadKey)
	admin.Post("/upload-key", h.ResetUploadKey)
	admin.Get("/client-ip", h.ClientIP)

	// SPA fallback（/api 404 由前面路由返回；其余未知路径回 index.html）
	app.Use(spaFallback())

	return app
}

func spaFallback() fiber.Handler {
	return func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Next()
		}
		p := strings.TrimPrefix(c.Path(), "/")
		if p == "" || strings.Contains(p, "..") {
			p = "index.html"
		}
		data, err := webDist.ReadFile("dist/" + p)
		if err != nil {
			idx, ierr := webDist.ReadFile("dist/index.html")
			if ierr != nil {
				// 前端未构建（仅占位）时的最小页面
				c.Type("html")
				return c.SendString(fallbackHTML)
			}
			c.Type("html")
			return c.Send(idx)
		}
		c.Type(extOf(p))
		return c.Send(data)
	}
}

const fallbackHTML = `<!doctype html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>danta</title></head>
<body>前端未构建，请运行 make build</body>
</html>`

func extOf(p string) string {
	if i := strings.LastIndex(p, "."); i >= 0 {
		return p[i+1:]
	}
	return "html"
}
