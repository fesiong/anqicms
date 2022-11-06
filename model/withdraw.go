package model

type UserWithdraw struct {
	Model
	UserId      uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Amount      int64  `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0"`             // 提现金额
	SuccessTime int64  `json:"success_time" gorm:"column:success_time;type:int(11);default:0"`             // 成功时间
	WithdrawWay int    `json:"withdraw_way" gorm:"column:withdraw_way;type:tinyint(1) not null;default:0"` // 提现去向，1 微信提现，
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`             //0-等待处理 1-已同意 2-已提现，-1 提现错误
	ErrorTimes  int    `json:"error_times" gorm:"column:error_times;type:int(10) not null;default:0"`      // 执行错误次数
	LastTime    int64  `json:"last_time" gorm:"column:last_time;type:int(10) not null;default:0"`          // 上次执行时间
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(250) not null;default:''"`
	UserName    string `json:"user_name" gorm:"-"`
}
