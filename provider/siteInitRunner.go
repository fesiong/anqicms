package provider

// 启动期站点初始化的并发调度。
//
// 为什么不再用一条 for 循环串行跑完（2026-11-17）：
// 迁移改成同步之后，单站点几秒到几分钟，106 个站点串行就是几十分钟到几小时。
// 而迁移闸门（migrateSem，默认 2）本来就是为「事故现场 75 个并发把 DDL 打满、
// 2 是能用上多核又不把 InnoDB IO 队列排满的折中」设的 —— 串行循环等于把这个
// 并发度在启动路径上作废。现在站点初始化按同一容量并行：同时 2 个站点在迁移，
// 迁完的那个立刻对外服务（逐站点放行见 provider/siteAccess.go）。
//
// 三条约束都不能退：
//  1. 站点 1 先单独跑完：它是默认库、后台入口，也是父子/多语言站点的主站，
//     没注册之前别的站点读主站会拿到 nil（GetMainWebsite / GetMultiLangSites）；
//  2. 子站点必须等父站点初始化完成 —— 父子站点共享域名与模板配置；
//  3. worker 里的 panic 不能把进程直接打崩、也不能静默吞掉：
//     带站 id 与栈记一条日志，然后重新 panic 交给 InitWebsitesAsync 标 failed
//     （否则 InitWebsites 正常返回 → 阶段被标成 ready，崩过的站点也放行了）。

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"kandaoni.com/anqicms/model"
)

// initSitesConcurrently 按 workers 并发初始化站点；sites 需按 id 升序。
//
// init 抽成参数只为单测能在不连库的情况下验证调度（并发度、父子顺序、panic 传播）。
func initSitesConcurrently(sites []*model.Website, workers int, init func(*model.Website)) {
	if len(sites) == 0 {
		return
	}
	if workers < 1 {
		workers = 1
	}
	// 站点 1 先串行跑完，理由见文件头第 1 条。
	if sites[0].Id == 1 {
		init(sites[0])
		sites = sites[1:]
	}
	if workers == 1 || len(sites) == 0 {
		for _, s := range sites {
			init(s)
		}
		return
	}

	sem := make(chan struct{}, workers)
	done := make(map[uint]chan struct{}, len(sites))
	for _, s := range sites {
		done[s.Id] = make(chan struct{})
	}
	dispatched := make(map[uint]bool, len(sites))

	var (
		wg       sync.WaitGroup
		panicMu  sync.Mutex
		panicVal any
	)
	for _, s := range sites {
		// 父站点已经在跑（id 更小）：等它彻底完成再派发子站点。
		// 父站点 id 更大（脏数据）时不等待，否则会自锁。
		if pid := s.ParentId; pid > 0 && dispatched[pid] {
			<-done[pid]
		}
		sem <- struct{}{}
		dispatched[s.Id] = true
		wg.Add(1)
		go func(site *model.Website) {
			defer wg.Done()
			defer func() { <-sem }()
			// 先 close 再让 wg.Done：等父站点的子站点不依赖 wg，靠这个 channel 放行。
			defer close(done[site.Id])
			defer func() {
				if r := recover(); r != nil {
					// 栈必须在这里打：抛到外层后 recover 拿到的是主 goroutine 的栈，
					// 看不出是哪个站点、哪一行崩的。
					slog.Error("站点初始化 panic：该站点将以未初始化状态运行，服务保持未就绪",
						"siteId", site.Id, "name", site.Name, "panic", r, "stack", string(debug.Stack()))
					panicMu.Lock()
					if panicVal == nil {
						panicVal = fmt.Errorf("站点 #%d「%s」初始化 panic：%v", site.Id, site.Name, r)
					}
					panicMu.Unlock()
				}
			}()
			init(site)
		}(s)
	}
	wg.Wait()
	if panicVal != nil {
		panic(panicVal)
	}
}
