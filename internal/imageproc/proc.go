package imageproc

import (
	"bytes"
	"errors"
	"image"
	"math"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"

	"github.com/bep/imagemeta"
	"github.com/deepteams/webp"
	"github.com/gabriel-vasile/mimetype"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

var (
	ErrUnsupported = errors.New("unsupported image type")
	ErrDecode      = errors.New("image decode failed")
)

// Result 压缩模式处理结果
type Result struct {
	Data     []byte
	Ext      string
	Mime     string
	Width    int
	Height   int
	Original bool // true=回退原图直存
}

// Detect magic 白名单检测，返回 ext/mime
func Detect(data []byte) (ext, mime string, ok bool) {
	mt := mimetype.Detect(data)
	switch mt.String() {
	case "image/jpeg":
		return "jpg", "image/jpeg", true
	case "image/png":
		return "png", "image/png", true
	case "image/gif":
		return "gif", "image/gif", true
	case "image/webp":
		return "webp", "image/webp", true
	case "image/bmp":
		return "bmp", "image/bmp", true
	case "image/avif":
		return "avif", "image/avif", true
	}
	return "", "", false
}

// DecodeConfig 头读取尺寸（不解码像素）；avif 等无解码器返回错误
func DecodeConfig(data []byte) (w, h int, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// Orientation 读 EXIF Orientation（1..8），无/失败返回 1
func Orientation(data []byte) int {
	var f imagemeta.ImageFormat
	switch ext, _, ok := Detect(data); {
	case !ok:
		return 1
	case ext == "png":
		f = imagemeta.PNG
	case ext == "webp":
		f = imagemeta.WebP
	case ext == "avif":
		f = imagemeta.AVIF
	default:
		f = imagemeta.JPEG
	}
	orient := 1
	_, err := imagemeta.Decode(imagemeta.Options{
		R:            bytes.NewReader(data),
		ImageFormat:  f,
		Sources:      imagemeta.EXIF,
		LimitNumTags: 100,
		HandleTag: func(info imagemeta.TagInfo) error {
			if info.Tag != "Orientation" {
				return nil
			}
			switch v := info.Value.(type) {
			case uint16:
				orient = int(v)
			case uint32:
				orient = int(v)
			case int:
				orient = v
			case int64:
				orient = int(v)
			}
			return nil
		},
	})
	if err != nil {
		return 1
	}
	if orient < 1 || orient > 8 {
		return 1
	}
	return orient
}

// Rotate 按 EXIF orientation 物理旋转/翻转
func Rotate(img image.Image, orientation int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var (
		dw, dh int
		m      f64.Aff3
	)
	switch orientation {
	case 2: // flip H
		dw, dh = w, h
		m = f64.Aff3{-1, 0, float64(w - 1), 0, 1, 0}
	case 3: // rotate 180
		dw, dh = w, h
		m = f64.Aff3{-1, 0, float64(w - 1), 0, -1, float64(h - 1)}
	case 4: // flip V
		dw, dh = w, h
		m = f64.Aff3{1, 0, 0, 0, -1, float64(h - 1)}
	case 5: // transpose
		dw, dh = h, w
		m = f64.Aff3{0, 1, 0, 1, 0, 0}
	case 6: // rotate 90 CW
		dw, dh = h, w
		m = f64.Aff3{0, 1, 0, -1, 0, float64(h - 1)}
	case 7: // transverse
		dw, dh = h, w
		m = f64.Aff3{0, -1, float64(w - 1), -1, 0, float64(h - 1)}
	case 8: // rotate 270 CW
		dw, dh = h, w
		m = f64.Aff3{0, -1, float64(w - 1), 1, 0, 0}
	default:
		return img
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	draw.BiLinear.Transform(dst, m, img, img.Bounds(), draw.Src, nil)
	return dst
}

// Scale 等比缩放至长边 ≤ maxDim；已达标返回原图
func Scale(img image.Image, maxDim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}
	longest := math.Max(float64(w), float64(h))
	k := float64(maxDim) / longest
	nw := int(float64(w) * k)
	nh := int(float64(h) * k)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// EncodeWebP 静态 WebP 编码
func EncodeWebP(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.EncoderOptions{Quality: float32(quality), Method: 4}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsAnimatedWebP 动画 WebP 检测
func IsAnimatedWebP(data []byte) bool {
	feat, err := webp.GetFeatures(bytes.NewReader(data))
	if err != nil {
		return false
	}
	return feat.HasAnimation
}

func fallback(data []byte, ext, mime string) *Result {
	w, h, _ := DecodeConfig(data)
	return &Result{Data: data, Ext: ext, Mime: mime, Width: w, Height: h, Original: true}
}

// ProcessCompressed 压缩模式完整流水线；返回结果（可能回退原图直存）
func ProcessCompressed(data []byte, resizeMaxDim, quality int) (*Result, error) {
	ext, mime, ok := Detect(data)
	if !ok {
		return nil, ErrUnsupported
	}

	// GIF 一律、动画 WebP 回退原图直存
	if ext == "gif" {
		return fallback(data, ext, mime), nil
	}
	if ext == "webp" {
		if IsAnimatedWebP(data) {
			return fallback(data, ext, mime), nil
		}
		if w, h, err := DecodeConfig(data); err == nil && w <= resizeMaxDim && h <= resizeMaxDim {
			return &Result{Data: data, Ext: "webp", Mime: "image/webp", Width: w, Height: h}, nil
		}
	}

	// 读 EXIF 方向并物理旋转（Go 解码不含 EXIF）
	orient := Orientation(data)
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fallback(data, ext, mime), nil
	}
	if orient != 1 {
		img = Rotate(img, orient)
	}
	img = Scale(img, resizeMaxDim)
	out, err := EncodeWebP(img, quality)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	return &Result{Data: out, Ext: "webp", Mime: "image/webp", Width: b.Dx(), Height: b.Dy()}, nil
}
