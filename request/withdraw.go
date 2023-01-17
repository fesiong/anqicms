package request

type UserWithdrawRequest struct {
	Id          uint   `json:"id"`
	UserId      uint   `json:"user_id"`
	Amount      int64  `json:"amount"`       // 提现金额
	SuccessTime int64  `json:"success_time"` // 成功时间
	WithdrawWay int    `json:"withdraw_way"` // 提现去向，1 微信提现，
	Status      int    `json:"status"`       //0-等待处理 1-已提现，-1 提现错误
	ErrorTimes  int    `json:"error_times"`  // 执行错误次数
	LastTime    int64  `json:"last_time"`    // 上次执行时间
	Remark      string `json:"remark"`
	UserName    string `json:"user_name"`
}
