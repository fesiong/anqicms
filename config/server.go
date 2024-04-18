package config

type ServerConfig struct {
	Env      string `json:"env"`
	Port     int    `json:"port"`
	LogLevel string `json:"log_level"`
}
