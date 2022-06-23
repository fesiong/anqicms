package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func SettingNav(ctx iris.Context) {
	typeId := uint(ctx.URLParamIntDefault("type_id", 1))
	navList, _ := provider.GetNavList(typeId)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func SettingNavForm(ctx iris.Context) {
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
			category := provider.GetCategoryFromCache(req.PageId)
			if category != nil {
				req.Title = category.Title
			}
		}
	}
	if req.Title == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请填写导航显示名称",
		})
		return
	}

	var nav *model.Nav
	var err error
	if req.Id > 0 {
		nav, err = provider.GetNavById(req.Id)
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

	err = nav.Save(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新导航信息：%d => %s", nav.Id, nav.Title))

	provider.DeleteCacheNavs()
	provider.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingNavDelete(ctx iris.Context) {
	var req request.NavConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	nav, err := provider.GetNavById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = nav.Delete(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除导航信息：%d => %s", nav.Id, nav.Title))

	provider.DeleteCacheNavs()
	provider.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "导航已删除",
	})
}

func SettingNavType(ctx iris.Context) {
	navTypes, _ := provider.GetNavTypeList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navTypes,
	})
}

func SettingNavTypeForm(ctx iris.Context) {
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
		navType, err = provider.GetNavTypeById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// 检查重复标题
		navType, err = provider.GetNavTypeByTitle(req.Title)
		if err != nil {
			navType = &model.NavType{}
		}
	}

	navType.Title = req.Title

	err = dao.DB.Save(navType).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新导航类别信息：%d => %s", navType.Id, navType.Title))

	provider.DeleteCacheNavs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingNavTypeDelete(ctx iris.Context) {
	var req request.NavTypeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	navType, err := provider.GetNavTypeById(req.Id)
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
			"msg":  "不允许删除默认类别",
		})
		return
	}

	// 删除更多信息
	// 删除一类导航
	dao.DB.Where("`type_id` = ?", navType).Delete(model.Nav{})
	err = dao.DB.Delete(navType).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除导航类别信息：%d => %s", navType.Id, navType.Title))

	provider.DeleteCacheNavs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "导航类别已删除",
	})
}
