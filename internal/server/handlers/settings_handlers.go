package handlers

import (
	"regexp"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/server/middleware"
)

var hostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?(:[0-9]+)?$`)

// publicSettings 只读响应（secret 掩码）
func (h *Handler) publicSettings() fiber.Map {
	s := h.Settings.Get()
	mask := func(v string) string {
		if v == "" {
			return ""
		}
		if len(v) <= 4 {
			return "****"
		}
		return v[:2] + "****" + v[len(v)-2:]
	}
	return fiber.Map{
		"cdn_host":              s.CDNHost,
		"proxy_mode":            s.ProxyMode,
		"max_upload_bytes":      s.MaxUploadBytes,
		"webp_quality":          s.WebPQuality,
		"resize_max_dim":        s.ResizeMaxDim,
		"cache_max_age":         s.CacheMaxAge,
		"r2_endpoint":           s.R2Endpoint,
		"r2_access_key_id":      s.R2AccessKeyID,
		"r2_secret_access_key":  mask(s.R2SecretAccessKey),
		"r2_bucket":             s.R2Bucket,
		"login_fail_limit":      s.LoginFailLimit,
		"login_fail_window":     s.LoginFailWindow,
		"login_ban_seconds":     s.LoginBanSeconds,
		"has_password":          s.AdminPasswordHash != "",
	}
}

// GET /api/admin/settings 读（secret 掩码）
func (h *Handler) GetSettings(c fiber.Ctx) error {
	return writeJSON(c, h.publicSettings())
}

// POST /api/admin/settings 写（secret 留空=不改动；old_password+new_password 改密码）
func (h *Handler) UpdateSettings(c fiber.Ctx) error {
	var body map[string]interface{}
	if err := c.Bind().Body(&body); err != nil {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid body")
	}

	// 改密码
	if np, ok := body["new_password"].(string); ok && np != "" {
		op, _ := body["old_password"].(string)
		s := h.Settings.Get()
		okPw, err := verifyPassword(op, s.AdminPasswordHash)
		if err != nil || !okPw {
			return writeErr(c, fiber.StatusBadRequest, "auth_failed", "old password wrong")
		}
		if len(np) < 8 {
			return writeErr(c, fiber.StatusBadRequest, "bad_request", "password too short")
		}
		hash, err := hashPassword(np)
		if err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "internal_error", "hash failed")
		}
		if err := h.Settings.SetOne("admin.password_hash", hash); err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
		}
	}

	updates := map[string]string{}
	strFields := map[string]string{
		"cdn_host":             "cdn_host",
		"proxy_mode":           "proxy_mode",
		"r2_endpoint":          "r2.endpoint",
		"r2_access_key_id":     "r2.access_key_id",
		"r2_bucket":            "r2.bucket",
	}
	for k, dbk := range strFields {
		if v, ok := body[k].(string); ok {
			updates[dbk] = v
		}
	}
	// secret 留空跳过（掩码回显场景）
	if v, ok := body["r2_secret_access_key"].(string); ok && v != "" {
		updates["r2.secret_access_key"] = v
	}
	intFields := map[string]string{
		"max_upload_bytes":   "max_upload_bytes",
		"webp_quality":       "webp_quality",
		"resize_max_dim":     "resize_max_dim",
		"cache_max_age":      "cache_max_age",
		"login_fail_limit":   "security.login_fail_limit",
		"login_fail_window":  "security.login_fail_window",
		"login_ban_seconds":  "security.login_ban_seconds",
	}
	for k, dbk := range intFields {
		switch v := body[k].(type) {
		case float64:
			updates[dbk] = intToStr(int(v))
		case string:
			updates[dbk] = v
		}
	}

	if v, ok := updates["cdn_host"]; ok {
		if !hostRe.MatchString(v) {
			return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid cdn_host")
		}
	}
	if v, ok := updates["proxy_mode"]; ok && v != "none" && v != "local" {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "invalid proxy_mode")
	}

	if len(updates) > 0 {
		if err := h.Settings.Set(updates); err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
		}
	}
	return writeJSON(c, h.publicSettings())
}

func intToStr(v int) string {
	return strconv.Itoa(v)
}

// POST /api/admin/settings/test-r2 测试 R2 连接
func (h *Handler) TestR2(c fiber.Ctx) error {
	s := h.Settings.Get()
	if !s.R2Configured() {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "R2 not configured")
	}
	stg, err := h.Storage.Storage(s)
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "bad R2 config")
	}
	r2, ok := stg.(interface{ Ping() error })
	if !ok {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "no ping")
	}
	if err := r2.Ping(); err != nil {
		return writeErr(c, fiber.StatusBadRequest, "r2_error", "connect failed: "+err.Error())
	}
	return writeJSON(c, fiber.Map{"ok": true})
}

// GET /api/admin/upload-key 查看上传 Key
func (h *Handler) GetUploadKey(c fiber.Ctx) error {
	return writeJSON(c, fiber.Map{"upload_key": h.Settings.Get().UploadKey})
}

// GET /api/admin/client-ip 当前请求 IP；可选 ?mode=none|local 按指定模式预览解析
func (h *Handler) ClientIP(c fiber.Ctx) error {
	s := h.Settings.Get()
	mode := c.Query("mode")
	if mode != "none" && mode != "local" {
		mode = s.ProxyMode
	}
	return writeJSON(c, fiber.Map{"ip": middleware.ClientIP(c, mode), "proxy_mode": mode})
}

// POST /api/admin/upload-key 重置上传 Key
func (h *Handler) ResetUploadKey(c fiber.Ctx) error {
	key, err := randomKey()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "generation failed")
	}
	if err := h.Settings.SetOne("upload_key", key); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "save failed")
	}
	return writeJSON(c, fiber.Map{"upload_key": key})
}
