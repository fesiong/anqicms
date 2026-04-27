package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

func GoogleAuthUrl(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	googleCfg := currentSite.GetGoogleAuthConfig(false)
	if googleCfg == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("GoogleAuthDisable"),
		})
		return
	}
	state := ctx.URLParam("state")
	if state == "" {
		state = library.GenerateRandString(32)
	}
	link := googleCfg.AuthCodeURL(state)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": iris.Map{
			"state": state,
			"url":   link,
		},
	})
}
