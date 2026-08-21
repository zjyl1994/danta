package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/securetoken"
	"github.com/zjyl1994/danta/internal/server/middleware"
	"github.com/zjyl1994/danta/internal/store"
)

// POST /api/setup 初始化管理员密码（仅 setup 阶段可用；完成后永久禁用）
func (h *Handler) Setup(c fiber.Ctx) error {
	if h.Settings.Configured() {
		return writeErr(c, fiber.StatusForbidden, "setup_required", "setup already done")
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := c.Bind().Body(&body); err != nil || len(body.Password) < 8 {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "password too short")
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "hash failed")
	}
	if err := h.Settings.SetOne("admin.password_hash", hash); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
	}
	return writeJSON(c, fiber.Map{"ok": true})
}

// uaDeviceName 从 User-Agent 生成可读的设备名（浏览器 · 系统）
func uaDeviceName(ua string) string {
	if ua == "" {
		return "未知设备"
	}
	osName := "未知系统"
	switch {
	case strings.Contains(ua, "Windows"):
		osName = "Windows"
	case strings.Contains(ua, "iPhone"):
		osName = "iPhone"
	case strings.Contains(ua, "iPad"):
		osName = "iPad"
	case strings.Contains(ua, "Android"):
		osName = "Android"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		osName = "macOS"
	case strings.Contains(ua, "Linux"):
		osName = "Linux"
	}
	browser := "浏览器"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "SamsungBrowser"):
		browser = "Samsung Internet"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox"):
		browser = "Firefox"
	case strings.Contains(ua, "MSIE"), strings.Contains(ua, "Trident"):
		browser = "IE"
	case strings.Contains(ua, "Safari"):
		browser = "Safari"
	}
	return browser + " · " + osName
}

// POST /api/login 密码登录 → 访问令牌 + 登录会话（refresh token）
func (h *Handler) Login(c fiber.Ctx) error {
	s := h.Settings.Get()
	ip := middleware.ClientIP(c, s.ProxyMode)

	var body struct {
		Password string `json:"password"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid body")
	}
	if s.AdminPasswordHash == "" {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "setup not done")
	}
	ok, err := verifyPassword(body.Password, s.AdminPasswordHash)
	if err != nil || !ok {
		h.Ban.Fail(ip)
		return writeErr(c, fiber.StatusUnauthorized, "auth_failed", "invalid credentials")
	}
	h.Ban.Reset(ip)

	raw, err := securetoken.Hex(32)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "generation failed")
	}
	name := uaDeviceName(c.Get("User-Agent"))
	now := time.Now()
	exp := now.Add(h.sessionTTL())
	sess := &store.Token{
		Kind:       "login",
		Name:       name,
		TokenHash:  store.TokenHash(raw),
		ExpiresAt:  &exp,
		LastUsedAt: &now,
		CreatedAt:  now,
	}
	if err := h.Store.CreateToken(sess); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
	}

	tok, err := middleware.SignJWT(s.MasterSecret, accessTTL)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "sign failed")
	}
	return writeJSON(c, fiber.Map{
		"token":         tok,
		"refresh_token": raw,
		"device_id":     sess.ID,
		"expires_at":    exp,
	})
}

// GET /api/status 公开状态 + 前端 canvas 预压缩参数
func (h *Handler) Status(c fiber.Ctx) error {
	s := h.Settings.Get()
	return writeJSON(c, fiber.Map{
		"configured":       s.AdminPasswordHash != "",
		"authed":           middleware.IsAdmin(c, h.Settings),
		"resize_max_dim":   s.ResizeMaxDim,
		"webp_quality":     s.WebPQuality,
		"max_upload_bytes": s.MaxUploadBytes,
		"background_url":   backgroundURL(s),
	})
}
