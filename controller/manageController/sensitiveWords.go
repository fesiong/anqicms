package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// SettingSensitiveWords 获取敏感词设置。
func SettingSensitiveWords(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	sensitiveWords := currentSite.SensitiveWords

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": sensitiveWords,
	})
}

// SettingSensitiveWordsForm 保存敏感词设置并重新加载敏感词。
// 前端展示时，敏感词将会被替换成*。
// 参数为敏感词数组，敏感词支持三种模式：
//   - 纯敏感词，如：第一
//   - 带替换词的词使用竖线分隔，如：第一|很好
//   - 用花括号包裹的正则表达式，如：{\d{6,12}}
//
// 参数说明：
//   - 请求体: 敏感词字符串数组。
func SettingSensitiveWordsForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SensitiveWordsRequest
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
	// reload
	w2.LoadSensitiveWords("")

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSensitiveWordConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingSensitiveWordsCheck 检查指定内容中命中的敏感词。
//
// 参数说明：
//   - 请求体 "content": 待检查的内容。
//   - 请求体 "title": 待检查的标题。
func SettingSensitiveWordsCheck(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SensitiveWordsCheckRequest
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

// SettingSensitiveWordsSync 从安企云服务同步敏感词列表
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
