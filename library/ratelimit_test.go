package library

import (
	"sync"
	"testing"
	"time"
)

// 限流器是「邮箱枚举 / 发信轰炸」唯一的技术闸门，行为必须可预期：
// 窗口内放行 N 次，第 N+1 次拒绝，窗口滑过后重新放行。
func TestRateLimiterAllowsUpToLimitThenBlocks(t *testing.T) {
	l := NewRateLimiter(3, time.Minute)

	for i := 1; i <= 3; i++ {
		if !l.Allow("1.1.1.1") {
			t.Fatalf("窗口内第 %d 次被拒绝，限额是 3", i)
		}
	}
	if l.Allow("1.1.1.1") {
		t.Fatal("第 4 次仍被放行：限流没生效")
	}
}

func TestRateLimiterRefillsAfterWindow(t *testing.T) {
	l := NewRateLimiter(2, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }

	l.Allow("a")
	l.Allow("a")
	if l.Allow("a") {
		t.Fatal("窗口内第 3 次应被拒绝")
	}

	now = now.Add(61 * time.Second)
	if !l.Allow("a") {
		t.Fatal("窗口滑过后应重新放行，否则正常用户会被永久锁死")
	}
}

// 被拒绝的调用不能延长封禁窗口，否则一个持续重试的客户端永远出不来。
func TestRateLimiterRejectedCallsDoNotExtendWindow(t *testing.T) {
	l := NewRateLimiter(1, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }

	l.Allow("a")
	for i := 0; i < 10; i++ {
		now = now.Add(5 * time.Second)
		l.Allow("a")
	}
	now = now.Add(30 * time.Second) // 距首次调用 80s，已超出窗口
	if !l.Allow("a") {
		t.Fatal("被拒绝的调用不该顺延窗口：首次调用满一分钟后就应放行")
	}
}

func TestRateLimiterCountsKeysIndependently(t *testing.T) {
	l := NewRateLimiter(1, time.Minute)

	if !l.Allow("a") {
		t.Fatal("key a 首次应放行")
	}
	if l.Allow("a") {
		t.Fatal("key a 第二次应拒绝")
	}
	if !l.Allow("b") {
		t.Fatal("key b 有独立额度，不该被 key a 的消耗影响")
	}
}

// limit <= 0 是「关闭限流」的约定写法：配置错误时必须谁都不拦，
// 不能因为一个 0 值把全站账号流程锁死。
func TestRateLimiterWithNonPositiveLimitAllowsAll(t *testing.T) {
	l := NewRateLimiter(0, time.Minute)
	for i := 0; i < 100; i++ {
		if !l.Allow("a") {
			t.Fatalf("limit=0 表示不限流，第 %d 次却拒绝了", i+1)
		}
	}
}

// 每个 IP 一条记录，长期运行必须回收，否则地图无限增长（白盒断言：
// 「过期记录不再占内存」只能从内部结构观察）。
func TestRateLimiterForgetsIdleKeys(t *testing.T) {
	l := NewRateLimiter(5, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }

	l.Allow("a")
	now = now.Add(2 * time.Minute)
	l.Allow("b")

	l.mu.Lock()
	remaining := len(l.hits)
	_, aStillThere := l.hits["a"]
	l.mu.Unlock()

	if aStillThere || remaining != 1 {
		t.Fatalf("过期 key 应被回收，期望只剩 1 个，实际 %d 个（a 还在=%v）", remaining, aStillThere)
	}
}

// 并发下计数必须精确：竞态会让限额被突破，限流形同虚设。
// 用 go test -race 跑才有意义。
func TestRateLimiterIsConcurrencySafe(t *testing.T) {
	l := NewRateLimiter(50, time.Minute)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("k") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 50 {
		t.Fatalf("并发下应恰好放行 50 次，实际 %d 次", allowed)
	}
}
