package model

type Commission struct {
	Model
	UserId      uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`                // 收益的用户
	OrderId     string `json:"order_id" gorm:"column:order_id;type:varchar(32) not null;default:'';index"`                  // 关联的OrderId
	OrderAmount int64  `json:"order_amount" gorm:"column:order_amount;type:bigint(20) not null;default:0"`                  // 订单金额
	Amount      int64  `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0"`                              // 获得佣金金额
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`                              //0-未提现 1-已提现，-1 其他原因导致退款
	WithdrawId  uint   `json:"withdraw_id" gorm:"column:withdraw_id;type:int(10) not null;default:0;index:idx_withdraw_id"` // 提现订单id
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(250) not null;default:''"`
	Order       *Order `json:"order" gorm:"-"`
	UserName    string `json:"user_name" gorm:"-"`
	CanWithdraw bool   `json:"can_withdraw" gorm:"-"`
}
