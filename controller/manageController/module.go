package manageController

import (
	"regexp"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func ModuleList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	modules, err := currentSite.GetModules()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": modules,
	})
}

func ModuleDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	module, err := currentSite.GetModuleById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": module,
	})
}

func ModuleDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ModuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	matched, err := regexp.MatchString(`^[a-z][a-z0-9_]*$`, req.TableName)
	if req.TableName == "" || !matched {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheModelTableNameCorrectly"),
		})
		return
	}

	matched, _ = regexp.MatchString(`^[a-z][a-z0-9_]*$`, req.UrlToken)
	if req.UrlToken == "" || !matched {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheUrlAliasCorrectly"),
		})
		return
	}

	module, err := currentSite.SaveModule(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, sub := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				if req.Id == 0 {
					req.Id = module.Id
					subModule, err := subSite.SaveModule(&req)
					if err == nil {
						// 同步成功，进行翻译
						if currentSite.MultiLanguage.AutoTranslate {
							transReq := provider.AnqiAiRequest{
								Title:      subModule.Title,
								Language:   currentSite.System.Language,
								ToLanguage: subSite.System.Language,
								Async:      false, // 同步返回结果
							}
							res, err := currentSite.AnqiTranslateString(&transReq)
							if err == nil {
								// 只处理成功的结果
								subSite.DB.Model(subModule).UpdateColumns(map[string]interface{}{
									"title": res.Title,
								})
							}
						}
					}
				} else {
					// 修改的话，排除 title
					tmpModule, err := subSite.GetModuleById(req.Id)
					if err == nil {
						req.Title = tmpModule.Title
						req.TitleName = tmpModule.TitleName
					}
					_, _ = subSite.SaveModule(&req)
				}

			}
		}
	}
	// 更新缓存
	go func() {
		currentSite.BuildModuleCache(ctx)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyDocumentModelLog", module.Id, module.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": module,
	})
}

func ModuleFieldsDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ModuleFieldRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteModuleField(req.Id, req.FieldName)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, sub := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				// 同步删除
				_ = subSite.DeleteModuleField(req.Id, req.FieldName)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteModelFieldLog", req.Id, req.FieldName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FieldDeleted"),
	})
}

func ModuleDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ModuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	module, err := currentSite.GetModuleById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if module.IsSystem == 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("BuiltInModelCannotBeDeleted"),
		})
		return
	}

	err = currentSite.DeleteModule(module)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, sub := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				// 同步删除
				_ = subSite.DeleteModule(module)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentModelLog", module.Id, module.Title))

	currentSite.DeleteCacheModules()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ModelDeleted"),
	})
}
