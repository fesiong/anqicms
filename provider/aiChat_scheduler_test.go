package provider

import (
	"os"
	"strings"
	"testing"
)

// TestSchedulerDoesNotPreMarkRunningAgents 调度器不能预置 runningAgents。
//
// 2026-10-03 实测踩过：checkDueAgents 先 runningAgents[id]=true 再 go 调 ExecuteAgent，
// 而 ExecuteAgent 开头又检查一次 runningAgents[id] 并对已运行直接返回
// "agent #N is already running" —— 于是**调度器这条路径 100% 失败**，
// 且调度器的 defer 会把标记删掉，下一个 tick 又进来再失败一次。
// 后果：cron 定时任务从未真正执行过（GEO Agent 今早 8 点漏跑，run_count 停在 1），
// 而日志里只有一条 ERROR，没有任何「为什么没跑」的线索。
//
// 防重入的唯一责任必须落在 ExecuteAgent 内部（Lock→检查→置位 是原子的），
// 调度器只负责挑出到期 Agent 并发起 goroutine。
func TestSchedulerDoesNotPreMarkRunningAgents(t *testing.T) {
	src, err := os.ReadFile("aiChat.go")
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	text := string(src)
	i := strings.Index(text, "func (svc *AiChatService) checkDueAgents(")
	if i < 0 {
		t.Fatal("未找到 checkDueAgents")
	}
	j := strings.Index(text[i+1:], "\nfunc ")
	if j < 0 {
		j = len(text) - i
	}
	body := text[i : i+j]

	// 调度器里不能再出现对 runningAgents 的写入
	for _, bad := range []string{
		"svc.runningAgents[agent.Id] = true",
		"delete(svc.runningAgents,",
	} {
		if strings.Contains(body, bad) {
			t.Errorf("checkDueAgents 不应再操作 runningAgents（会与 ExecuteAgent 的防重入冲突）：%q", bad)
		}
	}
	// 但仍应保留「Agent due, executing」这行可观测日志
	if !strings.Contains(body, "Agent due, executing") {
		t.Error("应保留 Agent due, executing 日志，否则定时执行失败时无从排查")
	}
}

// TestLoadAgentsKeepsOverdueForSingleCatchUp 启动时过期 Agent 只能补跑一次，
// 不能既重算到未来（静默跳过）也不做记录。
//
// 权衡：直接跳到下个周期 = 一次重启就让「每天出 3-5 篇」的内容 Agent 丢一天；
// 无限补跑 = 停机一周后启动连补 7 次，LLM 成本与内容都失控。
// 做法是保持过期（不改 NextRunAt），让首个 tick 捡起来跑一次，跑完由 ExecuteAgent 推进。
func TestLoadAgentsKeepsOverdueForSingleCatchUp(t *testing.T) {
	src, err := os.ReadFile("aiChat.go")
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	text := string(src)
	i := strings.Index(text, "func (svc *AiChatService) loadAgentsFromDB(")
	if i < 0 {
		t.Fatal("未找到 loadAgentsFromDB")
	}
	j := strings.Index(text[i+1:], "\nfunc ")
	if j < 0 {
		j = len(text) - i
	}
	body := text[i : i+j]

	// 过期分支必须留下可观测日志
	if !strings.Contains(body, "will run once now") {
		t.Error("过期 Agent 应记录「将补跑一次」的日志，否则漏跑完全静默")
	}
	// 关键：注册到 svc.agents 必须发生在过期分支的 continue **之前**。
	// 早前把注册放在判断之后、continue 之前，导致过期 Agent 压根没进内存 map，
	// 而调度器 tick 扫的就是这个 map —— 补跑日志打了却永远不会被执行。
	if !strings.Contains(body, "先入内存缓存") {
		t.Error("缺少「先入内存缓存」的结构说明/注册（continue 分支会跳过末尾注册）")
	}
	regAt := strings.Index(body, "svc.agents[agents[i].Id] = &agents[i]")
	// 只认「独立成行的 continue」：泛搜 "continue" 会命中闭包/循环里的其它关键字。
	contAt := -1
	off := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "continue" {
			contAt = off
			break
		}
		off += len(line) + 1
	}
	if regAt < 0 {
		t.Fatal("未找到注册语句 svc.agents[agents[i].Id] = &agents[i]")
	}
	if contAt < 0 {
		t.Fatal("未找到过期分支的独立 continue")
	}
	if regAt > contAt {
		t.Errorf("注册必须早于 continue（当前 regAt=%d > contAt=%d）：过期 Agent 会被跳过注册，调度器扫不到它",
			regAt, contAt)
	}
	// 注册只应出现一次（避免末尾残留一份被 continue 跳过）
	if strings.Count(body, "svc.agents[agents[i].Id] = &agents[i]") != 1 {
		t.Errorf("注册语句应只出现一次，实际 %d 次（末尾残留的那份会被 continue 跳过）",
			strings.Count(body, "svc.agents[agents[i].Id] = &agents[i]"))
	}
	// 过期的 Agent 不应再被重算到未来（旧实现正是这么做的，导致直接跳过）
	if strings.Contains(body, "agent next_run recomputed, missed run skipped") {
		t.Error("不应再把过期 Agent 重算到未来（等于静默跳过本次执行）")
	}
}
