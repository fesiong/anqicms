package provider

import (
	"context"
	"sync"
	"testing"
)

// TestSendMailResultConcurrentAccess 群发进度会被 HTTP 查询 goroutine 读、
// 发送 goroutine 写，跑 -race 时若锁写漏会直接报 data race。
func TestSendMailResultConcurrentAccess(t *testing.T) {
	r := &SendMailResult{JobID: "t1", Type: "email", Total: 100, Running: true}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// 必须走 inc()——测试自己若用 r.Sent++ 就复现了「无锁写」这个 bug，
		// 而那正是 -race 要守住的东西。
		for i := 0; i < 200; i++ {
			r.inc(1, 0)
			r.AddError("a@b.c", "boom")
		}
		r.finish()
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = r.snapshotPtr()
		}
	}()
	wg.Wait()

	s := r.snapshotPtr()
	if s.Running {
		t.Error("finish 后 Running 应为 false")
	}
	if s.Sent != 200 {
		t.Errorf("Sent = %d，期望 200", s.Sent)
	}
	if s.Processed != 200 {
		t.Errorf("Processed = %d，期望 200", s.Processed)
	}
}

// TestSendMailResultErrorsCapped 失败明细必须有上限，否则几千个坏邮箱
// 会把内存和 JSON 响应撑爆。
func TestSendMailResultErrorsCapped(t *testing.T) {
	r := &SendMailResult{JobID: "t2"}
	for i := 0; i < 500; i++ {
		r.AddError("x@y.z", "fail")
	}
	s := r.snapshotPtr()
	if len(s.Errors) > 50 {
		t.Errorf("Errors 条数 = %d，应被截断到 50", len(s.Errors))
	}
	if len(s.Errors) == 0 {
		t.Error("Errors 不应为空")
	}
}

// TestSendMailJobsEvictOldest 超过保留上限时淘汰最旧的，
// 避免长期运行后 sendMailJobs 无限增长。
func TestSendMailJobsEvictOldest(t *testing.T) {
	sendMailJobs.mu.Lock()
	sendMailJobs.jobs = map[string]*SendMailResult{}
	sendMailJobs.seq = 0
	sendMailJobs.mu.Unlock()

	var ids []string
	for i := 0; i < sendMailJobKeep+5; i++ {
		r := newSendMailResult("email", 1)
		ids = append(ids, r.JobID)
	}
	sendMailJobs.mu.RLock()
	n := len(sendMailJobs.jobs)
	sendMailJobs.mu.RUnlock()

	if n > sendMailJobKeep {
		t.Errorf("任务数 = %d，超过保留上限 %d", n, sendMailJobKeep)
	}
	// 最旧的应已被淘汰
	if _, ok := getSendMailResultForTest(ids[0]); ok {
		t.Error("最旧的任务应已被淘汰")
	}
	// 最新的应在
	if _, ok := getSendMailResultForTest(ids[len(ids)-1]); !ok {
		t.Error("最新的任务不应被淘汰")
	}
}

func getSendMailResultForTest(id string) (*SendMailResult, bool) {
	sendMailJobs.mu.RLock()
	defer sendMailJobs.mu.RUnlock()
	r, ok := sendMailJobs.jobs[id]
	if !ok {
		return nil, false
	}
	return r, true
}

// TestGetSendMailResultSnapshotIsolation 外部拿到的是副本，
// 改动它不能影响内部进度（否则查询接口会污染真实状态）。
func TestGetSendMailResultSnapshotIsolation(t *testing.T) {
	sendMailJobs.mu.Lock()
	sendMailJobs.jobs = map[string]*SendMailResult{}
	sendMailJobs.seq = 100
	sendMailJobs.mu.Unlock()

	inner := newSendMailResult("email", 5)
	got, ok := GetSendMailResult(inner.JobID)
	if !ok {
		t.Fatal("应能查到刚建的任务")
	}
	got.Sent = 999
	got.Errors = append(got.Errors, "伪造")

	again, _ := GetSendMailResult(inner.JobID)
	if again.Sent == 999 {
		t.Error("外部改动泄漏到了内部进度")
	}
	if len(again.Errors) != 0 {
		t.Errorf("外部追加的 errors 泄漏到了内部：%v", again.Errors)
	}
}

var _ = context.Background // 保留 context 引用以兼容后续测试扩展
