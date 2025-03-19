package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func SettingSensitiveWords(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	sensitiveWords := currentSite.SensitiveWords

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": sensitiveWords,
	})
}

func SettingSensitiveWordsForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req []string
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.SensitiveWords = req
	w2 := provider.GetWebsite(currentSite.Id)
	w2.SensitiveWords = req

	err := currentSite.SaveSettingValue(provider.SensitiveWordsKey, w2.SensitiveWords)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSensitiveWordConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func SettingSensitiveWordsCheck(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
	currentSite := provider.CurrentSubSite(ctx)

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
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}
