package provider

import (
	"log"
	"os"
	"testing"
	"time"

	"kandaoni.com/anqicms/request"
)

// TestSyncMultiLangSiteContent 是**手工联调**用例：在测试库里真的跑一次
// 「主站 → 子站」全量同步，每秒打印一次进度。
//
// ⚠️ 它原来是无条件 `for {}` 死循环，也没有任何断言——等于只要跑到这里，
// `go test ./provider/` 就必然被 10 分钟超时打断，整包其余测试的结果一起被吞掉
// （包括这次指针化整改的回归信号）。现在改成两道闸：
//  1. 默认跳过，设 ANQI_MULTILANG_SYNC_TEST=1 才执行；
//  2. 执行时也有硬超时，不会再把测试进程吊死。
func TestSyncMultiLangSiteContent(t *testing.T) {
	if os.Getenv("ANQI_MULTILANG_SYNC_TEST") == "" {
		t.Skip("手工联调用例：设置 ANQI_MULTILANG_SYNC_TEST=1 才执行")
	}
	InitWebsites()
	w := GetWebsite(1)
	if w == nil {
		t.Skip("站点未初始化，跳过")
	}

	status, err := w.NewMultiLangSync()
	if err != nil {
		t.Fatal(err)
	}
	go status.SyncMultiLangSiteContent(&request.PluginMultiLangSiteSyncRequest{Id: 3, ParentId: 1, Focus: false})

	// 只观察，不判定业务结果（子站配置因环境而异）；超时也算跑完，
	// 目的是「别把测试进程挂死」而不是「必须同步成功」。
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		log.Printf("多语言同步进度：%d/%d（%d%%）%s", status.FinishCount, status.TotalCount, status.Percent, status.Message)
		if status.Finished {
			return
		}
	}
	t.Logf("同步未在 60 秒内结束，放弃等待（最后状态：%d/%d）", status.FinishCount, status.TotalCount)
}
