package config

type AnqiUserConfig struct {
	UserName   string `json:"user_name"`
	AuthId     uint   `json:"auth_id"`
	LoginTime  int64  `json:"login_time"`
	CheckTime  int64  `json:"check_time"`
	ExpireTime int64  `json:"expire_time"`
	AiRemain   int64  `json:"ai_remain"`
	Integral   int64  `json:"integral"`
	Status     int    `json:"status"`
	Token      string `json:"token"`

	Valid bool `json:"valid"`
}
