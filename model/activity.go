package model

import "github.com/lib/pq"

// 营销功能表
type Activity struct {
	Id           int64          `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime  int64          `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime  int64          `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	GoodsType    string         `json:"goods_type" gorm:"column:goods_type;type:varchar(16) not null;default:'';index:idx_goods_type"`
	StartTime    int64          `json:"start_time" gorm:"column:start_time;type:int(11) not null;default:0;index:idx_start_time"`
	EndTime      int64          `json:"end_time" gorm:"column:end_time;type:int(11) not null;default:0;index:idx_end_time"`
	UserLimit    int64          `json:"user_limit" gorm:"column:user_limit;type:int(11) not null;default:0;index:idx_user_limit"`
	Title        string         `json:"title" gorm:"column:title;type:varchar(100) default null"`                                    // 活动名称
	Description  string         `json:"description" gorm:"column:description;type:varchar(1000) default null"`                       // 活动介绍
	Images       pq.StringArray `json:"images" gorm:"column:images;type:text default null"`                                          // 活动宣传图片
	GoodsId      int64          `json:"goods_id" gorm:"column:goods_id;type:int(10) unsigned not null;default:0;index:idx_goods_id"` // 关联商品id
	Price        int64          `json:"price" gorm:"column:price;type:int(10) unsigned not null;default:0"`                          // 活动价格
	Stock        int64          `json:"stock" gorm:"column:stock;type:int(10) unsigned not null;default:0"`                          // 活动库存
	SoldCount    int64          `json:"sold_count" gorm:"column:sold_count;type:bigint(20) unsigned not null;default:0"`             // 销量
	Extra        ExtraData      `json:"extra,omitempty" gorm:"column:extra;type:longtext default null"`                              // 附加设置内容
	CanUseCoupon bool           `json:"can_use_coupon" gorm:"column:can_use_coupon;type:tinyint(1) not null;default:0"`              // 是否支持其他优惠
	Status       uint           `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0;index:idx_status"`             // 活动状态，1正常，0失效

	Skus  []*ActivitySku `json:"skus" gorm:"-"`
	Goods *Archive       `json:"goods" gorm:"-"`
}

// 活动商品规格表
type ActivitySku struct {
	Id          int64 `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64 `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	ActivityId  int64 `json:"activity_id" gorm:"column:activity_id;type:int(10) unsigned not null;default:0;index:idx_activity_id"` // 关联的活动id
	SkuId       int64 `json:"sku_id" gorm:"column:sku_id;type:int(10) unsigned not null;default:0"`                                 // 关联的商品规格id
	GoodsId     int64 `json:"goods_id" gorm:"column:goods_id;type:int(10) unsigned not null;default:0"`                             // 关联的商品id
	Price       int64 `json:"price" gorm:"column:price;type:int(10) unsigned not null;default:0"`                                   // 活动价格
	Stock       int64 `json:"stock" gorm:"column:stock;type:int(10) unsigned not null;default:0"`                                   // 活动库存
	SoldCount   int64 `json:"sold_count" gorm:"column:sold_count;type:bigint(20) unsigned not null;default:0"`                      // 销量

	Sku *ArchiveSku `json:"sku" gorm:"-"`
}
