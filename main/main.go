//go:build !windows

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kandaoni.com/anqicms"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

func main() {
	port := flag.Int("port", config.Server.Server.Port, "运行端口号")

	// 离线迁移（把迁移从启动流程里剥出来）：
	// 106 个站点升级要几十分钟到几小时，混在启动里就只能"重启—没起完—再重启"，
	// 每次重启都从被中断的站点重来。这几个开关让客户可以分批、可以预演：
	//   ./anqicms -migrate-only -migrate-dry-run          # 先看哪些站点需要迁移
	//   ./anqicms -migrate-only -migrate-sites 1,3,9-20   # 分批跑
	//   ./anqicms -migrate-only -migrate-concurrency 1    # 库扛不住时降到 1
	migrateOnly := flag.Bool("migrate-only", false, "只执行站点库表迁移，完成后退出（不启动服务、不检查端口）")
	migrateDryRun := flag.Bool("migrate-dry-run", false, "配合 -migrate-only：只列出需要迁移的站点，不执行任何 DDL")
	migrateSites := flag.String("migrate-sites", "", "配合 -migrate-only：只处理这些站点 ID，如 1,3,9-20")
	migrateFrom := flag.Int("migrate-from", 0, "配合 -migrate-only：只处理 id 大于等于该值的站点")
	migrateLimit := flag.Int("migrate-limit", 0, "配合 -migrate-only：最多处理这么多个站点（分批）")
	migrateConcurrency := flag.Int("migrate-concurrency", 0, "配合 -migrate-only：同时迁移的站点数，默认 2")

	flag.Parse()
	config.Server.Server.Port = *port

	if *migrateOnly {
		// 迁移只碰数据库、不占端口，因此不走 ScanPort / checkProcesses：
		// 客户很可能是在服务还开着的时候先把迁移分批跑掉。
		applyEnvMysqlConfig()
		os.Exit(runMigrateOnly(*migrateSites, *migrateFrom, *migrateLimit, *migrateConcurrency, *migrateDryRun))
	}

	// 防止多次启动
	inuse := library.ScanPort("tcp", "", strconv.Itoa(config.Server.Server.Port))
	if inuse {
		//端口被占用，说明已经打开了
		log.Println("端口已经被占用，可能软件已经启动")
		os.Exit(-1)
	}

	checkProcesses()

	// 带缓冲，避免信号在 goroutine 启动前到达时丢失
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				fmt.Println("退出", s)
				config.RestartChan <- config.RestartConfig{Code: 2, SiteId: 0}
			default:
				fmt.Println("other", s)
			}
		}
	}()
	// env
	if applyEnvMysqlConfig() {
		db, err := provider.InitDB(&config.Server.Mysql)
		if err == nil && db.Migrator().HasTable(&model.Website{}) == false {
			provider.SetDefaultDB(db)
			provider.InitWebsites()
			website := provider.GetWebsite(1)
			// 安装默认数据
			if website != nil {
				err = website.RestoreDesignData(website.System.TemplateName)
			}
		}
	}

	if config.Server.Server.Env == "development" {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	// 生产环境默认不开放 pprof；排查性能问题时用环境变量显式打开，且只绑内网回环，
	// 排查完直接去掉环境变量重启即可，不需要重新编译。
	if addr := os.Getenv("ANQI_PPROF_ADDR"); addr != "" {
		go func() {
			log.Println("pprof 已开启：", addr)
			_ = http.ListenAndServe(addr, nil)
		}()
	}

	b := anqicms.New(config.Server.Server.Port, config.Server.Server.LogLevel)
	b.Serve()
}

func checkProcesses() {
	// 端口没被占用，但是程序启动了
	selfPid := os.Getpid()
	executable, _ := os.Executable()
	cmd := exec.Command("pgrep", "-f", executable)
	output, err := cmd.Output()
	if err == nil {
		// 有启动
		// pgrep 输出按行分隔 PID，用 Fields 兼容空格/换行
		tmpIds := strings.Fields(string(output))
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

// applyEnvMysqlConfig 把容器编排注入的 MYSQL_* 落到 config.Server.Mysql。
// 返回是否应用了环境变量配置（没配齐就沿用 config.json）。
func applyEnvMysqlConfig() bool {
	envDb := os.Getenv("MYSQL_DATABASE")
	envHost := os.Getenv("MYSQL_HOST")
	envUser := os.Getenv("MYSQL_USER")
	if envUser == "" {
		envUser = "root"
	}
	envPwd := os.Getenv("MYSQL_ROOT_PASSWORD")
	if envPwd == "" {
		envPwd = os.Getenv("MYSQL_PASSWORD")
	}
	envPort := os.Getenv("MYSQL_PORT")
	if len(envDb) == 0 || len(envHost) == 0 || len(envPwd) == 0 {
		return false
	}
	envPort2, _ := strconv.Atoi(envPort)
	if envPort2 == 0 {
		envPort2 = 3306
	}
	config.Server.Mysql = config.MysqlConfig{
		Database: envDb,
		User:     envUser,
		Password: envPwd,
		Host:     envHost,
		Port:     envPort2,
	}
	_ = config.WriteConfig()
	return true
}

// runMigrateOnly 执行离线迁移并返回进程退出码（0 成功，1 有站点失败，2 参数/环境错误）。
func runMigrateOnly(spec string, from, limit, concurrency int, dryRun bool) int {
	ids, err := provider.ParseSiteIDSpec(spec)
	if err != nil {
		log.Println("参数错误：", err)
		return 2
	}
	if provider.GetDefaultDB() == nil {
		log.Println("默认数据库未初始化：请检查 config.json 的 mysql 配置或 MYSQL_* 环境变量")
		return 2
	}
	log.Printf("开始离线迁移：预演=%v 指定站点=%v 起始站点=%d 上限=%d 并发=%d",
		dryRun, ids, from, limit, concurrency)
	res, mErr := provider.MigrateAllSites(provider.SiteMigrateOptions{
		Only:        ids,
		FromID:      from,
		Limit:       limit,
		DryRun:      dryRun,
		Concurrency: concurrency,
	})
	log.Printf("离线迁移结束：候选 %d 个站点，待迁移 %d 个，成功 %d 个，失败 %d 个，耗时 %s",
		res.Candidates, res.Pending, res.Migrated, res.Failed, res.Duration.Round(time.Second))
	if dryRun {
		log.Println("预演模式：未执行任何 DDL。去掉 -migrate-dry-run 才会真正迁移。")
	}
	if mErr != nil {
		log.Println("离线迁移有失败项：", mErr)
		return 1
	}
	return 0
}
