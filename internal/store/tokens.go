package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/zjyl1994/danta/internal/securetoken"
)

// ---- Token ----

func (s *Store) CreateToken(t *Token) error {
	return s.db.Create(t).Error
}

// TokenByHash 按哈希查令牌
func (s *Store) TokenByHash(hash string) (*Token, error) {
	var t Token
	err := s.db.Where("token_hash = ?", hash).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) TokenByID(id uint) (*Token, error) {
	var t Token
	err := s.db.First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTokens 按创建时间倒序；kind 空串返回全部
func (s *Store) ListTokens(kind string) ([]Token, error) {
	q := s.db.Order("created_at DESC, id DESC")
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var ts []Token
	err := q.Find(&ts).Error
	return ts, err
}

// RevokeToken 吊销令牌（幂等）
func (s *Store) RevokeToken(id uint) error {
	now := time.Now()
	return s.db.Model(&Token{}).Where("id = ?", id).Update("revoked_at", &now).Error
}

// TouchToken 更新最后活跃时间并滑动有效期
func (s *Store) TouchToken(id uint, lastUsed time.Time, newExpiry *time.Time) error {
	return s.db.Model(&Token{}).Where("id = ?", id).
		Updates(map[string]interface{}{"last_used_at": lastUsed, "expires_at": newExpiry}).Error
}

// MarkTokenUsed 更新最后活跃时间
func (s *Store) MarkTokenUsed(id uint) error {
	return s.db.Model(&Token{}).Where("id = ?", id).Update("last_used_at", time.Now()).Error
}

// TokenHash 计算令牌的存储哈希
func TokenHash(raw string) string {
	return securetoken.SHA256Hex(raw)
}
