package config

type AnqiUserConfig struct {
	UserName    string `json:"user_name"`
	AuthId      uint   `json:"auth_id"`
	LoginTime   int64  `json:"login_time"`
	CheckTime   int64  `json:"check_time"`
	CreatedTime int64  `json:"created_time"`
	HashKey     string `json:"hash_key"`
	ExpireTime  int64  `json:"expire_time"`
	Integral    int64  `json:"integral"`
	Status      int    `json:"status"`
	Token       string `json:"token"`
	// 付费和免费额度展示
	FreeToken  int64 `json:"free_token"`   // 免费额度
	TotalToken int64 `json:"total_token"`  // 累计使用额度
	UnPayToken int64 `json:"un_pay_token"` // 未支付额度
	IsOweFee   int   `json:"is_owe_fee"`   // 是否欠费 0 = 没欠费，1=欠费

	Valid bool `json:"valid"`
}
