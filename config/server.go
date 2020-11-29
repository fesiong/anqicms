package config

type serverConfig struct {
	SiteName string `json:"site_name"`
	Env      string `json:"env"`
	Port     int    `json:"port"`
	LogLevel string `json:"log_level"`
}
