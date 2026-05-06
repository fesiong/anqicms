package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginLinkList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	linkList, err := currentSite.GetLinkList()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": linkList,
	})
}

func PluginLinkDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginLink
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var link *model.Link
	var err error
	if req.Id > 0 {
		link, err = currentSite.GetLinkById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		link = &model.Link{
			Status: 0,
		}
	}

	link.Title = req.Title
	link.Link = req.Link
	link.BackLink = req.BackLink
	link.MyTitle = req.MyTitle
	link.MyLink = req.MyLink
	link.Contact = req.Contact
	link.Remark = req.Remark
	link.Nofollow = req.Nofollow
	link.Sort = req.Sort
	link.Status = 0

	err = link.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 保存完毕，实时监测
	go currentSite.PluginLinkCheck(link)

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyFriendlyLinkLog", link.Id, link.Link))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkUpdated"),
	})
}

func PluginLinkDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginLink
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	link, err := currentSite.GetLinkById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = link.Delete(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteFriendlyLinkLog", link.Id, link.Link))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkDeleted"),
	})
}

func PluginLinkCheck(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginLink
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	link, err := currentSite.GetLinkById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	result, err := currentSite.PluginLinkCheck(link)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CheckCompleted"),
		"data": result,
	})
}
