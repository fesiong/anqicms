package manageController

import (
	"os"
	"strconv"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGetTranslateConfig 获取翻译插件配置。
func PluginGetTranslateConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginTranslate

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveTranslateConfig 保存翻译插件配置。
func PluginSaveTranslateConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginTranslateConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	w2 := provider.GetWebsite(currentSite.Id)
	w2.PluginTranslate = &req
	err := currentSite.SaveSettingValue(provider.TranslateSettingKey, w2.PluginTranslate)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateTranslateConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginTranslateLogList 分页获取翻译日志列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
func PluginTranslateLogList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	var total int64
	var logs []*model.TranslateLog
	tx := currentSite.DB.Model(&model.TranslateLog{})
	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}

// PluginGetTranslateTextLog 分页获取文本翻译日志列表，支持按原文和译文前缀筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "text": 按原文前缀筛选。
//   - 查询参数 "translated": 按译文前缀筛选。
func PluginGetTranslateTextLog(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	text := ctx.URLParam("text")
	translated := ctx.URLParam("translated")

	var total int64
	var logs []*model.TranslateTextLog
	tx := currentSite.DB.Model(&model.TranslateTextLog{})
	if text != "" {
		tx = tx.Where("text like ?", text+"%")
	}
	if translated != "" {
		tx = tx.Where("translated like ?", translated+"%")
	}
	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}

// PluginRemoveTranslateTextLog 删除文本翻译日志，支持删除单条或全部。
//
// 参数说明：
//   - 请求体 "id": 要删除的日志ID，大于 0 时删除该条。
//   - 请求体 "all": 是否删除全部文本翻译日志。
func PluginRemoveTranslateTextLog(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.TranslateTextLogDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.All {
		// 删除所有
		currentSite.DB.Where("`id` > 0").Delete(model.TranslateTextLog{})
	} else if req.Id > 0 {
		// 必须按模型删除：此前误传请求 DTO &req，GORM 据此推导出
		// translate_text_log_delete_requests 这张不存在的表，单条删除恒报 1146。
		//
		// 先确认记录存在：GORM 的 Delete 对不存在的 id 是**0 行受影响但不报错**，
		// 于是 id=999999 会「删除成功」地返回——调用方以为记录已删，实际什么都没发生。
		var exist int64
		if err := currentSite.DB.Model(&model.TranslateTextLog{}).
			Where("`id` = ?", req.Id).Count(&exist).Error; err != nil {
			ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": err.Error()})
			return
		}
		if exist == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "翻译记录 id=" + strconv.FormatUint(uint64(req.Id), 10) + " 不存在，未做任何修改",
			})
			return
		}
		err := currentSite.DB.Where("`id` = ?", req.Id).Delete(model.TranslateTextLog{}).Error
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// 既没给 all 也没给 id：什么都不删却回「已删除」，是最容易骗过调用人的形态。
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请提供要删除的 id，或设置 all=true 删除全部翻译记录（当前未做任何修改）",
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteTranslateLog"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("Deleted"),
	})
}

// PluginSaveTranslateTextLog 添加/更新文本翻译，用于修正自动翻译错误的问题。
func PluginSaveTranslateTextLog(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.TranslateTextLog
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Text == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheText"),
		})
		return
	}
	if req.Language == "" {
		req.Language = currentSite.System.Language
	}
	textMd5 := library.Md5(req.Language + "-" + req.ToLanguage + "-" + req.Text)
	var textLog model.TranslateTextLog
	var originText string = req.Text
	if req.Id > 0 {
		err := currentSite.DB.Where("id = ?", req.Id).First(&textLog).Error
		if err == nil {
			// 更新
			originText = textLog.Translated
		}
	} else {
		if err := currentSite.DB.Where("`md5` = ?", textMd5).First(&textLog).Error; err == nil {
			// 更新
			originText = textLog.Translated
		}
	}
	if textLog.Id > 0 {
		textLog.Translated = req.Translated
		_ = currentSite.DB.Save(&textLog).Error
	} else {
		textLog.Language = req.Language
		textLog.ToLanguage = req.ToLanguage
		textLog.Text = req.Text
		textLog.Translated = req.Translated
		textLog.Md5 = textMd5
		_ = currentSite.DB.Create(&textLog).Error
	}
	// 更新了，则进行全局替换
	if originText != "" && originText != req.Translated {
		cacheKey := "translate-texts-" + req.ToLanguage
		currentSite.Cache.Delete(cacheKey)
		// 全局替换
		go func() {
			var startId uint = 0
			for {
				var htmlLogs []model.TranslateHtmlLog
				currentSite.DB.Where("id > ? and to_language = ?", startId, textLog.ToLanguage).Order("id asc").Limit(100).Find(&htmlLogs)
				if len(htmlLogs) == 0 {
					break
				}
				startId = htmlLogs[len(htmlLogs)-1].Id
				for _, item := range htmlLogs {
					uriHash := library.Md5(item.Uri)
					cachePath := currentSite.CachePath + "multiLang/" + textLog.ToLanguage + "/" + uriHash
					// 先检查缓存文件是否存在
					if _, err := os.Stat(cachePath); err == nil {
						// 读取缓存文件
						buf, err := os.ReadFile(cachePath)
						if err != nil {
							continue
						}
						buf, replaced := currentSite.ReplaceTranslateText(buf, originText, req.Translated)
						if replaced {
							// 替换缓存文件
							_ = os.WriteFile(cachePath, buf, 0644)
						}
					}
				}
			}
		}()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("SaveTranslateTextLog"))
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("Saved"),
		"data": textLog,
	})
}
