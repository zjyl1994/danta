package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/zjyl1994/danta/internal/server"
	"github.com/zjyl1994/danta/internal/settings"
	"github.com/zjyl1994/danta/internal/store"
)

// loadDotEnv 简易 .env 解析（已存在环境变量优先）
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	loadDotEnv()

	listen := envOr("DANTA_LISTEN", "127.0.0.1:32682")
	dataDir := envOr("DANTA_DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "danta.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	_ = db.Exec("PRAGMA journal_mode=WAL").Error
	_ = db.Exec("PRAGMA busy_timeout=5000").Error

	st := store.New(db)
	if err := st.AutoMigrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	sm := settings.New(st)
	if err := sm.Load(); err != nil {
		log.Fatalf("settings load: %v", err)
	}

	app := server.New(st, sm)
	log.Printf("danta listening on %s (data: %s)", listen, dbPath)
	if err := app.Listen(listen, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
