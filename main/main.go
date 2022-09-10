package main

import (
	"flag"
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
)

func main() {
	port := flag.Int("port", config.ServerConfig.Port, "运行端口号")
	flag.Parse()
	config.ServerConfig.Port = *port
	b := anqicms.New(config.ServerConfig.Port, config.ServerConfig.LogLevel)
	b.Serve()
}
