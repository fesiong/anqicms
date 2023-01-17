package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginLinkList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改友情链接：%d => %s", link.Id, link.Link))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "链接已更新",
	})
}

func PluginLinkDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除友情链接：%d => %s", link.Id, link.Link))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "链接已删除",
	})
}

func PluginLinkCheck(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
		"msg":  "检查结束",
		"data": result,
	})
}
