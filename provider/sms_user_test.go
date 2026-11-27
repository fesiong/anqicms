package provider

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// 本文件是任务 7 的护栏：3 条 go/ast 断言（每条带反向对照）+ 2 个真库测试函数
// （共 4 条真库用例）。go/ast 扫描器一律复用 channel_model_test.go 与
// sms_model_test.go 的既有实现（smsStructType / smsStructTag / parseGormIndexes /
// parseSmsSynthetic），此处只写「把 helper 组合成不变式」的检查器，不重写扫描器。

const (
	smsUserModelPath    = "../model/user.go"
	smsUserProviderPath = "user.go"
)

// loadSmsUserSource 读取真实源码文本 + AST。文本供反向对照做单点变异，
// AST 供正向检查器。步骤 2（RED）里符号尚未定义时会编译失败，护栏随即变红。
func loadSmsUserSource(t *testing.T, path string) (string, *ast.File) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", path, err)
	}
	return string(src), file
}

// ---- 检查器 1：model.User 区号列 ----

// checkUserPhoneCountryColumn 用 go/ast 穷举 model.User 字段：
//   - PhoneCountry 必须存在且为 string，gorm tag 逐字含 type:varchar(8) / not null / default:'86'
//   - Phone 字段穷举其索引声明，不得出现 uniqueIndex（规格 §4.1：phone 保持非唯一 idx_users_phone）
//
// 声明式判定不写死字段总数：新增字段不该让本护栏变红，缺字段/加唯一索引才该红。
func checkUserPhoneCountryColumn(file *ast.File) error {
	st := smsStructType(file, "User")
	if st == nil {
		return fmt.Errorf("源码里找不到 model.User 结构体：检查器无法判定")
	}
	found, gotType := false, ""
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name != "PhoneCountry" {
				continue
			}
			found = true
			if id, ok := f.Type.(*ast.Ident); ok {
				gotType = id.Name
			}
		}
	}
	if !found {
		return fmt.Errorf("model.User 缺少 PhoneCountry 字段：区号列无法落地")
	}
	if gotType != "string" {
		return fmt.Errorf("model.User.PhoneCountry 类型是 %q，期望 string", gotType)
	}
	pcTag, _ := smsStructTag(file, "User", "PhoneCountry")
	gorm := pcTag.Get("gorm")
	for _, want := range []string{"type:varchar(8)", "not null", "default:'86'"} {
		if !strings.Contains(gorm, want) {
			return fmt.Errorf("model.User.PhoneCountry 的 gorm tag 缺 %q：%q", want, gorm)
		}
	}
	phoneTag, ok := smsStructTag(file, "User", "Phone")
	if !ok {
		return fmt.Errorf("model.User 缺少 Phone 字段：检查器无法判定")
	}
	for idx, unique := range parseGormIndexes(phoneTag.Get("gorm")) {
		if unique {
			return fmt.Errorf("model.User.Phone 挂了唯一索引 %q：66 行空串会撞唯一约束，规格 §4.1 要求保持非唯一 idx_users_phone", idx)
		}
	}
	return nil
}

func TestUserPhoneCountryColumn(t *testing.T) {
	src, real := loadSmsUserSource(t, smsUserModelPath)
	if err := checkUserPhoneCountryColumn(real); err != nil {
		t.Fatal(err)
	}
	// 扫描器自检：字段 tag 必须真被读到，避免 smsStructTag 恒空让断言空转。
	if tag, ok := smsStructTag(real, "User", "Phone"); !ok || !strings.Contains(tag.Get("gorm"), "column:phone") {
		t.Fatal("扫描器失效：没读出 model.User.Phone 的 gorm tag")
	}
	t.Logf("[检查器] checkUserPhoneCountryColumn；变异锚点 = Phone 字段 tag 的 `;index` → `;uniqueIndex:uk_phone`")

	t.Run("反向对照_给Phone加uniqueIndex必须变红", func(t *testing.T) {
		badSrc := strings.Replace(src,
			"column:phone;type:varchar(20) not null;default:'';index",
			"column:phone;type:varchar(20) not null;default:'';uniqueIndex:uk_phone", 1)
		if badSrc == src {
			t.Fatal("反向对照锚点失效：Phone 的 index 声明没被替换")
		}
		if err := checkUserPhoneCountryColumn(parseSmsSynthetic(t, badSrc)); err == nil {
			t.Fatal("反向对照失效：把 Phone 标成 uniqueIndex 后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if err := checkUserPhoneCountryColumn(parseSmsSynthetic(t, src)); err != nil {
			t.Fatalf("恢复后同一个检查器仍报错：%v", err)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}

// ---- 检查器 2：RegisterUser 使用事务查重 ----

// checkRegisterUserUsesPhoneGuard 只检查 RegisterUser 的函数体（不搜全文件，
// 否则会被其它函数里合法的 w.DB.Save 干扰）：必须出现 createUserWithPhoneGuard 调用，
// 且不得出现任何 .Save 选择器调用（原 w.DB.Save(&user) 已被 guard 取代）。
func checkRegisterUserUsesPhoneGuard(file *ast.File) error {
	var body *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "RegisterUser" || fn.Body == nil || fn.Recv == nil {
			continue
		}
		body = fn.Body
		break
	}
	if body == nil {
		return fmt.Errorf("源码里找不到 RegisterUser 函数体：检查器无法判定")
	}
	hasGuard, hasSave := false, false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fun.Sel.Name == "createUserWithPhoneGuard" {
				hasGuard = true
			}
			if fun.Sel.Name == "Save" {
				hasSave = true
			}
		case *ast.Ident:
			if fun.Name == "createUserWithPhoneGuard" {
				hasGuard = true
			}
		}
		return true
	})
	if !hasGuard {
		return fmt.Errorf("RegisterUser 函数体内没有 createUserWithPhoneGuard 调用：手机号查重仍是事务外裸读后写")
	}
	if hasSave {
		return fmt.Errorf("RegisterUser 函数体内仍有裸 .Save 调用：用户写入未收进事务")
	}
	return nil
}

func TestRegisterUserUsesPhoneGuard(t *testing.T) {
	src, real := loadSmsUserSource(t, smsUserProviderPath)
	if err := checkRegisterUserUsesPhoneGuard(real); err != nil {
		t.Fatal(err)
	}
	t.Logf("[检查器] checkRegisterUserUsesPhoneGuard；变异锚点 = `w.createUserWithPhoneGuard(&user)` → `w.DB.Save(&user)`")

	t.Run("反向对照_仍用裸Save必须变红", func(t *testing.T) {
		badSrc := strings.Replace(src,
			"w.createUserWithPhoneGuard(&user)", "w.DB.Save(&user)", 1)
		if badSrc == src {
			t.Fatal("反向对照锚点失效：guard 调用没被替换")
		}
		if err := checkRegisterUserUsesPhoneGuard(parseSmsSynthetic(t, badSrc)); err == nil {
			t.Fatal("反向对照失效：改回裸 w.DB.Save 后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if err := checkRegisterUserUsesPhoneGuard(parseSmsSynthetic(t, src)); err != nil {
			t.Fatalf("恢复后同一个检查器仍报错：%v", err)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}

// ---- 检查器 3：VerifyCellphoneFormat 正则不得收紧 ----

// extractCellphonePattern 从 VerifyCellphoneFormat 函数体里读出 pattern 字符串字面量。
// 锚点字符串一律从文件真实文本读出，不在测试里凭印象写死正则字面量。
func extractCellphonePattern(file *ast.File) (string, error) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "VerifyCellphoneFormat" || fn.Body == nil {
			continue
		}
		for _, stmt := range fn.Body.List {
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok {
				continue
			}
			for i, lhs := range assign.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != "pattern" || i >= len(assign.Rhs) {
					continue
				}
				lit, ok := assign.Rhs[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				val, err := strconv.Unquote(lit.Value)
				if err != nil {
					return "", fmt.Errorf("无法解析 pattern 字面量：%v", err)
				}
				return val, nil
			}
		}
		return "", fmt.Errorf("VerifyCellphoneFormat 函数体里没找到 pattern 字符串字面量：扫描器失效")
	}
	return "", fmt.Errorf("源码里找不到 VerifyCellphoneFormat 函数体：扫描器失效")
}

func checkVerifyCellphoneFormat(file *ast.File) (string, error) {
	pattern, err := extractCellphonePattern(file)
	if err != nil {
		return "", err
	}
	if pattern == "" {
		return "", fmt.Errorf("读出的 pattern 为空：扫描器失效（恒空会让断言空转）")
	}
	if strings.ContainsAny(pattern, "^$") {
		return pattern, fmt.Errorf("正则被加了 ^/$ 锚点：%q（会让存量带前缀账号无法登录，新校验只用于短信路径）", pattern)
	}
	return pattern, nil
}

func TestVerifyCellphoneFormatUnchanged(t *testing.T) {
	src, real := loadSmsUserSource(t, smsUserProviderPath)
	pattern, err := checkVerifyCellphoneFormat(real)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("从 provider/user.go 真实文本读出的正则字面量 = %q（无 ^$ 锚点）", pattern)
	t.Logf("[检查器] checkVerifyCellphoneFormat；变异锚点 = 把读出的 pattern 字面量动态改成 `^<pattern>$`（不写死正则）")

	t.Run("反向对照_加^$锚点必须变红", func(t *testing.T) {
		badSrc := strings.Replace(src, "`"+pattern+"`", "`^"+pattern+"$`", 1)
		if badSrc == src {
			t.Fatal("反向对照锚点失效：pattern 字面量没被替换")
		}
		if _, err := checkVerifyCellphoneFormat(parseSmsSynthetic(t, badSrc)); err == nil {
			t.Fatal("反向对照失效：加了 ^...$ 锚点后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if _, err := checkVerifyCellphoneFormat(parseSmsSynthetic(t, src)); err != nil {
			t.Fatalf("恢复后同一个检查器仍报错：%v", err)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}

// ---- 真库测试（R52 / R53） ----

// smsUserTestDB 连接开发库。config.Server.Mysql 由 config 包 init() 从仓库根 config.json
// 加载；测试进程须在仓库根工作目录下启动 go test，initPath() 才能推导到正确的 config.json。
// 源码与报告都不写任何凭据。
func smsUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if config.Server.Mysql.Database == "" {
		t.Fatal("config.Server.Mysql.Database 为空：请在仓库根工作目录下运行 go test，确保能读到 config.json")
	}
	db, err := InitDB(&config.Server.Mysql)
	if err != nil {
		t.Fatalf("连接开发库失败：%v", err)
	}
	return db
}

// smsUserTestWebsite 构造只带 DB 的最小 Website。预热 w.Tr 以避开并发用例里
// I18n 懒加载的首次初始化竞争；createUserWithPhoneGuard 命中重复时会调 w.Tr。
func smsUserTestWebsite(db *gorm.DB) *Website {
	w := &Website{DB: db}
	_ = w.Tr("TheMobilePhoneNumberHasBeenRegistered")
	return w
}

// smsUserEnsureColumn 幂等确保 users 有 phone_country 列（规格 §4.1 已批准，R53）。
// 只 ADD COLUMN，绝不改/删既有列或索引；存量行由 MySQL 用 DEFAULT '86' 自动回填。
func smsUserEnsureColumn(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasColumn(&model.User{}, "phone_country") {
		t.Log("[R53 DDL] phone_country 列已存在，跳过 ADD COLUMN（幂等）")
		return
	}
	ddl := "ALTER TABLE `users` ADD COLUMN `phone_country` VARCHAR(8) NOT NULL DEFAULT '86'"
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("[R53 DDL] ADD COLUMN 失败：%v", err)
	}
	t.Logf("[R53 DDL] 执行：%s（存量 339 行由 DEFAULT '86' 自动回填）", ddl)
}

// smsUserRandomPhone 生成不可能撞真号的测试专用号：199 + 纳秒时间戳低 13 位，
// 固定 16 位，落在 phone varchar(20) 内。
func smsUserRandomPhone() string {
	return fmt.Sprintf("199%013d", time.Now().UnixNano()%10000000000000)
}

// smsUserCleanup 精确清理本测试创建的行并用只读 SQL 实测总行数（R53）。
// ids：所有成功插入的用户 ID（按 id 精确删）；
// fallbackPhones：非空的测试专用随机号，兜底删除（覆盖并发用例中未收集到的行）。
// 空号绝不按值兜底删除——库里本来就有 66 行真实空号，按 phone=” 删会误删（R53②）。
func smsUserCleanup(t *testing.T, db *gorm.DB, ids []uint, fallbackPhones []string) {
	t.Helper()
	for _, id := range ids {
		if id != 0 {
			db.Exec("DELETE FROM `users` WHERE `id` = ?", id)
		}
	}
	for _, p := range fallbackPhones {
		if p != "" {
			db.Exec("DELETE FROM `users` WHERE `phone` = ?", p)
		}
	}
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM `users`").Row().Scan(&n); err != nil {
		t.Logf("[R53 探针] SELECT COUNT(*) 失败：%v", err)
		return
	}
	t.Logf("[R53 探针] 清理后 users 总行数 = %d（基线 339）", n)
}

// TestCreateUserWithPhoneGuard 三条子用例，每条自带清理：
// ① 同 (country, phone) 插两次 → 第二次 error，该号仍 1 行；
// ② 同 phone 不同 country（86 vs 852）→ 两次成功，2 行；
// ③ phone == "" → 连插 3 个全部成功（空号不参与 FOR UPDATE 查重）。
func TestCreateUserWithPhoneGuard(t *testing.T) {
	db := smsUserTestDB(t)
	smsUserEnsureColumn(t, db)
	w := smsUserTestWebsite(db)

	t.Run("① 同(country,phone)插两次_第二次报错且该号仅1行", func(t *testing.T) {
		phone := smsUserRandomPhone()
		var ids []uint
		defer func() { smsUserCleanup(t, db, ids, []string{phone}) }()

		u1 := &model.User{UserName: "sms_guard_1a", Phone: phone, PhoneCountry: "86"}
		if err := w.createUserWithPhoneGuard(u1); err != nil {
			t.Fatalf("第一次插入应成功，却报错：%v", err)
		}
		ids = append(ids, u1.Id)

		u2 := &model.User{UserName: "sms_guard_1b", Phone: phone, PhoneCountry: "86"}
		err := w.createUserWithPhoneGuard(u2)
		if err == nil {
			t.Fatal("同 (86, phone) 第二次插入应返回 error（查重命中），却成功")
		}
		if u2.Id != 0 {
			ids = append(ids, u2.Id)
		}
		var cnt int64
		db.Raw("SELECT COUNT(*) FROM `users` WHERE `phone` = ? AND `phone_country` = ?", phone, "86").Row().Scan(&cnt)
		if cnt != 1 {
			t.Fatalf("该 (country,phone) 应有且仅有 1 行，实际 %d", cnt)
		}
		t.Logf("① 通过：第二次返回 error=%v，表中该号 %d 行", err, cnt)
	})

	t.Run("② 同phone不同country_两次成功共2行", func(t *testing.T) {
		phone := smsUserRandomPhone()
		var ids []uint
		defer func() { smsUserCleanup(t, db, ids, []string{phone}) }()

		for i, country := range []string{"86", "852"} {
			u := &model.User{UserName: fmt.Sprintf("sms_guard_2_%d", i), Phone: phone, PhoneCountry: country}
			if err := w.createUserWithPhoneGuard(u); err != nil {
				t.Fatalf("country=%s 插入应成功，却报错：%v", country, err)
			}
			ids = append(ids, u.Id)
		}
		var cnt int64
		db.Raw("SELECT COUNT(*) FROM `users` WHERE `phone` = ?", phone).Row().Scan(&cnt)
		if cnt != 2 {
			t.Fatalf("同 phone 不同 country 应有 2 行，实际 %d", cnt)
		}
		t.Logf("② 通过：86 与 852 各 1 行，共 %d 行", cnt)
	})

	t.Run("③ 空号连续插3个全部成功", func(t *testing.T) {
		var ids []uint
		// 空号只按 id 清理，不传兜底号，避免误删库里 66 行真实空号。
		defer func() { smsUserCleanup(t, db, ids, nil) }()

		for i := 0; i < 3; i++ {
			u := &model.User{UserName: fmt.Sprintf("sms_guard_3_%d", i), Phone: "", PhoneCountry: "86"}
			if err := w.createUserWithPhoneGuard(u); err != nil {
				t.Fatalf("第 %d 个空号插入应成功，却报错：%v", i+1, err)
			}
			ids = append(ids, u.Id)
		}
		t.Logf("③ 通过：3 个空号全部插入成功，ids=%v", ids)
	})

	// ④ 回归（R56）：调用方不给区号（模拟 RegisterUser 的 ApiRegisterRequest 无区号字段），
	// 依赖 createUserWithPhoneGuard 内部的空值规范化把区号补成 86，堵住同号无限重复注册。
	t.Run("④ PhoneCountry为空时同号第二次报错且落库区号为86", func(t *testing.T) {
		phone := smsUserRandomPhone()
		var ids []uint
		defer func() { smsUserCleanup(t, db, ids, []string{phone}) }()

		u1 := &model.User{UserName: "sms_guard_4a", Phone: phone}
		if err := w.createUserWithPhoneGuard(u1); err != nil {
			t.Fatalf("第一次插入应成功，却报错：%v", err)
		}
		ids = append(ids, u1.Id)

		u2 := &model.User{UserName: "sms_guard_4b", Phone: phone}
		err := w.createUserWithPhoneGuard(u2)
		if err == nil {
			t.Fatal("调用方未给区号时，同号第二次插入应返回 error（规范化后区号均为 86），却成功")
		}
		if u2.Id != 0 {
			ids = append(ids, u2.Id)
		}

		var cnt int64
		db.Raw("SELECT COUNT(*) FROM `users` WHERE `phone` = ?", phone).Row().Scan(&cnt)
		if cnt != 1 {
			t.Fatalf("该号应仅有 1 行，实际 %d", cnt)
		}
		var got string
		if scanErr := db.Raw("SELECT `phone_country` FROM `users` WHERE `phone` = ?", phone).Row().Scan(&got); scanErr != nil {
			t.Fatalf("读回 phone_country 失败：%v", scanErr)
		}
		if got != "86" {
			t.Fatalf("插入行 phone_country 应为规范化后的 86，实际 %q", got)
		}
		t.Logf("④ 通过：第二次返回 error=%v，表中该号 %d 行，落库区号 = %q", err, cnt, got)
	})
}

// TestCreateUserWithPhoneGuardConcurrent：10 个 goroutine 同时用同一 (country, phone)
// 调 createUserWithPhoneGuard → 恰好 1 个成功、表里 1 行。这条是应用层查重的存在理由，
// 跑在实测为 REPEATABLE-READ 的开发库上（依赖 RR 的 next-key / gap lock）。
func TestCreateUserWithPhoneGuardConcurrent(t *testing.T) {
	db := smsUserTestDB(t)
	smsUserEnsureColumn(t, db)
	w := smsUserTestWebsite(db)

	phone := smsUserRandomPhone()
	var (
		mu      sync.Mutex
		ids     []uint
		success int
		failed  int
	)
	defer func() { smsUserCleanup(t, db, ids, []string{phone}) }()

	const goroutines = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			u := &model.User{UserName: fmt.Sprintf("sms_guard_conc_%d", i), Phone: phone, PhoneCountry: "86"}
			err := w.createUserWithPhoneGuard(u)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				ids = append(ids, u.Id)
			} else {
				failed++
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if success != 1 {
		t.Fatalf("10 个并发 goroutine 应恰好 1 个成功，实际成功 %d、失败 %d（RR next-key lock 保证）", success, failed)
	}
	var cnt int64
	db.Raw("SELECT COUNT(*) FROM `users` WHERE `phone` = ? AND `phone_country` = ?", phone, "86").Row().Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("并发后该 (country,phone) 应恰好 1 行，实际 %d", cnt)
	}
	t.Logf("并发测试通过：10 goroutine → 成功 %d / 失败 %d，表中该号 %d 行（依赖 REPEATABLE-READ next-key lock）", success, failed, cnt)
}
