package provider

import (
	"database/sql"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

// 旧站升级不会建出短信表（AutoMigrateDB 只在版本变化时跑）。
// 本文件验证 ensureSmsSchema 不依赖版本号、可重复执行，并且能把被删掉的
// 表/列补回来。删除动作一律用 RENAME 到备份名再恢复，避免毁掉共享开发库的数据。
//
// 反向对照：
//   - 删掉 InitModelData 里的 `_ = w.ensureSmsSchema()` → TestSmsSchemaWiredIntoInitModelData 变红；
//   - 删掉 ensureSmsSchema 里的建表/加列步骤 → Recreates/Adds 两个用例变红。

func TestSmsSchemaEnsureIdempotent(t *testing.T) {
	w := newZeroValueTestSite(t)
	for i := 0; i < 2; i++ {
		if err := w.ensureSmsSchema(); err != nil {
			t.Fatalf("第 %d 次 ensureSmsSchema 失败：%v", i+1, err)
		}
	}
	if !w.DB.Migrator().HasTable(&model.SmsCode{}) {
		t.Fatal("sms_codes 表不存在")
	}
	if !w.DB.Migrator().HasTable(&model.SmsLog{}) {
		t.Fatal("sms_logs 表不存在")
	}
	if !w.DB.Migrator().HasColumn(&model.User{}, "phone_country") {
		t.Fatal("users.phone_country 列不存在")
	}
}

// TestSmsSchemaWiredIntoInitModelData 防止「函数写好了但没接进启动流程」：
// ensureSmsSchema 只有被 InitModelData 每次调用，才能覆盖「版本号没变」的老站升级。
func TestSmsSchemaWiredIntoInitModelData(t *testing.T) {
	src, err := os.ReadFile("dao.go")
	if err != nil {
		t.Fatalf("读取 dao.go 失败：%v", err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "dao.go", src, 0)
	if err != nil {
		t.Fatalf("解析 dao.go 失败：%v", err)
	}
	var body *ast.BlockStmt
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "InitModelData" && fn.Body != nil {
			body = fn.Body
			break
		}
	}
	if body == nil {
		t.Fatal("dao.go 里找不到 InitModelData 函数体")
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ensureSmsSchema" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("InitModelData 未调用 ensureSmsSchema：老站升级仍然不会建出短信表")
	}
}

func TestSmsSchemaRecreatesDroppedTable(t *testing.T) {
	w := newZeroValueTestSite(t)
	if err := w.ensureSmsSchema(); err != nil {
		t.Fatalf("准备阶段 ensureSmsSchema 失败：%v", err)
	}
	if !w.DB.Migrator().HasTable(&model.SmsCode{}) {
		t.Skip("环境里没有 sms_codes 表，跳过重建用例")
	}

	bak := fmt.Sprintf("sms_codes_bak_%d", time.Now().UnixNano())
	if err := w.DB.Exec("RENAME TABLE `sms_codes` TO `" + bak + "`").Error; err != nil {
		t.Fatalf("重命名 sms_codes 失败：%v", err)
	}
	t.Cleanup(func() {
		// 无条件恢复：先删掉 ensure 新建的空表，再把备份改回来。
		w.DB.Exec("DROP TABLE IF EXISTS `sms_codes`")
		if err := w.DB.Exec("RENAME TABLE `" + bak + "` TO `sms_codes`").Error; err != nil {
			t.Errorf("恢复 sms_codes 失败：%v", err)
		}
		if !w.DB.Migrator().HasTable(&model.SmsCode{}) {
			t.Errorf("恢复后 sms_codes 仍不存在")
		}
	})

	if w.DB.Migrator().HasTable(&model.SmsCode{}) {
		t.Fatal("重命名后 sms_codes 仍存在，前置条件不成立")
	}
	if err := w.ensureSmsSchema(); err != nil {
		t.Fatalf("ensureSmsSchema 失败：%v", err)
	}
	if !w.DB.Migrator().HasTable(&model.SmsCode{}) {
		t.Fatal("ensureSmsSchema 未重建 sms_codes（旧站升级会报 1146）")
	}
}

func TestSmsSchemaAddsPhoneCountry(t *testing.T) {
	w := newZeroValueTestSite(t)
	if err := w.ensureSmsSchema(); err != nil {
		t.Fatalf("准备阶段 ensureSmsSchema 失败：%v", err)
	}
	if !w.DB.Migrator().HasColumn(&model.User{}, "phone_country") {
		t.Skip("环境里没有 users.phone_country 列，跳过补列用例")
	}

	bak := fmt.Sprintf("phone_country_bak_%d", time.Now().UnixNano())
	if err := w.DB.Exec("ALTER TABLE `users` RENAME COLUMN `phone_country` TO `" + bak + "`").Error; err != nil {
		t.Fatalf("重命名 phone_country 失败：%v", err)
	}
	t.Cleanup(func() {
		w.DB.Exec("ALTER TABLE `users` DROP COLUMN `phone_country`")
		if err := w.DB.Exec("ALTER TABLE `users` RENAME COLUMN `" + bak + "` TO `phone_country`").Error; err != nil {
			t.Errorf("恢复 phone_country 失败：%v", err)
		}
		if !w.DB.Migrator().HasColumn(&model.User{}, "phone_country") {
			t.Errorf("恢复后 users.phone_country 仍不存在")
		}
	})

	if w.DB.Migrator().HasColumn(&model.User{}, "phone_country") {
		t.Fatal("重命名后 phone_country 仍存在，前置条件不成立")
	}
	if err := w.ensureSmsSchema(); err != nil {
		t.Fatalf("ensureSmsSchema 失败：%v", err)
	}
	if !w.DB.Migrator().HasColumn(&model.User{}, "phone_country") {
		t.Fatal("ensureSmsSchema 未加回 users.phone_country（旧站注册会报 1054）")
	}

	// 列定义必须与 model.User 的 gorm tag 逐字一致：varchar(8) / NOT NULL / DEFAULT '86'。
	// 断言 '86' 而不是空串，是因为模型 tag 就是 default:'86'，而新装站由
	// AutoMigrateDB 按 tag 建列 —— 两条路径（新装 / 老站升级）必须长出同一个形态，
	// 否则按 COLUMN_DEFAULT 判定的逻辑会二义（2026-11-15 实测：这里写 ''、
	// sms_user_test.go 断言 '86'，两条用例互相打架）。
	// 空串与 '86' 在业务上等价（normalizePhoneCountry 把两者都归一成 86），
	// 「未绑定手机号」不会被误判成 86 号码持有者。
	var def sql.NullString
	if err := w.DB.Raw(
		"SELECT COLUMN_DEFAULT FROM information_schema.COLUMNS " +
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'phone_country'").
		Row().Scan(&def); err != nil {
		t.Fatalf("读取 phone_country 默认值失败：%v", err)
	}
	// MariaDB 的 information_schema 把字符串默认值回成带单引号的 `'86'`，
	// 部分版本/驱动会回成裸的 `86`；两者都表示 86，只有空串或其他值才是错的。
	if !def.Valid || (def.String != "86" && def.String != "'86'") {
		t.Fatalf("phone_country 默认值必须与模型 tag 一致为 \"86\"，实际 %q（valid=%v）", def.String, def.Valid)
	}
	var colType string
	if err := w.DB.Raw(
		"SELECT COLUMN_TYPE FROM information_schema.COLUMNS " +
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'phone_country'").
		Row().Scan(&colType); err != nil {
		t.Fatalf("读取 phone_country 类型失败：%v", err)
	}
	if colType != "varchar(8)" {
		t.Fatalf("phone_country 类型必须与模型 tag 一致为 varchar(8)，实际 %q", colType)
	}
}
