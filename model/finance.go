package model

type Finance struct {
	Model
	UserId      uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"` // 用户
	Direction   int    `json:"direction" gorm:"column:direction;type:tinyint(1) not null;default:0"`         // 方向
	Amount      int64  `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0;comment:'金额'"`
	AfterAmount int64  `json:"after_amount" gorm:"column:after_amount;type:bigint(20) not null;default:0;comment:'变更后用户金额'"`
	Action      int    `json:"action" gorm:"column:action;type:tinyint(1) not null;default:0;comment:'资金类型，1出售，2购买，3退款，4充值，5提现，6推广，7返现,8佣金'"`
	OrderId     string `json:"order_id" gorm:"column:order_id;type:varchar(32) not null;default:'';index"` // 关联的OrderId
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`             //0-未提现 1-已提现，-1 其他原因导致退款
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(250) not null;default:''"`
	UserName    string `json:"user_name" gorm:"-"`
}
