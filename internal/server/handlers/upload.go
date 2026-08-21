package handlers

import (
	"crypto/sha256"
	"io"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/zjyl1994/danta/internal/idgen"
	"github.com/zjyl1994/danta/internal/imageproc"
	"github.com/zjyl1994/danta/internal/settings"
	"github.com/zjyl1994/danta/internal/store"
)

// uploadMu 单机上传锁：串行化 DB「查重→写入」关键区（R2 网络 IO 在锁外）
var uploadMu sync.Mutex

// POST /api/upload multipart：file + original（省略=压缩模式）
func (h *Handler) Upload(c fiber.Ctx) error {
	s := h.Settings.Get()
	if !s.R2Configured() {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "R2 or cdn_host not configured")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "missing file")
	}
	original := c.FormValue("original") == "true"

	// 读入内存（BodyLimit 已有硬上限，这里按当前配置精确校验）
	f, err := file.Open()
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "cannot open file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, s.MaxUploadBytes+1))
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "bad_request", "read failed")
	}
	if int64(len(data)) > s.MaxUploadBytes {
		return writeErr(c, fiber.StatusRequestEntityTooLarge, "too_large", "file too large")
	}

	// magic 白名单（全量，两种模式一致）
	ext, mime, ok := imageproc.Detect(data)
	if !ok {
		return writeErr(c, fiber.StatusBadRequest, "unsupported_type", "unsupported image type")
	}

	hash := sha256.Sum256(data)

	// 拿锁查重（第一次，锁外已算 hash）
	uploadMu.Lock()
	existing, err := h.Store.FindByHash(hash[:], original)
	uploadMu.Unlock()
	if err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "dedup failed")
	}
	if existing != nil {
		return h.uploadResult(c, s, existing, existing.Original)
	}

	// 锁外处理（编码并行）
	var res *imageproc.Result
	if original {
		w, ht, _ := imageproc.DecodeConfig(data)
		res = &imageproc.Result{Data: data, Ext: ext, Mime: mime, Width: w, Height: ht, Original: true}
	} else {
		res, err = imageproc.ProcessCompressed(data, s.ResizeMaxDim, s.WebPQuality)
		if err != nil {
			return writeErr(c, fiber.StatusBadRequest, "unsupported_type", "processing failed")
		}
	}

	// 生成 ObjectKey → 写 R2（锁外，网络 IO 不阻塞其他上传）
	key := idgen.New() + "." + res.Ext
	stg, err := h.Storage.Storage(s)
	if err != nil {
		return writeErr(c, fiber.StatusBadRequest, "config_required", "storage not configured")
	}
	if err := stg.Put(key, res.Data, res.Mime, s.CacheMaxAge); err != nil {
		return writeErr(c, fiber.StatusInternalServerError, "r2_error", "upload to R2 failed")
	}

	// 锁内二次查重：命中则补偿删刚写入的对象并返回既有记录；未命中写 DB（DB 失败补偿删 R2）
	uploadMu.Lock()
	defer uploadMu.Unlock()
	existing, err = h.Store.FindByHash(hash[:], res.Original)
	if err != nil {
		_ = stg.Delete([]string{key})
		return writeErr(c, fiber.StatusInternalServerError, "internal_error", "dedup failed")
	}
	if existing != nil {
		_ = stg.Delete([]string{key})
		return h.uploadResult(c, s, existing, res.Original)
	}
	img := &store.Image{
		ObjectKey: key,
		Name:      file.Filename,
		Mime:      res.Mime,
		Size:      int64(len(res.Data)),
		Width:     res.Width,
		Height:    res.Height,
		Hash:      hash[:],
		Original:  res.Original,
		CreatedAt: time.Now(),
	}
	if err := h.Store.CreateImage(img); err != nil {
		_ = stg.Delete([]string{key}) // 补偿删 R2
		return writeErr(c, fiber.StatusInternalServerError, "db_error", "write db failed")
	}
	return h.uploadResult(c, s, img, res.Original)
}

// uploadResult 上传/去重命中统一响应
func (h *Handler) uploadResult(c fiber.Ctx, s settings.Settings, img *store.Image, original bool) error {
	return writeJSON(c, fiber.Map{
		"id":         img.ID,
		"original":   original,
		"url":        imageURL(s.CDNHost, img.ObjectKey),
		"size":       img.Size,
		"width":      img.Width,
		"height":     img.Height,
		"mime":       img.Mime,
		"created_at": img.CreatedAt,
	})
}
