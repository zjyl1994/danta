package settings

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync"

	"github.com/zjyl1994/danta/internal/store"
)

// Settings typed 配置
type Settings struct {
	AdminPasswordHash string
	MasterSecret      string
	UploadKey         string

	CDNHost           string
	R2Endpoint        string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	ProxyMode         string

	MaxUploadBytes  int64
	WebPQuality     int
	ResizeMaxDim    int
	CacheMaxAge     int

	LoginFailLimit  int
	LoginFailWindow int
	LoginBanSeconds int
}

const (
	defaultProxyMode      = "none"
	defaultMaxUploadBytes = 15 * 1024 * 1024
	defaultWebPQuality    = 80
	defaultResizeMaxDim   = 2560
	defaultCacheMaxAge    = 86400
	defaultLoginFailLimit = 5
	defaultLoginWindow    = 900
	defaultLoginBanSec    = 900
)

// Manager 读取/写入 settings 表并缓存于内存
type Manager struct {
	store *store.Store
	mu    sync.RWMutex
	cur   Settings
}

func New(st *store.Store) *Manager {
	return &Manager{store: st}
}

func randHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func intVal(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func int64Val(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

// Load 从 DB 载入全部配置；master_secret 空则生成并落库（首次建库生成）
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret, err := m.store.GetSetting("master_secret")
	if err != nil {
		return err
	}
	if secret == "" {
		if secret, err = randHex(32); err != nil {
			return err
		}
		if err = m.store.SetSetting("master_secret", secret); err != nil {
			return err
		}
	}

	s := Settings{
		MasterSecret:      secret,
		ProxyMode:         defaultProxyMode,
		MaxUploadBytes:    defaultMaxUploadBytes,
		WebPQuality:       defaultWebPQuality,
		ResizeMaxDim:      defaultResizeMaxDim,
		CacheMaxAge:       defaultCacheMaxAge,
		LoginFailLimit:    defaultLoginFailLimit,
		LoginFailWindow:   defaultLoginWindow,
		LoginBanSeconds:   defaultLoginBanSec,
	}

	pairs := map[string]*string{
		"admin.password_hash":       &s.AdminPasswordHash,
		"upload_key":                &s.UploadKey,
		"cdn_host":                  &s.CDNHost,
		"r2.endpoint":               &s.R2Endpoint,
		"r2.access_key_id":          &s.R2AccessKeyID,
		"r2.secret_access_key":      &s.R2SecretAccessKey,
		"r2.bucket":                 &s.R2Bucket,
		"proxy_mode":                &s.ProxyMode,
	}
	for k, p := range pairs {
		v, err := m.store.GetSetting(k)
		if err != nil {
			return err
		}
		*p = v
	}
	if s.ProxyMode != "local" && s.ProxyMode != "none" {
		s.ProxyMode = defaultProxyMode
	}

	if v, err := m.store.GetSetting("max_upload_bytes"); err != nil {
		return err
	} else {
		s.MaxUploadBytes = int64Val(v, defaultMaxUploadBytes)
	}
	if v, err := m.store.GetSetting("webp_quality"); err != nil {
		return err
	} else {
		s.WebPQuality = intVal(v, defaultWebPQuality)
	}
	if v, err := m.store.GetSetting("resize_max_dim"); err != nil {
		return err
	} else {
		s.ResizeMaxDim = intVal(v, defaultResizeMaxDim)
	}
	if v, err := m.store.GetSetting("cache_max_age"); err != nil {
		return err
	} else {
		s.CacheMaxAge = intVal(v, defaultCacheMaxAge)
	}
	if v, err := m.store.GetSetting("security.login_fail_limit"); err != nil {
		return err
	} else {
		s.LoginFailLimit = intVal(v, defaultLoginFailLimit)
	}
	if v, err := m.store.GetSetting("security.login_fail_window"); err != nil {
		return err
	} else {
		s.LoginFailWindow = intVal(v, defaultLoginWindow)
	}
	if v, err := m.store.GetSetting("security.login_ban_seconds"); err != nil {
		return err
	} else {
		s.LoginBanSeconds = intVal(v, defaultLoginBanSec)
	}

	m.cur = s
	return nil
}

// Get 返回当前配置副本
func (m *Manager) Get() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cur
}

// Set 批量写配置并重载；value 为空表示不改动（配合掩码回显）
func (m *Manager) Set(values map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range values {
		if err := m.store.SetSetting(k, v); err != nil {
			return err
		}
	}
	return m.loadLocked()
}

// SetOne 写单个配置
func (m *Manager) SetOne(k, v string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.SetSetting(k, v); err != nil {
		return err
	}
	return m.loadLocked()
}

// Configured 是否完成 setup（已设密码）
func (m *Manager) Configured() bool {
	return m.Get().AdminPasswordHash != ""
}

// R2Configured R2 与 cdn_host 是否就绪
func (s Settings) R2Configured() bool {
	return s.CDNHost != "" && s.R2Endpoint != "" && s.R2AccessKeyID != "" &&
		s.R2SecretAccessKey != "" && s.R2Bucket != ""
}

func (m *Manager) loadLocked() error {
	s := m.cur
	pairs := map[string]*string{
		"admin.password_hash":       &s.AdminPasswordHash,
		"upload_key":                &s.UploadKey,
		"cdn_host":                  &s.CDNHost,
		"r2.endpoint":               &s.R2Endpoint,
		"r2.access_key_id":          &s.R2AccessKeyID,
		"r2.secret_access_key":      &s.R2SecretAccessKey,
		"r2.bucket":                 &s.R2Bucket,
		"proxy_mode":                &s.ProxyMode,
	}
	for k, p := range pairs {
		v, err := m.store.GetSetting(k)
		if err != nil {
			return err
		}
		*p = v
	}
	if s.ProxyMode != "local" && s.ProxyMode != "none" {
		s.ProxyMode = defaultProxyMode
	}
	if v, err := m.store.GetSetting("max_upload_bytes"); err != nil {
		return err
	} else {
		s.MaxUploadBytes = int64Val(v, defaultMaxUploadBytes)
	}
	if v, err := m.store.GetSetting("webp_quality"); err != nil {
		return err
	} else {
		s.WebPQuality = intVal(v, defaultWebPQuality)
	}
	if v, err := m.store.GetSetting("resize_max_dim"); err != nil {
		return err
	} else {
		s.ResizeMaxDim = intVal(v, defaultResizeMaxDim)
	}
	if v, err := m.store.GetSetting("cache_max_age"); err != nil {
		return err
	} else {
		s.CacheMaxAge = intVal(v, defaultCacheMaxAge)
	}
	if v, err := m.store.GetSetting("security.login_fail_limit"); err != nil {
		return err
	} else {
		s.LoginFailLimit = intVal(v, defaultLoginFailLimit)
	}
	if v, err := m.store.GetSetting("security.login_fail_window"); err != nil {
		return err
	} else {
		s.LoginFailWindow = intVal(v, defaultLoginWindow)
	}
	if v, err := m.store.GetSetting("security.login_ban_seconds"); err != nil {
		return err
	} else {
		s.LoginBanSeconds = intVal(v, defaultLoginBanSec)
	}
	m.cur = s
	return nil
}
