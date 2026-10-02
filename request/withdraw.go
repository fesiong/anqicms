package request

type UserWithdrawRequest struct {
	Id          uint   `json:"id"`           // 记录 ID
	UserId      uint   `json:"user_id"`      // 用户 ID
	Amount      int64  `json:"amount"`       // 提现金额
	SuccessTime int64  `json:"success_time"` // 成功时间
	WithdrawWay int    `json:"withdraw_way"` // 提现去向，1 微信提现，
	Status      int    `json:"status"`       // 状态：0=等待处理 1=已提现，-1=提现错误
	ErrorTimes  int    `json:"error_times"`  // 执行错误次数
	LastTime    int64  `json:"last_time"`    // 上次执行时间
	Remark      string `json:"remark"`       // 备注
	UserName    string `json:"user_name" ast:"-"`
}

type UserWithdrawApplyRequest struct {
	UserId uint `json:"user_id"`
}

type UserWithdrawApprovalRequest struct {
	Id uint `json:"id"` // 记录 ID
}
