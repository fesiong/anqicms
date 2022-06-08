package main

import (
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
)

func main() {
	b := anqicms.New(config.ServerConfig.Port, config.ServerConfig.LogLevel)
	b.Serve()
}
