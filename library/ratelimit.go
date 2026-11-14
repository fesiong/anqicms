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
	return r.allow(key, true)
}

// Check 只判断窗口内是否还有名额，绝不写 hits（不记新账、不消耗额度）。
// 与 Allow 共用同一私有 helper 的早返回、窗口清理与判定口径；
// collectGarbage 幂等，不改语义。用于组合限流的「先双检查、再双记账」（R69）。
func (r *RateLimiter) Check(key string) bool {
	return r.allow(key, false)
}

// allow 是 Allow / Check 的共享实现：record=true 为 Allow 语义（放行才记一次账），
// record=false 为 Check 语义（只在内存里算 kept 做判定，不写 hits）。
// 两者共用同一套早返回（r==nil / limit<=0 / window<=0 一律放行，配置错误兜底）、
// 同样的窗口清理、同样的「放行才记账」判定；Allow 既有行为一字不变。
func (r *RateLimiter) allow(key string, record bool) bool {
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
	if record {
		if allowed {
			kept = append(kept, now)
		}
		r.hits[key] = kept
	}
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
