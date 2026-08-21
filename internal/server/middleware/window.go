package middleware

import (
	"sync"
	"time"
)

// windowCounter 按 key 的滑动窗口超限判断器（限流 Rate 与封禁 Ban 共用的数据结构）。
//
// 职责内聚：窗口剪枝、入账、阈值判定、周期淘汰全部由本类型自行完成，
// 业务差异通过两个钩子注入：
//   - onOver  超限时的触发响应（如封禁该 key、拒绝入账等）
//   - onSweep 周期淘汰时清理业务附加状态（如封禁列表）
//
// 钩子在持有 mu 的锁内同步调用，回调内禁止再调用本类型的加锁方法；
// 嵌入者与窗口共享同一把锁，可在持锁状态下调用 *Locked 方法完成复合原子操作。
type windowCounter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	sweepN  uint32 // 清理策略：每多少次 add/check 触发一次全量淘汰（0=禁用）
	sweeps  uint32
	hits    map[string][]time.Time
	onOver  func(now time.Time, key string)
	onSweep func(now time.Time, cutoff time.Time)
}

func newWindowCounter(limit int, window time.Duration, sweepN uint32,
	onOver func(now time.Time, key string), onSweep func(now time.Time, cutoff time.Time)) *windowCounter {
	return &windowCounter{
		limit:   limit,
		window:  window,
		sweepN:  sweepN,
		hits:    map[string][]time.Time{},
		onOver:  onOver,
		onSweep: onSweep,
	}
}

// Add 线程安全：记录一次命中；超限不入账、触发 onOver 并返回 true
func (w *windowCounter) Add(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.addLocked(time.Now(), key)
}

// addLocked 窗口记录（须持有 mu）；超限不入账、触发 onOver 并返回 true
func (w *windowCounter) addLocked(now time.Time, key string) bool {
	cutoff := now.Add(-w.window)
	arr := w.hits[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	over := len(kept) >= w.limit
	if over {
		w.hits[key] = kept
		if w.onOver != nil {
			w.onOver(now, key)
		}
	} else {
		w.hits[key] = append(kept, now)
	}
	w.maybeSweep(now)
	return over
}

// maybeSweep 按清理策略（sweepN）周期触发全量淘汰（须持有 mu）
func (w *windowCounter) maybeSweep(now time.Time) {
	if w.sweepN > 0 && sweepEvery(&w.sweeps, w.sweepN) {
		w.sweepLocked(now)
	}
}

// sweepLocked 立即全量淘汰窗口外条目与业务附加状态（须持有 mu）
func (w *windowCounter) sweepLocked(now time.Time) {
	cutoff := now.Add(-w.window)
	pruneTimes(w.hits, cutoff)
	if w.onSweep != nil {
		w.onSweep(now, cutoff)
	}
}

// clearLocked 清空某 key 的全部记录（须持有 mu）
func (w *windowCounter) clearLocked(key string) {
	delete(w.hits, key)
}

// sweepEvery 自增计数；每 n 次返回 true（调用方据此触发一次全量淘汰）
func sweepEvery(trigger *uint32, n uint32) bool {
	*trigger++
	return *trigger%n == 0
}

// pruneTimes 删除窗口外的条目（时间有序，末尾即最新）；空切片一并删除，防 map 无限增长
func pruneTimes(m map[string][]time.Time, cutoff time.Time) {
	for k, ts := range m {
		if len(ts) == 0 || ts[len(ts)-1].Before(cutoff) {
			delete(m, k)
		}
	}
}

// pruneBanned 删除已到期（<= now）的封禁记录
func pruneBanned(m map[string]time.Time, now time.Time) {
	for k, until := range m {
		if !now.Before(until) {
			delete(m, k)
		}
	}
}
