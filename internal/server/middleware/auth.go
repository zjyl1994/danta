package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/zjyl1994/danta/internal/settings"
	"github.com/zjyl1994/danta/internal/store"
)

func bearerToken(c fiber.Ctx) string {
	h := c.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}

// SignJWT 签发管理员 JWT（HS256，master_secret 签名）
func SignJWT(secret string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseJWT 校验并返回是否有效管理员 JWT
func ParseJWT(tok, secret string) (bool, error) {
	if tok == "" || secret == "" {
		return false, nil
	}
	t, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return false, err
	}
	return t.Valid, nil
}

// validJWT 校验管理员 JWT
func validJWT(tok, secret string) bool {
	ok, _ := ParseJWT(tok, secret)
	return ok
}

// IsAdmin reports whether the request carries a valid administrator JWT.
func IsAdmin(c fiber.Ctx, st *settings.Manager) bool {
	s := st.Get()
	return validJWT(bearerToken(c), s.MasterSecret)
}

func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"code":    "auth_failed",
		"message": "unauthorized",
	})
}

// AdminAuth 管理 API 中间件：仅管理员 JWT
func AdminAuth(st *settings.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !IsAdmin(c, st) {
			return unauthorized(c)
		}
		return c.Next()
	}
}

// UploadAuth 上传中间件：管理员 JWT 或有效上传令牌二选一
func UploadAuth(st *settings.Manager, stg *store.Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		s := st.Get()
		tok := bearerToken(c)
		if validJWT(tok, s.MasterSecret) {
			return c.Next()
		}
		if tok != "" && stg != nil {
			t, err := stg.TokenByHash(store.TokenHash(tok))
			if err == nil && t != nil && t.Kind == "upload" && t.RevokedAt == nil &&
				(t.ExpiresAt == nil || time.Now().Before(*t.ExpiresAt)) {
				_ = stg.MarkTokenUsed(t.ID)
				return c.Next()
			}
		}
		return unauthorized(c)
	}
}

// BanGuard 登录/setup 封禁中间件
func BanGuard(b *Ban, st *settings.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		s := st.Get()
		b.Configure(s.LoginFailLimit, s.LoginFailWindow, s.LoginBanSeconds)
		ip := ClientIP(c, s.ProxyMode)
		banned, until := b.Check(ip)
		if banned {
			retry := int(time.Until(until).Seconds()) + 1
			if retry < 1 {
				retry = 1
			}
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":        "too_many_requests",
				"message":     "IP banned, retry later",
				"retry_after": retry,
			})
		}
		return c.Next()
	}
}
