package provider

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestSelectArchiveByFuncSingleRoundTrip 护栏：按条件取整篇文章只允许一次往返。
//
// 背景：GetArchiveByFunc 曾被写成「ToSQL 拼 SELECT id LIMIT 1 → Raw 取 id →
// WHERE id=? 取整行」，两次往返换一个马上就要被丢掉的 id。详情页要取正文 +
// 上一篇 + 下一篇，一次请求白烧三次。这个测试把往返次数钉死，改回去就红。
//
// 允许的第 2 条查询是 archive_products 的批量补全（一条 IN 查询，与行数无关），
// 不是 N+1。
func TestSelectArchiveByFuncSingleRoundTrip(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		d := &archiveQuerySQL{}
		w := newArchiveQuerySite(t, d, logger.Default.LogMode(logger.Silent))

		archive, err := w.selectArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("`url_token` = ?", "demo")
		})
		if err != nil {
			t.Fatalf("取文章失败：%v", err)
		}
		if archive.Id != 1 {
			t.Fatalf("id = %d，期望 1（假驱动返回的第一行）", archive.Id)
		}
		// 商品属性必须照样补上：不能为了省一次往返把侧表读取丢掉。
		if archive.Brand != "brand" {
			t.Fatalf("brand = %q，商品属性没补上", archive.Brand)
		}

		if len(d.queries) != 2 {
			t.Fatalf("SQL 往返 %d 次，期望 2（取行 + 批量取商品属性）：%v", len(d.queries), d.queries)
		}
		if !strings.Contains(d.queries[0], "SELECT *") {
			t.Fatalf("第一条不是整行查询（疑似又回到了「先取 id 再取行」）：%s", d.queries[0])
		}
		if !strings.Contains(d.queries[1], " FROM `archive_products`") || !strings.Contains(d.queries[1], " IN ") {
			t.Fatalf("第二条不是商品属性的批量查询：%s", d.queries[1])
		}
	})

	t.Run("miss", func(t *testing.T) {
		d := &archiveQuerySQL{empty: true}
		w := newArchiveQuerySite(t, d, logger.Default.LogMode(logger.Silent))

		if _, err := w.selectArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("`url_token` = ?", "missing")
		}); err == nil {
			t.Fatal("不存在的文章应该返回错误")
		}
		// 查不到时就该停手：白跑的探测查询会把这个数字顶上去。
		if len(d.queries) != 1 {
			t.Fatalf("不存在的文章发了 %d 条 SQL，期望 1：%v", len(d.queries), d.queries)
		}
	})
}
