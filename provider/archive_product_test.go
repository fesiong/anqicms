package provider

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ── 商品属性侧表护栏（3.7.1 拆表）──────────────────────────────────────
//
// 拆表后的失效形态和普通 bug 不同：没有报错、没有 panic，商品页照样打开，
// 只是运费算成 0、SKU 模式变「无 SKU」、品牌和划线价消失。原因是侧表没读到
// 或者没写进去。所以这里锁四件事：
//
//  1. Archive 上的 23 个商品字段必须是 gorm:"-"。留着 column 标签的话，
//     3.8.0 DROP 之后所有查询直接 Unknown column，整站 500。
//  2. 写：Create/Save 之后侧表必须有对应行；属性全默认时该行必须被删掉，
//     否则商品改成文章后读出来还是旧配置。
//  3. 读：所有取 Archive 的方式（Take / Find / Table+Scan / Raw+Scan）都要
//     补全商品属性。漏一条路径就是某个页面静默显示零值——这正是要用 GORM
//     查询回调而不是逐点显式加载的原因。
//  4. provider 的列名单与 model 的字段必须一一对应：漏一列，回填就少一列，
//     老数据的该项在这个库里永久丢失且无从发现。

// archiveProductFieldNames 是 model.Archive 上「应该被搬到侧表」的字段名。
// 与 archiveProductColumns（DB 列名）通过 gorm tag 对应。
var archiveProductFieldNames = []string{
	"Brand", "VideoUrl", "NeedLogistics", "IsFreeShipping",
	"SoldCount", "ReviewCount", "AverageScore", "OptionType",
	"Weight", "WeightUnit", "Length", "Width", "Height", "Volume", "LengthUnit",
	"AllowOversold", "OrderFields", "MinQuantity", "MaxQuantity", "StepQuantity",
	"ProductType", "OriginPrice", "CostPrice",
}

// testArchiveId 造一个不冲突的测试 ID。
// archives.id 是 int(10) unsigned（最大 4.29e9），直接拿 UnixNano 会溢出，
// 所以取纳秒低位 + 固定基数。
func testArchiveId(offset int64) int64 {
	return 2000000000 + (time.Now().UnixNano()/1000+offset)%900000000
}

func newArchiveProductTestSite(t *testing.T) *Website {
	t.Helper()
	w := newZeroValueTestSite(t)
	if err := w.ensureArchiveProductTable(); err != nil {
		t.Fatalf("建侧表失败：%v", err)
	}
	w.registerArchiveProductLoader()
	return w
}

// TestArchiveProductFieldsAreIgnoredByGorm 锁住 model 层的 tag。
//
// 这条测试看着「只是检查 tag」，实际防的是最贵的一种回归：有人觉得拆表后
// 字段留着更方便，把 gorm:"-" 改回 column 标签，本地（旧列还在）一切正常，
// 等到 3.8.0 DROP 列上线，所有商品的每一次读取都是 Unknown column。
func TestArchiveProductFieldsAreIgnoredByGorm(t *testing.T) {
	typ := reflect.TypeOf(model.Archive{})
	for _, name := range archiveProductFieldNames {
		f, ok := typ.FieldByName(name)
		if !ok {
			t.Fatalf("model.Archive 缺少字段 %s——字段名改了就要同步改这里的名单", name)
		}
		if got := f.Tag.Get("gorm"); got != "-" {
			t.Errorf("model.Archive.%s 的 gorm tag 应为 \"-\"（属性存 archive_products），实际 %q", name, got)
		}
		if f.Tag.Get("json") == "" {
			t.Errorf("model.Archive.%s 丢了 json tag，前端契约会断", name)
		}
	}
}

// TestArchiveProductColumnsMatchModel 锁住 provider 列名单与 model 字段的一一对应。
func TestArchiveProductColumnsMatchModel(t *testing.T) {
	typ := reflect.TypeOf(model.ArchiveProduct{})
	var fromModel []string
	for i := 0; i < typ.NumField(); i++ {
		col := typ.Field(i).Tag.Get("gorm")
		if col == "" {
			continue
		}
		// 取 "column:xxx;..." 里的 xxx
		const prefix = "column:"
		idx := 0
		for j := 0; j+len(prefix) <= len(col); j++ {
			if col[j:j+len(prefix)] == prefix {
				idx = j + len(prefix)
				break
			}
		}
		name := col[idx:]
		if k := indexByte(name, ';'); k >= 0 {
			name = name[:k]
		}
		if name == "archive_id" {
			continue
		}
		fromModel = append(fromModel, name)
	}
	sort.Strings(fromModel)

	fromProvider := append([]string(nil), archiveProductColumns...)
	sort.Strings(fromProvider)

	if len(fromModel) != len(fromProvider) {
		t.Fatalf("列数量不一致：model 有 %d 列，provider 名单有 %d 列\nmodel=%v\nprovider=%v",
			len(fromModel), len(fromProvider), fromModel, fromProvider)
	}
	for i := range fromModel {
		if fromModel[i] != fromProvider[i] {
			t.Errorf("第 %d 列不一致：model=%q provider=%q", i, fromModel[i], fromProvider[i])
		}
	}
	if len(fromModel) != 23 {
		t.Errorf("侧表应有 23 个业务列（24 个商品列减去留下的 stock），实际 %d", len(fromModel))
	}
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// TestArchiveProductWriteAndReadRoundTrip 是这张表的主护栏。
func TestArchiveProductWriteAndReadRoundTrip(t *testing.T) {
	w := newArchiveProductTestSite(t)

	id := testArchiveId(0)
	src := &model.Archive{
		Id:             id,
		Title:          fmt.Sprintf("侧表护栏-%d", id),
		ModuleId:       2,
		CategoryId:     1,
		UrlToken:       fmt.Sprintf("sidecar-%d", id),
		Brand:          "测试品牌",
		OptionType:     1,
		Weight:         1.25,
		WeightUnit:     "kg",
		OriginPrice:    19900,
		CostPrice:      9900,
		NeedLogistics:  true,
		IsFreeShipping: false,
		ProductType:    config.ProductTypeNormal,
	}
	defer func() {
		w.DB.Unscoped().Delete(&model.Archive{}, id)
		w.DB.Unscoped().Where("`archive_id` = ?", id).Delete(&model.ArchiveProduct{})
	}()

	// 写：Create → AfterSave → upsert 侧表
	if err := w.DB.Create(src).Error; err != nil {
		t.Fatalf("写入档案失败：%v", err)
	}
	var row model.ArchiveProduct
	if err := w.DB.Where("`archive_id` = ?", id).Take(&row).Error; err != nil {
		t.Fatalf("Create 之后侧表没有对应行：%v", err)
	}
	if row.Brand != "测试品牌" || row.OptionType != 1 || row.Weight != 1.25 ||
		row.OriginPrice != 19900 || row.CostPrice != 9900 || !row.NeedLogistics {
		t.Errorf("侧表内容不对：%+v", row)
	}

	// 读 ① 单条 Take（模板渲染、GetArchiveById 走这条）
	var one model.Archive
	if err := w.DB.Where("`id` = ?", id).Take(&one).Error; err != nil {
		t.Fatalf("Take 失败：%v", err)
	}
	if one.Brand != "测试品牌" || one.OptionType != 1 || one.OriginPrice != 19900 {
		t.Errorf("Take 之后商品属性没补上：brand=%q option_type=%d origin_price=%d",
			one.Brand, one.OptionType, one.OriginPrice)
	}

	// 读 ② 切片 Find（GetArchiveList、购物车、多语言同步走这条）
	var list []*model.Archive
	if err := w.DB.Where("`id` = ?", id).Find(&list).Error; err != nil {
		t.Fatalf("Find 失败：%v", err)
	}
	if len(list) != 1 || list[0].Brand != "测试品牌" {
		t.Errorf("Find 之后商品属性没补上：%+v", list)
	}

	// 读 ③ Table + Find + Select("*, 1 as status")（后台文档列表走这条）
	//
	// 原来这里用的是 .Scan()，GORM 的 Scan 走 Row()+ScanRows，既不过 Query
	// 处理器也不调 AfterFind，商品属性会静默变零值。已统一改成 Find。
	var scanned []*model.Archive
	err := w.DB.Table("`archives` as archives").Select("*, 1 as status").
		Where("archives.`id` = ?", id).Find(&scanned).Error
	if err != nil {
		t.Fatalf("Table+Find 失败：%v", err)
	}
	if len(scanned) != 1 || scanned[0].Brand != "测试品牌" {
		t.Errorf("Table+Find 之后商品属性没补上：brand=%q", func() string {
			if len(scanned) > 0 {
				return scanned[0].Brand
			}
			return "<空>"
		}())
	}
}

// TestArchiveProductDefaultRemovesRow 锁住「属性清空 = 删行」。
//
// 少了这条，商品改成文章后侧表还留着旧配置：列表页显示成商品、或者反过来
// 文章上冒出「销量 12 / 评分 4.8」。这种残留数据在后台完全看不见。
func TestArchiveProductDefaultRemovesRow(t *testing.T) {
	w := newArchiveProductTestSite(t)

	id := testArchiveId(1)
	src := &model.Archive{
		Id:          id,
		Title:       fmt.Sprintf("侧表清空-%d", id),
		ModuleId:    2,
		CategoryId:  1,
		UrlToken:    fmt.Sprintf("sidecar-clear-%d", id),
		Brand:       "要清空的品牌",
		OptionType:  2,
		ProductType: config.ProductTypeNormal,
	}
	defer func() {
		w.DB.Unscoped().Delete(&model.Archive{}, id)
		w.DB.Unscoped().Where("`archive_id` = ?", id).Delete(&model.ArchiveProduct{})
	}()

	if err := w.DB.Create(src).Error; err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	var cnt int64
	w.DB.Model(&model.ArchiveProduct{}).Where("`archive_id` = ?", id).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("有商品属性时侧表应有 1 行，实际 %d", cnt)
	}

	src.Brand = ""
	src.OptionType = 0
	if err := w.DB.Save(src).Error; err != nil {
		t.Fatalf("清空后保存失败：%v", err)
	}
	cnt = 0
	w.DB.Model(&model.ArchiveProduct{}).Where("`archive_id` = ?", id).Count(&cnt)
	if cnt != 0 {
		t.Errorf("商品属性全部清空后侧表行应被删除，实际还剩 %d 行", cnt)
	}
}

// TestArchiveProductArticleLeavesNoRow 锁住「文章不留垃圾行」。
//
// SaveArchive 新建文档时无条件把 product_type 写成 1（DB 默认值），如果
// IsDefault 把 1 当成「有商品配置」，一万篇文章就会在侧表留一万行空记录。
func TestArchiveProductArticleLeavesNoRow(t *testing.T) {
	w := newArchiveProductTestSite(t)

	id := testArchiveId(2)
	article := &model.Archive{
		Id:          id,
		Title:       fmt.Sprintf("纯文章-%d", id),
		ModuleId:    1,
		CategoryId:  1,
		UrlToken:    fmt.Sprintf("plain-%d", id),
		ProductType: config.ProductTypeNormal, // SaveArchive 新建时的默认值
	}
	defer func() {
		w.DB.Unscoped().Delete(&model.Archive{}, id)
		w.DB.Unscoped().Where("`archive_id` = ?", id).Delete(&model.ArchiveProduct{})
	}()
	if err := w.DB.Create(article).Error; err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	var cnt int64
	w.DB.Model(&model.ArchiveProduct{}).Where("`archive_id` = ?", id).Count(&cnt)
	if cnt != 0 {
		t.Errorf("纯文章不该在侧表留行（product_type=1 是默认值，不算商品配置），实际 %d 行", cnt)
	}
}

// TestNoScanIntoArchive 禁止用 .Scan() 取 Archive。
//
// GORM 的 Scan 实现是 Rows() + ScanRows（gorm v1.21.6 finisher_api.go:459），
// 完全绕过 Query 处理器和 AfterFind：商品属性会静默变成零值，页面上表现为
// 「运费 0、没有 SKU、品牌空白」，没有任何报错。
// 取 Archive 一律用 Find（Table(...).Find 与 Scan 语义等价）。
//
// 这条护栏是文本级的（不做类型推断），规则：同一文件里既出现 `.Scan(&X)`
// 又出现 `X []*model.Archive` / `X []model.Archive` / `X model.Archive`
// 就判定为违规。要绕过只能在另一个文件声明变量，实际不会发生。
func TestNoScanIntoArchive(t *testing.T) {
	roots := []string{".", "../controller"}
	scanRe := regexp.MustCompile(`\.Scan\(&(\w+)\)`)

	checked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == "graphify-out" || d.Name() == "public" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			src := string(raw)
			matches := scanRe.FindAllStringSubmatch(src, -1)
			if len(matches) == 0 {
				return nil
			}
			checked++
			for _, m := range matches {
				name := m[1]
				sliceRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s+\[\]\*?model\.Archive\b`)
				singleRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s+model\.Archive\b`)
				if sliceRe.MatchString(src) || singleRe.MatchString(src) {
					t.Errorf("%s: 用 .Scan(&%s) 取 Archive 会丢失商品属性（Scan 绕过 GORM 回调），改用 Find", path, name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("遍历 %s 失败：%v", root, err)
		}
	}
	if checked == 0 {
		t.Fatal("没扫到任何 .Scan() 调用——护栏本身失效了（路径或正则写错）")
	}
}

// TestSaveArchiveProductAttributesSurvive 是端到端的读-改-写闭环。
//
// 真实用户场景：后台商品页只改了标题，没碰品牌/重量/SKU 模式。这条链路是
// SaveArchive → 读回草稿（GORM 查询回调补商品属性）→ 改标题 → 再存（AfterSave
// 写回侧表）。中间任何一环断了，商品属性都会被静默清零，而接口照旧回成功。
func TestSaveArchiveProductAttributesSurvive(t *testing.T) {
	w := newArchiveProductTestSite(t)

	var category model.Category
	if err := w.DB.Where("`module_id` > 0").Order("id asc").Take(&category).Error; err != nil {
		t.Skipf("测试库没有可用分类，跳过：%v", err)
	}

	title := fmt.Sprintf("商品属性闭环-%d", time.Now().UnixNano())
	created, err := w.SaveArchive(&request.Archive{
		Title:       title,
		CategoryId:  category.Id,
		CategoryIds: library.Ptr([]uint{category.Id}),
		Draft:       true,
		Brand:       library.Ptr("闭环品牌"),
		OptionType:  library.Ptr(config.OptionTypePrice),
		Weight:      library.Ptr(2.5),
		WeightUnit:  library.Ptr("kg"),
		MinQuantity: library.Ptr(2),
		ProductType: library.Ptr(config.ProductTypeWholesale),
	})
	if err != nil {
		t.Fatalf("新建商品失败：%v", err)
	}
	defer func() {
		w.DB.Unscoped().Delete(&model.ArchiveDraft{}, created.Id)
		w.DB.Unscoped().Delete(&model.Archive{}, created.Id)
		w.DB.Unscoped().Where("`archive_id` = ?", created.Id).Delete(&model.ArchiveProduct{})
	}()

	if created.Brand != "闭环品牌" || created.Weight != 2.5 {
		t.Fatalf("新建返回的对象商品属性丢了吗：brand=%q weight=%v", created.Brand, created.Weight)
	}

	// 只改标题，商品属性一个都不传
	patched, err := w.SaveArchive(&request.Archive{Id: created.Id, Title: title + "-改"})
	if err != nil {
		t.Fatalf("PATCH 标题失败：%v", err)
	}
	if patched.Brand != "闭环品牌" || patched.OptionType != config.OptionTypePrice ||
		patched.Weight != 2.5 || patched.WeightUnit != "kg" || patched.MinQuantity != 2 {
		t.Errorf("PATCH 未传商品属性，原值被清零了：brand=%q option_type=%d weight=%v unit=%q min=%d",
			patched.Brand, patched.OptionType, patched.Weight, patched.WeightUnit, patched.MinQuantity)
	}
	if patched.ProductType != config.ProductTypeWholesale {
		t.Errorf("product_type 被改回默认值：期望 %d，实际 %d", config.ProductTypeWholesale, patched.ProductType)
	}

	// 换一个读入口再确认一次（不依赖上一步的对象）。
	// 注意 PATCH 没传 Draft 会把草稿发布到正式表，所以这里读的是 archives。
	saved, err := w.GetArchiveById(created.Id)
	if err != nil {
		t.Fatalf("读回档案失败：%v", err)
	}
	if saved.Brand != "闭环品牌" || saved.Weight != 2.5 {
		t.Errorf("GetArchiveById 没补上商品属性：brand=%q weight=%v", saved.Brand, saved.Weight)
	}
}

// TestArchiveProductBackfill 验证升级回填真的把旧列数据搬过来了。
//
// 用原生 SQL 造「拆表前」的行：model 已经把商品字段标成 gorm:"-"，走 GORM
// 写不出旧列数据，只能直接 INSERT。这条测试在 3.8.0 DROP 掉旧列之后会自动
// 跳过（HasColumn 为假）——那时候已经没有回填这回事了。
func TestArchiveProductBackfill(t *testing.T) {
	w := newZeroValueTestSite(t)

	if !w.DB.Migrator().HasColumn(&model.Archive{}, "brand") {
		t.Skip("archives 已经没有商品列（3.8.0 之后），无需回填")
	}
	if err := w.ensureArchiveProductTable(); err != nil {
		t.Fatalf("建表失败：%v", err)
	}

	fullId := testArchiveId(10)
	emptyId := testArchiveId(11)
	defer func() {
		w.DB.Unscoped().Delete(&model.Archive{}, fullId)
		w.DB.Unscoped().Delete(&model.Archive{}, emptyId)
		w.DB.Unscoped().Where("`archive_id` IN ?", []int64{fullId, emptyId}).Delete(&model.ArchiveProduct{})
	}()

	// ① 配过属性的商品：应该被搬进侧表
	if err := w.DB.Exec(
		"INSERT INTO `archives` (`id`,`title`,`module_id`,`category_id`,`url_token`,`brand`,`option_type`,`weight`,`weight_unit`,`product_type`) VALUES (?,?,?,?,?,?,?,?,?,?)",
		fullId, "回填用例-有属性", 2, 1, fmt.Sprintf("backfill-%d", fullId), "回填品牌", 1, 3.5, "kg", 1,
	).Error; err != nil {
		t.Fatalf("造旧数据失败：%v", err)
	}
	// ② 属性全默认的商品（测试库里 1 万篇文档都是这种）：不该留空行
	if err := w.DB.Exec(
		"INSERT INTO `archives` (`id`,`title`,`module_id`,`category_id`,`url_token`,`product_type`) VALUES (?,?,?,?,?,?)",
		emptyId, "回填用例-无属性", 2, 1, fmt.Sprintf("backfill-empty-%d", emptyId), 1,
	).Error; err != nil {
		t.Fatalf("造旧数据失败：%v", err)
	}

	// 重置标记，让回填重跑
	w.DB.Exec("DELETE FROM `settings` WHERE `key` = ?", "upgrade_archive_products")
	w.DB.Exec("DELETE FROM `archive_products` WHERE `archive_id` IN ?", []int64{fullId, emptyId})
	if err := w.ensureArchiveProductTable(); err != nil {
		t.Fatalf("回填失败：%v", err)
	}

	var row model.ArchiveProduct
	if err := w.DB.Where("`archive_id` = ?", fullId).Take(&row).Error; err != nil {
		t.Fatalf("配过属性的商品没有被回填：%v", err)
	}
	if row.Brand != "回填品牌" || row.OptionType != 1 || row.Weight != 3.5 || row.WeightUnit != "kg" {
		t.Errorf("回填内容不对：%+v", row)
	}

	var cnt int64
	w.DB.Model(&model.ArchiveProduct{}).Where("`archive_id` = ?", emptyId).Count(&cnt)
	if cnt != 0 {
		t.Errorf("属性全默认的文档不该产生侧表行，实际 %d 行", cnt)
	}
}
