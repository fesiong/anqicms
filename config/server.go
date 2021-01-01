package config

type serverConfig struct {
	SiteName    string `json:"site_name"`
	Env         string `json:"env"`
	Port        int    `json:"port"`
	LogLevel    string `json:"log_level"`
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	Icp         string `json:"icp"`
}
