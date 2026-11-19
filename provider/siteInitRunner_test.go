package provider

// 启动期并发初始化的护栏（provider/siteInitRunner.go）。
//
// 这里全部用假的 init 函数跑，不连库：验的是调度本身 —— 并发度是否真的到 2、
// 站点 1 是否先跑、子站点是否等父站点、worker 里的 panic 是否传播出去。
// 连库那部分由 siteMigration/siteAccess 的用例覆盖。

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

func fakeSites(n int) []*model.Website {
	sites := make([]*model.Website, 0, n)
	for i := 1; i <= n; i++ {
		sites = append(sites, &model.Website{Id: uint(i), Name: fmt.Sprintf("站点%d", i)})
	}
	return sites
}

// TestInitSitesConcurrentlyUsesWorkers 验并发度：同时最多 workers 个，且确实并发。
func TestInitSitesConcurrentlyUsesWorkers(t *testing.T) {
	sites := fakeSites(8)

	var (
		mu       sync.Mutex
		inFlight int
		peak     int
		seen     = map[uint]int{}
	)
	init := func(s *model.Website) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		seen[s.Id]++
		mu.Unlock()

		time.Sleep(15 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
	}

	initSitesConcurrently(sites, 2, init)

	if peak != 2 {
		t.Fatalf("并发度应为 2（闸门容量），实际峰值 %d", peak)
	}
	for _, s := range sites {
		if seen[s.Id] != 1 {
			t.Fatalf("站点 #%d 被处理了 %d 次，应恰好 1 次", s.Id, seen[s.Id])
		}
	}
}

// TestInitSitesConcurrentlySerialWhenOneWorker 是反向对照：workers=1 时必须
// 一个都不并行 —— 否则"库扛不住就降到 1"这个开关是假的。
func TestInitSitesConcurrentlySerialWhenOneWorker(t *testing.T) {
	sites := fakeSites(5)

	var mu sync.Mutex
	inFlight, peak := 0, 0
	init := func(s *model.Website) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
	}

	initSitesConcurrently(sites, 1, init)
	if peak != 1 {
		t.Fatalf("workers=1 时不能并行，实际峰值 %d", peak)
	}
}

// TestInitSitesSiteOneFirstAndParentBeforeChild 验派发顺序的两条硬约束。
func TestInitSitesSiteOneFirstAndParentBeforeChild(t *testing.T) {
	sites := []*model.Website{
		{Id: 1, Name: "主站"},
		{Id: 2, Name: "普通站"},
		{Id: 3, Name: "多语言主站"},
		{Id: 4, Name: "多语言子站", ParentId: 3},
	}

	var (
		mu        sync.Mutex
		first     uint
		startAt   = map[uint]time.Time{}
		endAt     = map[uint]time.Time{}
		firstOnce bool
	)
	init := func(s *model.Website) {
		mu.Lock()
		if !firstOnce {
			firstOnce = true
			first = s.Id
		}
		startAt[s.Id] = time.Now()
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		endAt[s.Id] = time.Now()
		mu.Unlock()
	}

	initSitesConcurrently(sites, 2, init)

	if first != 1 {
		t.Fatalf("站点 1 必须第一个跑（默认库/后台入口/主站都挂在它上面），实际先跑的是 #%d", first)
	}
	if startAt[4].Before(endAt[3]) {
		t.Fatal("子站点 #4 必须在父站点 #3 初始化完成之后才开始")
	}
}

// TestInitSitesConcurrentlyPropagatesPanic 验：worker 里 panic 不能被吞掉。
// 被吞掉的后果是 InitWebsites 正常返回 → 阶段标成 ready → 崩过的站点也放行了。
func TestInitSitesConcurrentlyPropagatesPanic(t *testing.T) {
	sites := fakeSites(4)
	var calls atomic.Int32
	init := func(s *model.Website) {
		calls.Add(1)
		if s.Id == 2 {
			panic("模拟初始化崩溃")
		}
		time.Sleep(5 * time.Millisecond)
	}

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		initSitesConcurrently(sites, 2, init)
	}()

	if recovered == nil {
		t.Fatal("worker 里的 panic 必须传到调用方，否则阶段会被误标为 ready")
	}
	if msg := fmt.Sprint(recovered); !strings.Contains(msg, "#2") {
		t.Fatalf("panic 信息里要带站点 id，方便定位：%v", recovered)
	}
	// panic 之后其余站点仍会被处理完（不中断已在跑的），但不会无限重试。
	if calls.Load() > int32(len(sites)) {
		t.Fatalf("站点被重复处理：%d 次 > %d 个站点", calls.Load(), len(sites))
	}
}

// TestWebsitesValuesSortedById 锁住注册表输出的确定性。
//
// 并发初始化让"先迁完的先注册"，注册顺序不再等于 id 顺序；而域名匹配是
// "第一个命中就返回"，同域名下主站与二级目录站点会因此随注册顺序漂移。
// Values() 必须按 id 排回来。
func TestWebsitesValuesSortedById(t *testing.T) {
	ow := NewWebsites()
	// 故意倒序注册（模拟并发：后者先完成）
	for _, id := range []uint{7, 3, 11, 1} {
		ow.Set(&Website{Id: id, Name: fmt.Sprintf("站点%d", id)})
	}
	values := ow.Values()
	want := []uint{1, 3, 7, 11}
	if len(values) != len(want) {
		t.Fatalf("站点数量不对：%d，期望 %d", len(values), len(want))
	}
	for i, id := range want {
		if values[i].Id != id {
			got := make([]uint, 0, len(values))
			for _, v := range values {
				got = append(got, v.Id)
			}
			t.Fatalf("Values() 必须按 id 升序，实际 %v，期望 %v", got, want)
		}
	}
}
