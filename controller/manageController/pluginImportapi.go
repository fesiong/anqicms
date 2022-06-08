package manageController

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"time"
)

func PluginImportApi(ctx iris.Context) {
	importApi := config.JsonData.PluginImportApi

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"token":    importApi.Token,
			"base_url": config.JsonData.System.BaseUrl,
		},
	})
}

func PluginUpdateApiToken(ctx iris.Context) {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%d", time.Now().Nanosecond())))
	config.JsonData.PluginImportApi.Token = hex.EncodeToString(h.Sum(nil))
	// 回写
	_ = config.WriteConfig()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "Token已更新",
	})
}