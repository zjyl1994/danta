package handlers

import (
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/server/middleware"
	"github.com/zjyl1994/danta/internal/settings"
	"github.com/zjyl1994/danta/internal/storage"
	"github.com/zjyl1994/danta/internal/store"
)

// jwtTTL 管理员 JWT 有效期
const jwtTTL = 7 * 24 * time.Hour

// StorageProvider 按 settings 提供对象存储实例（生产=R2 Provider，测试可注入假实现）
type StorageProvider interface {
	Storage(s settings.Settings) (storage.Storage, error)
}

// Handler 各 API 处理器依赖
type Handler struct {
	Store    *store.Store
	Settings *settings.Manager
	Storage  StorageProvider
	Ban      *middleware.Ban
}

// writeErr 统一错误响应 { code, message }
func writeErr(c fiber.Ctx, status int, code, msg string) error {
	return c.Status(status).JSON(fiber.Map{"code": code, "message": msg})
}

func writeJSON(c fiber.Ctx, v interface{}) error {
	return c.JSON(v)
}

// imageURL 拼接 CDN 直链（对象键逐段 PathEscape，保留层级）
func imageURL(cdnHost, objectKey string) string {
	parts := strings.Split(objectKey, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "https://" + cdnHost + "/" + strings.Join(parts, "/")
}

// backgroundURL 自定义背景图片直链（未设置返回空串）
func backgroundURL(s settings.Settings) string {
	if s.BackgroundImage == "" || s.CDNHost == "" {
		return ""
	}
	return imageURL(s.CDNHost, s.BackgroundImage)
}

// item 列表项结构
func imageItem(s settings.Settings, img *store.Image) fiber.Map {
	return fiber.Map{
		"id":         img.ID,
		"objectkey":  img.ObjectKey,
		"name":       img.Name,
		"original":   img.Original,
		"mime":       img.Mime,
		"size":       img.Size,
		"width":      img.Width,
		"height":     img.Height,
		"created_at": img.CreatedAt,
		"url":        imageURL(s.CDNHost, img.ObjectKey),
	}
}
