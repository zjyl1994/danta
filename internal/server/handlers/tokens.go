package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/securetoken"
	"github.com/zjyl1994/danta/internal/server/middleware"
	"github.com/zjyl1994/danta/internal/store"
)

// POST /api/refresh 用 refresh token 静默续期（滑动有效期）
func (h *Handler) Refresh(c fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind().Body(&body); err != nil || strings.TrimSpace(body.RefreshToken) == "" {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "missing refresh_token")
	}
	tok, err := h.Store.TokenByHash(store.TokenHash(body.RefreshToken))
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "query failed")
	}
	now := time.Now()
	if tok == nil || tok.Kind != "login" || tok.RevokedAt != nil ||
		(tok.ExpiresAt != nil && now.After(*tok.ExpiresAt)) {
		return writeErr(c, fiber.StatusUnauthorized, "auth_failed", "session expired or revoked")
	}
	exp := now.Add(h.sessionTTL())
	if err := h.Store.TouchToken(tok.ID, now, &exp); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
	}
	access, err := middleware.SignJWT(h.Settings.Get().MasterSecret, accessTTL)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "sign failed")
	}
	return writeJSON(c, fiber.Map{"token": access, "expires_at": exp})
}

// POST /api/logout 吊销当前登录会话
func (h *Handler) Logout(c fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind().Body(&body); err != nil || strings.TrimSpace(body.RefreshToken) == "" {
		return writeJSON(c, fiber.Map{"ok": true})
	}
	tok, err := h.Store.TokenByHash(store.TokenHash(body.RefreshToken))
	if err == nil && tok != nil && tok.Kind == "login" && tok.RevokedAt == nil {
		_ = h.Store.RevokeToken(tok.ID)
	}
	return writeJSON(c, fiber.Map{"ok": true})
}

// sessionCleanupRetention 已吊销/已过期 token 的保留期，超过后随列表读取兜底删除
const sessionCleanupRetention = 30 * 24 * time.Hour

// GET /api/admin/sessions 仅返回有效登录会话与上传令牌，并兜底删除超保留期记录
func (h *Handler) ListSessions(c fiber.Ctx) error {
	_, _ = h.Store.PruneTokens(time.Now().Add(-sessionCleanupRetention))
	toks, err := h.Store.ListActiveTokens("")
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "query failed")
	}
	items := make([]fiber.Map, 0, len(toks))
	for i := range toks {
		t := toks[i]
		items = append(items, fiber.Map{
			"id":           t.ID,
			"kind":         t.Kind,
			"name":         t.Name,
			"created_at":   t.CreatedAt,
			"last_used_at": t.LastUsedAt,
			"expires_at":   t.ExpiresAt,
			"revoked_at":   t.RevokedAt,
		})
	}
	return writeJSON(c, fiber.Map{"sessions": items})
}

// POST /api/admin/sessions/cleanup 立即删除全部已吊销/已过期的登录会话与上传令牌
func (h *Handler) CleanupSessions(c fiber.Ctx) error {
	n, err := h.Store.PruneTokens(time.Now())
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "cleanup failed")
	}
	return writeJSON(c, fiber.Map{"deleted": n})
}

// POST /api/admin/tokens 创建上传令牌；days=0 表示永不过期
func (h *Handler) CreateUploadToken(c fiber.Ctx) error {
	var body struct {
		Name string `json:"name"`
		Days int    `json:"days"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid body")
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		body.Name = "上传令牌"
	}
	if len(body.Name) > 40 {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "name too long")
	}
	if body.Days < 0 || body.Days > 3650 {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid days")
	}
	raw, err := securetoken.Key()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "generation failed")
	}
	now := time.Now()
	var exp *time.Time
	if body.Days > 0 {
		e := now.Add(time.Duration(body.Days) * 24 * time.Hour)
		exp = &e
	}
	tok := &store.Token{
		Kind:      "upload",
		Name:      body.Name,
		TokenHash: store.TokenHash(raw),
		ExpiresAt: exp,
		CreatedAt: now,
	}
	if err := h.Store.CreateToken(tok); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
	}
	return writeJSON(c, fiber.Map{
		"id":         tok.ID,
		"token":      raw,
		"name":       tok.Name,
		"expires_at": exp,
	})
}

// POST /api/admin/sessions/:id/revoke 吊销登录会话或上传令牌
func (h *Handler) RevokeSession(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid id")
	}
	tok, err := h.Store.TokenByID(uint(id))
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "query failed")
	}
	if tok == nil {
		return writeErr(c, fiber.StatusNotFound, "not_found", "session not found")
	}
	if tok.RevokedAt == nil {
		if err := h.Store.RevokeToken(tok.ID); err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
		}
	}
	return writeJSON(c, fiber.Map{"ok": true})
}
