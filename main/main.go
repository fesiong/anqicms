//go:build !darwin && !windows

package main

import (
	"flag"
	"fmt"
	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

	checkProcesses()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				fmt.Println("退出", s)
				config.RestartChan <- 2
			default:
				fmt.Println("other", s)
			}
		}
	}()

	b := anqicms.New(config.Server.Server.Port, config.Server.Server.LogLevel)
	b.Serve()
}

func checkProcesses() {
	// 端口没被占用，但是程序启动了
	selfPid := os.Getpid()
	executable, _ := os.Executable()
	binName := filepath.Base(executable)
	cmd := exec.Command("pidof", binName)
	output, err := cmd.Output()
	if err == nil {
		// 有启动
		tmpIds := strings.Split(strings.TrimSpace(string(output)), " ")
		for i := range tmpIds {
			pid, _ := strconv.Atoi(tmpIds[i])
			if pid > 0 && pid != selfPid {
				// kill process
				_ = killProcess(pid)
			}
		}
	}
}

func killProcess(pid int) error {
	pro, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return pro.Signal(syscall.SIGKILL)
}
