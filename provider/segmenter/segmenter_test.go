package segmenter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/huichen/sego"
)

// resetForTest 把进程级状态清空，让用例能重新走一遍加载流程。
// 同包测试专用；这些用例都**不能** t.Parallel（共用全局 once）。
func resetForTest() {
	once = sync.Once{}
	shared = nil
	state = Status{}
	loads = 0
}

// TestSharedLoadsOnceUnderConcurrency 是本包存在的理由：
// 无论多少站点/goroutine 同时来要分词器，进程里只能加载一份。
//
// 反向对照（历史行为）：把 sync.Once 去掉、恢复成
// `if shared == nil { load() }`，64 个 goroutine 会各自加载一遍，
// loads 远大于 1 —— 那正是 106 站点启动时机器被吃满的原因。
func TestSharedLoadsOnceUnderConcurrency(t *testing.T) {
	// 用临时词典，避免依赖仓库里那份占位词典在不在（也避免"都没加载成功、
	// 64 个 nil 相等"这种假绿）。
	path := writeTempDict(t)
	prev := resolvePath
	resolvePath = func() (string, error) { return path, nil }
	defer func() { resolvePath = prev }()
	resetForTest()
	defer resetForTest()

	const n = 64
	got := make([]*sego.Segmenter, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i] = Shared()
		}(i)
	}
	wg.Wait()

	if got[0] == nil {
		t.Fatalf("词典可用时必须返回非 nil 分词器，实际全为 nil（%+v）", StatusNow())
	}
	for i := 1; i < n; i++ {
		if got[i] != got[0] {
			t.Fatalf("第 %d 个调用拿到的分词器与第 0 个不同（%p vs %p）：词典被加载了多份", i, got[i], got[0])
		}
	}
	if loads != 1 {
		t.Fatalf("词典加载次数 = %d，应为 1（每多一次就是多一份数百 MB 常驻内存）", loads)
	}
	if LoadCount() != 1 {
		t.Fatalf("LoadCount() = %d，应为 1", LoadCount())
	}
	status := StatusNow()
	if !status.Loaded {
		t.Fatalf("Loaded 应为 true：%+v", status)
	}
	if status.Path != path {
		t.Fatalf("Status.Path = %q，应为 %q", status.Path, path)
	}
	if status.Tokens == 0 {
		t.Fatalf("Status.Tokens 应反映真实词条数：%+v", status)
	}
	// 反向对照：分词器真的能用，不是个空壳
	words := sego.SegmentsToSlice(Shared().Segment([]byte("安企内容管理系统")), false)
	if len(words) == 0 {
		t.Fatal("共享分词器切不出任何词")
	}
}

// writeTempDict 写一份最小可用词典，返回路径。
// sego 只收「词 频率 词性」，频率 <2 的会被丢掉。
func writeTempDict(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, DictionaryFileName)
	content := "安企 100 n\n内容 100 n\n管理 100 n\n系统 100 n\n内容管理系统 50 n\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写临时词典失败：%v", err)
	}
	return path
}

// TestResolveInSkipsMissingAndEmpty 锁住两条"看起来能找到、其实用不了"的路径：
// 目录里没有词典、以及词典是 0 字节空文件。
func TestResolveInSkipsMissingAndEmpty(t *testing.T) {
	dir := t.TempDir()

	// 1) 文件不存在 → 报错，且错误里带上找过的路径（否则现场根本不知道它去哪找过）
	_, err := resolveIn([]string{dir})
	if err == nil {
		t.Fatal("目录里没有词典时 resolveIn 必须报错")
	}
	if !strings.Contains(err.Error(), DictionaryFileName) {
		t.Fatalf("错误信息应包含文件名 %q：%v", DictionaryFileName, err)
	}

	// 2) 空文件 → 仍然报错，并明确标注"空文件"
	empty := filepath.Join(dir, DictionaryFileName)
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("写空词典失败：%v", err)
	}
	_, err = resolveIn([]string{dir})
	if err == nil || !strings.Contains(err.Error(), "空文件") {
		t.Fatalf("0 字节词典应被跳过并标注空文件，实际 err=%v", err)
	}

	// 3) 有内容 → 命中
	if err := os.WriteFile(empty, []byte("安企 1 n\n"), 0o644); err != nil {
		t.Fatalf("写词典失败：%v", err)
	}
	path, err := resolveIn([]string{dir})
	if err != nil {
		t.Fatalf("有内容的词典应能找到，实际 err=%v", err)
	}
	if path != empty {
		t.Fatalf("应命中 %s，实际 %s", empty, path)
	}

	// 4) 多目录顺序：第一个目录没有，第二个有 → 用第二个
	first, second := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(second, DictionaryFileName), []byte("安企 1 n\n"), 0o644); err != nil {
		t.Fatalf("写第二个目录的词典失败：%v", err)
	}
	path, err = resolveIn([]string{first, second})
	if err != nil {
		t.Fatalf("按顺序查找应命中第二个目录，实际 err=%v", err)
	}
	if path != filepath.Join(second, DictionaryFileName) {
		t.Fatalf("应命中第二个目录，实际 %s", path)
	}
}

// TestMissingDictionaryDegradesWithoutExit 锁住"词典缺失不杀进程"。
//
// 历史行为：sego.LoadDictionary 对打不开的文件是 log.Fatalf —— 也就是
// os.Exit(1)，defer recover 拦不住。现场表现是"进程毫无征兆地退出"，
// 而 trigger 只是某个站点开了全文搜索。现在必须是：返回 nil + 记错误 + 继续跑。
func TestMissingDictionaryDegradesWithoutExit(t *testing.T) {
	prev := resolvePath
	resolvePath = func() (string, error) {
		return "", errors.New("dictionary.txt 未找到（测试构造）")
	}
	defer func() { resolvePath = prev }()
	resetForTest()
	defer resetForTest()

	if got := Shared(); got != nil {
		t.Fatalf("词典不可用时 Shared() 必须返回 nil，实际 %p", got)
	}
	status := StatusNow()
	if status.Loaded {
		t.Fatalf("加载失败时 Status.Loaded 不能为 true：%+v", status)
	}
	if !strings.Contains(status.Error, "未找到") {
		t.Fatalf("Status.Error 应保留失败原因，实际 %q", status.Error)
	}
	// 反向对照：失败也计一次"尝试加载"，不能是 0（0 会被误读成"还没开始"）
	if loads != 1 {
		t.Fatalf("失败路径也应记录一次加载尝试，实际 loads=%d", loads)
	}
	// 再调一次仍然是 nil，且不会重复尝试
	if Shared() != nil || loads != 1 {
		t.Fatalf("失败后重复调用不应重新加载，loads=%d", loads)
	}
}
