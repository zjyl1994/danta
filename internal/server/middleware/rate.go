package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Rate IP 滑动窗口限流：纯复用 windowCounter，超限即拒绝，无附加响应与附加状态。
type Rate struct {
	*windowCounter
	mode func() string // 动态解析当前 proxy_mode（设置变更后无需重建）
}

func NewRate(limit int, window time.Duration, mode func() string) *Rate {
	return &Rate{windowCounter: newWindowCounter(limit, window, 64, nil, nil), mode: mode}
}

func (r *Rate) Allow(ip string) bool {
	return !r.Add(ip)
}

func (r *Rate) Handler(c fiber.Ctx) error {
	if !r.Allow(ClientIP(c, r.mode())) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"code":    "too_many_requests",
			"message": "rate limit exceeded",
		})
	}
	return c.Next()
}
