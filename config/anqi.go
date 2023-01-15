package config

type AnqiUserConfig struct {
	UserName        string `json:"user_name"`
	AuthId          uint   `json:"auth_id"`
	LoginTime       int64  `json:"login_time"`
	CheckTime       int64  `json:"check_time"`
	ExpireTime      int64  `json:"expire_time"`
	PseudoRemain    int64  `json:"pseudo_remain"`
	TranslateRemain int64  `json:"translate_remain"`
	FreeTransRemain int64  `json:"free_trans_remain"`
	Status          int    `json:"status"`
	Token           string `json:"token"`

	Valid bool `json:"valid"`
}
