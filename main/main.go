//go:build !darwin && !windows

package main

import (
	"flag"
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"os"
)

func main() {
	port := flag.Int("port", config.Server.Server.Port, "运行端口号")
	flag.Parse()
	config.Server.Server.Port = *port
	// 防止多次启动
	inuse := library.ScanPort("tcp", "", config.Server.Server.Port)
	if inuse {
		//端口被占用，说明已经打开了
		log.Println("端口已经被占用，可能软件已经启动")
		os.Exit(-1)
	}
	b := anqicms.New(config.Server.Server.Port, config.Server.Server.LogLevel)
	b.Serve()
}
