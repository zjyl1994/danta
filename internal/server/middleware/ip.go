package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ClientIP 解析客户端 IP：proxy_mode=local 时信任 loopback 的 X-Forwarded-For，否则取直连地址
func ClientIP(c fiber.Ctx, mode string) string {
	peer := c.RequestCtx().RemoteIP().String()
	if mode == "local" && (peer == "127.0.0.1" || peer == "::1") {
		if xff := c.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	return peer
}
