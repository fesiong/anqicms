package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

// 本文件是 T12b R2/R3 的护栏：通知侧的两条生命周期与身份边界。
//
//  1. agent.failed 只能从「绑定用户会话」的失败审计派生：管理员试发/后台操作失败的审计
//     没有绑定身份（binding_id=0），不是智能体任务失败，不能进群。
//  2. 站点删除与进程退出必须先停通知工作者再关数据库：工作者每 30 秒扫一次表并 UPDATE
//     投递行，DB 关闭后会持续打空转。

// newNotifyTestBindingAudit 建一条 result=failed 的审计，binding_id 由调用方给定。
func newNotifyTestBindingAudit(t *testing.T, w *Website, action string, bindingId uint) *model.ChannelAudit {
	t.Helper()
	audit := &model.ChannelAudit{
		SiteId:    w.Id,
		Action:    action,
		BindingId: bindingId,
		Result:    "failed",
	}
	if err := w.DB.Create(audit).Error; err != nil {
		t.Fatalf("创建失败审计失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", audit.Id, w.Id).Delete(&model.ChannelAudit{}).Error; err != nil {
			t.Logf("清理失败审计 %d 失败：%v", audit.Id, err)
		}
	})
	return audit
}

// notifyTestAuditEventKey 是某条审计对应的 agent.failed 事件键。
func notifyTestAuditEventKey(audit *model.ChannelAudit) string {
	return fmt.Sprintf("%s:audit:%d", NotifyEventAgentFailed, audit.Id)
}

// TestNotifyDeriveSkipsUnboundAudits 锁住 R2：只有绑定了用户会话的失败才算智能体任务失败。
//
// 断言按「我自己造的两条审计的事件键」精确计数，不依赖站点的全局派生总数：
// 真实开发库里的历史失败审计也会被同一轮扫到，全局计数无法区分是谁派生的。
func TestNotifyDeriveSkipsUnboundAudits(t *testing.T) {
	w := notifyTestSite(t)
	if err := w.DB.AutoMigrate(&model.ChannelAudit{}); err != nil {
		t.Fatalf("迁移渠道审计表失败：%v", err)
	}

	// 管理员试发失败：binding_id=0（writeChannelAudit(..., 0, ...) 的写法）。
	unbound := newNotifyTestBindingAudit(t, w, "channel.test", 0)
	// 智能体工具调用失败：属于某个绑定会话。
	bound := newNotifyTestBindingAudit(t, w, "tool.call", 424242)

	unboundKey := notifyTestAuditEventKey(unbound)
	boundKey := notifyTestAuditEventKey(bound)
	cleanupNotifyBusinessKeys(t, w, unboundKey, boundKey)

	w.deriveAgentFailedNotificationEvents()

	if got := notifyTestCountEventsByKey(t, w, unboundKey); got != 0 {
		t.Fatalf("binding_id=0 的失败审计不得派生 agent.failed 事件（试发失败不是任务失败），实际 %d 条", got)
	}
	if got := notifyTestCountEventsByKey(t, w, boundKey); got != 1 {
		t.Fatalf("binding_id>0 的失败审计必须派生 1 条 agent.failed 事件，实际 %d 条", got)
	}

	// 派生出的事件必须指向绑定会话那条审计：载荷里的 audit_id 是唯一身份。
	event := notifyTestLoadEvent(t, w, NotifyEventAgentFailed, "audit:"+fmt.Sprintf("%d", bound.Id))
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.PayloadJson), &payload); err != nil {
		t.Fatalf("事件载荷不是合法 JSON：%v", err)
	}
	if fmt.Sprintf("%v", payload["audit_id"]) != fmt.Sprintf("%d", bound.Id) {
		t.Fatalf("事件载荷 audit_id 必须指向绑定会话的审计 %d，实际 %v", bound.Id, payload["audit_id"])
	}

	// 反向对照：同一条无绑定审计在过滤被去掉时会派生事件（历史实现即如此）。
	// 这里直接证明「测试抓得住」：按旧口径（不过滤 binding_id）扫描，unbound 必须命中。
	var wouldDerive int64
	if err := w.DB.Model(&model.ChannelAudit{}).
		Where("site_id = ? AND result = ? AND created_time >= ?", w.Id, "failed", time.Now().Unix()-int64(notifyDeriveAuditWindow/time.Second)).
		Where("id = ?", unbound.Id).Count(&wouldDerive).Error; err != nil {
		t.Fatalf("按旧口径统计失败：%v", err)
	}
	if wouldDerive != 1 {
		t.Fatalf("反向对照前提不成立：无绑定审计本身就在派生窗口内（应命中 1 条，实际 %d）", wouldDerive)
	}
}

// TestNotifyWorkerStopsBeforeDatabaseCloseOnShutdown 锁住 R3 的前半段：进程退出路径
// （shutdownWebsite）必须先停通知工作者再关数据库。
func TestNotifyWorkerStopsBeforeDatabaseCloseOnShutdown(t *testing.T) {
	w := channelManagerTestSite(t)

	fakeId := uint(900000) + uint(time.Now().UnixNano()%100000)
	// 独立连接池：shutdownWebsite 会关它，不能借用站点 1 的句柄。
	db, err := InitDB(w.Mysql)
	if err != nil {
		t.Fatalf("打开独立数据库连接失败：%v", err)
	}
	fakeSite := &Website{Id: fakeId, Mysql: w.Mysql, DB: db, Initialed: true}
	t.Cleanup(func() {
		fakeSite.StopChannelManager()
		fakeSite.StopNotifyWorker()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	fakeSite.startNotifyWorker(context.Background(), notifyWorkerTestOptions())
	if worker := notifyWorkerRegistry.bySite[fakeId]; worker == nil {
		t.Fatal("前置条件不成立：通知工作者没有登记到注册表")
	}

	shutdownWebsite(fakeSite)

	if worker := notifyWorkerRegistry.bySite[fakeId]; worker != nil {
		t.Fatal("进程退出路径必须停掉通知工作者")
	}
	if manager := channelManagerRegistry.bySite[fakeId]; manager != nil {
		t.Fatal("进程退出路径必须停掉渠道管理器")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接失败：%v", err)
	}
	if err = sqlDB.Ping(); err == nil {
		t.Fatal("进程退出路径必须关闭数据库连接")
	}

	// 顺序：源码里 StopNotifyWorker 必须在关库之前（关库之后再停等于白停）。
	assertShutdownStopsNotifyBeforeClosingDB(t)
}

// notifyWorkerTestOptions 是生命周期用例用的工作者参数：
// 周期取 50ms 只为让「有没有在跑」可观测，站点没有已批准目标，轮次不会发送任何消息。
func notifyWorkerTestOptions() notifyWorkerOptions {
	opts := notifyTestOptions(newNotifyFakeAdapter(), nil)
	opts.interval = 50 * time.Millisecond
	return opts
}

// shutdownOrderScan 记录函数体顶层语句里三类动作首次出现的下标：
// stopChannel（停渠道管理器）、stopNotify（停通知工作者）、closeDB（关库）。
func shutdownOrderScan(body *ast.BlockStmt) map[string]int {
	order := map[string]int{}
	record := func(key string, index int) {
		if _, exists := order[key]; !exists {
			order[key] = index
		}
	}
	for index, stmt := range body.List {
		ast.Inspect(stmt, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "StopChannelManager":
				record("stopChannel", index)
			case "StopNotifyWorker":
				record("stopNotify", index)
			case "Close":
				record("closeDB", index)
			}
			return true
		})
	}
	return order
}

// shutdownOrderViolation 返回顺序违规的说明；顺序正确返回空串。
func shutdownOrderViolation(order map[string]int) string {
	for _, key := range []string{"stopChannel", "stopNotify", "closeDB"} {
		if _, exists := order[key]; !exists {
			return "函数体里缺少 " + key + " 动作（停机序列被改写，护栏需要同步）"
		}
	}
	if order["stopChannel"] >= order["stopNotify"] {
		return "停通知工作者必须排在停渠道管理器之后：StopChannelManager 先停长连接，StopNotifyWorker 再停投递"
	}
	if order["stopNotify"] >= order["closeDB"] {
		return "停通知工作者必须排在关库之前：关库后再停等于白停，工作者会先打一轮空转"
	}
	return ""
}

// assertShutdownStopsNotifyBeforeClosingDB 用 go/ast 断言 shutdownWebsite 里
// 「停渠道 → 停通知 → 关库」的先后。行为断言（注册表已空、连接已关）只能证明都做了，
// 证明不了顺序，所以顺序单独用源码断言锁住。
func assertShutdownStopsNotifyBeforeClosingDB(t *testing.T) {
	t.Helper()
	_, file := parseGoFileForGuard(t, "anqi.go")
	body := findFuncBodyForGuard(t, file, "", "shutdownWebsite")
	if violation := shutdownOrderViolation(shutdownOrderScan(body)); violation != "" {
		t.Fatalf("shutdownWebsite 停机顺序不对：%s", violation)
	}

	// 反向对照：同一个扫描器/判定器对「关库后才停通知」的写法必须报违规。
	const regression = `package provider

func shutdownWebsite(w *Website) {
	w.StopChannelManager()
	if w.DB != nil {
		db, err := w.DB.DB()
		if err == nil {
			_ = db.Close()
		}
	}
	w.StopNotifyWorker()
}`
	_, regressionFile := parseGoSourceForGuard(t, regression)
	regressionBody := findFuncBodyForGuard(t, regressionFile, "", "shutdownWebsite")
	if violation := shutdownOrderViolation(shutdownOrderScan(regressionBody)); violation == "" {
		t.Fatal("扫描器抓不住「先关库再停通知工作者」的写法")
	}
}
