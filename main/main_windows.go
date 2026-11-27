//go:build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
)

func main() {
	port := flag.Int("port", config.Server.Server.Port, "运行端口号")
	flag.Parse()
	config.Server.Server.Port = *port

	inuse := library.ScanPort("tcp", "", strconv.Itoa(config.Server.Server.Port))
	if inuse {
		//端口被占用，说明已经打开了
		log.Println("端口已经被占用，可能软件已经启动")
		_ = open.Run("http://127.0.0.1:" + strconv.Itoa(config.Server.Server.Port))

		os.Exit(-1)
	}

	b := anqicms.New(config.Server.Server.Port, config.Server.Server.LogLevel)
	go b.Serve()

	systray.Run(onReady, onExit)
}

func onReady() {
	// 托盘菜单是桌面版唯一的常驻界面，品牌与链接全部取自编译期注入的品牌配置：
	// 漏一处就是客户天天看到官方名字。
	brand := config.Brand()
	systray.SetTemplateIcon(anqicms.AppIcon, anqicms.AppIcon)
	systray.SetIcon(anqicms.AppIcon)
	systray.SetTitle(brand.NameCN)
	about := systray.AddMenuItem("关于", brand.NameCN)
	systray.AddSeparator()
	openMain := systray.AddMenuItem("后台管理", "打开网站后台管理页面")
	var openHelp *systray.MenuItem
	if brand.Docs != "" {
		openHelp = systray.AddMenuItem("使用帮助", "打开使用帮助")
	}
	openOrigination := systray.AddMenuItem("访问官网", brand.Website)
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出"+brand.NameCN)

	go func() {
		for {
			select {
			case <-about.ClickedCh:
				_ = open.Run(brand.Website + "/about.html")
			case <-openMain.ClickedCh:
				_ = open.Run(fmt.Sprintf("http://127.0.0.1:%d/system/", config.Server.Server.Port))

			case <-helpClicked(openHelp):
				_ = open.Run(brand.Docs)

			case <-openOrigination.ClickedCh:
				_ = open.Run(brand.Website)

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// helpClicked 在没有帮助入口时返回一个永不触发的通道，
// 避免 select 里对 nil 菜单项取值 panic。
func helpClicked(item *systray.MenuItem) chan struct{} {
	if item != nil {
		return item.ClickedCh
	}
	return nil
}

func onExit() {
	// clean up here
	log.Println("退出程序")
}
