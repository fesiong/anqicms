package provider

import (
	"fmt"
	"log"
	"strings"
	"time"

	"kandaoni.com/anqicms/model"
)

// archiveSlimTables 3.8.0 瘦身的表。
//
// archive_drafts 必须一起做：model.ArchiveDraft 嵌入 model.Archive，两张表同构，
// 草稿发布（ArchiveDraft → Archive）是整行复制。只删 archives 的列，
// 发布时 INSERT INTO archives (...) SELECT ... FROM archive_drafts
// 立刻 Unknown column，整条发布链路断掉。
var archiveSlimTables = []string{"archives", "archive_drafts"}

// archiveDeadColumns 是历史迭代的残留列，全仓 Go 代码零引用。
//
// 逐个核实过（不是"看着像死的"）：
//   - deleted_time：早期软删除方案，现在草稿删除看 archive_drafts.status
//   - logo：主图改用 images 派生（Archive.Logo 是 gorm:"-" 计算字段）
//   - sku_type：被 option_type 取代
//   - rating_count / is_wholesale / has_order_fields：只在极早期 PHP 版存在
var archiveDeadColumns = []string{
	"deleted_time", "logo", "sku_type", "rating_count", "is_wholesale", "has_order_fields",
}

// archiveObsoleteColumns 3.8.0 要从主表删掉的全部列。
//
// 商品专用列直接复用 archiveProductColumns：那份名单是侧表回填的数据来源，
// 两边共用能保证"搬完就删"，不会出现搬了 23 列只删 21 列、
// 剩下的两列从此无人读写的中间态。
var archiveObsoleteColumns = func() []string {
	cols := make([]string, 0, len(archiveProductColumns)+len(archiveDeadColumns))
	cols = append(cols, archiveProductColumns...)
	cols = append(cols, archiveDeadColumns...)
	return cols
}()

// archiveRedundantIndex 描述一组冗余索引：keep 的列序列已经覆盖 drop。
type archiveRedundantIndex struct {
	drop string
	keep string
}

// archiveRedundantIndexes archives 表上的重复索引。
//
// 判据是「drop 的列序列是 keep 的前缀」而不是按名字硬删：47 个站库的索引是
// 十几轮升级叠出来的，同名索引在不同库里可能索引的列并不一样（archive_drafts
// 的 idx_updated_time 就是正确的，见 fixArchiveUpdatedTimeIndex），
// 名字对了列不对时必须跳过。
var archiveRedundantIndexes = []archiveRedundantIndex{
	{drop: "idx_articles_url_token", keep: "idx_archives_url_token"},
	{drop: "idx_fixed_link", keep: "idx_archives_fixed_link"},
	{drop: "idx_category_id", keep: "idx_category_archive_id"},
}

// ensureArchiveSlimSchema 执行 3.8.0 的删列与删索引。
//
// 为什么放在 InitModelData 而不是 AutoMigrateDB：后者只在
// last_run_version != config.Version 时跑，而且是 go 出去的异步任务
// （provider/website.go:476）。删列必须在「回填 archive_products 成功之后、
// 服务对外提供读取之前」完成，只能是同步且可重试的。
//
// 幂等：setting 标记只写一次；即使标记被清掉，"列还在吗"的 HasColumn
// 判断也会让第二次执行变成空操作。
func (w *Website) ensureArchiveSlimSchema() error {
	if w.DB == nil {
		return nil
	}
	const flagKey = "upgrade_archive_slim_380"
	if w.GetSettingValue(flagKey) != "" {
		return nil
	}
	if !w.DB.Migrator().HasTable(&model.Archive{}) {
		return nil
	}

	for _, table := range archiveSlimTables {
		if !w.DB.Migrator().HasTable(table) {
			continue
		}
		var drops []string
		for _, col := range archiveObsoleteColumns {
			if w.DB.Migrator().HasColumn(table, col) {
				drops = append(drops, "DROP COLUMN `"+col+"`")
			}
		}
		if len(drops) == 0 {
			continue
		}
		// 一条 ALTER 删完：MySQL 8 单条多列 DROP 只重建一次表，
		// 逐列执行会把整表重建 29 次（万行库就是几十秒的表锁）。
		sql := fmt.Sprintf("ALTER TABLE `%s` %s", table, strings.Join(drops, ", "))
		started := time.Now()
		if err := w.DB.Exec(sql).Error; err != nil {
			return fmt.Errorf("表 %s 删除历史列失败：%w", table, err)
		}
		log.Printf("升级 3.8.0：%s 删除 %d 个历史列，耗时 %s", table, len(drops), time.Since(started))
	}

	if err := w.dropArchiveRedundantIndexes(); err != nil {
		return err
	}
	if err := w.fixArchiveUpdatedTimeIndex(); err != nil {
		return err
	}

	_ = w.SaveSettingValue(flagKey, time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// dropArchiveRedundantIndexes 删掉与其它索引完全重复的索引。
func (w *Website) dropArchiveRedundantIndexes() error {
	const table = "archives"
	if !w.DB.Migrator().HasTable(table) {
		return nil
	}
	for _, pair := range archiveRedundantIndexes {
		if !w.DB.Migrator().HasIndex(table, pair.drop) || !w.DB.Migrator().HasIndex(table, pair.keep) {
			continue
		}
		dropCols := w.indexColumns(table, pair.drop)
		keepCols := w.indexColumns(table, pair.keep)
		if !isIndexPrefix(dropCols, keepCols) {
			// 列对不上就说明这个库里的同名索引另有含义，宁可不删。
			log.Printf("升级 3.8.0：跳过索引 %s（列 %v 不是 %s 的前缀 %v）", pair.drop, dropCols, pair.keep, keepCols)
			continue
		}
		if err := w.DB.Exec("ALTER TABLE `" + table + "` DROP INDEX `" + pair.drop + "`").Error; err != nil {
			return fmt.Errorf("删除冗余索引 %s 失败：%w", pair.drop, err)
		}
	}
	return nil
}

// fixArchiveUpdatedTimeIndex 修正 archives 上错建在 created_time 列的 idx_updated_time。
//
// 现象：archives 的 idx_updated_time 索引的是 created_time，与 idx_created_time
// 完全重复，而 updated_time 本身没有索引（草稿定时清理和 timefactor 都按
// updated_time 过滤）。archive_drafts 的同名索引是对的，所以这段只处理 archives。
func (w *Website) fixArchiveUpdatedTimeIndex() error {
	const table = "archives"
	const index = "idx_updated_time"
	if !w.DB.Migrator().HasIndex(table, index) {
		return nil
	}
	cols := w.indexColumns(table, index)
	created := w.indexColumns(table, "idx_created_time")
	if !sameIndexColumns(cols, created) {
		// 已经是正确的 updated_time 索引，什么都不用做。
		return nil
	}
	if err := w.DB.Exec("ALTER TABLE `" + table + "` DROP INDEX `" + index + "`").Error; err != nil {
		return fmt.Errorf("删除错建的 %s 失败：%w", index, err)
	}
	if !w.DB.Migrator().HasColumn(table, "updated_time") {
		return nil
	}
	// AutoMigrate 也是 go 出去的异步任务，可能在我们 DROP 之后、CREATE 之前
	// 按 model tag 把它建好了（tag 名就是 idx_updated_time）。再查一次，
	// 否则 CREATE 会撞 Duplicate key name 让整个升级标记写不下去。
	if w.DB.Migrator().HasIndex(table, index) {
		return nil
	}
	if err := w.DB.Exec("CREATE INDEX `" + index + "` ON `" + table + "` (`updated_time`)").Error; err != nil {
		return fmt.Errorf("重建 %s（updated_time 列）失败：%w", index, err)
	}
	return nil
}

// indexColumns 返回索引的列序列（按序）。
//
// 不用 GORM Migrator 的接口：它没有"取索引列"的方法，而这些迁移的判断
// 恰恰依赖"同名索引在不同库里可能索引不同列"这个事实。
func (w *Website) indexColumns(table, index string) []string {
	rows, err := w.DB.Raw(
		"SELECT COLUMN_NAME FROM information_schema.STATISTICS "+
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ? "+
			"ORDER BY SEQ_IN_INDEX", table, index).Rows()
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil
		}
		cols = append(cols, name)
	}
	return cols
}

// isIndexPrefix 判断 a 是否是 b 的最左前缀（含完全相等）。
// 空序列一律判否：拿不到列信息时不能当成"是前缀"，否则会误删索引。
func isIndexPrefix(a, b []string) bool {
	if len(a) == 0 || len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameIndexColumns 判断两个索引的列序列是否完全一致。
func sameIndexColumns(a, b []string) bool {
	return len(a) > 0 && len(a) == len(b) && isIndexPrefix(a, b)
}

// initArchiveSchema 是商品属性拆表的启动入口：建侧表 → 回填 → 注册读回调 → 删旧列。
//
// 顺序不能动：
//   - 回填必须早于删列。回填是那 23 列最后的搬运机会，反过来就是
//     "列删了，属性没搬，商品全部归零"且不可恢复。
//   - 回填失败时**不删列**。宁可这次升级少做一步（下次启动会重试，
//     标记没写），也不能把唯一的源数据删掉。
//   - 建侧表失败就不注册读回调，否则每次查询都多打一条必然报错的 SQL。
func (w *Website) initArchiveSchema() {
	if w.DB == nil {
		return
	}
	if err := w.ensureArchiveProductTable(); err != nil {
		log.Println("archive_products 初始化失败，跳过 3.8.0 删列：", err)
		return
	}
	w.registerArchiveProductLoader()
	if err := w.ensureArchiveSlimSchema(); err != nil {
		log.Println("3.8.0 删列/删索引失败（下次启动重试）：", err)
	}
}
