package provider

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"kandaoni.com/anqicms/library"
)

// sqlRecorder 捕获本次测试执行过的 SQL，用来断言「有没有真的下发 count」。
// 光看返回的 total 证明不了什么——count 跑了但结果被丢弃，total 一样正确。
type sqlRecorder struct {
	logger.Interface
	mu  sync.Mutex
	sql []string
}

func (r *sqlRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	s, _ := fc()
	r.mu.Lock()
	r.sql = append(r.sql, s)
	r.mu.Unlock()
}

func (r *sqlRecorder) collected() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.sql))
	copy(out, r.sql)
	return out
}

func (r *sqlRecorder) reset() {
	r.mu.Lock()
	r.sql = nil
	r.mu.Unlock()
}

// hasCount 判断捕获的 SQL 里是否出现过 count(*)。
func (r *sqlRecorder) hasCount() bool {
	for _, s := range r.collected() {
		if strings.Contains(strings.ToLower(s), "count(") {
			return true
		}
	}
	return false
}

// newTagListTestSite 返回一个挂了 SQL 记录器的站点，缓存每次重置。
func newTagListTestSite(t *testing.T) (*Website, *sqlRecorder) {
	t.Helper()
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Skipf("本地无可用站点库，跳过：%v", err)
	}
	InitWebsite(dbSite)
	w := GetWebsite(1)
	if w == nil || w.DB == nil {
		t.Skip("站点未初始化，跳过")
	}
	// 全局站点跨测试共享，缺了 Cache 会让其他测试 panic（见 anchor_test.go）。
	w.Cache = library.InitMemoryCache()
	rec := &sqlRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	w.DB = w.DB.Session(&gorm.Session{Logger: rec})
	return w, rec
}

// list 模式（needTotal=false）不能下发 count —— 这正是打爆 tags 表 16 万行的那条
// SELECT count(*) FROM tags。
func TestGetTagListListModeSkipsCount(t *testing.T) {
	w, rec := newTagListTestSite(t)
	rec.reset()

	tags, total, err := w.GetTagList(0, "", nil, "", 0, 10, false, "id desc")
	if err != nil {
		t.Fatalf("GetTagList 出错：%v", err)
	}
	if rec.hasCount() {
		t.Fatalf("list 模式仍执行了 count(*)，捕获 SQL：%v", rec.collected())
	}
	// list 模式没有精确总数，只能返回本页条数（与 GetArchiveList 一致）。
	if total != int64(len(tags)) {
		t.Fatalf("list 模式 total 应等于本页条数 %d，实际 %d", len(tags), total)
	}
}

// 分页模式（needTotal=true）必须给出精确总数。
func TestGetTagListPageModeReturnsTotal(t *testing.T) {
	w, rec := newTagListTestSite(t)
	rec.reset()

	tags, total, err := w.GetTagList(0, "", nil, "", 0, 10, true, "id desc")
	if err != nil {
		t.Fatalf("GetTagList 出错：%v", err)
	}
	if !rec.hasCount() {
		t.Fatalf("分页模式必须执行 count(*)，捕获 SQL：%v", rec.collected())
	}
	if total < int64(len(tags)) {
		t.Fatalf("total(%d) 不应小于本页条数(%d)", total, len(tags))
	}
}

// 分页模式的 count 结果要进缓存，第二次请求不许再扫表。
func TestGetTagListPageModeCachesTotal(t *testing.T) {
	w, rec := newTagListTestSite(t)
	rec.reset()

	if _, _, err := w.GetTagList(0, "", nil, "", 0, 10, true, "id desc"); err != nil {
		t.Fatalf("第一次 GetTagList 出错：%v", err)
	}
	if !rec.hasCount() {
		t.Fatal("第一次请求应执行 count(*) 并写入缓存")
	}

	rec.reset()
	if _, _, err := w.GetTagList(0, "", nil, "", 0, 10, true, "id desc"); err != nil {
		t.Fatalf("第二次 GetTagList 出错：%v", err)
	}
	if rec.hasCount() {
		t.Fatalf("第二次请求命中了缓存却仍执行 count(*)，捕获 SQL：%v", rec.collected())
	}
}

// offset 由调用方决定，GetTagList 不许再按 currentPage 二次推算把它覆盖掉。
func TestGetTagListHonorsExplicitOffset(t *testing.T) {
	w, _ := newTagListTestSite(t)

	first, _, err := w.GetTagList(0, "", nil, "", 0, 1, false, "id desc")
	if err != nil {
		t.Fatalf("GetTagList 出错：%v", err)
	}
	second, _, err := w.GetTagList(0, "", nil, "", 1, 1, false, "id desc")
	if err != nil {
		t.Fatalf("GetTagList 出错：%v", err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Skip("tags 表数据不足两条，跳过 offset 断言")
	}
	if first[0].Id == second[0].Id {
		t.Fatalf("offset=1 返回的仍是 offset=0 的同一条记录（id=%d），偏移未生效", first[0].Id)
	}
}

// 缓存只是优化：站点的 Cache 可能还没建好（见 anchor_test.go 里
// 「InitWebsite 只填站点结构，不建 Cache」），缺了不能让取数 panic。
func TestGetTagListToleratesNilCache(t *testing.T) {
	w, _ := newTagListTestSite(t)
	orig := w.Cache
	w.Cache = nil
	defer func() { w.Cache = orig }()

	_, total, err := w.GetTagList(0, "", nil, "", 0, 10, true, "id desc")
	if err != nil {
		t.Fatalf("Cache 为 nil 时 GetTagList 出错：%v", err)
	}
	if total < 0 {
		t.Fatalf("Cache 为 nil 时 total 异常：%d", total)
	}
}
