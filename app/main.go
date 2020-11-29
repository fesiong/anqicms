package main

import (
	"irisweb"
	"irisweb/config"
)

func main() {
	b := irisweb.New(config.ServerConfig.Port, config.ServerConfig.LogLevel)
	b.Serve()
}
