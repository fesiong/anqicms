package provider

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// TestArchiveProductScopeGating 护栏：侧表只在「商城模式 + 产品模型」下读。
//
// 两条闸门各自都能独立关掉整条查询：
//   - 内容站（ShopMode=0）里商品属性永远是默认值，读它纯粹是每页多一次往返；
//   - 文章行（modules.kind=article）就算 ID 撞上侧表记录也不该被读出来。
//
// 关键点不止「少查一条」：跳过查询时**必须仍然打 productLoaded 标记**，否则
// model.Archive.AfterFind 兜底会对每一行各查一次，N+1 反而更糟。这里用
// 「总查询数 == 1」同时钉住这两件事——漏打标记会多出逐行查询，数字立刻变。
func TestArchiveProductScopeGating(t *testing.T) {
	cases := []struct {
		name       string
		shopMode   int
		moduleId   int64
		moduleKind string
		wantQuery  int
		wantBrand  string
	}{
		{name: "商城模式+产品模型", shopMode: 1, moduleId: 2, moduleKind: model.ModuleKindProduct, wantQuery: 2, wantBrand: "brand"},
		{name: "商城模式+文章模型", shopMode: 1, moduleId: 1, moduleKind: model.ModuleKindArticle, wantQuery: 1},
		{name: "内容模式+产品模型", shopMode: 0, moduleId: 2, moduleKind: model.ModuleKindProduct, wantQuery: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &archiveQuerySQL{moduleId: tc.moduleId}
			w := newArchiveQuerySite(t, d, logger.Default.LogMode(logger.Silent))
			w.System.ShopMode = tc.shopMode
			_ = w.Cache.Set("module_kinds", map[uint]string{uint(tc.moduleId): tc.moduleKind}, 0)

			archive, err := w.selectArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` = ?", 1)
			})
			if err != nil {
				t.Fatalf("取文章失败：%v", err)
			}

			if len(d.queries) != tc.wantQuery {
				t.Fatalf("SQL 往返 %d 次，期望 %d：%v", len(d.queries), tc.wantQuery, d.queries)
			}
			for _, q := range d.queries {
				// 逐行兜底查询的形状是 `archive_id = ?`；批量是 `IN (?)`。
				// 出现逐行查询说明 productLoaded 没打上，N+1 回来了。
				if strings.Contains(q, " FROM `archive_products`") && !strings.Contains(q, " IN ") {
					t.Fatalf("出现了逐行回查（productLoaded 未标记）：%s", q)
				}
			}
			if archive.Brand != tc.wantBrand {
				t.Fatalf("brand = %q，期望 %q", archive.Brand, tc.wantBrand)
			}
			// 没读侧表的行等价于「侧表没有行」：product_type 要补回默认值 1。
			if archive.ProductType != config.ProductTypeNormal {
				t.Fatalf("product_type = %d，期望默认值 %d", archive.ProductType, config.ProductTypeNormal)
			}
		})
	}
}
