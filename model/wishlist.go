package model

type Wishlist struct {
	Id          int64  `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId   int64  `json:"archive_id" gorm:"column:archive_id;type:bigint(20) unsigned not null;default:0;index"`
	SkuId       int64  `json:"sku_id" gorm:"column:sku_id;type:bigint(20) unsigned not null;default:0;index"`
	UserId      int64  `json:"user_id" gorm:"column:user_id;type:bigint(20) unsigned not null;default:0;index"`
	OriginPrice int64  `json:"origin_price" gorm:"column:origin_price;type:bigint(20) unsigned not null;default:0"` // 添加时的价格，不是最新价
	TargetPrice int64  `json:"target_price" gorm:"column:target_price;type:bigint(20) unsigned not null;default:0"` // 期望价格，当商品降价到此价格时，会发送提醒
	Priority    int    `json:"priority" gorm:"column:priority;type:int(10) unsigned not null;default:0"`            // 优先级
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`             // 0 = 隐私，1=公开
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(255)"`                                       // 备注
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
}
