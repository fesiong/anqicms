package config

type AnqiUserConfig struct {
	UserName  string `json:"user_name"`
	AuthId    uint   `json:"auth_id"`
	LoginTime int64  `json:"login_time"`
	CheckTime int64  `json:"check_time"`
	Token     string `json:"token"`
}
