package provider

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// archiveProductLoaderCallback 查询后自动补商品属性的回调名。
const archiveProductLoaderCallback = "anqi:archive_product_loader"

// archiveProductColumns 侧表承载的列名单。
//
// 必须与 model.ArchiveProduct 的 23 个业务字段一一对应——两侧漂移会导致
// 「回填漏列 → 商品属性读出来是零值」这种没有任何报错的静默失效，
// 所以 provider/archive_product_guard_test.go 会拿 model 的字段名比对这两份名单。
var archiveProductColumns = []string{
	"brand", "video_url", "need_logistics", "is_free_shipping",
	"sold_count", "review_count", "average_score", "option_type",
	"weight", "weight_unit", "length", "width", "height", "volume", "length_unit",
	"allow_oversold", "order_fields", "min_quantity", "max_quantity", "step_quantity",
	"product_type", "origin_price", "cost_price",
}

// ensureArchiveProductTable 建侧表并回填历史数据。
//
// 为什么不能只靠 AutoMigrateDB：它只在 last_run_version != config.Version
// 时执行，新表在老站库上不会自动出现；而商品属性是模板渲染和下单的必经字段，
// 缺表会让所有商品查询直接报错。所以放在 InitModelData 里（每次初始化都跑）。
//
// 回填用 setting 标记只做一次。老库（archives 还停留在 23 列那一代）根本没有
// 商品列，HasColumn 判断为假就跳过，不需要额外的版本分支。
func (w *Website) ensureArchiveProductTable() error {
	if w.DB == nil || !w.DB.Migrator().HasTable(&model.Archive{}) {
		return nil
	}
	if !w.DB.Migrator().HasTable(&model.ArchiveProduct{}) {
		if err := w.DB.Migrator().CreateTable(&model.ArchiveProduct{}); err != nil {
			return err
		}
	}

	const flagKey = "upgrade_archive_products"
	if w.GetSettingValue(flagKey) != "" {
		return nil
	}

	// 正式稿和草稿共用同一套 ID，两边都要搬：草稿发布时主表之间搬行、ID 不变，
	// 侧表因此不用跟着搬；但历史草稿从来没进过 archives，不单独回填就丢了。
	if w.DB.Migrator().HasColumn(&model.Archive{}, "brand") {
		if err := w.backfillArchiveProducts("archives"); err != nil {
			return err
		}
	}
	if w.DB.Migrator().HasColumn("archive_drafts", "brand") {
		if err := w.backfillArchiveProducts("archive_drafts"); err != nil {
			return err
		}
	}

	_ = w.SaveSettingValue(flagKey, time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// backfillArchiveProducts 把 srcTable 里商品模型下的行搬进侧表。
//
// 列清单按源表实际存在的列动态拼：47 个站库的 archives 从 23 列到 59 列不等，
// 写死 23 列会让缺列的老库直接 Unknown column，整站起不来。
func (w *Website) backfillArchiveProducts(srcTable string) error {
	cols := make([]string, 0, len(archiveProductColumns))
	vals := make([]string, 0, len(archiveProductColumns))
	for _, col := range archiveProductColumns {
		cols = append(cols, "`"+col+"`")
		if !w.DB.Migrator().HasColumn(srcTable, col) {
			vals = append(vals, archiveProductDefaultExpr(col))
			continue
		}
		// 源表的 weight_unit / length_unit 允许 NULL（历史定义是 default null），
		// 侧表对应列是 not null default ''，直接搬会 Error 1048 让整个升级中断。
		switch col {
		case "order_fields":
			vals = append(vals, "`"+col+"`")
		case "brand", "video_url", "weight_unit", "length_unit":
			vals = append(vals, "IFNULL(`"+col+"`,'')")
		default:
			vals = append(vals, "IFNULL(`"+col+"`,0)")
		}
	}
	// NOT EXISTS 而不是 ON DUPLICATE：这条 SQL 只在首次升级时跑一次，
	// 但可能被「修复数据库」重复触发，必须幂等。
	//
	// 还要过滤「属性全默认」的行：只按 module kind 判断会造出一堆空记录
	// （测试库 10094 篇文档全挂在产品模块下，实测全是默认值）。判据与
	// ArchiveProduct.IsDefault 保持一致——product_type=1 是默认值，不算配置。
	sql := fmt.Sprintf(
		"INSERT INTO `archive_products` (`archive_id`,%s) SELECT `src`.`id`,%s FROM `%s` AS `src` "+
			"WHERE `src`.`module_id` IN (SELECT `id` FROM `modules` WHERE `kind` = ?) "+
			"AND (%s) "+
			"AND NOT EXISTS (SELECT 1 FROM `archive_products` AS `p` WHERE `p`.`archive_id` = `src`.`id`)",
		strings.Join(cols, ","), strings.Join(vals, ","), srcTable,
		strings.Join(w.archiveProductNonDefaultConds(srcTable), " OR "),
	)
	return w.DB.Exec(sql, model.ModuleKindProduct).Error
}

// archiveProductNonDefaultConds 生成「这一行确实配过商品属性」的判定条件。
//
// 只收录源表里真实存在的列：老库缺列时拼进去会 Unknown column。
func (w *Website) archiveProductNonDefaultConds(srcTable string) []string {
	// 数值列：<> 0
	numeric := []string{
		"need_logistics", "is_free_shipping", "sold_count", "review_count",
		"average_score", "option_type", "weight", "length", "width", "height",
		"volume", "allow_oversold", "min_quantity", "max_quantity", "step_quantity",
		"origin_price", "cost_price",
	}
	// 字符串列：<> ''（NULL 也算默认）
	text := []string{"brand", "video_url", "weight_unit", "length_unit"}
	// 可空的文本列：非 NULL 且非空
	nullableText := []string{"order_fields"}

	conds := make([]string, 0, len(numeric)+len(text)+len(nullableText)+1)
	for _, col := range numeric {
		if w.DB.Migrator().HasColumn(srcTable, col) {
			conds = append(conds, "`src`.`"+col+"` <> 0")
		}
	}
	for _, col := range text {
		if w.DB.Migrator().HasColumn(srcTable, col) {
			conds = append(conds, "IFNULL(`src`.`"+col+"`,'') <> ''")
		}
	}
	for _, col := range nullableText {
		if w.DB.Migrator().HasColumn(srcTable, col) {
			conds = append(conds, "IFNULL(`src`.`"+col+"`,'') <> ''")
		}
	}
	// product_type 默认 1，不等于 1 才算配置过（批发/定制/虚拟…）
	if w.DB.Migrator().HasColumn(srcTable, "product_type") {
		conds = append(conds, "`src`.`product_type` <> 1")
	}
	// 一个条件都没有（老库连 brand 都没有）时不能拼出空 IN 条件，
	// 给个恒真条件让整条 SQL 退化成纯 module 判断。
	if len(conds) == 0 {
		conds = append(conds, "1 = 1")
	}
	return conds
}

// archiveProductDefaultExpr 给源表缺失的列提供同类型的默认值。
func archiveProductDefaultExpr(col string) string {
	switch col {
	case "brand", "video_url", "weight_unit", "length_unit":
		return "''"
	case "order_fields":
		return "NULL"
	case "product_type":
		return "1"
	}
	return "0"
}

// registerArchiveProductLoader 注册「查询完成就补商品属性」的全局回调。
//
// 为什么用回调而不是在 GetArchiveById / GetArchiveList 里显式加载：
// 从 DB 取出 Archive 的地方有 30 多处（后台列表、模板渲染、购物车、订单、
// 全文索引、HTML 缓存、多语言同步、模型复制……），显式加载漏一处就是
// 「商品属性静默变零值」——不报错、不 panic，只是运费算成 0、SKU 模式变无。
// Query().After 是全仓唯一能覆盖所有 Find/Scan/Take/First 的位置。
func (w *Website) registerArchiveProductLoader() {
	if w.DB == nil {
		return
	}
	// 站点可以重复初始化（后台改配置、修复数据库），Register 同名回调会
	// 追加而不是替换（gorm callbacks.go 的 Register 直接 append），因此
	// 必须先判重，否则同一次查询会把侧表读 N 遍。
	if w.DB.Callback().Query().Get(archiveProductLoaderCallback) != nil {
		return
	}
	_ = w.DB.Callback().Query().After("gorm:query").Register(archiveProductLoaderCallback, fillArchiveProducts)
}

// fillArchiveProducts 是 registerArchiveProductLoader 注册的回调本体。
func fillArchiveProducts(tx *gorm.DB) {
	if tx.Error != nil || tx.Statement == nil {
		return
	}
	ids, apply := archiveProductTargets(tx.Statement.Dest)
	if len(ids) == 0 {
		return
	}
	// NewDB 另起 statement：直接复用 tx 会把正在执行的这条查询的 Dest/Table
	// 改写成 archive_products。新 statement 的 Dest 不是 Archive，所以不会
	// 再次进入本回调（无递归）。
	var rows []model.ArchiveProduct
	err := tx.Session(&gorm.Session{NewDB: true}).
		Model(&model.ArchiveProduct{}).
		Where("`archive_id` IN ?", ids).
		Find(&rows).Error
	if err != nil {
		return
	}
	// 侧表一行都没有时也要往下走：apply 带空 map 会把 product_type 补回默认值 1，
	// 并打上「已加载」标记，AfterFind 就不会再逐条查一遍。
	byID := make(map[int64]*model.ArchiveProduct, len(rows))
	for i := range rows {
		byID[rows[i].ArchiveId] = &rows[i]
	}
	apply(byID)
}

// archiveProductTargets 从查询目标里取出 archive id，并返回「怎么把查到的
// 侧表记录填回去」的闭包。目标不是 Archive/ArchiveDraft 时返回 nil。
func archiveProductTargets(dest interface{}) ([]int64, func(map[int64]*model.ArchiveProduct)) {
	switch v := dest.(type) {
	case *model.Archive:
		if v == nil || v.Id == 0 {
			return nil, nil
		}
		id := v.Id
		return []int64{id}, func(m map[int64]*model.ArchiveProduct) { v.ApplyProduct(m[id]) }

	case *[]model.Archive:
		if v == nil || len(*v) == 0 {
			return nil, nil
		}
		list := *v
		ids := make([]int64, 0, len(list))
		for i := range list {
			if list[i].Id > 0 {
				ids = append(ids, list[i].Id)
			}
		}
		return ids, func(m map[int64]*model.ArchiveProduct) {
			for i := range list {
				list[i].ApplyProduct(m[list[i].Id])
			}
		}

	case *[]*model.Archive:
		if v == nil || len(*v) == 0 {
			return nil, nil
		}
		list := *v
		ids := make([]int64, 0, len(list))
		for _, a := range list {
			if a != nil && a.Id > 0 {
				ids = append(ids, a.Id)
			}
		}
		return ids, func(m map[int64]*model.ArchiveProduct) {
			for _, a := range list {
				if a != nil {
					a.ApplyProduct(m[a.Id])
				}
			}
		}

	case *model.ArchiveDraft:
		if v == nil || v.Id == 0 {
			return nil, nil
		}
		id := v.Id
		return []int64{id}, func(m map[int64]*model.ArchiveProduct) { v.ApplyProduct(m[id]) }

	case *[]model.ArchiveDraft:
		if v == nil || len(*v) == 0 {
			return nil, nil
		}
		list := *v
		ids := make([]int64, 0, len(list))
		for i := range list {
			if list[i].Id > 0 {
				ids = append(ids, list[i].Id)
			}
		}
		return ids, func(m map[int64]*model.ArchiveProduct) {
			for i := range list {
				list[i].ApplyProduct(m[list[i].Id])
			}
		}

	case *[]*model.ArchiveDraft:
		if v == nil || len(*v) == 0 {
			return nil, nil
		}
		list := *v
		ids := make([]int64, 0, len(list))
		for _, d := range list {
			if d != nil && d.Id > 0 {
				ids = append(ids, d.Id)
			}
		}
		return ids, func(m map[int64]*model.ArchiveProduct) {
			for _, d := range list {
				if d != nil {
					d.ApplyProduct(m[d.Id])
				}
			}
		}
	}
	return nil, nil
}
