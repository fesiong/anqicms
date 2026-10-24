package model

type Cart struct {
	Id           int64               `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId    int64               `json:"archive_id" gorm:"column:archive_id;type:bigint(20) unsigned not null;default:0;index"`
	SkuId        int64               `json:"sku_id" gorm:"column:sku_id;type:bigint(20) unsigned not null;default:0;index"`
	SkuValues    ArchiveOptionValues `json:"sku_values" gorm:"column:sku_values;type:text default null"` // SKU 组合字段
	UserId       int64               `json:"user_id" gorm:"column:user_id;type:bigint(20) unsigned not null;default:0;index"`
	Token        string              `json:"token" gorm:"column:token;type:varchar(64) not null;default:'';index"` // cart  token , 用户没登录的时候使用，登录的时候使用用户id
	Quantity     int                 `json:"quantity" gorm:"column:quantity;type:int(10) not null;default:0"`
	OriginPrice  int64               `json:"origin_price" gorm:"column:origin_price;type:bigint(20) unsigned not null;default:0"` // 添加时的价格，不是最新价
	Status       int                 `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`             // 0 = 隐私，1=公开
	CreatedTime  int64               `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime  int64               `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	OrderExtra   ExtraData           `json:"order_extra,omitempty" gorm:"column:order_extra;type:longtext default null"` // 订单自定义字段
	Archive      *Archive            `json:"archive,omitempty" gorm:"-"`
	Sku          *ArchiveSku         `json:"sku,omitempty" gorm:"-"`
	Price        int64               `json:"price,omitempty" gorm:"-"` // 当前价格
	Amount       int64               `json:"amount,omitempty" gorm:"-"`
	OriginAmount int64               `json:"origin_amount,omitempty" gorm:"-"`
}
