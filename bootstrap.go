package irisweb

import (
	"context"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/rrylee/go-graceful"
	"irisweb/config"
	"irisweb/middleware"
	"irisweb/model"
	"irisweb/route"
	"log"
	"net"
	"time"
)

type Bootstrap struct {
	Application *iris.Application
	Port        int
	LoggerLevel string
}

func New(port int, loggerLevel string) *Bootstrap {
	var bootstrap Bootstrap
	bootstrap.Application = iris.New()
	bootstrap.Port = port
	bootstrap.LoggerLevel = loggerLevel

	return &bootstrap
}

func (bootstrap *Bootstrap) loadGlobalMiddleware() {
	bootstrap.Application.Use(recover.New())
	bootstrap.Application.Use(middleware.Cors)
	bootstrap.Application.Use(middleware.Auth)
}

func (bootstrap *Bootstrap) Serve() {
	//自动迁移表
	if config.DB != nil {
		_ = model.AutoMigrateDB(config.DB)
	}

	bootstrap.Application.Logger().SetLevel(bootstrap.LoggerLevel)
	bootstrap.loadGlobalMiddleware()
	route.Register(bootstrap.Application)

	pugEngine := iris.Django("./template", ".html")
	if config.ServerConfig.Env == "development" {
		//测试环境下动态加载
		pugEngine.Reload(true)
	}

	pugEngine.AddFunc("stampToDate", TimestampToDate)
	bootstrap.Application.RegisterView(pugEngine)

	grace := graceful.New()
	grace.RegisterService(graceful.NewAddress(fmt.Sprintf("127.0.0.1:%d", bootstrap.Port), "tcp"), func(ln net.Listener) error {
		return bootstrap.Application.Run(
			iris.Listener(ln),
			iris.WithoutServerError(iris.ErrServerClosed),
			iris.WithoutBodyConsumptionOnUnmarshal,
		)
	}, func() error {
		return bootstrap.Application.Shutdown(context.Background())
	})
	err := grace.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func TimestampToDate(in int64, layout string) string {
	t := time.Unix(in, 0)
	return t.Format(layout)
}

func (bootstrap *Bootstrap) Shutdown() error {
	bootstrap.Application.Shutdown(context.Background())

	return nil
}
