package model

// 订单附加服务定义表
type OrderService struct {
	Id          int64  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	ServiceCode string `json:"service_code" gorm:"column:service_code;type:varchar(255);not null;unique"`
	ServiceName string `json:"service_name" gorm:"column:service_name;type:varchar(255);not null"`
	ServiceType string `json:"service_type" gorm:"column:service_type;type:varchar(255);not null"`
	Description string `json:"description" gorm:"column:description;type:text;default null"`
	Enabled     bool   `json:"enabled" gorm:"column:enabled;type:tinyint(1) not null;default:0"`
	Sort        uint   `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:0;index:idx_sort"`
	Fee         int64  `json:"fee" gorm:"column:fee;type:bigint(20) not null;default:0"`         // 费用
	FeeType     int    `json:"fee_type" gorm:"column:fee_type;type:tinyint(4)"`                  // 0 = 固定金额， 2 = 订单金额比例
	MaxFee      int64  `json:"max_fee" gorm:"column:max_fee;type:bigint(20) not null;default:0"` // 最高金额
	// 其它配置，用于高度自定义配置内容
	Extra ExtraData `json:"extra,omitempty" gorm:"column:extra;type:longtext default null"`
}

// 订单附加服务关联
type OrderServiceAddon struct {
	Id          int64  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	OrderId     string `json:"order_id" gorm:"column:order_id;type:varchar(36);not null;index"`
	ServiceId   int64  `json:"service_id" gorm:"column:service_id;type:int(10) unsigned not null;default:0"`
	ServiceCode string `json:"service_code" gorm:"column:service_code;type:varchar(255);not null;unique"`
	ServiceFee  int64  `json:"service_fee" gorm:"column:service_fee;type:bigint(20) not null;default:0"`

	OrderService *OrderService `json:"order_service" gorm:"-"`
}
