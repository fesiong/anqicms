package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func SettingNav(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	typeId := uint(ctx.URLParamIntDefault("type_id", 1))
	navList, _ := currentSite.GetNavList(typeId)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func SettingNavForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.NavConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	nav, err := currentSite.SaveNav(&req)
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
				// 插入记录
				if req.Id == 0 {
					req.Id = nav.Id
					subNav, err := subSite.SaveNav(&req)
					if err == nil {
						// 同步成功，进行翻译
						if currentSite.MultiLanguage.AutoTranslate {
							transReq := provider.AnqiAiRequest{
								Title:      subNav.Title,
								Content:    subNav.Description,
								Language:   currentSite.System.Language,
								ToLanguage: subSite.System.Language,
								Async:      false, // 同步返回结果
							}
							res, err := currentSite.AnqiTranslateString(&transReq)
							if err == nil {
								// 只处理成功的结果
								subSite.DB.Model(subNav).UpdateColumns(map[string]interface{}{
									"title":       res.Title,
									"description": res.Content,
								})
							}
						}
					}
				} else {
					// 修改的话，就排除 title, content，description，keywords 字段
					tmpNav, err := subSite.GetNavById(req.Id)
					if err == nil {
						req.Title = tmpNav.Title
						req.SubTitle = tmpNav.SubTitle
						req.Description = tmpNav.Description
					}
					_, _ = subSite.SaveNav(&req)
				}
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateNavigationLog", nav.Id, nav.Title))

	currentSite.DeleteCacheNavs()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func SettingNavDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.NavConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	nav, err := currentSite.GetNavById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = nav.Delete(currentSite.DB)
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
				_ = nav.Delete(subSite.DB)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteNavigationLog", nav.Id, nav.Title))

	currentSite.DeleteCacheNavs()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("NavigationDeleted"),
	})
}

func SettingNavType(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	navTypes, _ := currentSite.GetNavTypeList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navTypes,
	})
}

func SettingNavTypeForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.NavTypeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	navType, err := currentSite.SaveNavType(&req)
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
				// 插入记录
				if req.Id == 0 {
					req.Id = navType.Id
					_, _ = subSite.SaveNavType(&req)
				} else {
					// 修改的话，就排除 title, content，description，keywords 字段
					tmpNav, err := subSite.GetNavById(req.Id)
					if err == nil {
						req.Title = tmpNav.Title
					}
					_, _ = subSite.SaveNavType(&req)
				}
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateNavigationCategoryLog", navType.Id, navType.Title))

	currentSite.DeleteCacheNavs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func SettingNavTypeDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.NavTypeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	navType, err := currentSite.GetNavTypeById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if navType.Id == 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("DeleteDefaultCategoryIsNotAllowed"),
		})
		return
	}

	// 删除更多信息
	// 删除一类导航
	currentSite.DB.Where("`type_id` = ?", navType).Delete(&model.Nav{})
	err = currentSite.DB.Delete(navType).Error
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
				// 删除一类导航
				subSite.DB.Where("`type_id` = ?", navType).Delete(&model.Nav{})
				subSite.DB.Delete(navType)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteNavigationCategoryLog", navType.Id, navType.Title))

	currentSite.DeleteCacheNavs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("NavigationCategoryHasBeenDeleted"),
	})
}
