package idgen

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// New 生成 26 字符小写 ULID（扁平桶根对象键前缀）
func New() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(rand.Reader, 0))
	return strings.ToLower(id.String())
}
