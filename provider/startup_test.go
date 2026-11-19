package provider

// 启动阶段的护栏：端口必须先开、站点初始化必须在后台跑。
//
// 事故背景（2026-11-16，客户 106 站点）：站点初始化的库表迁移被改成同步后，
// 它卡在 `go bootstrap.Start()` 之前，于是升级期间进程活着但端口几十分钟不开，
// 运维无法区分"在跑"和"卡死"。下面这些用例锁的就是"别再改回去"。

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// resetStartupStateForTest 把包级启动状态复位，避免用例之间互相污染。
func resetStartupStateForTest() {
	stopStartupHeartbeat()
	startupPhaseValue.Store(int32(StartupPhaseIdle))
	startupStartedAt.Store(0)
	startupFinishedAt.Store(0)
	startupCurrentMu.Lock()
	startupCurrents = map[uint]string{}
	startupCurrentMu.Unlock()
	startupCurrent.Store("")
	BeginSiteMigrationRound(0)
}

func TestStartupPhaseLifecycle(t *testing.T) {
	resetStartupStateForTest()
	defer resetStartupStateForTest()

	// 未开始初始化：放行（全新安装要走 /install，测试要能直接打 handler）
	if !StartupReady() {
		t.Fatal("Idle 阶段必须放行，否则全新安装的 /install 会被挡成 503")
	}

	BeginStartup()
	if got := StartPhase(); got != StartupPhaseRunning {
		t.Fatalf("BeginStartup 后阶段应为 running，实际 %s", got)
	}
	if StartupReady() {
		t.Fatal("running 阶段不能放行：注册表里可能只有部分站点")
	}

	// 重复调用不能让阶段回退或重启心跳
	BeginStartup()
	if got := StartPhase(); got != StartupPhaseRunning {
		t.Fatalf("BeginStartup 必须幂等，实际阶段变成了 %s", got)
	}

	FinishStartup(false)
	if got := StartPhase(); got != StartupPhaseReady {
		t.Fatalf("FinishStartup(false) 后应为 ready，实际 %s", got)
	}
	if !StartupReady() {
		t.Fatal("ready 阶段必须放行")
	}

	// 反向对照：panic 分支（failed）必须保持关闭
	resetStartupStateForTest()
	BeginStartup()
	FinishStartup(true)
	if got := StartPhase(); got != StartupPhaseFailed {
		t.Fatalf("FinishStartup(true) 后应为 failed，实际 %s", got)
	}
	if StartupReady() {
		t.Fatal("failed 阶段必须保持未就绪：注册表可能是半初始化的，放开等于把半迁移结构暴露给线上流量")
	}
}

func TestStartupSnapshotReportsProgress(t *testing.T) {
	resetStartupStateForTest()
	defer resetStartupStateForTest()

	BeginSiteMigrationRound(106)
	// 直接操纵计数器：这里验的是快照的换算与文案，不重复验证迁移本身。
	migrateRoundDone = 12
	migrateRoundSkipped = 70
	migrateRoundFailed = 1
	SetStartupCurrent(13, "客户站点十三")
	BeginStartup()

	snap := StartupSnapshotNow()
	if snap.Phase != StartupPhaseRunning.String() || snap.Ready {
		t.Fatalf("快照阶段/就绪标记不对：%+v", snap)
	}
	if snap.Total != 106 || snap.Processed != 83 {
		t.Fatalf("进度换算不对：total=%d processed=%d（应为 106 / 83）", snap.Total, snap.Processed)
	}
	if !strings.Contains(snap.Current, "#13") || !strings.Contains(snap.Current, "客户站点十三") {
		t.Fatalf("快照没有带上当前站点：%q", snap.Current)
	}
	if !strings.Contains(snap.Message, "83/106") {
		t.Fatalf("快照文案应带进度：%q", snap.Message)
	}

	FinishStartup(false)
	snap = StartupSnapshotNow()
	if !snap.Ready || snap.Current != "" {
		t.Fatalf("就绪后应 ready 且清空当前站点：%+v", snap)
	}
	if snap.Elapsed == "" {
		t.Fatal("就绪后必须带耗时")
	}
}

// serveListenScan 是 bootstrap.Serve 里"端口先开还是先初始化"的扫描结果。
type serveListenScan struct {
	listenPos    token.Pos // go bootstrap.Listen()
	asyncInitPos token.Pos // provider.InitWebsitesAsync()
	syncInitPos  token.Pos // provider.InitWebsites()（同步，不允许出现在 Serve 里）
}

func scanServeListenOrder(body *ast.BlockStmt) serveListenScan {
	var scan serveListenScan
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			if scan.listenPos == token.NoPos && isQualifiedCall(node.Call, "bootstrap", "Listen") {
				scan.listenPos = node.Pos()
			}
		case *ast.ExprStmt:
			if scan.asyncInitPos == token.NoPos && isQualifiedCall(node.X, "provider", "InitWebsitesAsync") {
				scan.asyncInitPos = node.Pos()
			}
			if scan.syncInitPos == token.NoPos && isQualifiedCall(node.X, "provider", "InitWebsites") {
				scan.syncInitPos = node.Pos()
			}
		}
		return true
	})
	return scan
}

func checkServeListenOrder(scan serveListenScan) string {
	if scan.syncInitPos != token.NoPos {
		return "bootstrap.Serve 里直接调用了同步的 provider.InitWebsites()：106 个站点做库表迁移时端口会几十分钟不开"
	}
	if scan.listenPos == token.NoPos {
		return "bootstrap.Serve 里没有 go bootstrap.Listen()：端口不会先开"
	}
	if scan.asyncInitPos == token.NoPos {
		return "bootstrap.Serve 里没有 provider.InitWebsitesAsync()：站点永远不会初始化"
	}
	if scan.listenPos > scan.asyncInitPos {
		return "provider.InitWebsitesAsync() 排在 go bootstrap.Listen() 之前：初始化会抢在监听前跑完"
	}
	return ""
}

func TestServeListensBeforeSiteInit(t *testing.T) {
	_, bootstrapFile := parseGoFileForGuard(t, filepath.Join("..", "bootstrap.go"))
	body := findFuncBodyForGuard(t, bootstrapFile, "Bootstrap", "Serve")
	if msg := checkServeListenOrder(scanServeListenOrder(body)); msg != "" {
		t.Fatal(msg)
	}

	// 反向对照一：修复前的写法（同步初始化 → 才 go Start）必须被判为不合格。
	const syncFirst = `package anqicms

func (bootstrap *Bootstrap) Serve() {
	if provider.GetDefaultDB() != nil {
		provider.InitWebsites()
	}
	go bootstrap.Start()
}`
	_, f1 := parseGoSourceForGuard(t, syncFirst)
	if msg := checkServeListenOrder(scanServeListenOrder(findFuncBodyForGuard(t, f1, "Bootstrap", "Serve"))); msg == "" {
		t.Fatal("扫描器没认出同步 InitWebsites：这条护栏就没意义了")
	}

	// 反向对照二：两个都异步，但顺序反了（先初始化再监听）同样不合格。
	const wrongOrder = `package anqicms

func (bootstrap *Bootstrap) Serve() {
	provider.InitWebsitesAsync()
	go bootstrap.Listen()
}`
	_, f2 := parseGoSourceForGuard(t, wrongOrder)
	if msg := checkServeListenOrder(scanServeListenOrder(findFuncBodyForGuard(t, f2, "Bootstrap", "Serve"))); msg == "" {
		t.Fatal("扫描器没认出「先初始化后监听」的顺序错误")
	}
}

func TestMySQLDSNHasDialTimeout(t *testing.T) {
	cfg := config.MysqlConfig{User: "root", Password: "root", Host: "127.0.0.1", Port: 3306, Database: "anqicms"}
	dsn := mysqlDSN(&cfg, "anqicms")
	for _, want := range []string{"timeout=10s", "/anqicms?", "charset=utf8mb4"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("DSN 缺少 %q：%s", want, dsn)
		}
	}
	// 只连服务不选库（建库前的探测连接）也必须带拨号超时
	if !strings.Contains(mysqlDSN(&cfg, ""), "timeout=10s") {
		t.Fatal("无库名的 DSN 也要有拨号超时，否则建库探测同样会挂 2 分钟")
	}
	// 反向对照：读超时不能有 —— archives 回填在万行库上本来就要跑几分钟。
	for _, bad := range []string{"readTimeout", "writeTimeout"} {
		if strings.Contains(dsn, bad) {
			t.Fatalf("DSN 不应带 %s：会打断正常的长时间回填", bad)
		}
	}
}

// TestStartupCurrentShowsAllInFlightSites 锁住并发初始化下的进度展示：
// 默认 2 个站点同时在跑，只显示其中一个会让人以为另一个卡住了。
func TestStartupCurrentShowsAllInFlightSites(t *testing.T) {
	resetStartupStateForTest()
	defer resetStartupStateForTest()
	BeginStartup()

	// 故意乱序登记：展示必须按 id 升序，否则每次心跳顺序都在跳。
	SetStartupCurrent(4, "四号站")
	SetStartupCurrent(3, "三号站")

	snap := StartupSnapshotNow()
	if !strings.Contains(snap.Current, "#3「三号站」") || !strings.Contains(snap.Current, "#4「四号站」") {
		t.Fatalf("并发初始化时要把在跑的站点都显示出来：%q", snap.Current)
	}
	if strings.Index(snap.Current, "#3") > strings.Index(snap.Current, "#4") {
		t.Fatalf("展示顺序必须按 id 升序（map 迭代随机）：%q", snap.Current)
	}

	// 处理完必须摘掉，否则进度会一直挂着早就结束的站点。
	ClearStartupCurrent(3)
	snap = StartupSnapshotNow()
	if strings.Contains(snap.Current, "#3") {
		t.Fatalf("ClearStartupCurrent 之后不该再出现 #3：%q", snap.Current)
	}
	if !strings.Contains(snap.Current, "#4") {
		t.Fatalf("#4 还在跑，不能被一起清掉：%q", snap.Current)
	}
}
