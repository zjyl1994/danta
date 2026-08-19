package middleware

import (
	"sync"
	"time"
)

// Ban 登录失败内存滑窗封禁
type Ban struct {
	mu        sync.Mutex
	failLimit int
	window    time.Duration
	banTime   time.Duration
	fails     map[string][]time.Time
	banned    map[string]time.Time
}

func NewBan(failLimit, windowSec, banSec int) *Ban {
	return &Ban{
		failLimit: failLimit,
		window:    time.Duration(windowSec) * time.Second,
		banTime:   time.Duration(banSec) * time.Second,
		fails:     map[string][]time.Time{},
		banned:    map[string]time.Time{},
	}
}

func (b *Ban) Configure(failLimit, windowSec, banSec int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failLimit = failLimit
	b.window = time.Duration(windowSec) * time.Second
	b.banTime = time.Duration(banSec) * time.Second
}

// Check 返回是否被封禁及解封时间
func (b *Ban) Check(ip string) (bool, time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	until, ok := b.banned[ip]
	if !ok {
		return false, time.Time{}
	}
	if time.Now().Before(until) {
		return true, until
	}
	delete(b.banned, ip)
	return false, time.Time{}
}

// Fail 记录一次失败；达到阈值封禁
func (b *Ban) Fail(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-b.window)
	arr := b.fails[ip]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	if len(kept) >= b.failLimit {
		b.banned[ip] = now.Add(b.banTime)
		delete(b.fails, ip)
		return
	}
	b.fails[ip] = kept
}

// Reset 成功登录后清零
func (b *Ban) Reset(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.fails, ip)
	delete(b.banned, ip)
}
