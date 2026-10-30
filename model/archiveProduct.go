package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
)

// ArchiveProduct 商品的专用属性（品牌、物流、尺寸重量、SKU 模式、购买数量限制等）。
//
// 为什么拆出来：这 23 列原本长在 archives 上，但只有产品模型
// （modules.kind='product'）才用得上，文章行上永久是默认值。拆表后文章不再
// 为这些用不到的列付出：SELECT * 的行宽、以及改商品属性时与文章正文抢同一
// 行的行锁。
//
// 与主表的关系：archive_id 与 archives.id / archive_drafts.id 共享同一套 ID
// （GetNextArchiveId 同时检查两张表），所以一张侧表同时服务正式稿和草稿，
// 发布/撤回只在主表之间搬行，侧表不用动。
//
// ⚠️ stock 和 price 故意不在这里：
//   - stock 有 4 处原子表达式写（下单扣减、退单回补、SKU 汇总重算），拆表就
//     没法用一条 UPDATE 完成；
//   - price 同时承担「商品售价」和「付费阅读门槛」两种含义，拆之前要先做产品
//     决策，不属于本次结构调整。
//
// ⚠️ 侧表没有行 = 商品属性全默认。文章永远没有行；属性全默认的商品也不会有，
// 两者对读方没有区别（都是零值）。
type ArchiveProduct struct {
	ArchiveId      int64        `json:"archive_id" gorm:"column:archive_id;type:bigint(20);primaryKey;autoIncrement:false"`
	Brand          string       `json:"brand" gorm:"column:brand;type:varchar(100);not null;default:''"`
	VideoUrl       string       `json:"video_url" gorm:"column:video_url;type:varchar(250);not null;default:''"`                // 视频主图
	NeedLogistics  bool         `json:"need_logistics" gorm:"column:need_logistics;type:tinyint(1);not null;default:0"`         // 需要物流
	IsFreeShipping bool         `json:"is_free_shipping" gorm:"column:is_free_shipping;type:tinyint(1);not null;default:0"`     // 免运费
	SoldCount      int64        `json:"sold_count" gorm:"column:sold_count;type:bigint(20) unsigned;not null;default:0"`        // 销量
	ReviewCount    int64        `json:"review_count" gorm:"column:review_count;type:bigint(20) unsigned;not null;default:0"`    // 评价数量
	AverageScore   float32      `json:"average_score" gorm:"column:average_score;type:decimal(10,2);not null;default:0.00"`     // 平均评分
	OptionType     int          `json:"option_type" gorm:"column:option_type;type:tinyint(1);not null;default:0"`               // sku模式，0 无SKU，1 组合SKU，2 加价SKU
	Weight         float64      `json:"weight" gorm:"column:weight;type:decimal(12,6);not null;default:0.000000"`               // 重量 (kg/g/lb)
	WeightUnit     string       `json:"weight_unit" gorm:"column:weight_unit;type:varchar(20);not null;default:''"`             // 重量单位 kg/g/lb
	Length         float64      `json:"length" gorm:"column:length;type:decimal(12,2);not null;default:0.00"`                   // 长 (用户单位)
	Width          float64      `json:"width" gorm:"column:width;type:decimal(12,2);not null;default:0.00"`                     // 宽 (用户单位)
	Height         float64      `json:"height" gorm:"column:height;type:decimal(12,2);not null;default:0.00"`                   // 高 (用户单位)
	Volume         float64      `json:"volume" gorm:"column:volume;type:decimal(12,6);not null;default:0.000000"`               // 体积 (m³)
	LengthUnit     string       `json:"length_unit" gorm:"column:length_unit;type:varchar(20);not null;default:''"`             // 长度单位 m/cm/inch
	AllowOversold  bool         `json:"allow_oversold" gorm:"column:allow_oversold;type:tinyint(1);not null;default:0"`         // 允许超售
	OrderFields    moduleFields `json:"order_fields,omitempty" gorm:"column:order_fields;type:text default null"`               // 下单附加字段
	MinQuantity    int          `json:"min_quantity,omitempty" gorm:"column:min_quantity;type:bigint(20);not null;default:0"`   // 最小购买数量
	MaxQuantity    int          `json:"max_quantity,omitempty" gorm:"column:max_quantity;type:bigint(20);not null;default:0"`   // 最大购买数量
	StepQuantity   int          `json:"step_quantity,omitempty" gorm:"column:step_quantity;type:bigint(20);not null;default:0"` // 购买数量步长
	ProductType    int          `json:"product_type" gorm:"column:product_type;type:tinyint(1);not null;default:1"`             // 1 普通商品，2 wholesale，3 custom，4 vertual，5 service，6 subscription
	OriginPrice    int64        `json:"origin_price" gorm:"column:origin_price;type:bigint(20);not null;default:0"`             // 原价（划线价）
	CostPrice      int64        `json:"cost_price" gorm:"column:cost_price;type:bigint(20);not null;default:0"`                 // 成本价
}

// ToProduct 把扁平的商城字段抽成一条侧表记录。
//
// Archive 上的这 23 个字段只是「转接点」：DB 里它们属于 archive_products，
// 但模板、购物车、运费计算都还在用 a.Brand / a.OptionType 这种写法，所以
// 结构体保留同名字段，由 ToProduct / ApplyProduct 两头搬运。
func (a *Archive) ToProduct() *ArchiveProduct {
	return &ArchiveProduct{
		ArchiveId:      a.Id,
		Brand:          a.Brand,
		VideoUrl:       a.VideoUrl,
		NeedLogistics:  a.NeedLogistics,
		IsFreeShipping: a.IsFreeShipping,
		SoldCount:      a.SoldCount,
		ReviewCount:    a.ReviewCount,
		AverageScore:   a.AverageScore,
		OptionType:     a.OptionType,
		Weight:         a.Weight,
		WeightUnit:     a.WeightUnit,
		Length:         a.Length,
		Width:          a.Width,
		Height:         a.Height,
		Volume:         a.Volume,
		LengthUnit:     a.LengthUnit,
		AllowOversold:  a.AllowOversold,
		OrderFields:    a.OrderFields,
		MinQuantity:    a.MinQuantity,
		MaxQuantity:    a.MaxQuantity,
		StepQuantity:   a.StepQuantity,
		ProductType:    a.ProductType,
		OriginPrice:    a.OriginPrice,
		CostPrice:      a.CostPrice,
	}
}

// ApplyProduct 把侧表记录摊平回 Archive 的 23 个字段。
// p 为 nil 表示这条 archive 在侧表没有行（文章，或属性全默认的商品），
// 此时保持零值——调用方不该把它当成「加载失败」。
//
// 无论 p 是否为 nil 都会置 productLoaded：调用方（provider 的批量回调或
// 本文件的 AfterFind 兜底）已经查过了，AfterFind 不该再查一遍。
func (a *Archive) ApplyProduct(p *ArchiveProduct) {
	a.productLoaded = true
	if p == nil {
		// 侧表没有行 = 商品属性全默认。product_type 要补回库默认值 1（普通商品）：
		// SaveArchive 新建文档时无条件写 1，直接留 0 会让「新建商品不选类型」
		// 在读回来后变成未知类型（P0 的零值护栏测试抓到的就是这一条）。
		a.ProductType = config.ProductTypeNormal
		return
	}
	a.Brand = p.Brand
	a.VideoUrl = p.VideoUrl
	a.NeedLogistics = p.NeedLogistics
	a.IsFreeShipping = p.IsFreeShipping
	a.SoldCount = p.SoldCount
	a.ReviewCount = p.ReviewCount
	a.AverageScore = p.AverageScore
	a.OptionType = p.OptionType
	a.Weight = p.Weight
	a.WeightUnit = p.WeightUnit
	a.Length = p.Length
	a.Width = p.Width
	a.Height = p.Height
	a.Volume = p.Volume
	a.LengthUnit = p.LengthUnit
	a.AllowOversold = p.AllowOversold
	a.OrderFields = p.OrderFields
	a.MinQuantity = p.MinQuantity
	a.MaxQuantity = p.MaxQuantity
	a.StepQuantity = p.StepQuantity
	a.ProductType = p.ProductType
	a.OriginPrice = p.OriginPrice
	a.CostPrice = p.CostPrice
}

// IsDefault 判断这条侧表记录是否「什么都没配」，等价于根本没有这一行。
//
// product_type 单独处理：SaveArchive 新建文档时无条件把 product_type 写成 1
// （DB 默认值，GORM 的 Save 写全列所以必须显式给），所以 1 不能算作「有数据」，
// 否则每篇文章都会在侧表留一行。非 1（批发/定制/虚拟…）则一定是商品配置。
func (p *ArchiveProduct) IsDefault() bool {
	return p.Brand == "" && p.VideoUrl == "" && !p.NeedLogistics && !p.IsFreeShipping &&
		p.SoldCount == 0 && p.ReviewCount == 0 && p.AverageScore == 0 && p.OptionType == 0 &&
		p.Weight == 0 && p.WeightUnit == "" && p.Length == 0 && p.Width == 0 && p.Height == 0 &&
		p.Volume == 0 && p.LengthUnit == "" && !p.AllowOversold && len(p.OrderFields) == 0 &&
		p.MinQuantity == 0 && p.MaxQuantity == 0 && p.StepQuantity == 0 &&
		(p.ProductType == 0 || p.ProductType == config.ProductTypeNormal) &&
		p.OriginPrice == 0 && p.CostPrice == 0
}

// AfterSave 把商品属性同步到 archive_products。
// 为什么用 hook 而不是在 SaveArchive 里显式调用：写档案的入口不止一个
// （后台保存、快速保存、定时发布、采集入库、多语言同步、模型复制），
// 逐个补迟早漏一个。GORM 的 Save → Create/Update 两条链都会经过
// AfterSave，是唯一能覆盖全部 `Save()` 写点的地方。
//
// ⚠️ 覆盖不到 UpdateColumn / UpdateColumns：GORM 对这两个 API 硬写了
// SkipHooks=true（gorm v1.21.6 finisher_api.go:352），改商品字段的原子写
// （sold_count / review_count / average_score）必须在调用处显式同步侧表，
// 见 provider/order.go 与 model/orderReview.go。
func (a *Archive) AfterSave(tx *gorm.DB) error {
	if a == nil || a.Id == 0 {
		return nil
	}
	p := a.ToProduct()
	// 用 NewDB 另起 statement：hook 里直接复用 tx 会把主语句的 Dest/Table
	// 改写成 archive_products，Save 之后跟着执行的代码会读到错表。
	db := tx.Session(&gorm.Session{NewDB: true})
	if p.IsDefault() {
		// 从商品改成文章、或用户把所有商品属性清空——把残留行删掉，
		// 否则下次读出来还是旧配置。
		return db.Where("`archive_id` = ?", a.Id).Delete(&ArchiveProduct{}).Error
	}
	return db.Clauses(clause.OnConflict{UpdateAll: true}).Create(p).Error
}

// AfterFind 是批量回调之外的兜底路径。
//
// Find / First / Take 走 provider 注册的 Query().After 回调，一条 IN 查询搞定
// 整批；但 Scan()（`Table("...").Scan(&list)`、`Raw(...).Scan(&list)`）根本不
// 经过 Query 处理器——GORM 内部是 Row()+ScanRows。这条路径在后台文档列表
// （controller/manageController/archive.go 的 Select("*, 1 as status")）和多个
// 插件里真实存在，漏了就是「列表页商品属性整列变空」，没有任何报错。
//
// 所以再兜一层逐条查询：批量回调填过的实例带 productLoaded 标记，直接跳过，
// 正常路径不会产生 N+1；只有 Scan 出来的实例才会各查一次。
func (a *Archive) AfterFind(tx *gorm.DB) error {
	if a == nil || a.Id == 0 || a.productLoaded {
		return nil
	}
	var p ArchiveProduct
	err := tx.Session(&gorm.Session{NewDB: true}).
		Where("`archive_id` = ?", a.Id).
		Take(&p).Error
	if err != nil {
		// 侧表没有行 = 属性全默认（文章），属于正常情况。
		// 真正的读失败也不往上抛：表还没建好时不能让整站打不开，
		// 商品属性退化成零值，文章照常显示。
		a.productLoaded = true
		return nil
	}
	a.ApplyProduct(&p)
	return nil
}
