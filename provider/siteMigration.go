package provider

// 站点库表迁移的调度层：并发闸门 + 预检 + 进度 + 错误出口 + 离线迁移入口。
//
// 为什么需要这一层（2026-11-16 客户 106 站点事故）：
// InitWebsites 按 id 串行遍历站点，但迁移过去是 `go AutoMigrateDB(...)` 出去的，
// 106 个站点最多就有 106 份迁移同时在跑。MySQL 的 ALTER TABLE ... MODIFY COLUMN
// 是整表重建，并发几十份直接把 IO/CPU 打满：单条 ALTER 从 0.5s 退化到 2.9s，
// 启动 2.5 分钟只走到第 75 个站点。客户看到的是「前十几个站点正常、后面的全部
// 初始化失败」，而日志里**一条报错都没有**，只有 6489 条慢 SQL（总耗时 4402 秒 /
// 墙钟 135 秒 ≈ 33 倍并行，峰值 75 倍）。
//
// 现在的约定，四条都不能退：
//  1. 迁移必须拿到闸门令牌才开跑（默认 2 个站点并行），不再无限并发；
//  2. 迁移**同步**跑完才继续 InitSetting/InitModelData —— 过去两条链路抢同一张表的
//     元数据锁（InitModelData 里的 initArchiveSchema 也要改 archives）；
//  3. 迁移完成前站点不进注册表，失败则一律标未初始化：半迁移的表结构被请求读到，
//     比"站点暂时不可用"危险得多；
//  4. 迁移错误必须 slog 出来。`go AutoMigrateDB(...)` 丢弃返回值的写法，会让
//     "迁移失败"和"迁移成功但慢"在日志里长得一模一样。

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

const (
	// defaultMigrateConcurrency 是单进程内同时迁移的站点数。
	// 2 是"能把多核用起来、又不至于让 InnoDB 的 IO 队列排满"的折中：
	// 事故现场峰值 75 个站点同时迁移，单条 ALTER 已被拖到 2.9 秒。
	defaultMigrateConcurrency = 2
	// migrateConcurrencyEnv 允许运维不改代码就调整并发（例如临时降到 1）。
	migrateConcurrencyEnv = "ANQI_MIGRATE_CONCURRENCY"
)

var (
	migrateSemMu   sync.Mutex
	migrateSem     chan struct{}
	migrateSemSize int

	// 单轮迁移的进度计数。BeginSiteMigrationRound 复位，日志里读它拼 "x/总数"。
	migrateRoundTotal   int64
	migrateRoundDone    int64
	migrateRoundSkipped int64
	migrateRoundFailed  int64
)

// SetMigrateConcurrency 设置迁移并发（<1 视为 1）。
// 必须在开始迁移之前调用；闸门已按别的容量建好后不再变更，只记一条警告，
// 因为 channel 容量不能热改，静默忽略会让运维以为设置生效了。
func SetMigrateConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	migrateSemMu.Lock()
	defer migrateSemMu.Unlock()
	if migrateSem != nil && cap(migrateSem) != n {
		slog.Warn("迁移并发已生效，忽略本次设置", "current", cap(migrateSem), "requested", n)
		return
	}
	migrateSemSize = n
	if migrateSem == nil {
		migrateSem = make(chan struct{}, n)
	}
}

func resolveMigrateConcurrency() int {
	migrateSemMu.Lock()
	defer migrateSemMu.Unlock()
	return resolveMigrateConcurrencyLocked()
}

// resolveMigrateConcurrencyLocked 是上面那个的无锁版本，**调用方必须已持有
// migrateSemMu**。拆开是因为 migrateSlot 在持锁状态下要建闸门：
// 若在持锁时调加锁版本，就是自己等自己（第一次真实迁移实测挂死 3 分钟没发一条 SQL）。
func resolveMigrateConcurrencyLocked() int {
	if migrateSemSize > 0 {
		return migrateSemSize
	}
	n := defaultMigrateConcurrency
	if raw := strings.TrimSpace(os.Getenv(migrateConcurrencyEnv)); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			n = v
		} else {
			slog.Warn("环境变量取值非法，按默认并发迁移", "env", migrateConcurrencyEnv, "value", raw, "fallback", n)
		}
	}
	if n < 1 {
		n = 1
	}
	migrateSemSize = n
	return n
}

// migrateSlot 取一个迁移令牌，返回释放函数。
// 用 defer release() 立刻成对使用，不要手动调。
func migrateSlot() func() {
	migrateSemMu.Lock()
	if migrateSem == nil {
		migrateSem = make(chan struct{}, resolveMigrateConcurrencyLocked())
	}
	sem := migrateSem
	migrateSemMu.Unlock()
	sem <- struct{}{}
	return func() { <-sem }
}

// BeginSiteMigrationRound 复位本轮进度。total 只用于日志里的分母。
func BeginSiteMigrationRound(total int) {
	atomic.StoreInt64(&migrateRoundTotal, int64(total))
	atomic.StoreInt64(&migrateRoundDone, 0)
	atomic.StoreInt64(&migrateRoundSkipped, 0)
	atomic.StoreInt64(&migrateRoundFailed, 0)
}

// SiteMigrationPending 判断该站点库要不要迁移。
//
// 返回 error 分两种情况，调用方都不能当成"不用迁移"：
//   - 新装站的库连 settings 表都还没有（1146），必须迁移；
//   - 库暂时不可读，也按需要迁移处理，让真正的错误在迁移里暴露。
func SiteMigrationPending(db *gorm.DB) (pending bool, lastVersion string, err error) {
	if db == nil {
		return false, "", nil
	}
	err = db.Model(&model.Setting{}).Where("`key` = ?", LastRunVersionKey).Pluck("value", &lastVersion).Error
	if err != nil {
		return true, lastVersion, err
	}
	return lastVersion != config.Version, lastVersion, nil
}

// MigrateSiteSchema 在并发闸门内同步完成单个站点的库表迁移。
//
// 返回 error 时该站点**不得**被注册为可用站点：半迁移的表结构会让正常请求读出
// 错误结果，比整站不可用更难排查。错误一定会进日志（原因见文件头第 4 条）。
func MigrateSiteSchema(siteID uint, siteName string, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	pending, lastVersion, err := SiteMigrationPending(db)
	if err != nil {
		// 新装站的正常路径也会走到这里，所以只记 Info，不当作失败。
		slog.Info("站点迁移预检未读到版本标记，按需要迁移处理",
			"siteId", siteID, "name", siteName, "reason", err)
	}
	if !pending {
		if err := ensureVideoTaskTable(db); err != nil {
			atomic.AddInt64(&migrateRoundFailed, 1)
			slog.Error("站点补建视频任务表失败，该站点将以未初始化状态运行",
				"siteId", siteID, "name", siteName, "reason", err)
			return err
		}
		atomic.AddInt64(&migrateRoundSkipped, 1)
		return nil
	}

	release := migrateSlot()
	defer release()

	started := time.Now()
	slog.Info("站点迁移开始",
		"siteId", siteID, "name", siteName,
		"from", displayVersion(lastVersion), "to", config.Version)
	if mErr := AutoMigrateDB(db, false); mErr != nil {
		atomic.AddInt64(&migrateRoundFailed, 1)
		slog.Error("站点迁移失败，该站点将以未初始化状态运行（库表可能已被部分修改，修复后重启会重试）",
			"siteId", siteID, "name", siteName,
			"from", displayVersion(lastVersion), "to", config.Version,
			"cost", time.Since(started).Round(time.Second).String(),
			"reason", mErr)
		return mErr
	}
	done := atomic.AddInt64(&migrateRoundDone, 1)
	total := atomic.LoadInt64(&migrateRoundTotal)
	// 客户在面板上看不到"进度"，只能看到站点一个个变正常；迁移要几十分钟时，
	// 没有进度日志的启动过程和"进程卡死"完全无法区分。
	log.Printf("站点迁移进度 %d/%d：站点 #%d「%s」完成 %s → %s，耗时 %s",
		done, total, siteID, siteName, displayVersion(lastVersion), config.Version,
		time.Since(started).Round(time.Second))
	return nil
}

func ensureVideoTaskTable(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	complete, err := videoTaskSchemaStatus(db)
	if err != nil {
		return err
	}
	if complete {
		return nil
	}
	release := migrateSlot()
	defer release()
	complete, err = videoTaskSchemaStatus(db)
	if err != nil {
		return err
	}
	if complete {
		return nil
	}
	if err := migrateVideoTaskTimeColumns(db); err != nil {
		return fmt.Errorf("convert video_tasks timestamps: %w", err)
	}
	complete, err = videoTaskSchemaStatus(db)
	if err != nil {
		return err
	}
	if complete {
		return nil
	}
	if err := db.AutoMigrate(&model.VideoTask{}); err != nil {
		return fmt.Errorf("create or repair video_tasks: %w", err)
	}
	return nil
}

var videoTaskTimeColumns = []string{
	"created_time", "updated_time", "next_retry_at", "started_at", "finished_at", "lease_until",
}

func videoTaskSchemaComplete(db *gorm.DB) bool {
	complete, _ := videoTaskSchemaStatus(db)
	return complete
}

func videoTaskSchemaStatus(db *gorm.DB) (bool, error) {
	if db == nil || !db.Migrator().HasTable(&model.VideoTask{}) {
		return false, nil
	}
	for _, column := range []string{
		"id", "created_time", "updated_time", "attachment_id", "status", "progress",
		"attempts", "error_msg", "config", "playlist", "artifacts", "next_retry_at",
		"started_at", "finished_at", "lease_until", "lease_owner",
	} {
		if !db.Migrator().HasColumn(&model.VideoTask{}, column) {
			return false, nil
		}
	}
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
		DataType   string `gorm:"column:data_type"`
	}
	if err := db.Raw(
		"SELECT COLUMN_NAME, DATA_TYPE FROM information_schema.COLUMNS "+
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME IN ?",
		"video_tasks", videoTaskTimeColumns,
	).Scan(&columns).Error; err != nil {
		return false, fmt.Errorf("inspect video_tasks timestamp columns: %w", err)
	}
	types := make(map[string]string, len(columns))
	for _, column := range columns {
		types[column.ColumnName] = column.DataType
	}
	for _, column := range videoTaskTimeColumns {
		if !strings.EqualFold(types[column], "bigint") {
			return false, nil
		}
	}
	for _, index := range []string{"idx_attachment_status", "idx_status_retry", "idx_created_time"} {
		if !db.Migrator().HasIndex(&model.VideoTask{}, index) {
			return false, nil
		}
	}
	return true, nil
}

func migrateVideoTaskTimeColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&model.VideoTask{}) {
		return nil
	}
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
		DataType   string `gorm:"column:data_type"`
	}
	if err := db.Raw(
		"SELECT COLUMN_NAME, DATA_TYPE FROM information_schema.COLUMNS "+
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME IN ?",
		"video_tasks", videoTaskTimeColumns,
	).Scan(&columns).Error; err != nil {
		return err
	}
	types := make(map[string]string, len(columns))
	for _, column := range columns {
		types[column.ColumnName] = column.DataType
	}
	legacy := make([]string, 0, len(videoTaskTimeColumns))
	for _, column := range videoTaskTimeColumns {
		dataType, exists := types[column]
		if exists && !strings.EqualFold(dataType, "bigint") {
			legacy = append(legacy, column)
		}
	}
	if len(legacy) == 0 {
		return nil
	}
	for _, column := range legacy {
		staging := column + "_unix"
		if db.Migrator().HasColumn(&model.VideoTask{}, staging) {
			continue
		}
		if err := db.Exec(fmt.Sprintf(
			"ALTER TABLE `video_tasks` ADD COLUMN `%s` BIGINT NOT NULL DEFAULT 0", staging,
		)).Error; err != nil {
			return err
		}
	}
	for _, column := range legacy {
		if err := backfillVideoTaskUnixColumn(db, column); err != nil {
			return err
		}
	}
	changes := make([]string, 0, len(legacy)*2+2)
	dropRetryIndex := false
	dropCreatedIndex := false
	for _, column := range legacy {
		if column == "next_retry_at" || column == "lease_until" {
			dropRetryIndex = true
		}
		if column == "created_time" {
			dropCreatedIndex = true
		}
	}
	if dropRetryIndex && db.Migrator().HasIndex(&model.VideoTask{}, "idx_status_retry") {
		changes = append(changes, "DROP INDEX `idx_status_retry`")
	}
	if dropCreatedIndex && db.Migrator().HasIndex(&model.VideoTask{}, "idx_created_time") {
		changes = append(changes, "DROP INDEX `idx_created_time`")
	}
	for _, column := range legacy {
		changes = append(changes,
			fmt.Sprintf("DROP COLUMN `%s`", column),
			fmt.Sprintf("CHANGE COLUMN `%s_unix` `%s` BIGINT NOT NULL DEFAULT 0", column, column),
		)
	}
	if err := db.Exec("ALTER TABLE `video_tasks` " + strings.Join(changes, ", ")).Error; err != nil {
		return err
	}
	return nil
}

func backfillVideoTaskUnixColumn(db *gorm.DB, column string) error {
	const batchSize = 500
	var lastID uint64
	for {
		var rows []struct {
			ID    uint64
			Value *time.Time
		}
		// DATETIME 历来由 loc=Local 扫描，必须在 Go 侧取 Unix，不能依赖 MySQL 会话时区。
		if err := db.Raw(
			fmt.Sprintf("SELECT id, `%s` AS value FROM `video_tasks` WHERE id > ? ORDER BY id LIMIT ?", column),
			lastID,
			batchSize,
		).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			var timestamp int64
			if row.Value != nil && !row.Value.IsZero() {
				timestamp = row.Value.Unix()
			}
			if err := db.Exec(
				fmt.Sprintf("UPDATE `video_tasks` SET `%s_unix` = ? WHERE id = ?", column),
				timestamp,
				row.ID,
			).Error; err != nil {
				return err
			}
			lastID = row.ID
		}
		if len(rows) < batchSize {
			return nil
		}
	}
}

// MigrateRoundSummary 返回本轮进度的可读摘要，供启动收尾与离线迁移打印。
func MigrateRoundSummary() string {
	return fmt.Sprintf("已迁移 %d 个站点，跳过 %d 个，失败 %d 个（共 %d 个）",
		atomic.LoadInt64(&migrateRoundDone),
		atomic.LoadInt64(&migrateRoundSkipped),
		atomic.LoadInt64(&migrateRoundFailed),
		atomic.LoadInt64(&migrateRoundTotal))
}

var reInlineNotNull = regexp.MustCompile(`(?i)\s+not\s+null`)

// normalizeSchemaFieldNullability 把 `type:xxx not null` 这类写法就地规范化，
// 返回被改动的字段数。
//
// 背景：全仓模型把非空写进了类型字符串 —— `gorm:"type:varchar(32) not null;default:”"`，
// 共 752 列（53 个模型文件）。gorm 只认独立的 `not null` 设定，于是：
//   - field.NotNull == false（它以为这一列可空）
//   - FullDataTypeOf 却因为类型字符串里带着 "not null" 而照旧输出 NOT NULL
//
// 迁移比对到可空性时看到「库列 NOT NULL、模型可空」，就判成要 ALTER ...
// MODIFY COLUMN。而它生成的 SQL 与现有列定义一字不差 —— 改完没变化，下次再改。
// 本地实测（空库、一轮）：660 条空转 ALTER、193 秒；106 个站点一次升级就是
// 数万次整表重建，这才是「3.8.4 多站点起不来」的真正重量。
//
// 修 tag 需要改 752 处（风险与收益不匹配），所以在迁移入口做一次等价改写：
// "not null" 从 DataType 挪进 NotNull，生成的 DDL 完全一致（NOT NULL 仍在，
// 只是换了个来源），但比对逻辑不再误判。
//
// 安全前提：field.NotNull 在 gorm v1.31.2 里只有两处使用
// （migrator.FullDataTypeOf / migrator.MigrateColumn），都在 migrator 内，
// 不参与查询与写入路径。改动同时发生在 gorm 的 schema 缓存对象上，
// 因此同一个 db 句柄后续的 AutoMigrate 直接受益；幂等，重复调用不会再计数。
func normalizeSchemaFieldNullability(s *schema.Schema) int {
	if s == nil {
		return 0
	}
	fixed := 0
	for _, f := range s.Fields {
		if f.NotNull {
			continue
		}
		raw := string(f.DataType)
		if !reInlineNotNull.MatchString(raw) {
			continue
		}
		trimmed := strings.TrimSpace(reInlineNotNull.ReplaceAllString(raw, ""))
		if trimmed == "" {
			// 极端写法：type 里只有 "not null"。保留原样，宁可空转也不猜。
			continue
		}
		f.DataType = schema.DataType(trimmed)
		f.NotNull = true
		fixed++
	}
	return fixed
}

// normalizeModelFieldNullability 对迁移清单里的每个模型做上面的规范化。
// 走 stmt.Parse 是为了命中 gorm 的 schema 缓存（AutoMigrate 内部读的是同一份）。
func normalizeModelFieldNullability(db *gorm.DB, models []interface{}) int {
	if db == nil {
		return 0
	}
	fixed := 0
	for _, m := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil || stmt.Schema == nil {
			slog.Warn("迁移前解析模型失败，跳过非空标记规范化", "model", fmt.Sprintf("%T", m), "reason", err)
			continue
		}
		fixed += normalizeSchemaFieldNullability(stmt.Schema)
	}
	return fixed
}

// displayVersion 把"没有版本标记"渲染成人话，避免日志里出现 `完成  → 3.8.4` 这种空档。
func displayVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "（新装）"
	}
	return v
}

// SiteMigrateOptions 是离线迁移（-migrate-only）的参数。
type SiteMigrateOptions struct {
	Only        []int // 只处理这些站点 ID；空表示全部
	FromID      int   // 只处理 id >= FromID 的站点（分批用）
	Limit       int   // 最多处理多少个站点（分批用）
	DryRun      bool  // 只预检、不执行任何 DDL
	Concurrency int   // 覆盖默认并发；<=0 表示用默认值
}

// SiteMigrateResult 是离线迁移的汇总。
type SiteMigrateResult struct {
	Candidates int
	Pending    int
	Migrated   int
	Failed     int
	Duration   time.Duration
}

// MigrateAllSites 只做迁移、不初始化站点，供 `-migrate-only` 使用。
//
// 存在的意义：把迁移从启动流程里剥出来单独跑。106 个站点的升级要几十分钟到
// 几小时，混在启动里就只能"重启—没起完—再重启"，永远推进不了几个站点；
// 单独跑还能分批（-migrate-sites / -migrate-from / -migrate-limit）和预演（DryRun）。
func MigrateAllSites(opts SiteMigrateOptions) (res SiteMigrateResult, err error) {
	started := time.Now()
	// 具名返回值 + defer：用普通返回值时 defer 改的是副本，耗时永远是 0s。
	defer func() { res.Duration = time.Since(started) }()

	if opts.Concurrency > 0 {
		SetMigrateConcurrency(opts.Concurrency)
	}
	db := GetDefaultDB()
	if db == nil {
		return res, errors.New("默认数据库未初始化：请先确认 config.json 中的数据库配置")
	}
	var sites []*model.Website
	if err = db.Order("`id` asc").Find(&sites).Error; err != nil {
		return res, fmt.Errorf("读取站点列表失败：%w", err)
	}
	selected := selectSitesForMigration(sites, opts)
	res.Candidates = len(selected)
	BeginSiteMigrationRound(len(selected))

	workers := resolveMigrateConcurrency()
	if opts.DryRun {
		// 预演不占闸门、不开并发：只连库读版本号。开并发只会在客户库上多排队。
		workers = 1
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		failed []int
	)
	jobs := make(chan *model.Website)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mw := range jobs {
				migrateOneSite(mw, opts.DryRun, &mu, &res, &failed)
			}
		}()
	}
	for _, mw := range selected {
		jobs <- mw
	}
	close(jobs)
	wg.Wait()

	res.Failed = len(failed)
	if len(failed) > 0 {
		return res, fmt.Errorf("%d 个站点迁移失败：%v", len(failed), failed)
	}
	return res, nil
}

func migrateOneSite(mw *model.Website, dryRun bool, mu *sync.Mutex, res *SiteMigrateResult, failed *[]int) {
	siteDB, err := openSiteDBForMigration(mw)
	if err != nil {
		slog.Error("离线迁移：站点数据库连接失败", "siteId", mw.Id, "name", mw.Name, "reason", err)
		mu.Lock()
		*failed = append(*failed, int(mw.Id))
		atomic.AddInt64(&migrateRoundFailed, 1)
		mu.Unlock()
		return
	}
	pending, lastVersion, lookErr := SiteMigrationPending(siteDB)
	if lookErr != nil {
		slog.Info("离线迁移：未读到版本标记，按需要迁移处理", "siteId", mw.Id, "name", mw.Name, "reason", lookErr)
	}
	if !pending {
		atomic.AddInt64(&migrateRoundSkipped, 1)
		return
	}
	mu.Lock()
	res.Pending++
	mu.Unlock()
	if dryRun {
		log.Printf("预演：站点 #%d「%s」需要迁移（%s → %s，库 %s）",
			mw.Id, mw.Name, displayVersion(lastVersion), config.Version, siteDB.Migrator().CurrentDatabase())
		return
	}
	if err := MigrateSiteSchema(mw.Id, mw.Name, siteDB); err != nil {
		mu.Lock()
		*failed = append(*failed, int(mw.Id))
		mu.Unlock()
		return
	}
	mu.Lock()
	res.Migrated++
	mu.Unlock()
}

// openSiteDBForMigration 复刻 InitWebsite 里的连库规则：
// 站点 1 用默认库，其余站点可用 use_default 继承默认库的连接参数。
func openSiteDBForMigration(mw *model.Website) (*gorm.DB, error) {
	if mw.Id == 1 {
		db := GetDefaultDB()
		if db == nil {
			return nil, errors.New("默认数据库未初始化")
		}
		return db, nil
	}
	cfg := mw.Mysql
	if cfg.UseDefault {
		cfg.User = config.Server.Mysql.User
		cfg.Password = config.Server.Mysql.Password
		cfg.Host = config.Server.Mysql.Host
		cfg.Port = config.Server.Mysql.Port
	}
	return InitDB(&cfg)
}

func selectSitesForMigration(sites []*model.Website, opts SiteMigrateOptions) []*model.Website {
	only := make(map[int]bool, len(opts.Only))
	for _, id := range opts.Only {
		only[id] = true
	}
	selected := make([]*model.Website, 0, len(sites))
	for _, mw := range sites {
		if len(only) > 0 && !only[int(mw.Id)] {
			continue
		}
		if opts.FromID > 0 && int(mw.Id) < opts.FromID {
			continue
		}
		selected = append(selected, mw)
		if opts.Limit > 0 && len(selected) >= opts.Limit {
			break
		}
	}
	return selected
}

// ParseSiteIDSpec 解析 `-migrate-sites` 的取值：`1,3,9-20`。
// 空串返回 nil（表示不过滤）。任何非法片段都整体报错 —— 分批迁移漏掉站点
// 比"少跑一批"更危险（客户会以为全部迁完了）。
func ParseSiteIDSpec(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	seen := map[int]bool{}
	var ids []int
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("站点 ID 列表含空项：%q", spec)
		}
		if strings.Contains(part, "-") {
			ab := strings.SplitN(part, "-", 2)
			from, err1 := strconv.Atoi(strings.TrimSpace(ab[0]))
			to, err2 := strconv.Atoi(strings.TrimSpace(ab[1]))
			if err1 != nil || err2 != nil || from <= 0 || to < from {
				return nil, fmt.Errorf("站点 ID 区间非法：%q", part)
			}
			for id := from; id <= to; id++ {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("站点 ID 非法：%q", part)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}
