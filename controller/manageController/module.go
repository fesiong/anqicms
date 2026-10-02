package manageController

import (
	"regexp"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// ModuleList 查询全部内容模型列表。
//
// 参数说明：
//   - 查询参数 "exclude_id": 需要排除的模型 ID，默认不排除。
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
	excludeId := ctx.URLParamIntDefault("exclude_id", 0)
	if excludeId > 0 {
		for i := range modules {
			if modules[i].Id == uint(excludeId) {
				modules = append(modules[:i], modules[i+1:]...)
				break
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": modules,
	})
}

// ModuleDetail 查询单个内容模型的详情。
//
// 参数说明：
//   - 查询参数 "id": 模型 ID，必填。
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

// ModuleDetailForm 新建或更新内容模型。
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
	req.UpdateAll = true
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
			if sub.Id == currentSite.Id || sub.Id == 0 {
				continue
			}
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				if req.Id == 0 {
					req.Id = module.Id
					subModule, err := subSite.SaveModule(&req)
					if err == nil {
						// 同步成功，进行翻译
						if currentSite.MultiLanguage.AutoTranslate {
							transReq := &provider.AnqiTranslateTextRequest{
								Text: []string{
									subModule.Title,       // 0
									subModule.Description, // 1
									subModule.Keywords,    // 2
								},
								Language:   currentSite.System.Language,
								ToLanguage: subSite.System.Language,
							}
							res, err := currentSite.AnqiTranslateString(transReq)
							if err == nil {
								// 只处理成功的结果
								subSite.DB.Model(subModule).UpdateColumns(map[string]interface{}{
									"title":       res.Text[0],
									"description": res.Text[1],
									"keywords":    res.Text[2],
								})
							}
						}
					}
				} else {
					// 修改的话，排除 title
					tmpModule, err := subSite.GetModuleById(req.Id)
					if err == nil {
						req.Title = tmpModule.Title
						//	req.TitleName = tmpModule.TitleName
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

// ModuleFieldsDelete 删除内容模型中的指定自定义字段。
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
			if sub.Id == currentSite.Id || sub.Id == 0 {
				continue
			}
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

// ModuleDelete 删除指定内容模型。
func ModuleDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DeleteModuleRequest
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
			if sub.Id == currentSite.Id || sub.Id == 0 {
				continue
			}
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
