package library

import (
	"sync"
	"time"
)

// RateLimiter 是进程内的滑动窗口限流器，按 key（通常是客户端 IP）计数。
//
// 别和 provider.Limiter 混用，两者定位不同：
//   - provider.Limiter 是站点级流量限制，超限封禁 1 小时，且豁免内网 IP；
//   - RateLimiter 用于「被批量调用就会泄漏信息或消耗外部配额」的端点
//     （邮箱枚举、发信轰炸），不封禁、不豁免内网，只限制窗口内调用次数。
//
// limit <= 0 表示不限制（配置错误时的兜底：宁可放开，也不能把全站账号流程锁死）。
type RateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration

	// now 便于测试注入时钟；生产恒为 time.Now。
	now func() time.Time

	lastGC time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
		now:    time.Now,
	}
}

// Allow 记录一次调用并返回是否放行。被拒绝的调用不记账，
// 因此持续重试不会顺延窗口。
func (r *RateLimiter) Allow(key string) bool {
	if r == nil || r.limit <= 0 || r.window <= 0 {
		return true
	}

	now := r.now()
	cutoff := now.Add(-r.window)

	r.mu.Lock()
	defer r.mu.Unlock()

	kept := make([]time.Time, 0, len(r.hits[key])+1)
	for _, t := range r.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	allowed := len(kept) < r.limit
	if allowed {
		kept = append(kept, now)
	}
	r.hits[key] = kept
	r.collectGarbage(now, cutoff)

	return allowed
}

// Reset 清空所有计数，供测试和管理操作使用。
func (r *RateLimiter) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hits = make(map[string][]time.Time)
	r.lastGC = time.Time{}
}

// collectGarbage 回收窗口内无调用的 key。每个窗口最多扫一次，避免每次请求都遍历整张表。
// 调用方需持有 r.mu。
func (r *RateLimiter) collectGarbage(now, cutoff time.Time) {
	if now.Sub(r.lastGC) < r.window {
		return
	}
	r.lastGC = now
	for key, times := range r.hits {
		if len(times) == 0 || !times[len(times)-1].After(cutoff) {
			delete(r.hits, key)
		}
	}
}
