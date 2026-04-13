package model

type IntegralLog struct {
	Id            uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime   int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime   int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	UserId        uint   `json:"user_id" gorm:"column:user_id;type:bigint(20) not null;default:0;index"`
	Integral      int64  `json:"integral" gorm:"column:integral;type:bigint(20) not null;default:0"` //积分对应金钱： 1积分 等于1分钱
	AfterIntegral int64  `json:"after_integral" gorm:"column:after_integral;type:bigint(20) not null;default:0"`
	OrderId       string `json:"order_id" gorm:"column:order_id;type:varchar(32) not null;default:'';index"`
	Remark        string `json:"remark" gorm:"column:remark;type:varchar(250) not null;default:''"`
	Status        int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	Type          int    `json:"type" gorm:"column:type;type:tinyint(4) not null;default:0"`
	TaskId        int    `json:"task_id" gorm:"column:task_id;type:int(10) not null;default:0"`
	UserName      string `json:"user_name" gorm:"-"`
}
