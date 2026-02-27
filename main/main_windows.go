//go:build windows

package main

import (
	"flag"
	"fmt"
	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"os"
	"strconv"
)

func main() {
	port := flag.Int("port", config.Server.Server.Port, "运行端口号")
	flag.Parse()
	config.Server.Server.Port = *port

	inuse := library.ScanPort("tcp", "", strconv.Itoa(config.Server.Server.Port))
	if inuse {
		//端口被占用，说明已经打开了
		log.Println("端口已经被占用，可能软件已经启动")
		_ = open.Run(fmt.Sprintf("http://127.0.0.1:%d", config.Server.Server.Port))

		os.Exit(-1)
	}

	b := anqicms.New(config.Server.Server.Port, config.Server.Server.LogLevel)
	go b.Serve()

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTemplateIcon(anqicms.AppIcon, anqicms.AppIcon)
	systray.SetIcon(anqicms.AppIcon)
	systray.SetTitle("安企CMS")
	about := systray.AddMenuItem("关于", "安企CMS")
	systray.AddSeparator()
	openMain := systray.AddMenuItem("后台管理", "打开网站后台管理页面")
	openHelp := systray.AddMenuItem("使用帮助", "打开使用帮助")
	openOrigination := systray.AddMenuItem("访问官网", "https://www.anqicms.com")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出安企CMS")

	go func() {
		for {
			select {
			case <-about.ClickedCh:
				_ = open.Run("https://www.anqicms.com/about.html")
			case <-openMain.ClickedCh:
				_ = open.Run(fmt.Sprintf("http://127.0.0.1:%d/system/", config.Server.Server.Port))

			case <-openHelp.ClickedCh:
				_ = open.Run("https://www.anqicms.com/help")

			case <-openOrigination.ClickedCh:
				_ = open.Run("https://www.anqicms.com/")

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	// clean up here
	log.Println("退出程序")
}
