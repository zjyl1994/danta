package handlers

import (
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/storage"
	"github.com/zjyl1994/danta/internal/store"
)

// importExtWhitelist 导入扩展名白名单
var importExtWhitelist = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true,
	"webp": true, "bmp": true, "avif": true,
}

type scanItem struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified string    `json:"last_modified"`
}

// scanImport 扫描桶（只读 List），分类 new/existing/ignored
func (h *Handler) scanImport(prefix string) (total, new, existing, ignored int, items []scanItem, err error) {
	s := h.Settings.Get()
	stg, err := h.Storage.Storage(s)
	if err != nil {
		return
	}
	keySet, err := h.Store.AllObjectKeys()
	if err != nil {
		return
	}
	collect := func(o storage.ObjectInfo) error {
		total++
		ext := strings.ToLower(filepath.Ext(o.Key))
		ext = strings.TrimPrefix(ext, ".")
		if !importExtWhitelist[ext] {
			ignored++
			return nil
		}
		if _, ok := keySet[o.Key]; ok {
			existing++
			return nil
		}
		new++
		if len(items) < 100 {
			items = append(items, scanItem{Key: o.Key, Size: o.Size, LastModified: o.LastModified.Format("2006-01-02 15:04:05")})
		}
		return nil
	}
	if prefix != "" {
		err = stg.ListPrefix(prefix, collect)
	} else {
		err = stg.ListAll(collect)
	}
	return
}

// GET /api/admin/import/scan 预扫描 dry-run
func (h *Handler) ImportScan(c fiber.Ctx) error {
	prefix := c.Query("prefix")
	total, new, existing, ignored, items, err := h.scanImport(prefix)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "r2_error", "scan failed")
	}
	return writeJSON(c, fiber.Map{
		"total":    total,
		"new":      new,
		"existing": existing,
		"ignored":  ignored,
		"items":    items,
	})
}

// POST /api/admin/import/run 执行导入（只读 List，绝不 Put/Delete/改键）
func (h *Handler) ImportRun(c fiber.Ctx) error {
	var body struct {
		Prefix string `json:"prefix"`
	}
	_ = c.Bind().Body(&body)

	total, new, existing, ignored, _, err := h.scanImport(body.Prefix)
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "r2_error", "scan failed")
	}
	if new == 0 {
		return writeJSON(c, fiber.Map{"imported": 0, "skipped": existing, "ignored": ignored, "errors": []string{}, "total": total})
	}

	// 二次扫描执行（与 scan 一致，List 只读）
	s := h.Settings.Get()
	stg, err := h.Storage.Storage(s)
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "storage not configured")
	}
	keySet, err := h.Store.AllObjectKeys()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "query failed")
	}
	var (
		toInsert []store.Image
		skipN    int
		ignN     int
		errs     []string
	)
	collect := func(o storage.ObjectInfo) error {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(o.Key), "."))
		if !importExtWhitelist[ext] {
			ignN++
			return nil
		}
		if _, ok := keySet[o.Key]; ok {
			skipN++
			return nil
		}
		toInsert = append(toInsert, store.Image{
			ObjectKey: o.Key,
			Name:      filepath.Base(o.Key),
			Size:      o.Size,
			Original:  true,
			CreatedAt: o.LastModified,
		})
		return nil
	}
	if body.Prefix != "" {
		err = stg.ListPrefix(body.Prefix, collect)
	} else {
		err = stg.ListAll(collect)
	}
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "r2_error", "list failed")
	}

	imported := int64(0)
	// 分批插入
	const batch = 500
	for i := 0; i < len(toInsert); i += batch {
		end := i + batch
		if end > len(toInsert) {
			end = len(toInsert)
		}
		n, ierr := h.Store.InsertImages(toInsert[i:end])
		if ierr != nil {
			errs = append(errs, ierr.Error())
			continue
		}
		imported += n
	}
	return writeJSON(c, fiber.Map{
		"imported": imported,
		"skipped":  skipN,
		"ignored":  ignN,
		"errors":   errs,
	})
}
