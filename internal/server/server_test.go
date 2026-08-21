package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/zjyl1994/danta/internal/imageproc"
	"github.com/zjyl1994/danta/internal/server"
	"github.com/zjyl1994/danta/internal/settings"
	"github.com/zjyl1994/danta/internal/storage"
	"github.com/zjyl1994/danta/internal/store"
)

const boundary = "XXBOUNDARY"

// ---- test helpers ----

func newTestStore(t *testing.T) (*store.Store, *settings.Manager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if err := st.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	sm := settings.New(st)
	if err := sm.Load(); err != nil {
		t.Fatal(err)
	}
	return st, sm
}

type fakeStorage struct {
	keys map[string]storage.ObjectInfo
	data map[string][]byte
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{keys: map[string]storage.ObjectInfo{}, data: map[string][]byte{}}
}

func (f *fakeStorage) Put(key string, data []byte, _ string, _ int) error {
	f.keys[key] = storage.ObjectInfo{Key: key, Size: int64(len(data)), LastModified: time.Now()}
	f.data[key] = data
	return nil
}
func (f *fakeStorage) Delete(keys []string) error {
	for _, k := range keys {
		delete(f.keys, k)
		delete(f.data, k)
	}
	return nil
}
func (f *fakeStorage) ListAll(fn func(storage.ObjectInfo) error) error {
	for _, o := range f.keys {
		if err := fn(o); err != nil {
			return err
		}
	}
	return nil
}
func (f *fakeStorage) ListPrefix(prefix string, fn func(storage.ObjectInfo) error) error {
	for k, o := range f.keys {
		if strings.HasPrefix(k, prefix) {
			if err := fn(o); err != nil {
				return err
			}
		}
	}
	return nil
}

type fakeProvider struct{ stg *fakeStorage }

func (p *fakeProvider) Storage(_ settings.Settings) (storage.Storage, error) {
	return p.stg, nil
}

func setupConfigured(t *testing.T) (*fiber.App, *fakeStorage, *store.Store) {
	t.Helper()
	st, sm := newTestStore(t)
	fs := newFakeStorage()
	app := server.NewWithDeps(st, sm, &fakeProvider{stg: fs})
	if err := sm.Set(map[string]string{
		"cdn_host":             "cdn.example.com",
		"r2.endpoint":          "https://x.r2.cloudflarestorage.com",
		"r2.access_key_id":     "ak",
		"r2.secret_access_key": "sk",
		"r2.bucket":            "bucket",
	}); err != nil {
		t.Fatal(err)
	}
	return app, fs, st
}

func doReq(t *testing.T, app *fiber.App, method, path string, headers map[string]string, body []byte) (*http.Response, map[string]interface{}) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if strings.Contains(res.Header.Get("Content-Type"), "json") {
		_ = json.NewDecoder(res.Body).Decode(&out)
	}
	res.Body.Close()
	return res, out
}

func doTest(t *testing.T, app *fiber.App, method, path string, headers map[string]string, body []byte) (*http.Response, map[string]interface{}) {
	return doReq(t, app, method, path, headers, body)
}

func multipartBody(data []byte, filename, original string) []byte {
	var b bytes.Buffer
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", filename))
	b.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	b.Write(data)
	b.WriteString("\r\n")
	if original != "" {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Disposition: form-data; name=\"original\"\r\n\r\n")
		b.WriteString(original)
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.Bytes()
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	pal := color.Palette{color.Black, color.White, color.RGBA{255, 0, 0, 255}}
	pm := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for i := range pm.Pix {
		pm.Pix[i] = uint8(i % 3)
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, pm, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ---- tests ----

func TestStatusNotConfigured(t *testing.T) {
	st, sm := newTestStore(t)
	app := server.New(st, sm)
	res, out := doTest(t, app, http.MethodGet, "/api/status", nil, nil)
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if out["configured"] != false {
		t.Fatalf("expected not configured, got %v", out["configured"])
	}
}

func TestSetupLoginUploadFlow(t *testing.T) {
	app, fs, _ := setupConfigured(t)
	jsonHdr := map[string]string{"Content-Type": "application/json"}

	// setup
	res, _ := doTest(t, app, http.MethodPost, "/api/setup", jsonHdr, []byte(`{"password":"testpass123"}`))
	if res.StatusCode != 200 {
		t.Fatalf("setup %d", res.StatusCode)
	}

	// login
	res, out := doTest(t, app, http.MethodPost, "/api/login", jsonHdr, []byte(`{"password":"testpass123"}`))
	if res.StatusCode != 200 {
		t.Fatalf("login %d", res.StatusCode)
	}
	token, _ := out["token"].(string)
	refresh, _ := out["refresh_token"].(string)
	if token == "" || refresh == "" {
		t.Fatalf("empty token/refresh: %v", out)
	}

	authHdr := map[string]string{"Authorization": "Bearer " + token}
	res, out = doTest(t, app, http.MethodGet, "/api/status", authHdr, nil)
	if res.StatusCode != http.StatusOK || out["authed"] != true {
		t.Fatalf("authenticated status = %d, %v", res.StatusCode, out)
	}

	// 原图上传
	pngBytes := makePNG(t, 64, 32)
	res, out = doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + token, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(pngBytes, "test.png", "true"))
	if res.StatusCode != 200 {
		t.Fatalf("upload %d: %v", res.StatusCode, out)
	}
	if out["original"] != true {
		t.Fatalf("expected original upload, got %v", out["original"])
	}
	urlStr, _ := out["url"].(string)
	if !strings.HasPrefix(urlStr, "https://cdn.example.com/") {
		t.Fatalf("bad url %s", urlStr)
	}
	if len(fs.data) != 1 {
		t.Fatalf("expected 1 object, got %d", len(fs.data))
	}

	// 去重：同字节再次上传应命中，不新增对象
	res, _ = doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + token, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(pngBytes, "test.png", "true"))
	if res.StatusCode != 200 {
		t.Fatalf("upload dedup %d", res.StatusCode)
	}
	if len(fs.data) != 1 {
		t.Fatalf("dedup should not add object, got %d", len(fs.data))
	}

	// 压缩模式：PNG→WebP（尺寸未超限则直接重编码为 webp，尺寸不变）
	res, out = doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + token, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(makePNG(t, 100, 50), "small.png", ""))
	if res.StatusCode != 200 {
		t.Fatalf("upload compressed %d: %v", res.StatusCode, out)
	}
	if out["original"] == true {
		t.Fatalf("expected compressed, got original")
	}
	if len(fs.data) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(fs.data))
	}

	// stats
	res, out = doTest(t, app, http.MethodGet, "/api/admin/stats", authHdr, nil)
	if res.StatusCode != 200 {
		t.Fatalf("stats %d", res.StatusCode)
	}
	if out["images"].(float64) != 2 {
		t.Fatalf("expected 2 images, got %v", out["images"])
	}

	// 管理列表
	res, out = doTest(t, app, http.MethodGet, "/api/admin/images?page=1&size=10", authHdr, nil)
	if res.StatusCode != 200 {
		t.Fatalf("list %d", res.StatusCode)
	}
	if out["total"].(float64) != 2 {
		t.Fatalf("expected total 2, got %v", out["total"])
	}

	// refresh：续期后拿到新访问令牌
	res, out = doTest(t, app, http.MethodPost, "/api/refresh", jsonHdr, []byte(`{"refresh_token":"`+refresh+`"}`))
	if res.StatusCode != 200 || out["token"].(string) == "" {
		t.Fatalf("refresh = %d, %v", res.StatusCode, out)
	}
	newToken := out["token"].(string)

	// 原 refresh token 已滑动，可再次使用
	res, _ = doTest(t, app, http.MethodPost, "/api/refresh", jsonHdr, []byte(`{"refresh_token":"`+refresh+`"}`))
	if res.StatusCode != 200 {
		t.Fatalf("refresh slide %d", res.StatusCode)
	}

	// logout 吊销后 refresh 应 401
	doTest(t, app, http.MethodPost, "/api/logout", jsonHdr, []byte(`{"refresh_token":"`+refresh+`"}`))
	res, _ = doTest(t, app, http.MethodPost, "/api/refresh", jsonHdr, []byte(`{"refresh_token":"`+refresh+`"}`))
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d, want 401", res.StatusCode)
	}

	// 新令牌仍可访问管理 API
	authHdr2 := map[string]string{"Authorization": "Bearer " + newToken}
	res, _ = doTest(t, app, http.MethodGet, "/api/admin/images?page=1&size=10", authHdr2, nil)
	if res.StatusCode != 200 {
		t.Fatalf("list with refreshed token %d", res.StatusCode)
	}
}

func TestUploadTokenFlow(t *testing.T) {
	app, fs, _ := setupConfigured(t)
	token := setupAndLogin(t, app)
	authHdr := map[string]string{"Authorization": "Bearer " + token}
	jsonHdr := map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"}

	// 创建上传令牌
	res, out := doTest(t, app, http.MethodPost, "/api/admin/tokens", jsonHdr, []byte(`{"name":"命令行","days":30}`))
	if res.StatusCode != 200 {
		t.Fatalf("create token %d: %v", res.StatusCode, out)
	}
	uploadTok, _ := out["token"].(string)
	if uploadTok == "" {
		t.Fatal("empty upload token")
	}

	// 用上传令牌上传
	res, out = doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + uploadTok, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(makePNG(t, 16, 16), "tok.png", "true"))
	if res.StatusCode != 200 {
		t.Fatalf("upload with token %d: %v", res.StatusCode, out)
	}
	if len(fs.data) != 1 {
		t.Fatalf("expected 1 object, got %d", len(fs.data))
	}

	// 会话列表包含登录会话与上传令牌
	res, out = doTest(t, app, http.MethodGet, "/api/admin/sessions", authHdr, nil)
	if res.StatusCode != 200 {
		t.Fatalf("list sessions %d", res.StatusCode)
	}
	sessions, _ := out["sessions"].([]interface{})
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %v", len(sessions), out)
	}

	// 吊销上传令牌后应无法上传
	var revokeID interface{}
	for _, s := range sessions {
		m := s.(map[string]interface{})
		if m["kind"] == "upload" {
			revokeID = m["id"]
		}
	}
	res, _ = doTest(t, app, http.MethodPost, "/api/admin/sessions/"+fmt.Sprintf("%v", revokeID)+"/revoke", authHdr, nil)
	if res.StatusCode != 200 {
		t.Fatalf("revoke %d", res.StatusCode)
	}
	res, out = doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + uploadTok, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(makePNG(t, 16, 16), "tok2.png", "true"))
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("upload after revoke = %d, want 401: %v", res.StatusCode, out)
	}
}

func TestLoginSessionNameFromUA(t *testing.T) {
	app, _, st := setupConfigured(t)
	jsonHdr := map[string]string{"Content-Type": "application/json"}
	if res, _ := doTest(t, app, http.MethodPost, "/api/setup", jsonHdr, []byte(`{"password":"testpass123"}`)); res.StatusCode != 200 {
		t.Fatalf("setup %d", res.StatusCode)
	}
	loginHdr := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	}
	res, out := doTest(t, app, http.MethodPost, "/api/login", loginHdr, []byte(`{"password":"testpass123"}`))
	if res.StatusCode != 200 {
		t.Fatalf("login %d", res.StatusCode)
	}
	tok, _ := out["token"].(string)
	refresh, _ := out["refresh_token"].(string)
	res, out = doTest(t, app, http.MethodGet, "/api/admin/sessions", map[string]string{"Authorization": "Bearer " + tok}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("sessions %d", res.StatusCode)
	}
	sessions, _ := out["sessions"].([]interface{})
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d: %v", len(sessions), out)
	}
	name := sessions[0].(map[string]interface{})["name"]
	if name != "Safari · iPhone" {
		t.Fatalf("device name = %v, want Safari · iPhone", name)
	}
	sess, err := st.TokenByHash(store.TokenHash(refresh))
	if err != nil || sess == nil {
		t.Fatalf("load login session: %v", err)
	}
	if sess.LastUsedAt == nil {
		t.Fatal("login session last_used_at is nil")
	}
}

func TestUploadTokenValidation(t *testing.T) {
	app, _, _ := setupConfigured(t)
	// 未授权 token 上传应 401
	res, out := doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer not-a-token", "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(makePNG(t, 8, 8), "x.png", "true"))
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized upload = %d, want 401: %v", res.StatusCode, out)
	}
}

func TestSPAReturnsCompressedResponse(t *testing.T) {
	app, _, _ := setupConfigured(t)
	res, _ := doTest(t, app, http.MethodGet, "/", map[string]string{"Accept-Encoding": "gzip"}, nil)
	if got := res.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := res.Header.Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
}

func TestUploadRejectsNonImage(t *testing.T) {
	app, _, _ := setupConfigured(t)
	token := setupAndLogin(t, app)
	res, out := doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + token, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody([]byte("this is not an image at all"), "x.txt", ""))
	if res.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	if out["code"] != "unsupported_type" {
		t.Fatalf("expected unsupported_type, got %v", out["code"])
	}
}

func TestCleanupRespectsGrace(t *testing.T) {
	app, fs, _ := setupConfigured(t)
	token := setupAndLogin(t, app)
	// 上传一张
	_, _ = doTest(t, app, http.MethodPost, "/api/upload",
		map[string]string{"Authorization": "Bearer " + token, "Content-Type": "multipart/form-data; boundary=" + boundary},
		multipartBody(makePNG(t, 10, 10), "a.png", "true"))
	// 制造孤儿对象（在 DB 无记录）
	fs.Put("orphan-old", []byte("x"), "image/png", 60)
	fs.keys["orphan-old"] = storage.ObjectInfo{Key: "orphan-old", Size: 1, LastModified: time.Now().Add(-30 * time.Minute)}
	// 宽限内孤儿（不应删）
	fs.Put("orphan-fresh", []byte("y"), "image/png", 60)
	fs.keys["orphan-fresh"] = storage.ObjectInfo{Key: "orphan-fresh", Size: 1, LastModified: time.Now()}

	res, out := doTest(t, app, http.MethodPost, "/api/admin/cleanup",
		map[string]string{"Authorization": "Bearer " + token}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("cleanup %d: %v", res.StatusCode, out)
	}
	if _, ok := fs.keys["orphan-old"]; ok {
		t.Fatal("orphan-old should be deleted")
	}
	if _, ok := fs.keys["orphan-fresh"]; !ok {
		t.Fatal("orphan-fresh deleted unexpectedly")
	}
}

func TestImportClassifiesAndImportsCurrentObjects(t *testing.T) {
	app, fs, st := setupConfigured(t)
	token := setupAndLogin(t, app)
	modified := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	fs.keys["archive/new.JPG"] = storage.ObjectInfo{Key: "archive/new.JPG", Size: 10, LastModified: modified}
	fs.keys["archive/existing.png"] = storage.ObjectInfo{Key: "archive/existing.png", Size: 20, LastModified: modified}
	fs.keys["archive/notes.txt"] = storage.ObjectInfo{Key: "archive/notes.txt", Size: 30, LastModified: modified}
	if err := st.CreateImage(&store.Image{ObjectKey: "archive/existing.png", Name: "existing.png", Original: true, CreatedAt: modified}); err != nil {
		t.Fatal(err)
	}

	authHdr := map[string]string{"Authorization": "Bearer " + token}
	res, out := doTest(t, app, http.MethodGet, "/api/admin/import/scan?prefix=archive/", authHdr, nil)
	if res.StatusCode != http.StatusOK || out["total"] != float64(3) || out["new"] != float64(1) || out["existing"] != float64(1) || out["ignored"] != float64(1) {
		t.Fatalf("scan = %d, %v", res.StatusCode, out)
	}

	res, out = doTest(t, app, http.MethodPost, "/api/admin/import/run", map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}, []byte(`{"prefix":"archive/"}`))
	if res.StatusCode != http.StatusOK || out["imported"] != float64(1) || out["skipped"] != float64(1) || out["ignored"] != float64(1) {
		t.Fatalf("import = %d, %v", res.StatusCode, out)
	}
	if _, total, err := st.ListImages(1, 10); err != nil || total != 2 {
		t.Fatalf("imported image count = %d, err = %v", total, err)
	}
}

// ---- imageproc tests ----

func TestProcessCompressedJPEG(t *testing.T) {
	res, err := imageproc.ProcessCompressed(makeJPEG(t, 4000, 2000), 2560, 80)
	if err != nil {
		t.Fatal(err)
	}
	if res.Original {
		t.Fatal("expected compressed, got fallback")
	}
	if res.Ext != "webp" || res.Mime != "image/webp" {
		t.Fatalf("ext/mime %s/%s", res.Ext, res.Mime)
	}
	if res.Width != 2560 || res.Height != 1280 {
		t.Fatalf("dims %dx%d", res.Width, res.Height)
	}
}

func TestProcessCompressedGIFFallback(t *testing.T) {
	res, err := imageproc.ProcessCompressed(makeGIF(t, 4, 4), 2560, 80)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Original || res.Ext != "gif" {
		t.Fatalf("expected gif fallback, got original=%v ext=%s", res.Original, res.Ext)
	}
}

func TestDetectRejectsNonImage(t *testing.T) {
	if _, _, ok := imageproc.Detect([]byte("hello world not an image")); ok {
		t.Fatal("should reject non-image")
	}
}

func setupAndLogin(t *testing.T, app *fiber.App) string {
	t.Helper()
	jsonHdr := map[string]string{"Content-Type": "application/json"}
	res, _ := doTest(t, app, http.MethodPost, "/api/setup", jsonHdr, []byte(`{"password":"testpass123"}`))
	if res.StatusCode != 200 {
		t.Fatalf("setup %d", res.StatusCode)
	}
	_, out := doTest(t, app, http.MethodPost, "/api/login", jsonHdr, []byte(`{"password":"testpass123"}`))
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("empty token")
	}
	return token
}
