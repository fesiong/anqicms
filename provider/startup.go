package provider

// 启动阶段状态机：把「端口开始监听」和「站点初始化完成」解耦。
//
// 为什么需要它（2026-11-16 客户 106 站点事故的后续）：
// 站点初始化现在**同步**跑库表迁移（见 provider/siteMigration.go），
// 单站点几秒到几分钟，106 个站点就是几十分钟到几小时。这段过去排在
// `go bootstrap.Start()` 之前，于是进程活着、日志不动、端口一直不开。
// 对运维来说这和"进程卡死"完全无法区分 —— 客户现场就是这样反复重启的，
// 而每次重启又要从被中断的那个站点重来。
//
// 现在的约定：
//  1. 端口先监听，站点初始化在后台按闸门限流执行；
//  2. 初始化没完成之前，**未就绪的站点**返回 503 + Retry-After；
//     已经迁移完并注册的站点立即正常服务（粒度是站点，不是进程，
//     判定见 provider/siteAccess.go）—— 106 个站点串行迁移时，
//     不能让前几个已完成的站点陪跑到最后；
//  3. 进度有两个出口：GET /__startup（JSON，同时当就绪探针）和
//     每 15 秒一条的心跳日志；
//  4. 初始化 panic 不能静默：标 failed、保持未就绪、把栈打进日志。

import (
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// StartupStatusPath 是启动阶段唯一无条件放行的端点。
	// 它必须排在通配路由之前、且由闸门直接接管：站点还没注册完时，
	// CurrentSite 拿到的是一张空表，任何业务 handler 都可能 nil 解引用。
	StartupStatusPath = "/__startup"

	// startupHeartbeatInterval 是「启动期间日志静默多久算可疑」的口径。
	// 单个站点的商品属性回填在万行库上要跑几分钟，期间一条输出都没有；
	// 15 秒一条心跳才能让 tail -f 的人区分"在跑"和"卡死"。
	startupHeartbeatInterval = 15 * time.Second
)

// StartupPhase 是启动阶段。零值 Idle 表示"还没开始"。
type StartupPhase int32

const (
	// StartupPhaseIdle：没有默认库（全新安装）、测试、命令行工具。
	// 闸门对 Idle **放行**，否则 /install 和所有直接打 handler 的测试都会 503。
	StartupPhaseIdle StartupPhase = iota
	// StartupPhaseRunning：站点初始化/库表迁移进行中，闸门关闭。
	StartupPhaseRunning
	// StartupPhaseReady：全部站点处理完毕（含失败项），闸门打开。
	StartupPhaseReady
	// StartupPhaseFailed：初始化 panic。保持未就绪直到重启 ——
	// 中途 panic 的注册表里可能是"部分站点就绪、部分站点半初始化"，
	// 放开请求等于把半迁移的表结构暴露给线上流量。
	StartupPhaseFailed
)

func (p StartupPhase) String() string {
	switch p {
	case StartupPhaseRunning:
		return "running"
	case StartupPhaseReady:
		return "ready"
	case StartupPhaseFailed:
		return "failed"
	default:
		return "idle"
	}
}

// StartupSnapshot 是启动进度快照，直接作为 /__startup 的响应体。
type StartupSnapshot struct {
	Phase     string `json:"phase"`
	Ready     bool   `json:"ready"`
	Total     int64  `json:"total"`
	Migrated  int64  `json:"migrated"`
	Skipped   int64  `json:"skipped"`
	Failed    int64  `json:"failed"`
	Processed int64  `json:"processed"`
	Current   string `json:"current"`
	Elapsed   string `json:"elapsed"`
	Message   string `json:"message"`
}

var (
	startupPhaseValue atomic.Int32
	startupStartedAt  atomic.Int64 // UnixNano，0 表示未开始
	startupFinishedAt atomic.Int64 // UnixNano，0 表示进行中
	// startupCurrent 存 string（"#12「xxx」"）。atomic.Value 要求同一具体类型，
	// 所以只存字符串，不存结构体。
	// 现在同时可能有多个站点在初始化（默认 2 个），所以底层维护的是一个
	// 「站点 id → 展示名」的集合，发布时按 id 升序拼成 "#3「a」、#4「b」"。
	startupCurrent   atomic.Value
	startupCurrentMu sync.Mutex
	startupCurrents  = map[uint]string{}

	startupHeartbeatMu     sync.Mutex
	startupHeartbeatCancel chan struct{}
	startupHeartbeatDone   chan struct{}
)

func init() {
	startupCurrent.Store("")
}

// StartPhase 返回当前阶段。
func StartPhase() StartupPhase {
	return StartupPhase(startupPhaseValue.Load())
}

// StartupReady 表示可以对外提供业务请求。
// Idle 也算"可以放行"：没有默认库时站点由安装流程创建，闸门不能挡。
func StartupReady() bool {
	switch StartPhase() {
	case StartupPhaseRunning, StartupPhaseFailed:
		return false
	default:
		return true
	}
}

// BeginStartup 把阶段从 Idle 推到 Running，并启动心跳日志。幂等。
func BeginStartup() {
	if !startupPhaseValue.CompareAndSwap(int32(StartupPhaseIdle), int32(StartupPhaseRunning)) {
		return
	}
	startupStartedAt.Store(time.Now().UnixNano())
	startupFinishedAt.Store(0)
	log.Printf("站点初始化开始：逐个站点迁移/加载中，完成前对外返回 503（进度见 GET %s 或本日志每 %s 一条心跳）",
		StartupStatusPath, startupHeartbeatInterval)
	startStartupHeartbeat()
}

// PrepareStartupGate 在有默认库时先把闸门关上（幂等）。
//
// 必须在端口监听之前调用：端口打开到初始化开始之间的几百微秒里，
// 注册表还是空的，请求会拿到"全新安装"语义的一堆 404。
func PrepareStartupGate() {
	if GetDefaultDB() == nil {
		return
	}
	BeginStartup()
}

// InitWebsitesAsync 在后台完成站点初始化（含库表迁移）。
//
// 没有默认库（全新安装）时直接返回：阶段停在 Idle，闸门放行，
// 安装流程（controller/install.go）自己会调 InitWebsite。
func InitWebsitesAsync() {
	if GetDefaultDB() == nil {
		return
	}
	BeginStartup()
	go func() {
		failed := false
		defer func() {
			if r := recover(); r != nil {
				failed = true
				slog.Error("站点初始化 panic：服务保持未就绪，修复后需重启（重启会按站点重试迁移）",
					"panic", r, "stack", string(debug.Stack()))
			}
			FinishStartup(failed)
		}()
		InitWebsites()
	}()
}

// FinishStartup 落定阶段并停掉心跳。failed 为真时保持未就绪。
func FinishStartup(failed bool) {
	startupFinishedAt.Store(time.Now().UnixNano())
	if failed {
		startupPhaseValue.Store(int32(StartupPhaseFailed))
	} else {
		startupPhaseValue.Store(int32(StartupPhaseReady))
	}
	stopStartupHeartbeat()

	snap := StartupSnapshotNow()
	if failed {
		slog.Error("站点初始化未完成，服务保持 503",
			"processed", snap.Processed, "total", snap.Total, "elapsed", snap.Elapsed)
		return
	}
	// 不在这里重复 MigrateRoundSummary：InitWebsites 收尾已经打过一条
	// 「站点初始化完成：已迁移 x 个…」，两条一样的日志会让人以为是两轮。
	log.Printf("服务已就绪：站点初始化总耗时 %s", snap.Elapsed)
}

// SetStartupCurrent 记录"当前正在处理哪个站点"，供心跳日志与 /__startup 显示。
//
// 可以同时登记多个站点（启动期默认 2 个并发），结束时必须调 ClearStartupCurrent ——
// 漏调会让进度一直显示一个早就处理完的站点。
func SetStartupCurrent(siteID uint, siteName string) {
	startupCurrentMu.Lock()
	startupCurrents[siteID] = fmt.Sprintf("#%d「%s」", siteID, siteName)
	publishStartupCurrentLocked()
	startupCurrentMu.Unlock()
}

// ClearStartupCurrent 把某个站点从"当前处理中"里摘掉。
func ClearStartupCurrent(siteID uint) {
	startupCurrentMu.Lock()
	delete(startupCurrents, siteID)
	publishStartupCurrentLocked()
	startupCurrentMu.Unlock()
}

// publishStartupCurrentLocked 按 id 升序拼出展示串。调用方必须持锁。
// 排序是必须的：map 迭代顺序随机，心跳日志每次换个顺序会让人以为站点在跳。
func publishStartupCurrentLocked() {
	if len(startupCurrents) == 0 {
		startupCurrent.Store("")
		return
	}
	ids := make([]int, 0, len(startupCurrents))
	for id := range startupCurrents {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, startupCurrents[uint(id)])
	}
	startupCurrent.Store(strings.Join(parts, "、"))
}

// StartupSnapshotNow 返回当前进度快照。
func StartupSnapshotNow() StartupSnapshot {
	phase := StartPhase()
	started := startupStartedAt.Load()
	finished := startupFinishedAt.Load()
	var elapsed time.Duration
	if started > 0 {
		end := time.Now().UnixNano()
		if finished > 0 {
			end = finished
		}
		elapsed = time.Duration(end - started)
	}
	migrated := atomic.LoadInt64(&migrateRoundDone)
	skipped := atomic.LoadInt64(&migrateRoundSkipped)
	failed := atomic.LoadInt64(&migrateRoundFailed)
	total := atomic.LoadInt64(&migrateRoundTotal)
	processed := migrated + skipped + failed

	current, _ := startupCurrent.Load().(string)
	if phase != StartupPhaseRunning {
		current = ""
	}

	snap := StartupSnapshot{
		Phase:     phase.String(),
		Ready:     StartupReady(),
		Total:     total,
		Migrated:  migrated,
		Skipped:   skipped,
		Failed:    failed,
		Processed: processed,
		Current:   current,
		Elapsed:   elapsed.Round(time.Second).String(),
	}
	switch phase {
	case StartupPhaseRunning:
		snap.Message = fmt.Sprintf("正在逐个升级站点（已处理 %d/%d），已完成升级的站点可正常访问", processed, total)
	case StartupPhaseFailed:
		snap.Message = "站点初始化失败，服务未就绪；请查看进程输出 / cache/error.log 后重启"
	case StartupPhaseReady:
		snap.Message = "服务已就绪"
	default:
		snap.Message = "尚未开始站点初始化"
	}
	return snap
}

func startStartupHeartbeat() {
	startupHeartbeatMu.Lock()
	defer startupHeartbeatMu.Unlock()
	if startupHeartbeatCancel != nil {
		return
	}
	cancel := make(chan struct{})
	done := make(chan struct{})
	startupHeartbeatCancel = cancel
	startupHeartbeatDone = done
	go startupHeartbeatLoop(cancel, done)
}

func stopStartupHeartbeat() {
	startupHeartbeatMu.Lock()
	cancel, done := startupHeartbeatCancel, startupHeartbeatDone
	startupHeartbeatCancel, startupHeartbeatDone = nil, nil
	startupHeartbeatMu.Unlock()
	if cancel == nil {
		return
	}
	close(cancel)
	// 等心跳 goroutine 真的退出：它可能正在写日志，让它和 FinishStartup 的
	// 收尾日志交错会让人误读成"完成后还在跑"。
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func startupHeartbeatLoop(cancel <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(startupHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-cancel:
			return
		case <-ticker.C:
			snap := StartupSnapshotNow()
			if snap.Ready || snap.Phase == StartupPhaseFailed.String() {
				return
			}
			current := snap.Current
			if current == "" {
				current = "（准备中）"
			}
			log.Printf("站点初始化进行中：已处理 %d/%d（迁移 %d、跳过 %d、失败 %d），当前 %s，已耗时 %s",
				snap.Processed, snap.Total, snap.Migrated, snap.Skipped, snap.Failed, current, snap.Elapsed)
		}
	}
}
