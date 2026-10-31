package provider

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

// gormColumnName 从 gorm tag 里取列名，取不到返回空串。
// tag 形如 `column:title;type:varchar(190)...`，也可能只有 `-`。
func gormColumnName(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	const prefix = "column:"
	for i := 0; i+len(prefix) <= len(tag); i++ {
		if tag[i:i+len(prefix)] != prefix {
			continue
		}
		name := tag[i+len(prefix):]
		if k := indexByte(name, ';'); k >= 0 {
			name = name[:k]
		}
		return name
	}
	return ""
}

// TestArchiveObsoleteColumnsNotMappedByModel 防的是最贵的一种回归：
// 有人觉得某列"反正没人用"就塞进 DROP 名单，而 model 里它其实还在映射。
//
// 后果不是报错而是灾难：ARCHIVE 一启动就 ALTER TABLE DROP COLUMN，然后
// 每一次 SELECT * 都 Unknown column，整站打不开，且 DROP 不可回滚。
// 唯一放行的情形是字段的 gorm tag 明确是 "-"（数据存在侧表或纯计算字段）。
func TestArchiveObsoleteColumnsNotMappedByModel(t *testing.T) {
	typ := reflect.TypeOf(model.Archive{})
	mapped := map[string]string{}
	for i := 0; i < typ.NumField(); i++ {
		col := gormColumnName(typ.Field(i).Tag.Get("gorm"))
		if col != "" {
			mapped[col] = typ.Field(i).Name
		}
	}
	if len(mapped) == 0 {
		t.Fatal("解析不出 model.Archive 的列映射——护栏本身失效了")
	}

	for _, col := range archiveObsoleteColumns {
		if field, ok := mapped[col]; ok {
			t.Errorf("列 %q 仍在 model.Archive.%s 上映射（gorm tag 不是 \"-\"），删掉它会让每次查询 Unknown column",
				col, field)
		}
	}

	// 反向：名单里不能是空串/重复，重复说明有人手抖粘贴两遍。
	seen := map[string]bool{}
	for _, col := range archiveObsoleteColumns {
		if col == "" {
			t.Error("archiveObsoleteColumns 含空串")
		}
		if seen[col] {
			t.Errorf("archiveObsoleteColumns 重复列 %q：ALTER TABLE 会因重复 DROP 报错", col)
		}
		seen[col] = true
	}
}

// TestArchiveRedundantIndexesAreRealPrefixes 保证冗余索引名单的自洽性。
func TestArchiveRedundantIndexesAreRealPrefixes(t *testing.T) {
	// 名单里的 drop 必须能被 keep 的前缀覆盖——用构造的列序列验证判据函数，
	// 而不是连库（连库版本见 TestArchiveSlimSchemaMigration）。
	cases := []struct {
		a, b []string
		want bool
	}{
		{[]string{"url_token"}, []string{"url_token"}, true},
		{[]string{"category_id"}, []string{"category_id", "id"}, true},
		{[]string{"category_id", "id"}, []string{"category_id"}, false},
		{[]string{"created_time"}, []string{"updated_time"}, false},
		{nil, []string{"id"}, false},
		{[]string{"id"}, nil, false},
	}
	for _, c := range cases {
		if got := isIndexPrefix(c.a, c.b); got != c.want {
			t.Errorf("isIndexPrefix(%v, %v) = %v，期望 %v", c.a, c.b, got, c.want)
		}
	}

	for _, pair := range archiveRedundantIndexes {
		if pair.drop == "" || pair.keep == "" {
			t.Errorf("冗余索引名单有空项：%+v", pair)
		}
		if pair.drop == pair.keep {
			t.Errorf("冗余索引 %s 的 drop 与 keep 相同", pair.drop)
		}
	}
}

// slimTestColumnTypes 给"拆表前"的列指定贴近真实的类型。
//
// 不能全用 varchar：回填 SQL 里的 IFNULL(col,0) 与 `col <> 0` 判断依赖类型，
// 全按字符串造会让「属性是否非默认」的判断失真，测不出真实行为。
var slimTestColumnTypes = map[string]string{
	"need_logistics":   "tinyint(1) NOT NULL DEFAULT 0",
	"is_free_shipping": "tinyint(1) NOT NULL DEFAULT 0",
	"allow_oversold":   "tinyint(1) NOT NULL DEFAULT 0",
	"option_type":      "int(10) NOT NULL DEFAULT 0",
	"product_type":     "int(10) NOT NULL DEFAULT 1",
	"min_quantity":     "int(10) NOT NULL DEFAULT 0",
	"max_quantity":     "int(10) NOT NULL DEFAULT 0",
	"step_quantity":    "int(10) NOT NULL DEFAULT 0",
	"sold_count":       "bigint(20) NOT NULL DEFAULT 0",
	"review_count":     "bigint(20) NOT NULL DEFAULT 0",
	"origin_price":     "bigint(20) NOT NULL DEFAULT 0",
	"cost_price":       "bigint(20) NOT NULL DEFAULT 0",
	"average_score":    "float NOT NULL DEFAULT 0",
	"weight":           "double NOT NULL DEFAULT 0",
	"length":           "double NOT NULL DEFAULT 0",
	"width":            "double NOT NULL DEFAULT 0",
	"height":           "double NOT NULL DEFAULT 0",
	"volume":           "double NOT NULL DEFAULT 0",
	"order_fields":     "text DEFAULT NULL",
	// 真实库里 weight_unit / length_unit 是 default null，回填要靠 IFNULL 兜住，
	// 否则 Error 1048 让整个升级中断。
	"weight_unit":  "varchar(20) DEFAULT NULL",
	"length_unit":  "varchar(20) DEFAULT NULL",
	"deleted_time": "bigint(20) NOT NULL DEFAULT 0",
}

// testSlimSchemaDDL 造一张"拆表前"的 archives / archive_drafts。
//
// 刻意不复用真实库的 CREATE TABLE ... LIKE：测试库本身也会被迁移，
// 一旦 anqitest 已经删过列，LIKE 出来的表就没有商品列，迁移测试会退化成
// "什么都没删"却照样通过——夹具必须自洽。
func testSlimSchemaDDL(table string) string {
	cols := []string{
		"`id` bigint(20) NOT NULL AUTO_INCREMENT",
		"`parent_id` bigint(20) NOT NULL DEFAULT 0",
		"`created_time` bigint(20) NOT NULL DEFAULT 0",
		"`updated_time` bigint(20) NOT NULL DEFAULT 0",
		"`title` varchar(190) NOT NULL DEFAULT ''",
		"`url_token` varchar(190) NOT NULL DEFAULT ''",
		"`fixed_link` varchar(190) DEFAULT NULL",
		"`category_id` int(10) unsigned NOT NULL DEFAULT 0",
		"`module_id` int(10) unsigned NOT NULL DEFAULT 1",
		"`stock` bigint(20) NOT NULL DEFAULT 9999999",
		"`price` bigint(20) NOT NULL DEFAULT 0",
	}
	// 23 个商品列 + 6 个死列：类型按需精确，未登记的走 varchar 兜底。
	for _, col := range archiveObsoleteColumns {
		typ, ok := slimTestColumnTypes[col]
		if !ok {
			typ = "varchar(255) DEFAULT NULL"
		}
		cols = append(cols, "`"+col+"` "+typ)
	}
	cols = append(cols, "PRIMARY KEY (`id`)")
	return fmt.Sprintf("CREATE TABLE `%s` (%s) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4", table, joinStrings(cols, ", "))
}

func joinStrings(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}

// slimSchemaBaseSite 缓存站点初始化结果。
//
// 每个迁移测试都要一个"已连上真实库的 Website"来执行 CREATE DATABASE，
// 而 InitWebsite 会跑一遍完整 AutoMigrate（本机 anqitest 上万行，十几秒）。
// 不缓存的话 5 个测试就是 5 次全量迁移，测试从 20 秒变 3 分钟。
var (
	slimSchemaBaseOnce sync.Once
	slimSchemaBase     *Website
)

func slimSchemaBaseSite(t *testing.T) *Website {
	t.Helper()
	slimSchemaBaseOnce.Do(func() {
		dbSite, err := GetDBWebsiteInfo(1)
		if err != nil {
			return
		}
		InitWebsite(dbSite)
		w := GetWebsite(1)
		if w == nil || w.DB == nil {
			return
		}
		w.Cache = library.InitMemoryCache()
		slimSchemaBase = w
	})
	if slimSchemaBase == nil {
		t.Skip("本地无可用站点库，跳过")
	}
	return slimSchemaBase
}

// newSlimSchemaTestSite 建一个只含 archives / archive_drafts / settings 的临时库，
// 返回连到该库的站点。销毁由调用方负责（t.Cleanup 里 DROP DATABASE）。
func newSlimSchemaTestSite(t *testing.T) (*Website, string) {
	t.Helper()
	base := slimSchemaBaseSite(t)

	tmpDB := fmt.Sprintf("anqi_slim_test_%d", time.Now().UnixNano()%100000000)
	if err := base.DB.Exec("CREATE DATABASE `" + tmpDB + "` DEFAULT CHARACTER SET utf8mb4").Error; err != nil {
		t.Skipf("无法创建临时库（跳过）：%v", err)
	}
	t.Cleanup(func() {
		_ = base.DB.Exec("DROP DATABASE IF EXISTS `" + tmpDB + "`").Error
	})

	db, err := InitDB(&config.MysqlConfig{
		Host: base.Mysql.Host, Port: base.Mysql.Port,
		User: base.Mysql.User, Password: base.Mysql.Password,
		Database: tmpDB,
	})
	if err != nil {
		t.Fatalf("连接临时库失败：%v", err)
	}

	w := &Website{
		DB:       db,
		Mysql:    &config.MysqlConfig{Database: tmpDB},
		Cache:    base.Cache,
		Id:       base.Id,
		RootPath: base.RootPath,
	}
	for _, table := range archiveSlimTables {
		if err := db.Exec(testSlimSchemaDDL(table)).Error; err != nil {
			t.Fatalf("建表 %s 失败：%v", table, err)
		}
	}
	// settings 表：GetSettingValue / SaveSettingValue 的落点。
	if err := db.Exec("CREATE TABLE `settings` (`id` bigint NOT NULL AUTO_INCREMENT, " +
		"`key` varchar(100) NOT NULL DEFAULT '', `value` text, PRIMARY KEY (`id`)) ENGINE=InnoDB").Error; err != nil {
		t.Fatalf("建 settings 表失败：%v", err)
	}
	return w, tmpDB
}

// TestArchiveSlimSchemaMigration 在临时库上真跑一次 3.8.0 迁移。
func TestArchiveSlimSchemaMigration(t *testing.T) {
	w, _ := newSlimSchemaTestSite(t)

	// 造索引：3 组冗余 + 1 个错建的 idx_updated_time。
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_created_time` (`created_time`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_updated_time` (`created_time`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_archives_url_token` (`url_token`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_articles_url_token` (`url_token`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_archives_fixed_link` (`fixed_link`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_fixed_link` (`fixed_link`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_category_archive_id` (`category_id`, `id`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_category_id` (`category_id`)")

	// 造数据：迁移不能动行数和正文。
	mustExec(t, w.DB, "INSERT INTO `archives` (`id`,`title`,`url_token`,`category_id`,`stock`,`brand`) VALUES (1,'文章一','a1',3,9999999,'')")
	mustExec(t, w.DB, "INSERT INTO `archives` (`id`,`title`,`url_token`,`category_id`,`stock`,`brand`) VALUES (2,'商品二','a2',4,88,'品牌X')")
	mustExec(t, w.DB, "INSERT INTO `archive_drafts` (`id`,`title`,`url_token`) VALUES (3,'草稿三','a3')")

	if err := w.ensureArchiveSlimSchema(); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}

	for _, table := range archiveSlimTables {
		left := columnsIn(w.DB, table)
		for _, col := range archiveObsoleteColumns {
			if left[col] {
				t.Errorf("%s 里的 %q 没被删掉", table, col)
			}
		}
		// 保留列不能误删。
		for _, col := range []string{"id", "title", "url_token", "created_time", "updated_time", "category_id", "stock", "price"} {
			if !left[col] {
				t.Errorf("%s 的保留列 %q 被误删了", table, col)
			}
		}
	}

	var title string
	if err := w.DB.Raw("SELECT `title` FROM `archives` WHERE `id` = 2").Row().Scan(&title); err != nil {
		t.Fatalf("迁移后读不到数据：%v", err)
	}
	if title != "商品二" {
		t.Errorf("迁移后行内容变了：%q", title)
	}
	var cnt int64
	w.DB.Raw("SELECT COUNT(*) FROM `archives`").Row().Scan(&cnt)
	if cnt != 2 {
		t.Errorf("迁移后行数变了：期望 2，实际 %d", cnt)
	}

	// 冗余索引消失。
	for _, idx := range []string{"idx_articles_url_token", "idx_fixed_link", "idx_category_id"} {
		if w.DB.Migrator().HasIndex("archives", idx) {
			t.Errorf("冗余索引 %s 没被删掉", idx)
		}
	}
	// 保留索引还在。
	for _, idx := range []string{"idx_archives_url_token", "idx_archives_fixed_link", "idx_category_archive_id", "idx_created_time"} {
		if !w.DB.Migrator().HasIndex("archives", idx) {
			t.Errorf("该保留的索引 %s 被误删了", idx)
		}
	}
	// idx_updated_time 修好了：列是 updated_time 而不是 created_time。
	if !w.DB.Migrator().HasIndex("archives", "idx_updated_time") {
		t.Error("idx_updated_time 没有被重建，updated_time 会退化成全表扫描")
	} else if cols := w.indexColumns("archives", "idx_updated_time"); !sameIndexColumns(cols, []string{"updated_time"}) {
		t.Errorf("idx_updated_time 的列还是错的：%v", cols)
	}

	// 标记已写，第二次是空操作。
	if w.GetSettingValue("upgrade_archive_slim_380") == "" {
		t.Error("迁移标记没写，下次启动会重复跑 ALTER")
	}
	if err := w.ensureArchiveSlimSchema(); err != nil {
		t.Errorf("第二次执行应该幂等，实际报错：%v", err)
	}
}

// TestArchiveSlimSchemaKeepsPrefixOnlyIndex 验证"同名索引列不对就不删"。
//
// 真实场景：archive_drafts 的 idx_updated_time 索引的是 updated_time（正确），
// 而 archives 上错建成 created_time。只按名字删索引会把草稿表的正确索引删掉。
func TestArchiveSlimSchemaKeepsPrefixOnlyIndex(t *testing.T) {
	w, _ := newSlimSchemaTestSite(t)

	// archives 故意造一个"列不匹配"的 idx_fixed_link（索引的是 title），
	// 与 idx_archives_fixed_link（索引 fixed_link）不是前缀关系。
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_archives_fixed_link` (`fixed_link`)")
	mustExec(t, w.DB, "ALTER TABLE `archives` ADD INDEX `idx_fixed_link` (`title`)")

	if err := w.ensureArchiveSlimSchema(); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	if !w.DB.Migrator().HasIndex("archives", "idx_fixed_link") {
		t.Error("idx_fixed_link 与 idx_archives_fixed_link 列不同（title vs fixed_link），不该被当成冗余删掉")
	}
}

// TestArchiveSlimSchemaSkipsWhenColsAbsent 验证重复执行/新装库场景不报错。
func TestArchiveSlimSchemaSkipsWhenColsAbsent(t *testing.T) {
	w, _ := newSlimSchemaTestSite(t)
	if err := w.ensureArchiveSlimSchema(); err != nil {
		t.Fatalf("首次迁移失败：%v", err)
	}
	// 清掉标记模拟"标记丢了但列已删"。
	_ = w.DB.Exec("DELETE FROM `settings` WHERE `key` = ?", "upgrade_archive_slim_380")
	if err := w.ensureArchiveSlimSchema(); err != nil {
		t.Fatalf("列已不存在时应该安静跳过，实际报错：%v", err)
	}
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("执行失败：%s\n%v", sql, err)
	}
}

// columnsIn 返回表的列名集合。
//
// 用 information_schema 而不是 Migrator().ColumnTypes：后者要靠 Statement
// 解析出模型才能取列，传裸表名时对没注册的模型会静默返回空集合——断言
// 循环于是一个都不执行，测试全是假绿（写这条测试时踩过一次）。
func columnsIn(db *gorm.DB, table string) map[string]bool {
	out := map[string]bool{}
	rows, err := db.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS "+
		"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Rows()
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return out
		}
		out[name] = true
	}
	return out
}

// TestArchiveSlimWithBackfill 是"3.7.1 回填 + 3.8.0 删列"一步到位的端到端验证。
//
// 这是整次改造风险最高的地方：客户直接装 3.8.0 时，同一次启动里要先跑
// ensureArchiveProductTable（把 23 列搬进侧表）再跑 ensureArchiveSlimSchema
// （把 23 列删掉）。顺序颠倒、或者回填失败仍继续删列，商品属性就永久归零，
// 而接口照旧回成功——线上表现是"运费全 0、SKU 模式变无、品牌空白"。
func TestArchiveSlimWithBackfill(t *testing.T) {
	w, _ := newSlimSchemaTestSite(t)

	mustExec(t, w.DB, "CREATE TABLE `modules` (`id` int(10) unsigned NOT NULL AUTO_INCREMENT, "+
		"`table_name` varchar(100) NOT NULL DEFAULT '', `kind` varchar(20) NOT NULL DEFAULT 'article', "+
		"PRIMARY KEY (`id`)) ENGINE=InnoDB")
	mustExec(t, w.DB, "INSERT INTO `modules` (`id`,`table_name`,`kind`) VALUES (1,'article','article'),(2,'product','product')")

	// 商品：配过品牌/重量/SKU 模式。文章：一个商品属性都没有。
	mustExec(t, w.DB, "INSERT INTO `archives` (`id`,`title`,`module_id`,`brand`,`weight`,`weight_unit`,`option_type`) "+
		"VALUES (10,'真商品',2,'品牌A',1.5,'kg',2)")
	mustExec(t, w.DB, "INSERT INTO `archives` (`id`,`title`,`module_id`) VALUES (11,'普通文章',1)")

	// 走真实启动入口：回填 → 删列 → 注册读回调。
	// 刻意不分别调用两个子步骤，否则 initArchiveSchema 里的顺序被改反了
	// （先删列后回填）测试也照样绿——而那个顺序错误等于商品属性永久归零。
	w.initArchiveSchema()

	var productRows int64
	w.DB.Raw("SELECT COUNT(*) FROM `archive_products`").Row().Scan(&productRows)
	if productRows != 1 {
		t.Fatalf("回填应只给配过属性的商品建 1 行，实际 %d 行（顺序错了或者被跳过）", productRows)
	}
	if columnsIn(w.DB, "archives")["brand"] {
		t.Fatal("brand 列没被删掉")
	}

	// 删列之后，属性必须还能从侧表读回来
	var got model.Archive
	if err := w.DB.Where("`id` = ?", 10).Take(&got).Error; err != nil {
		t.Fatalf("删列后读取商品失败：%v", err)
	}
	if got.Brand != "品牌A" || got.Weight != 1.5 || got.WeightUnit != "kg" || got.OptionType != 2 {
		t.Errorf("删列后商品属性丢了：brand=%q weight=%v unit=%q option_type=%d",
			got.Brand, got.Weight, got.WeightUnit, got.OptionType)
	}
	// 列表读取（批量补全路径）同样要成立
	var list []*model.Archive
	if err := w.DB.Where("`id` IN ?", []int64{10, 11}).Find(&list).Error; err != nil {
		t.Fatalf("删列后列表读取失败：%v", err)
	}
	for _, a := range list {
		if a.Id == 10 && a.Brand != "品牌A" {
			t.Errorf("列表读取没补上商品属性：brand=%q", a.Brand)
		}
		if a.Id == 11 && a.Brand != "" {
			t.Errorf("文章不该有品牌：%q", a.Brand)
		}
	}
}

// TestArchiveSlimAbortsWhenBackfillFails 验证"回填失败不许删列"这条闸门。
//
// 用一张没有 modules 表的库模拟回填失败（回填 SQL 会因 Unknown table 报错），
// 然后走真正的启动入口 initArchiveSchema。列必须原样保留——宁可升级卡住
// （标记没写，下次启动重试），也不能把唯一的源数据删掉。
func TestArchiveSlimAbortsWhenBackfillFails(t *testing.T) {
	w, _ := newSlimSchemaTestSite(t)
	// 不建 modules 表 → ensureArchiveProductTable 的回填必然失败
	if err := w.ensureArchiveProductTable(); err == nil {
		t.Fatal("modules 表不存在时回填应该报错，实际返回 nil——测试前提不成立")
	}

	// 走真实入口：回填失败 → 不许进入删列
	w.initArchiveSchema()
	if !columnsIn(w.DB, "archives")["brand"] {
		t.Fatal("回填失败却把列删了：商品属性数据永久丢失")
	}
	if !columnsIn(w.DB, "archive_drafts")["brand"] {
		t.Fatal("archive_drafts 也不该在回填失败时被删列")
	}
	if w.GetSettingValue("upgrade_archive_slim_380") != "" {
		t.Error("删列没执行却写了标记，下次启动不会再重试")
	}
}
