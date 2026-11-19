package provider

// 迁移调度层（provider/siteMigration.go）的护栏。
//
// 这一层来自一次真实事故：106 个站点，每个站点都 `go AutoMigrateDB` 无限并发，
// MySQL 被 DDL 打满，日志里 0 条报错、6489 条慢 SQL，客户看到的是
// 「前十几个站点正常、后面的全部初始化失败」。下面的用例锁住那四条约定，
// 每条都带反向对照 —— 改坏了必须变红，否则护栏就是空转。

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm/schema"
	"kandaoni.com/anqicms/model"
)

// resetMigrateGateForTest 换掉进程级的闸门并在用例结束后复原。
// 闸门容量不能热改，所以只能在测试里直接替换 channel。
func resetMigrateGateForTest(t *testing.T, size int) {
	t.Helper()
	migrateSemMu.Lock()
	oldSem, oldSize := migrateSem, migrateSemSize
	migrateSem = make(chan struct{}, size)
	migrateSemSize = size
	migrateSemMu.Unlock()
	t.Cleanup(func() {
		migrateSemMu.Lock()
		migrateSem, migrateSemSize = oldSem, oldSize
		migrateSemMu.Unlock()
	})
}

// TestMigrateGateCapsConcurrency 证明闸门真的把并发压住了。
// 反向对照：同一段 worker 不走闸门时并发必须超过 1 —— 否则「peak==1」可能是
// 因为 worker 根本没能并行，护栏就空转了。
func TestMigrateGateCapsConcurrency(t *testing.T) {
	const workers = 8
	var inFlight, peak int32

	work := func() {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
	}

	resetMigrateGateForTest(t, 1)
	atomic.StoreInt32(&peak, 0)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := migrateSlot()
			defer release()
			work()
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Fatalf("闸门容量 1 时并发应为 1，实际 %d", got)
	}

	// 反向对照：不走闸门。
	atomic.StoreInt32(&peak, 0)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			work()
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&peak); got <= 1 {
		t.Fatalf("反向对照失败：不走闸门时并发应大于 1，实际 %d（说明本用例测不出并发）", got)
	}
}

// TestMigrateSlotLazyInitDoesNotDeadlock 锁住闸门的惰性初始化不能自锁。
//
// 真实踩过：migrateSlot 持着 migrateSemMu 去调 migrateConcurrency()，而后者也要
// 同一把锁 —— 第一次真实迁移直接挂死，三分钟没发出一条 SQL，日志停在「迁移预检」。
// 这类死锁不会报错、不会 panic，只会让启动停在半路，所以必须有超时护栏。
func TestMigrateSlotLazyInitDoesNotDeadlock(t *testing.T) {
	migrateSemMu.Lock()
	oldSem, oldSize := migrateSem, migrateSemSize
	migrateSem, migrateSemSize = nil, 0
	migrateSemMu.Unlock()
	t.Cleanup(func() {
		migrateSemMu.Lock()
		migrateSem, migrateSemSize = oldSem, oldSize
		migrateSemMu.Unlock()
	})

	done := make(chan struct{})
	go func() {
		release := migrateSlot()
		release()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("migrateSlot 在闸门未初始化时死锁：持锁时调用了加锁的并发解析")
	}
	if got := cap(migrateSem); got != defaultMigrateConcurrency {
		t.Fatalf("惰性建闸门容量应为 %d，实际 %d", defaultMigrateConcurrency, got)
	}
}

// fieldTypeSQL 复刻 gorm migrator.FullDataTypeOf 的输出（只覆盖本仓库用到的写法），
// 用来断言「规范化前后生成的 DDL 一字不差」。
func fieldTypeSQL(f *schema.Field) string {
	s := string(f.DataType)
	if f.NotNull {
		s += " NOT NULL"
	}
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// TestNormalizeSchemaFieldNullability 锁住「空转 ALTER」的根因与修法。
//
// 根因：模型把非空写进类型字符串 `type:varchar(32) not null`，gorm 于是
// field.NotNull == false，可生成的 DDL 里又带着 NOT NULL。迁移比对可空性时
// 判定「库列非空、模型可空」就要 ALTER，改完没变化、下次再改。
func TestNormalizeSchemaFieldNullability(t *testing.T) {
	s, err := schema.Parse(&model.Admin{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("解析模型失败：%v", err)
	}
	var target *schema.Field
	for _, f := range s.Fields {
		if f.DBName == "user_name" {
			target = f
			break
		}
	}
	if target == nil {
		t.Fatal("没找到 admins.user_name，用例失效")
	}

	// 1. 复现误判前提：NotNull 为 false，但 DDL 里已经有 NOT NULL。
	if target.NotNull {
		t.Fatalf("预期 field.NotNull == false（这正是误判来源），实际 true —— tag 写法可能已改，请重新评估本 shim 是否还需要")
	}
	before := fieldTypeSQL(target)
	if !strings.Contains(before, "not null") {
		t.Fatalf("预期类型字符串里带 not null，实际 %q", before)
	}

	// 2. 规范化后：NotNull 为 true，且生成的列定义完全一致。
	if fixed := normalizeSchemaFieldNullability(s); fixed == 0 {
		t.Fatal("规范化没有命中任何字段，用例失效")
	}
	if !target.NotNull {
		t.Fatal("规范化后 field.NotNull 仍为 false，空转 ALTER 不会被消除")
	}
	if strings.Contains(strings.ToLower(string(target.DataType)), "not null") {
		t.Fatalf("规范化后 DataType 里仍残留 not null：%q", target.DataType)
	}
	if after := fieldTypeSQL(target); after != before {
		t.Fatalf("规范化改变了列定义：before=%q after=%q", before, after)
	}

	// 3. 幂等：再跑一次不应再计数（否则每次迁移都重复劳动、日志也会骗人）。
	if fixed := normalizeSchemaFieldNullability(s); fixed != 0 {
		t.Fatalf("幂等性失败：第二次规范化仍命中 %d 个字段", fixed)
	}
}

// TestInlineNotNullDebtIsTracked 记录这笔债的规模。
//
// 全仓模型把非空写进了类型字符串（`type:xxx not null`），迁移期靠
// normalizeSchemaFieldNullability 兜住。等 tag 全部改成 `;not null` 之后
// 本用例会变红，提醒把 shim 连同调用点一起删掉，别留死代码。
func TestInlineNotNullDebtIsTracked(t *testing.T) {
	pat := regexp.MustCompile(`type:[^;"\x60]*\s+not\s+null`)
	entries, err := os.ReadDir("../model")
	if err != nil {
		t.Fatalf("读取 model 目录失败：%v", err)
	}
	total, files := 0, 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join("../model", e.Name()))
		if err != nil {
			continue
		}
		if n := len(pat.FindAllString(string(src), -1)); n > 0 {
			total += n
			files++
		}
	}
	if total == 0 {
		t.Fatal("模型里已经没有 `type:xxx not null` 写法了 —— 请删除 normalizeSchemaFieldNullability 及其调用点，并移除本用例")
	}
	t.Logf("仍有 %d 处列定义把 not null 写进类型字符串（涉及 %d 个模型文件），迁移入口靠 shim 消除空转 ALTER", total, files)
}

// TestInitWebsiteMigratesBeforeInitSetting 锁住顺序：迁移 → InitSetting/InitModelData → 注册。
func TestInitWebsiteMigratesBeforeInitSetting(t *testing.T) {
	src, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读取 website.go 失败：%v", err)
	}
	for _, p := range checkInitWebsiteOrder(src) {
		t.Error(p)
	}

	// 反向对照：把注册/初始化提到迁移之前，必须被判为违规。
	bad := []byte(`package provider

func InitWebsite(mw *int) {
	websites.Set(&w)
	w.InitSetting()
	w.InitModelData()
	_ = MigrateSiteSchema(1, "", nil)
}
`)
	if got := checkInitWebsiteOrder(bad); len(got) == 0 {
		t.Fatal("反向对照失败：调换顺序后仍判为合规，说明本护栏抓不住问题")
	}
}

func checkInitWebsiteOrder(src []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "website.go", src, 0)
	if err != nil {
		return []string{"解析失败：" + err.Error()}
	}
	var body *ast.BlockStmt
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "InitWebsite" && fd.Body != nil {
			body = fd.Body
			break
		}
	}
	if body == nil {
		return []string{"找不到 InitWebsite 函数体"}
	}
	pos := map[string]int{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, key := range callKeys(call) {
			if _, seen := pos[key]; !seen {
				pos[key] = fset.Position(call.Pos()).Line
			}
		}
		return true
	})
	migrate, ok := pos["MigrateSiteSchema"]
	if !ok {
		return []string{"InitWebsite 里没有调用 MigrateSiteSchema —— 迁移又被挪走了？"}
	}
	var out []string
	for _, later := range []string{"InitSetting", "InitModelData", "websites.Set"} {
		line, ok := pos[later]
		if !ok {
			out = append(out, "InitWebsite 里找不到 "+later+"，护栏锚点失效")
			continue
		}
		if line < migrate {
			out = append(out, later+" 出现在迁移之前：站点会在半迁移的表结构上被初始化/注册")
		}
	}
	return out
}

// callKeys 返回该调用的可查名字：裸函数名、选择子方法名、以及 `x.Y` 全名。
func callKeys(call *ast.CallExpr) []string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return []string{fn.Name}
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			return []string{fn.Sel.Name, x.Name + "." + fn.Sel.Name}
		}
		return []string{fn.Sel.Name}
	}
	return nil
}

func callFullName(call *ast.CallExpr) string {
	keys := callKeys(call)
	if len(keys) == 0 {
		return ""
	}
	return keys[len(keys)-1]
}

// TestNoAsyncSiteMigration 锁住「迁移不得再 go 出去」。
func TestNoAsyncSiteMigration(t *testing.T) {
	if hits := scanAsyncAutoMigrate(t, "."); len(hits) > 0 {
		t.Fatalf("发现异步迁移调用（%d 处）：%v —— 无限并发正是 106 站点事故的根因", len(hits), hits)
	}
	// 反向对照：合成一份含 `go AutoMigrateDB(...)` 的源码，必须被抓到。
	bad := []byte(`package provider

func bad(db *gorm.DB) {
	go AutoMigrateDB(db, false)
}
`)
	if got := scanAsyncAutoMigrateSource([]byte(bad), "snippet.go"); len(got) == 0 {
		t.Fatal("反向对照失败：含 go AutoMigrateDB 的源码没被抓到，护栏空转")
	}
}

func scanAsyncAutoMigrate(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败：%v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, scanAsyncAutoMigrateSource(src, e.Name())...)
	}
	return out
}

func scanAsyncAutoMigrateSource(src []byte, file string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return nil
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		goStmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		// goStmt.Call 本身就是 *ast.CallExpr，直接取全名即可。
		if strings.HasSuffix(callFullName(goStmt.Call), "AutoMigrateDB") {
			out = append(out, fmt.Sprintf("%s:%d", file, fset.Position(goStmt.Pos()).Line))
		}
		return true
	})
	return out
}

// TestMigrateFailureIsLogged 锁住「迁移错误必须 slog 出来」。
func TestMigrateFailureIsLogged(t *testing.T) {
	src, err := os.ReadFile("siteMigration.go")
	if err != nil {
		t.Fatalf("读取 siteMigration.go 失败：%v", err)
	}
	if !migrateErrorLogged(src) {
		t.Fatal("MigrateSiteSchema 里 AutoMigrateDB 的错误没有被 slog.Error 记录 —— 半迁移站点将无迹可查")
	}
	// 反向对照：把那条 slog.Error 换成 log.Print 后必须判为不合格。
	broken := strings.Replace(string(src), `slog.Error("站点迁移失败`, `log.Print("站点迁移失败`, 1)
	if migrateErrorLogged([]byte(broken)) {
		t.Fatal("反向对照失败：去掉 slog.Error 后仍判为合规，护栏空转")
	}
}

func migrateErrorLogged(src []byte) bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "siteMigration.go", src, 0)
	if err != nil {
		return false
	}
	found := false
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "MigrateSiteSchema" || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok || ifs.Init == nil {
				return true
			}
			assign, ok := ifs.Init.(*ast.AssignStmt)
			if !ok || len(assign.Rhs) != 1 {
				return true
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok || callFullName(call) != "AutoMigrateDB" {
				return true
			}
			// 迁移错误分支里必须有 slog.Error。
			ast.Inspect(ifs.Body, func(inner ast.Node) bool {
				c, ok := inner.(*ast.CallExpr)
				if ok && callFullName(c) == "slog.Error" {
					found = true
				}
				return true
			})
			return true
		})
	}
	return found
}

// TestParseSiteIDSpec 锁住 -migrate-sites 的解析：分批迁移漏站点比少跑一批更危险。
func TestParseSiteIDSpec(t *testing.T) {
	cases := []struct {
		in      string
		want    []int
		wantErr bool
	}{
		{in: "", want: nil},
		{in: "1", want: []int{1}},
		{in: "1,3,9-12", want: []int{1, 3, 9, 10, 11, 12}},
		{in: " 5 , 7 ", want: []int{5, 7}},
		{in: "5,5,5", want: []int{5}},
		{in: "0", wantErr: true},
		{in: "-3", wantErr: true},
		{in: "9-3", wantErr: true},
		{in: "1,,2", wantErr: true},
		{in: "a", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseSiteIDSpec(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%q 应该报错，实际得到 %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q 不应报错：%v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%q 解析结果 %v，期望 %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q 解析结果 %v，期望 %v", c.in, got, c.want)
				break
			}
		}
	}
}
