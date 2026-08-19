package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/storage"
)

// cleanupGrace 上传宽限：对象最近修改 10 分钟内视为可能"R2 已写、DB 未写"，跳过
const cleanupGrace = 10 * time.Minute

// POST /api/admin/cleanup 孤儿清理：删除 R2 中无 DB 记录且超过宽限的对象
func (h *Handler) Cleanup(c fiber.Ctx) error {
	s := h.Settings.Get()
	stg, err := h.Storage.Storage(s)
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "storage not configured")
	}
	keys, err := h.Store.AllObjectKeys()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "query failed")
	}
	var (
		toDelete []string
		skipped  int
		now      = time.Now()
	)
	err = stg.ListAll(func(o storage.ObjectInfo) error {
		if _, ok := keys[o.Key]; ok {
			return nil
		}
		if now.Sub(o.LastModified) < cleanupGrace {
			skipped++
			return nil
		}
		toDelete = append(toDelete, o.Key)
		return nil
	})
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "r2_error", "list failed")
	}
	if len(toDelete) > 0 {
		if err := stg.Delete(toDelete); err != nil {
			return writeErr(c, fiber.StatusInternalServerError, "r2_error", "delete failed")
		}
	}
	return writeJSON(c, fiber.Map{"deleted": len(toDelete), "skipped_grace": skipped})
}
