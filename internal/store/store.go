package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 提供图片与设置的数据库访问
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Image{}, &Setting{})
}

// ---- Image ----

// ListImages 倒序分页
func (s *Store) ListImages(page, size int) ([]Image, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	var total int64
	if err := s.db.Model(&Image{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Image
	err := s.db.Order("created_at DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

// FindByHash 按 (hash, original) 查重
func (s *Store) FindByHash(hash []byte, original bool) (*Image, error) {
	var img Image
	err := s.db.Where("hash = ? AND original = ?", hash, original).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (s *Store) GetByID(id uint) (*Image, error) {
	var img Image
	err := s.db.First(&img, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (s *Store) ImageExists(key string) (bool, error) {
	var n int64
	err := s.db.Model(&Image{}).Where("object_key = ?", key).Count(&n).Error
	return n > 0, err
}

func (s *Store) CreateImage(img *Image) error {
	return s.db.Create(img).Error
}

// ImagesByIDs 批量取记录（含 ObjectKey，供删除）
func (s *Store) ImagesByIDs(ids []uint) ([]Image, error) {
	var items []Image
	err := s.db.Where("id IN ?", ids).Find(&items).Error
	return items, err
}

func (s *Store) DeleteByIDs(ids []uint) error {
	return s.db.Where("id IN ?", ids).Delete(&Image{}).Error
}

// InsertImages 导入批量插入（已存在跳过）
func (s *Store) InsertImages(imgs []Image) (int64, error) {
	res := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&imgs)
	return res.RowsAffected, res.Error
}

// AllObjectKeys 返回 DB 中全部对象键
func (s *Store) AllObjectKeys() (map[string]struct{}, error) {
	var keys []string
	if err := s.db.Model(&Image{}).Pluck("object_key", &keys).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return set, nil
}

// Stats 统计：图片总数 / 总存储 / 近24h新增 / 原图直存数
func (s *Store) Stats() (images int64, totalSize int64, uploads24h int64, originals int64, err error) {
	if err = s.db.Model(&Image{}).Count(&images).Error; err != nil {
		return
	}
	if err = s.db.Model(&Image{}).Select("COALESCE(SUM(size),0)").Scan(&totalSize).Error; err != nil {
		return
	}
	since := time.Now().Add(-24 * time.Hour)
	if err = s.db.Model(&Image{}).Where("created_at > ?", since).Count(&uploads24h).Error; err != nil {
		return
	}
	if err = s.db.Model(&Image{}).Where("original = ?", true).Count(&originals).Error; err != nil {
		return
	}
	return
}

// ---- Setting ----

func (s *Store) GetSetting(key string) (string, error) {
	var row Setting
	err := s.db.First(&row, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Value, nil
}

func (s *Store) SetSetting(key, value string) error {
	return s.db.Save(&Setting{Key: key, Value: value, UpdatedAt: time.Now()}).Error
}
