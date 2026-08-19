package storage

import (
	"errors"
	"fmt"
	"sync"

	"github.com/zjyl1994/danta/internal/settings"
)

// Provider 按 settings 惰性构建并缓存 R2 客户端；R2 配置变更时自动重建
type Provider struct {
	mu     sync.Mutex
	stg    Storage
	cfgKey string
}

func (p *Provider) Storage(s settings.Settings) (Storage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s.R2Bucket == "" || s.R2Endpoint == "" || s.R2AccessKeyID == "" || s.R2SecretAccessKey == "" {
		p.stg = nil
		p.cfgKey = ""
		return nil, ErrNotConfigured
	}
	key := fmt.Sprintf("%s|%s|%s|%s", s.R2Endpoint, s.R2AccessKeyID, s.R2SecretAccessKey, s.R2Bucket)
	if p.stg == nil || p.cfgKey != key {
		stg, err := NewR2(s.R2Endpoint, s.R2AccessKeyID, s.R2SecretAccessKey, s.R2Bucket)
		if err != nil {
			return nil, err
		}
		p.stg = stg
		p.cfgKey = key
	}
	return p.stg, nil
}

var ErrNoStorage = errors.New("no storage")
