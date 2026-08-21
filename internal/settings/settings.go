package settings

import (
	"strconv"
	"sync"

	"github.com/zjyl1994/danta/internal/securetoken"
	"github.com/zjyl1994/danta/internal/store"
)

// Settings typed 配置
type Settings struct {
	AdminPasswordHash string
	MasterSecret      string

	CDNHost           string
	R2Endpoint        string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	ProxyMode         string
	BackgroundImage   string

	MaxUploadBytes int64
	WebPQuality    int
	ResizeMaxDim   int
	CacheMaxAge    int

	LoginFailLimit  int
	LoginFailWindow int
	LoginBanSeconds int
	SessionTTL      int
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
	defaultSessionTTL     = 30
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
		if secret, err = securetoken.Hex(32); err != nil {
			return err
		}
		if err = m.store.SetSetting("master_secret", secret); err != nil {
			return err
		}
	}

	return m.loadLocked(secret)
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
	return m.loadLocked(m.cur.MasterSecret)
}

// SetOne 写单个配置
func (m *Manager) SetOne(k, v string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.SetSetting(k, v); err != nil {
		return err
	}
	return m.loadLocked(m.cur.MasterSecret)
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

func (m *Manager) loadLocked(masterSecret string) error {
	s, err := m.read(masterSecret)
	if err != nil {
		return err
	}
	m.cur = s
	return nil
}

// read 读取持久化配置并应用默认值；调用方负责持有 m.mu。
func (m *Manager) read(masterSecret string) (Settings, error) {
	s := Settings{
		MasterSecret:    masterSecret,
		ProxyMode:       defaultProxyMode,
		MaxUploadBytes:  defaultMaxUploadBytes,
		WebPQuality:     defaultWebPQuality,
		ResizeMaxDim:    defaultResizeMaxDim,
		CacheMaxAge:     defaultCacheMaxAge,
		LoginFailLimit:  defaultLoginFailLimit,
		LoginFailWindow: defaultLoginWindow,
		LoginBanSeconds: defaultLoginBanSec,
		SessionTTL:      defaultSessionTTL,
	}
	pairs := map[string]*string{
		"admin.password_hash":  &s.AdminPasswordHash,
		"cdn_host":             &s.CDNHost,
		"r2.endpoint":          &s.R2Endpoint,
		"r2.access_key_id":     &s.R2AccessKeyID,
		"r2.secret_access_key": &s.R2SecretAccessKey,
		"r2.bucket":            &s.R2Bucket,
		"proxy_mode":           &s.ProxyMode,
		"background_image":     &s.BackgroundImage,
	}
	for k, p := range pairs {
		v, err := m.store.GetSetting(k)
		if err != nil {
			return Settings{}, err
		}
		*p = v
	}
	if s.ProxyMode != "local" && s.ProxyMode != "none" {
		s.ProxyMode = defaultProxyMode
	}
	values := []struct {
		key   string
		apply func(string)
	}{
		{"max_upload_bytes", func(v string) { s.MaxUploadBytes = int64Val(v, defaultMaxUploadBytes) }},
		{"webp_quality", func(v string) { s.WebPQuality = intVal(v, defaultWebPQuality) }},
		{"resize_max_dim", func(v string) { s.ResizeMaxDim = intVal(v, defaultResizeMaxDim) }},
		{"cache_max_age", func(v string) { s.CacheMaxAge = intVal(v, defaultCacheMaxAge) }},
		{"security.login_fail_limit", func(v string) { s.LoginFailLimit = intVal(v, defaultLoginFailLimit) }},
		{"security.login_fail_window", func(v string) { s.LoginFailWindow = intVal(v, defaultLoginWindow) }},
		{"security.login_ban_seconds", func(v string) { s.LoginBanSeconds = intVal(v, defaultLoginBanSec) }},
		{"security.session_ttl", func(v string) { s.SessionTTL = intVal(v, defaultSessionTTL) }},
	}
	for _, value := range values {
		v, err := m.store.GetSetting(value.key)
		if err != nil {
			return Settings{}, err
		}
		value.apply(v)
	}
	return s, nil
}
