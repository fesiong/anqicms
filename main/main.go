//go:build !darwin && !windows

package main

import (
	"flag"
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
)

func main() {
	port := flag.Int("port", config.Server.Server.Port, "运行端口号")
	flag.Parse()
	config.Server.Server.Port = *port
	b := anqicms.New(config.Server.Server.Port, config.Server.Server.LogLevel)
	b.Serve()
}
