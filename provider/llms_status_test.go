package provider

import (
	"errors"
	"testing"
	"time"

	"kandaoni.com/anqicms/config"
)

var errTestLLMsFailed = errors.New("disk full")

// TestLLMsBuildStatusSharedAcrossSiteCopies 锁住「构建进度永远 undefined」的根因。
//
// 背景（2026-11-19 实测）：CurrentSite/CurrentSubSite 返回的是请求副本
// （cloneWithContext 浅拷贝）。构建状态原先挂在 Website 字段上，副本里
// `w.llmsBuildStatus = &LLMsBuildStatus{}` 只改了副本自身，主实例仍然是 nil，
// 于是 GET /plugin/llms/status 永远走到「无进行中构建」分支，前端把
// {finished, building} 当进度对象渲染成 "undefined / undefined (undefined%)"，
// 并且三个停止条件都不成立 → 每秒轮询不止。
//
// 反向对照：把状态改回 Website 字段（在副本上分配）时，
// 本测试第 1 步的 site.GetLLMsBuildStatus() 会是 nil，测试必红。
func TestLLMsBuildStatusSharedAcrossSiteCopies(t *testing.T) {
	site := &Website{Id: 987654}
	websites.Set(site)
	t.Cleanup(func() {
		websites.Delete(site.Id)
		llmsBuildStatuses.Delete(site.Id)
	})

	// 请求副本：状态必须在副本上创建也能被主实例读到
	clone := cloneWithContext(site, nil)

	// 1. 副本上开始构建，主实例必须看得到
	status := clone.llmsBuildState()
	if status == nil {
		t.Fatal("llmsBuildState 不应返回 nil")
	}
	if !status.begin() {
		t.Fatal("首次 begin 应返回 true")
	}
	got := site.GetLLMsBuildStatus()
	if got == nil {
		t.Fatal("主实例读不到副本创建的构建状态（进度会渲染成 undefined）")
	}
	if got.Status != 1 {
		t.Errorf("构建中 status 应为 1，实际=%d", got.Status)
	}

	// 2. 重复 begin 必须被拒，否则并发构建会互相重置进度
	if status.begin() {
		t.Error("构建中再次 begin 应返回 false")
	}

	// 3. 进度推进与完成对主实例、副本都可见
	status.setTotal(4)
	status.step()
	status.step()
	if got = clone.GetLLMsBuildStatus(); got.Percent != 50 {
		t.Errorf("2/4 应为 50%%，实际=%d", got.Percent)
	}
	status.finish()
	if got = site.GetLLMsBuildStatus(); got.Status != 2 || got.Percent != 100 {
		t.Errorf("完成后应 status=2 percent=100，实际=%+v", got)
	}

	// 4. 失败要落成 status=-1 + 原因，异步构建才不会永远卡在"进行中"
	status.begin()
	status.fail(errTestLLMsFailed)
	got = site.GetLLMsBuildStatus()
	if got.Status != -1 || got.Msg != errTestLLMsFailed.Error() {
		t.Errorf("失败后应 status=-1 且带原因，实际=%+v", got)
	}

	// 5. GetLLMsBuildStatus 必须返回快照副本，外部改不动内部状态
	got.Percent = 7
	if site.GetLLMsBuildStatus().Percent == 7 {
		t.Error("GetLLMsBuildStatus 应返回快照副本，不能被调用方改到内部状态")
	}

	// 6. 未知站点仍返回 nil（"本次进程还没有构建记录"）
	if (&Website{Id: 987655}).GetLLMsBuildStatus() != nil {
		t.Error("没有构建记录的站点应返回 nil")
	}
}

// TestStartLLMsBuildIsAsyncAndSurvivesPanic 锁住手动构建的两条契约：
//
//  1. POST /plugin/llms/build 必须立刻返回（同步占位 status=1），
//     否则前端那条 1s 轮询在响应回来时构建早已结束，进度永远看不到；
//  2. 后台 goroutine 是裸 go，没有 iris 的 per-request recover，
//     构建里任何 panic 都必须落成 status=-1，而不是打挂进程或永远停在"进行中"。
func TestStartLLMsBuildIsAsyncAndSurvivesPanic(t *testing.T) {
	site := &Website{Id: 987656, PublicPath: t.TempDir() + "/"}
	site.PluginLLMs = &config.PluginLLMsConfig{Open: true}
	websites.Set(site)
	t.Cleanup(func() {
		websites.Delete(site.Id)
		llmsBuildStatuses.Delete(site.Id)
	})

	if !site.StartLLMsBuild() {
		t.Fatal("首次启动应返回 true")
	}
	// 同步占位：函数返回时状态已经是"进行中"，前端第一次轮询就能拿到数字
	if st := site.GetLLMsBuildStatus(); st == nil || st.Status == 0 {
		t.Fatalf("返回时状态应为进行中，实际=%+v", st)
	}
	// 已在构建中／刚结束都不该重复启动
	if site.StartLLMsBuild() && site.GetLLMsBuildStatus().Status == 1 {
		t.Error("构建中重复调用 StartLLMsBuild 不应再次启动")
	}

	// 这个零值站点没有 DB / System，构建必然 panic：
	// 必须被 recover 成 status=-1，进程不能挂、状态不能卡在 1。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st := site.GetLLMsBuildStatus(); st.Status == -1 {
			if st.Msg == "" {
				t.Error("失败状态应带上原因")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("后台构建 panic 后应落成 status=-1，实际=%+v", site.GetLLMsBuildStatus())
}
