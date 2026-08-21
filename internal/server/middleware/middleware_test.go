package middleware

import (
	"fmt"
	"testing"
	"time"
)

func TestRateEvictsIdle(t *testing.T) {
	r := NewRate(100, time.Minute, func() string { return "none" })
	for i := 0; i < 10; i++ {
		r.Allow("1.2.3.4")
	}
	// 模拟时间流逝后，过期条目应被淘汰；用 64 个不同 IP 触发 sweep
	r.hits["idle"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	for i := 0; i < 70; i++ {
		r.Allow(fmt.Sprintf("ip-%d", i))
	}
	if _, ok := r.hits["idle"]; ok {
		t.Fatal("idle entry should be evicted")
	}
	if _, ok := r.hits["1.2.3.4"]; !ok {
		t.Fatal("active entry should remain")
	}
}

func TestRateLimitAndExpireCycle(t *testing.T) {
	r := NewRate(2, time.Minute, func() string { return "none" })
	if !r.Allow("a") || !r.Allow("a") {
		t.Fatal("should allow within limit")
	}
	if r.Allow("a") {
		t.Fatal("should be limited")
	}
	// 淘汰空切片条目（用不同 IP 触发 sweep）
	r.hits["empty"] = []time.Time{}
	for i := 0; i < 70; i++ {
		r.Allow(fmt.Sprintf("ip-%d", i))
	}
	if _, ok := r.hits["empty"]; ok {
		t.Fatal("empty entry should be evicted")
	}
}

func TestBanEvictsExpired(t *testing.T) {
	b := NewBan(5, 900, 900)
	b.banned["old"] = time.Now().Add(-time.Minute)
	b.banned["active"] = time.Now().Add(time.Minute)
	for i := 0; i < 32; i++ {
		b.Check("probe")
	}
	if _, ok := b.banned["old"]; ok {
		t.Fatal("expired ban should be evicted")
	}
	if _, ok := b.banned["active"]; !ok {
		t.Fatal("active ban should remain")
	}
	b.hits["idle"] = []time.Time{time.Now().Add(-2 * time.Hour)}
	for i := 0; i < 32; i++ {
		b.Fail("other")
	}
	if _, ok := b.hits["idle"]; ok {
		t.Fatal("stale fail entry should be evicted")
	}
}
