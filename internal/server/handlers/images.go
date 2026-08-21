package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// queryInt 读取整型 query 参数（fiber v3 无 c.QueryInt）
func queryInt(c fiber.Ctx, key string, def int) int {
	v, err := strconv.Atoi(fiber.Query(c, key, ""))
	if err != nil {
		return def
	}
	return v
}

// GET /api/admin/images 倒序分页
func (h *Handler) ListImages(c fiber.Ctx) error {
	s := h.Settings.Get()
	page := queryInt(c, "page", 1)
	size := queryInt(c, "size", 20)
	items, total, err := h.Store.ListImages(page, size)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "list failed")
	}
	out := make([]fiber.Map, 0, len(items))
	for i := range items {
		out = append(out, imageItem(s, &items[i]))
	}
	return writeJSON(c, fiber.Map{"items": out, "total": total, "page": page, "size": size})
}

// GET /api/admin/stats 统计
func (h *Handler) Stats(c fiber.Ctx) error {
	images, totalSize, uploads24h, originals, err := h.Store.Stats()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "stats failed")
	}
	return writeJSON(c, fiber.Map{
		"images":     images,
		"total_size": totalSize,
		"uploads_24h": uploads24h,
		"originals":  originals,
	})
}

// POST /api/admin/images/delete 单删/批量删除 { ids:[uint] }
func (h *Handler) DeleteImages(c fiber.Ctx) error {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.Bind().Body(&body); err != nil || len(body.IDs) == 0 {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "ids required")
	}
	s := h.Settings.Get()
	imgs, err := h.Store.ImagesByIDs(body.IDs)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "query failed")
	}
	keys := make([]string, 0, len(imgs))
	ids := make([]uint, 0, len(imgs))
	for i := range imgs {
		keys = append(keys, imgs[i].ObjectKey)
		ids = append(ids, imgs[i].ID)
	}
	// R2 优先
	if len(keys) > 0 {
		stg, err := h.Storage.Storage(s)
		if err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "r2_error", "storage not configured")
		}
		if err := stg.Delete(keys); err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "r2_error", "delete R2 failed")
		}
	}
	if err := h.Store.DeleteByIDs(ids); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "db_error", "delete db failed")
	}
	// 若被删图片正是自定义背景，则同步清除
	if s.BackgroundImage != "" {
		for _, k := range keys {
			if k == s.BackgroundImage {
				_ = h.Settings.SetOne("background_image", "")
				break
			}
		}
	}
	return writeJSON(c, fiber.Map{"deleted": len(ids)})
}
