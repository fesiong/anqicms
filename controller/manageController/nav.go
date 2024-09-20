package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func SettingNav(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	typeId := uint(ctx.URLParamIntDefault("type_id", 1))
	navList, _ := currentSite.GetNavList(typeId)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func SettingNavForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.NavConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Title == "" {
		if req.NavType == model.NavTypeCategory {
			category := currentSite.GetCategoryFromCache(req.PageId)
			if category != nil {
				req.Title = category.Title
			}
		} else if req.NavType == model.NavTypeArchive {
			archive, _ := currentSite.GetArchiveById(req.PageId)
			if archive != nil {
				req.Title = archive.Title
			}
		}
	}
	if req.Title == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheNavigationDisplayName"),
		})
		return
	}

	var nav *model.Nav
	var err error
	if req.Id > 0 {
		nav, err = currentSite.GetNavById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		nav = &model.Nav{
			Status: 1,
		}
	}

	nav.Title = req.Title
	nav.SubTitle = req.SubTitle
	nav.Description = req.Description
	nav.ParentId = req.ParentId
	nav.NavType = req.NavType
	nav.PageId = req.PageId
	nav.TypeId = req.TypeId
	nav.Link = req.Link
	nav.Sort = req.Sort
	nav.Status = 1

	err = nav.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
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
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteNavigationLog", nav.Id, nav.Title))

	currentSite.DeleteCacheNavs()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("NavigationDeleted"),
	})
}

func SettingNavType(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	navTypes, _ := currentSite.GetNavTypeList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navTypes,
	})
}

func SettingNavTypeForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.NavTypeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var navType *model.NavType
	var err error
	if req.Id > 0 {
		navType, err = currentSite.GetNavTypeById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// 检查重复标题
		navType, err = currentSite.GetNavTypeByTitle(req.Title)
		if err != nil {
			navType = &model.NavType{}
		}
	}

	navType.Title = req.Title

	err = currentSite.DB.Save(navType).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateNavigationCategoryLog", navType.Id, navType.Title))

	currentSite.DeleteCacheNavs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func SettingNavTypeDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteNavigationCategoryLog", navType.Id, navType.Title))

	currentSite.DeleteCacheNavs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("NavigationCategoryHasBeenDeleted"),
	})
}
