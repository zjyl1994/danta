package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/securetoken"
	"github.com/zjyl1994/danta/internal/server/middleware"
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
	// 上传 Key 后台生成（1 个，可重置）
	key, err := securetoken.Key()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "key generation failed")
	}
	if err := h.Settings.SetOne("upload_key", key); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
	}
	return writeJSON(c, fiber.Map{"ok": true})
}

// POST /api/login 密码登录 → JWT
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
	tok, err := middleware.SignJWT(s.MasterSecret, jwtTTL)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "sign failed")
	}
	return writeJSON(c, fiber.Map{"token": tok})
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
