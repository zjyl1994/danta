package imageproc

import (
	"testing"
)

func TestExceedsDecodeLimit(t *testing.T) {
	cases := []struct {
		w, h int
		want bool
	}{
		{2560, 1440, false},
		{16384, 16384, true},
		{100000, 100000, true},
		{20000, 100, true},  // 单边超限
		{10000, 5000, true}, // 总像素超限 (50M > 40M)
		{4000, 4000, false}, // 16M 像素，正常压缩路径
	}
	for _, c := range cases {
		if got := exceedsDecodeLimit(c.w, c.h); got != c.want {
			t.Errorf("exceedsDecodeLimit(%d,%d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}
