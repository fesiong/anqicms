package model

type ProductSyncStatus struct {
	Id              int64  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime     int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime     int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	MerchantId      uint64 `json:"merchant_id" gorm:"column:merchant_id;type:bigint(20) unsigned not null;index"`
	ProductId       int64  `json:"product_id" gorm:"column:product_id;type:int(10) unsigned not null;default:0;index"` // 关联商品id
	SkuId           int64  `json:"sku_id" gorm:"column:sku_id;type:int(10) unsigned not null;default:0"`               // 关联的skuID
	GoogleProductId string `json:"google_product_id" gorm:"column:google_product_id;type:varchar(100) default null"`
	GoogleStatus    string `json:"google_status" gorm:"column:google_status;type:varchar(16) not null;default:''"`
	LastSyncTime    int64  `json:"last_sync_time" gorm:"column:last_sync_time;type:int(11);default:0"`
	LastMessage     string `json:"last_message" gorm:"column:last_message;type:varchar(250) default null"`
	LastSuccessTime int64  `json:"last_success_time" gorm:"column:last_success_time;type:int(11);default:0"`

	Archive *Archive `json:"archive" gorm:"-"`
}

type ProductSyncLog struct {
	Id          int64  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	MerchantId  uint64 `json:"merchant_id" gorm:"column:merchant_id;type:bigint(20) unsigned not null;index"`
	SyncType    string `json:"sync_type" gorm:"column:sync_type;type:varchar(32) not null;default:''"`
	StartId     int64  `json:"start_id" gorm:"column:start_id;type:int(10) unsigned not null;default:0"`
	EndId       int64  `json:"end_id" gorm:"column:end_id;type:int(10) unsigned not null;default:0"`
	Status      int    `json:"status" gorm:"column:status;type:int(1) not null;default:0"` // 同步状态，0-初始化，1-同步中，2-同步完成，3-同步失败
	Message     string `json:"message" gorm:"column:message;type:varchar(1000) default null"`
}
