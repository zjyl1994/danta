package middleware

import "time"

// Ban 登录失败封禁：基于 windowCounter 统计失败次数。
// 触发响应（onOver）= 达到阈值时封禁该 IP 并清空其失败记录；
// 清理策略（onSweep）= 周期清除到期封禁。
// banned / banTime 为业务附加状态，与窗口共用同一把锁（嵌入 windowCounter），操作保持原子。
type Ban struct {
	*windowCounter
	banned  map[string]time.Time
	banTime time.Duration
}

func NewBan(failLimit, windowSec, banSec int) *Ban {
	b := &Ban{banned: map[string]time.Time{}, banTime: time.Duration(banSec) * time.Second}
	b.windowCounter = newWindowCounter(
		failLimit, time.Duration(windowSec)*time.Second, 32,
		func(now time.Time, key string) { // onOver：封禁并清空失败记录
			b.banned[key] = now.Add(b.banTime)
			delete(b.hits, key)
		},
		func(now time.Time, _ time.Time) { // onSweep：清除到期封禁
			pruneBanned(b.banned, now)
		},
	)
	return b
}

func (b *Ban) Configure(failLimit, windowSec, banSec int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.limit = failLimit
	b.window = time.Duration(windowSec) * time.Second
	b.banTime = time.Duration(banSec) * time.Second
}

// Check 返回是否被封禁及解封时间
func (b *Ban) Check(ip string) (bool, time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.maybeSweep(now)
	until, ok := b.banned[ip]
	if !ok {
		return false, time.Time{}
	}
	if now.Before(until) {
		return true, until
	}
	delete(b.banned, ip)
	return false, time.Time{}
}

// Fail 记录一次失败；达到阈值由 onOver 自动封禁
func (b *Ban) Fail(ip string) {
	b.Add(ip)
}

// Reset 成功登录后清零
func (b *Ban) Reset(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clearLocked(ip)
	delete(b.banned, ip)
}
