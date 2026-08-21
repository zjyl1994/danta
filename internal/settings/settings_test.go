package settings

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/zjyl1994/danta/internal/store"
)

func newTestManager(t *testing.T) (*store.Store, *Manager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if err := st.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	return st, New(st)
}

func TestLoadAndReloadUseTheSameDefaults(t *testing.T) {
	st, manager := newTestManager(t)
	if err := manager.Load(); err != nil {
		t.Fatal(err)
	}
	initial := manager.Get()
	if len(initial.MasterSecret) != 64 {
		t.Fatalf("master secret length = %d, want 64", len(initial.MasterSecret))
	}

	if err := manager.Set(map[string]string{
		"upload_key":                 "upload-key",
		"proxy_mode":                 "invalid",
		"max_upload_bytes":           "invalid",
		"webp_quality":               "invalid",
		"resize_max_dim":             "invalid",
		"cache_max_age":              "invalid",
		"security.login_fail_limit":  "invalid",
		"security.login_fail_window": "invalid",
		"security.login_ban_seconds": "invalid",
	}); err != nil {
		t.Fatal(err)
	}
	reloaded := manager.Get()
	if reloaded.MasterSecret != initial.MasterSecret || reloaded.UploadKey != "upload-key" {
		t.Fatalf("reload lost persisted values: %#v", reloaded)
	}
	if reloaded.ProxyMode != defaultProxyMode || reloaded.MaxUploadBytes != defaultMaxUploadBytes ||
		reloaded.WebPQuality != defaultWebPQuality || reloaded.ResizeMaxDim != defaultResizeMaxDim ||
		reloaded.CacheMaxAge != defaultCacheMaxAge || reloaded.LoginFailLimit != defaultLoginFailLimit ||
		reloaded.LoginFailWindow != defaultLoginWindow || reloaded.LoginBanSeconds != defaultLoginBanSec {
		t.Fatalf("unexpected defaults after reload: %#v", reloaded)
	}

	loadedAgain := New(st)
	if err := loadedAgain.Load(); err != nil {
		t.Fatal(err)
	}
	if got := loadedAgain.Get(); got != reloaded {
		t.Fatalf("Load and reload differ:\nLoad: %#v\nReload: %#v", got, reloaded)
	}
}
