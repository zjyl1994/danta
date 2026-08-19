package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Rate IP 滑动窗口限流
type Rate struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
	mode   string
}

func NewRate(limit int, window time.Duration, mode string) *Rate {
	return &Rate{limit: limit, window: window, hits: map[string][]time.Time{}, mode: mode}
}

func (r *Rate) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-r.window)
	arr := r.hits[ip]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.limit {
		r.hits[ip] = kept
		return false
	}
	r.hits[ip] = append(kept, now)
	return true
}

func (r *Rate) Handler(c fiber.Ctx) error {
	if !r.Allow(ClientIP(c, r.mode)) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"code":    "too_many_requests",
			"message": "rate limit exceeded",
		})
	}
	return c.Next()
}
