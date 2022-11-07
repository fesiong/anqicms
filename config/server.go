package config

type ServerConfig struct {
	SiteName    string `json:"site_name"`
	Env         string `json:"env"`
	Port        int    `json:"port"`
	LogLevel    string `json:"log_level"`
	TokenSecret string `json:"token_secret"`
}
