package store

import "time"

// Image 图片记录（对象存储元数据镜像）
type Image struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ObjectKey string    `gorm:"uniqueIndex" json:"objectkey"`              // R2 对象键；URL={cdn_host}/{ObjectKey}
	Name      string    `json:"name"`                                       // 文件名（上传=原始名；导入=键 basename；仅后台可见）
	Mime      string    `json:"mime"`                                       // 实际存储 MIME（上传有值；导入恒空）
	Size      int64     `json:"size"`                                       // R2 存储字节数
	Width     int       `json:"width"`                                      // 压缩=缩放后；原图=原尺寸；导入恒 0
	Height    int       `json:"height"`
	Hash      []byte    `gorm:"uniqueIndex:idx_hash_original" json:"-"`     // SHA-256 原始字节；与 Original 复合唯一，可空（导入恒为 NULL）
	Original  bool      `gorm:"uniqueIndex:idx_hash_original" json:"original"` // true=原图直存；false=缩放+WebP；导入默认 true
	CreatedAt time.Time `gorm:"index" json:"created_at"`                    // 上传时间/导入=LastModified
}

// Setting 键值设置
type Setting struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `json:"value"` // JSON
	UpdatedAt time.Time `json:"updated_at"`
}
