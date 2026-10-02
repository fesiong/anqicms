package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// 分页工具的契约：工具自己按预算决定返回多少，并且**如实说明**返回了哪一段。
// 通用截断器在工具承诺之后盲切，会让模型把被腰斩的输出当成完整内容。

func TestFitWindow_CutsAtItemBoundary(t *testing.T) {
	items := []string{"aaaa", "bbbb", "cccc"}
	render := func(_ int, s string) string { return s + "\n" }

	if body, n := fitWindow(items, 15, render); n != 3 || body != "aaaa\nbbbb\ncccc\n" {
		t.Fatalf("预算足够时应全量返回: n=%d body=%q", n, body)
	}
	// 第 3 条会撑破 10 字节：必须停在条目边界，而不是切半条
	if body, n := fitWindow(items, 10, render); n != 2 || body != "aaaa\nbbbb\n" {
		t.Fatalf("应在条目边界停下: n=%d body=%q", n, body)
	}
	if body, n := fitWindow(items[:0], 100, render); n != 0 || body != "" {
		t.Fatalf("空输入应返回空: n=%d body=%q", n, body)
	}
}

// 至少返回一条：否则续读游标原地打转，模型永远读不到后面的内容。
func TestFitWindow_AlwaysEmitsOneItem(t *testing.T) {
	items := []string{strings.Repeat("x", 5000), "tail"}
	if body, n := fitWindow(items, 10, renderNl); n != 1 || body != items[0]+"\n" {
		t.Fatalf("超预算的首条仍须返回: n=%d len=%d", n, len(body))
	}
	// 负预算（header 已吃满整个预算）同样不能返回空
	if _, n := fitWindow(items, -100, renderNl); n != 1 {
		t.Fatalf("负预算也应返回一条: n=%d", n)
	}
}

func renderNl(_ int, s string) string { return s + "\n" }

func TestWindowTrailer_ThreeStates(t *testing.T) {
	tests := []struct {
		name      string
		first     int
		last      int
		total     int
		budgetCut bool
		want      string
		wantNot   string
	}{
		{"读到末尾", 1, 100, 100, false, "已全部返回", "offset="},
		{"被预算切到", 1, 60, 100, true, "续读：offset=61", "已全部返回"},
		{"本次区间结束", 400, 449, 1000, false, "已到本次请求区间末尾", "已全部返回"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := windowTrailer("行", tc.first, tc.last, tc.total, 10000, tc.budgetCut)
			if !strings.Contains(got, tc.want) {
				t.Errorf("应包含 %q，实际 %q", tc.want, got)
			}
			if tc.wantNot != "" && strings.Contains(got, tc.wantNot) {
				t.Errorf("不应包含 %q，实际 %q", tc.wantNot, got)
			}
		})
	}
}

// read_file 的核心不变量：输出永不超过预算，且尾部给出的续读坐标可直接用。
func TestReadFile_WindowStaysWithinBudgetAndResumable(t *testing.T) {
	svc := tempFileService(t)
	writeLineFile(t, svc.projectRoot, "big.go", 1200)

	h := svc.Handlers["read_file"]
	budget := svc.resultBudget()

	out, err := h(context.Background(), `{"file_path":"big.go","offset":400,"limit":400}`)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if len(out) > budget {
		t.Fatalf("输出 %d 字节超过预算 %d，中间件会接管盲切", len(out), budget)
	}
	if strings.Contains(out, "结果已截断") {
		t.Fatal("工具不应留下通用截断标记")
	}
	last, more := reportedWindowEnd(t, out)
	if !more {
		t.Fatal("offset=400/limit=400 只覆盖到 799 行，不该声称读完")
	}
	if last < 400 || last >= 799 {
		t.Fatalf("续读游标越界: last=%d（应落在 400..798）", last)
	}

	// 照着尾部提示逐次续读：游标必须单调前进，并且区间之间不留空洞。
	seen := map[int]bool{}
	for offset := 400; ; {
		out, err = h(context.Background(), fmt.Sprintf(`{"file_path":"big.go","offset":%d,"limit":400}`, offset))
		if err != nil {
			t.Fatalf("续读 offset=%d: %v", offset, err)
		}
		if len(out) > budget {
			t.Fatalf("续读 offset=%d 输出 %d 字节超过预算", offset, len(out))
		}
		last, more = reportedWindowEnd(t, out)
		if last < offset {
			t.Fatalf("游标原地打转: offset=%d last=%d", offset, last)
		}
		for ln := offset; ln <= last; ln++ {
			seen[ln] = true
		}
		if !more {
			break
		}
		offset = last + 1
	}
	if last != 1200 {
		t.Fatalf("续读循环应收敛到文件末尾，实际 last=%d", last)
	}
	for ln := 400; ln <= 1200; ln++ {
		if !seen[ln] {
			t.Fatalf("第 %d 行从未被任何一次窗口覆盖（有洞）", ln)
		}
	}
}

// 骨架视图本身也是列表：同样要落在预算内，且不能残留没人填参数的格式串。
func TestReadFile_SkeletonRespectsBudget(t *testing.T) {
	svc := tempFileService(t)
	writeLineFile(t, svc.projectRoot, "big.go", 4000) // 符号足够多，逼出骨架自身的分页

	out, err := svc.Handlers["read_file"](context.Background(), `{"file_path":"big.go"}`)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if len(out) > svc.resultBudget() {
		t.Fatalf("骨架 %d 字节超过预算 %d", len(out), svc.resultBudget())
	}
	for _, bad := range []string{"%d", "%!"} {
		if strings.Contains(out, bad) {
			t.Fatalf("骨架输出残留未格式化的 %q:\n%s", bad, out)
		}
	}
	if !strings.Contains(out, "offset=") {
		t.Fatalf("骨架被切开时必须给出续读坐标:\n%s", out)
	}
}

// tempFileService 返回一个以临时目录为项目根的服务实例（内置文件工具按 projectRoot 校验路径）。
func tempFileService(t *testing.T) *AiChatService {
	t.Helper()
	svc := testService()
	svc.projectRoot = tempRoot(t)
	return svc
}

// grep / glob 同样是分页工具：预算内切窗 + 可续读，且不许留下通用截断标记。
func TestGrepAndGlob_WindowResumableWithinBudget(t *testing.T) {
	svc := tempFileService(t)
	writeManyGoFiles(t, svc.projectRoot, 250, "var XMARK = 1\nvar Y = 2\n")
	budget := svc.resultBudget()

	cases := []struct {
		tool  string
		argsT string // 含一个 %d 占位，填 offset
		total int
	}{
		{"grep", `{"pattern":"XMARK","offset":%d}`, 250},
		{"glob", `{"pattern":"**.go","offset":%d}`, 250},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			seen := map[int]bool{}
			offset, last, more, rounds := 1, 0, true, 0
			for more {
				rounds++
				if rounds > 50 {
					t.Fatal("续读循环未收敛")
				}
				out, err := svc.Handlers[tc.tool](context.Background(), fmt.Sprintf(tc.argsT, offset))
				if err != nil {
					t.Fatalf("%s offset=%d: %v", tc.tool, offset, err)
				}
				if len(out) > budget {
					t.Fatalf("%s 输出 %d 字节超过预算 %d", tc.tool, len(out), budget)
				}
				if strings.Contains(out, "结果已截断") {
					t.Fatalf("%s 不应留下通用截断标记", tc.tool)
				}
				last, more = reportedWindowEnd(t, out)
				if last < offset {
					t.Fatalf("游标原地打转: offset=%d last=%d", offset, last)
				}
				for n := offset; n <= last; n++ {
					seen[n] = true
				}
				offset = last + 1
			}
			if last != tc.total {
				t.Fatalf("应收敛到第 %d 项，实际 %d", tc.total, last)
			}
			for n := 1; n <= tc.total; n++ {
				if !seen[n] {
					t.Fatalf("第 %d 项从未出现在任何窗口里（有洞）", n)
				}
			}
		})
	}
}

// 单次调用放不下全部结果时，必须显式给出续读坐标而不是默默少返回。
func TestGrepAndGlob_PromiseResumeWhenCut(t *testing.T) {
	svc := tempFileService(t)
	writeManyNamedFiles(t, svc.projectRoot, 300, "a_padding_name_to_make_paths_long_%03d.go",
		"var Z = \"long value to blow up the rendered size\"\n")

	for _, tc := range []struct{ tool, args string }{
		{"grep", `{"pattern":"long value"}`},
		{"glob", `{"pattern":"**.go"}`},
	} {
		out, err := svc.Handlers[tc.tool](context.Background(), tc.args)
		if err != nil {
			t.Fatalf("%s: %v", tc.tool, err)
		}
		if !strings.Contains(out, "续读：offset=") {
			t.Fatalf("%s 单次装不下时必须提示续读:\n…%s", tc.tool, tail(out, 300))
		}
		if len(out) > svc.resultBudget() {
			t.Fatalf("%s 输出 %d 字节超过预算", tc.tool, len(out))
		}
	}
}

// search_replace 改写不完时必须说出来：过去它默默丢掉第 21 个之后的匹配文件，
// 模型却以为整个范围已经替换干净（这是写操作，比读错更贵）。
func TestSearchReplace_ReportsUnscannedTail(t *testing.T) {
	svc := tempFileService(t)
	writeManyGoFiles(t, svc.projectRoot, 25, "var Old = 1\n")
	h := svc.Handlers["search_replace"]

	out, err := h(context.Background(), `{"search":"var Old","replace":"var New","glob":"**.go"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "改写 20 个") {
		t.Fatalf("应改写满单次上限 20 个:\n%s", out)
	}
	if !strings.Contains(out, "未检视") || !strings.Contains(out, "offset=21") {
		t.Fatalf("必须报告未检视区间并给出续读 offset:\n%s", out)
	}
	if !fileContains(t, svc.projectRoot, "f024.go", "var Old") {
		t.Fatal("第 25 个文件此刻不应已被改写")
	}

	out, err = h(context.Background(), `{"search":"var Old","replace":"var New","glob":"**.go","offset":21}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "未检视") {
		t.Fatalf("续接到末尾后不该再有未检视:\n%s", out)
	}
	if !strings.Contains(out, "改写 5 个") {
		t.Fatalf("剩余 5 个应被改写:\n%s", out)
	}
	if !fileContains(t, svc.projectRoot, "f024.go", "var New") {
		t.Fatal("第 25 个文件应已被改写")
	}
}

func fileContains(t *testing.T, dir, name, want string) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(data), want)
}

func writeManyGoFiles(t *testing.T, dir string, count int, body string) {
	t.Helper()
	writeManyNamedFiles(t, dir, count, "f%03d.go", body)
}

// writeManyNamedFiles 建 count 个 .go 文件，文件名由 format 给出。
// 需要逼出「单次装不下」时用长文件名，比堆几千个文件便宜得多。
func writeManyNamedFiles(t *testing.T, dir string, count int, format, body string) {
	t.Helper()
	for i := 0; i < count; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf(format, i)),
			[]byte("package p\n"+body), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// reportedWindowEnd 解析尾部说明，返回「最后交付的序号」和「是否还有后续」。
func reportedWindowEnd(t *testing.T, out string) (last int, more bool) {
	t.Helper()
	i := strings.LastIndex(out, "[已返回第 ")
	if i < 0 {
		t.Fatalf("缺少窗口说明:\n%s", out)
	}
	trailer := out[i:]
	more = !strings.Contains(trailer, "已全部返回")
	head := trailer[len("[已返回第 "):]
	sp := strings.IndexByte(head, ' ')
	if sp < 0 {
		t.Fatalf("无法解析窗口区间: %q", trailer)
	}
	if _, err := fmt.Sscanf(head[:sp], "%d–%d", new(int), &last); err != nil {
		t.Fatalf("无法解析末行号: %v / %q", err, head[:sp])
	}
	return last, more
}

func writeLineFile(t *testing.T, dir, name string, lines int) {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "func f%04d() { x := %d }\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
}

// tempRoot 返回真实文件工具可用的项目根。
// macOS 的 t.TempDir() 在 /var → /private/var 这条符号链接下面，而 safePathResolve
// 会把「解析过链接的路径」和未解析的 root 做前缀比较，于是所有路径都被判成越界。
func tempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return dir
}

// 不可分页的输出（命令输出、网页正文）给不出续读坐标，但裁剪标记必须与预算同源：
// 工具里写死的 50000/30000 上限永远到不了模型眼前——中间件在 10000 就先切了。
func TestCapNonPagingResult_StaysWithinBudgetAndNamesTotal(t *testing.T) {
	const budget = 10000
	long := strings.Repeat("line of output\n", 2000) // 30000 字节

	got := capNonPagingResult(long, budget, "")
	if len(got) > budget {
		t.Fatalf("裁剪后仍超预算: %d > %d", len(got), budget)
	}
	if !strings.Contains(got, fmt.Sprintf("输出共 %d 字节", len(long))) {
		t.Errorf("标记应写明原始规模，实际尾部 %q", tailOf(got))
	}
	if !strings.Contains(got, "无法续读") {
		t.Error("应明确尾部不可恢复，避免模型以为自己看完了")
	}
	body := got[:strings.Index(got, "\n\n[")]
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("应停在行边界，实际 %q", tailOf(body))
	}
	if !utf8.ValidString(got) {
		t.Error("裁剪结果不是合法 UTF-8")
	}
	if s := "short\n"; capNonPagingResult(s, budget, "") != s {
		t.Error("预算内不应改动内容")
	}
}

func TestBashTool_CapsOutputWithinBudget(t *testing.T) {
	svc := tempFileService(t)
	handler, ok := svc.Handlers["bash"]
	if !ok {
		t.Skip("bash 工具未注册")
	}
	budget := svc.resultBudget()

	out, err := handler(context.Background(),
		`{"command":"for i in $(seq 1 4000); do echo \"line $i padding padding padding\"; done","timeout":30}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > budget {
		t.Fatalf("bash 输出超出预算: %d > %d", len(out), budget)
	}
	if strings.Contains(out, "结果已截断") || strings.Contains(out, "超过 50000 字符") {
		t.Errorf("不应出现被中间件抢先的旧截断标记: %q", tailOf(out))
	}
	if !strings.Contains(out, "输出共") {
		t.Errorf("应说明原始输出规模: %q", tailOf(out))
	}
	if !strings.HasPrefix(out, "$ for i in") {
		t.Errorf("命令回显应留在开头: %q", out[:40])
	}
}

// 失败输出（stderr）过长时，退出码同样不能被裁掉。
func TestBashTool_KeepsExitCodeWhenOutputFloods(t *testing.T) {
	svc := tempFileService(t)
	handler, ok := svc.Handlers["bash"]
	if !ok {
		t.Skip("bash 工具未注册")
	}
	out, err := handler(context.Background(),
		`{"command":"for i in $(seq 1 4000); do echo \"noise $i noise noise noise\"; done; echo boom >&2; exit 3","timeout":30}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[退出码/错误:") {
		t.Errorf("超长输出下仍须保留退出码: %q", out[:120])
	}
}

func tailOf(s string) string {
	if len(s) > 160 {
		return s[len(s)-160:]
	}
	return s
}

// 网络类工具（web_fetch/web_search）没有 offset 参数：超预算时必须把全文存档，
// 让被裁掉的尾部重新变成可取——取不回来的"完整说明"就是谎报。
func TestCapWebResult_ArchiveIsResumableByReadFile(t *testing.T) {
	svc := tempFileService(t)
	const needle = "TAIL_NEEDLE_只出现在最后"

	var b strings.Builder
	for i := 1; i <= 2000; i++ {
		fmt.Fprintf(&b, "line %04d fetched page text padding padding\n", i)
	}
	content := b.String() + needle + "\n"

	got := svc.capWebResult("web_fetch", content)
	if len(got) > svc.resultBudget() {
		t.Fatalf("存档版仍超预算: %d > %d", len(got), svc.resultBudget())
	}
	if strings.Contains(got, "无法续读") {
		t.Errorf("已存档却仍说不可续读: %q", tailOf(got))
	}
	m := regexp.MustCompile(`存档为项目文件：(\S+)`).FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("标记未给出存档路径: %q", tailOf(got))
	}
	rel := m[1]
	if !strings.HasPrefix(rel, archivedResultDir+"/") {
		t.Fatalf("存档应落在约定目录内，实际 %q", rel)
	}

	// 承诺 1：续读坐标必须无缝衔接——标记说覆盖了前 N 行，read_file 就得从 N+1 行开始
	deliveredMatch := regexp.MustCompile(`完整覆盖前 (\d+) 行`).FindStringSubmatch(got)
	if deliveredMatch == nil {
		t.Fatalf("标记未说明覆盖到第几行: %q", tailOf(got))
	}
	delivered, _ := strconv.Atoi(deliveredMatch[1])
	out, err := svc.Handlers["read_file"](context.Background(),
		fmt.Sprintf(`{"path":%q,"offset":%d,"limit":5}`, rel, delivered+1))
	if err != nil {
		t.Fatalf("续读存档失败: %v", err)
	}
	if !strings.Contains(out, fmt.Sprintf("%6d| line %04d ", delivered+1, delivered+1)) {
		t.Errorf("续读起点应为第 %d 行: %q", delivered+1, out[:min(len(out), 200)])
	}

	// 承诺 2：被裁掉的尾部仍在文件里，能取回来
	totalMatch := regexp.MustCompile(`输出共 \d+ 字节 / (\d+) 行`).FindStringSubmatch(got)
	if totalMatch == nil {
		t.Fatalf("标记未说明总行数: %q", tailOf(got))
	}
	total, _ := strconv.Atoi(totalMatch[1])
	tail, err := svc.Handlers["read_file"](context.Background(),
		fmt.Sprintf(`{"path":%q,"offset":%d,"limit":5}`, rel, total))
	if err != nil {
		t.Fatalf("读取存档末尾失败: %v", err)
	}
	if !strings.Contains(tail, needle) {
		t.Errorf("存档末尾应保留被裁掉的尾部: %q", tailOf(tail))
	}
	if strings.Contains(tail, "结果已截断") {
		t.Error("续读不应触发通用截断器")
	}
}

// 存档里同样要脱敏：落盘等于把内容写进用户的磁盘，不能只保证返回给模型的副本干净。
func TestArchiveToolOutput_RedactsBeforeWriting(t *testing.T) {
	svc := tempFileService(t)
	rel := svc.archiveToolOutput("web_fetch", "页面内容 api_key=sk-VerySecretValue123456 结束")
	if rel == "" {
		t.Fatal("存档失败")
	}
	data, err := os.ReadFile(filepath.Join(svc.projectRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sk-VerySecretValue123456") {
		t.Error("存档内容未脱敏")
	}
}

// 预算内的结果不该产生任何文件：存档只为「确实丢了东西」存在。
func TestCapWebResult_NoArchiveWhenWithinBudget(t *testing.T) {
	svc := tempFileService(t)
	got := svc.capWebResult("web_fetch", "short page\n")
	if got != "short page\n" {
		t.Error("预算内应原样返回")
	}
	if _, err := os.Stat(filepath.Join(svc.projectRoot, archivedResultDir)); err == nil {
		t.Error("预算内不应创建存档目录")
	}
}

// 存档必须自限：过期即清、超量截尾，否则 AI 跑久了会在用户项目里堆出无人回收的垃圾。
func TestPruneArchivedResults_RemovesStaleAndOverflow(t *testing.T) {
	svc := tempFileService(t)
	dir := filepath.Join(svc.projectRoot, archivedResultDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	stale := filepath.Join(dir, "web_fetch-stale.txt")
	if err := os.WriteFile(stale, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	dead := time.Now().Add(-archivedResultTTL - time.Hour)
	if err := os.Chtimes(stale, dead, dead); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < archivedResultMaxFiles+20; i++ {
		p := filepath.Join(dir, fmt.Sprintf("web_fetch-%03d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if rel := svc.archiveToolOutput("web_fetch", strings.Repeat("a\n", 10)); rel == "" {
		t.Fatal("存档失败")
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("过期存档应被清理")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txt") {
			count++
		}
	}
	if count > archivedResultMaxFiles+1 {
		t.Errorf("超量存档应被裁到上限（+1 是本次新写的），实际 %d", count)
	}
}

// grep 必须能为「点名要搜的大文件」破例：真机回归里 web_fetch 的存档有 6375 行，
// 被 ">5000 行" 保护默默跳过，于是 grep 报「未找到匹配」——而它一眼都没看。
func TestGrep_PathSearchesOversizedFileAndReportsSkips(t *testing.T) {
	svc := tempFileService(t)
	const needle = "ZZZ_ONLY_IN_TAIL"

	// 一个 6000 行的文件（超过 grep 的广度扫描保护线），needle 放在最后
	var big strings.Builder
	for i := 1; i <= 5999; i++ {
		fmt.Fprintf(&big, "filler %04d some prose text here\n", i)
	}
	big.WriteString("the last line " + needle + "\n")
	if err := os.WriteFile(filepath.Join(svc.projectRoot, "archive.txt"), []byte(big.String()), 0644); err != nil {
		t.Fatal(err)
	}
	// 再放一个小文件，保证广度扫描本身有结果
	if err := os.WriteFile(filepath.Join(svc.projectRoot, "small.txt"), []byte("harmless\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 不带 path：应如实说明"跳过了大文件"，而不是假装搜过
	wide, err := svc.Handlers["grep"](context.Background(), `{"pattern":"ZZZ_ONLY_IN_TAIL"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wide, "跳过 1 个超大文件") {
		t.Errorf("广度扫描应报告被跳过的大文件: %q", tailOf(wide))
	}
	if strings.Contains(wide, "超过 5000 行") == false || !strings.Contains(wide, "archive.txt") {
		t.Errorf("跳过说明应点名文件与原因: %q", tailOf(wide))
	}

	// 带 path 点名该文件：必须搜到
	one, err := svc.Handlers["grep"](context.Background(),
		`{"pattern":"ZZZ_ONLY_IN_TAIL","path":"archive.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(one, needle) {
		t.Errorf("path 指定单文件时不应受大文件跳过限制: %q", tailOf(one))
	}
	if len(one) > svc.resultBudget() {
		t.Errorf("grep 结果超出预算: %d > %d", len(one), svc.resultBudget())
	}
}

func TestGrep_PathRejectsOutsideRoot(t *testing.T) {
	svc := tempFileService(t)
	if _, err := svc.Handlers["grep"](context.Background(), `{"pattern":"x","path":"../../etc/passwd"}`); err == nil {
		t.Error("越界 path 应被拒绝")
	}
	out, err := svc.Handlers["grep"](context.Background(), `{"pattern":"x","path":"no-such-file.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "路径不存在") {
		t.Errorf("不存在的路径应明确说明: %q", out)
	}
}
