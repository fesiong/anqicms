package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func SettingSensitiveWords(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	sensitiveWords := currentSite.SensitiveWords

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": sensitiveWords,
	})
}

func SettingSensitiveWordsForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req []string
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.SensitiveWords = req

	err := currentSite.SaveSettingValue(provider.SensitiveWordsKey, currentSite.SensitiveWords)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新敏感词配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingSensitiveWordsCheck(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	matches := currentSite.MatchSensitiveWords(req.Content)
	matches2 := currentSite.MatchSensitiveWords(req.Title)
	if len(matches2) > 0 {
		matches = append(matches, matches2...)
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": matches,
	})
}

func SettingSensitiveWordsSync(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	err := currentSite.AnqiSyncSensitiveWords()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
