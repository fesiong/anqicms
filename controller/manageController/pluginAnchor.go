package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginAnchorList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("keyword")

	linkList, total, err := currentSite.GetAnchorList(keyword, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  linkList,
	})
}

func PluginAnchorDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	anchor, err := currentSite.GetAnchorById(id)
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
		"data": anchor,
	})
}

func PluginAnchorDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginAnchor
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.Link = strings.TrimPrefix(req.Link, currentSite.System.BaseUrl)

	var anchor *model.Anchor
	var err error

	var changeTitle bool
	var changeLink bool

	if req.Id > 0 {
		anchor, err = currentSite.GetAnchorById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//去重
		exists, err := currentSite.GetAnchorByTitle(req.Title)
		if err == nil && exists.Id != anchor.Id {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  fmt.Errorf("已存在锚文本关键词%s，修改失败", req.Title),
			})
			return
		}
		//只有旧的才需要处理
		if anchor.Title != req.Title {
			changeTitle = true
		}
		if anchor.Link != req.Link {
			changeLink = true
		}

	} else {
		anchor, err = currentSite.GetAnchorByTitle(req.Title)
		if err != nil {
			//不存在，则创建它
			anchor = &model.Anchor{
				Status: 1,
			}
		}
	}

	anchor.Title = req.Title
	anchor.Link = req.Link
	anchor.Weight = req.Weight

	err = anchor.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if changeTitle || changeLink {
		//锚文本名称更改了，不管连接有没有更改，都删掉旧的
		go currentSite.ChangeAnchor(anchor, changeTitle)
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改锚文本：%d => %s", anchor.Id, anchor.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "链接已更新",
	})
}

func PluginAnchorReplace(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginAnchor
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		//更新单个
		anchor, err := currentSite.GetAnchorById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		go currentSite.ReplaceAnchor(anchor)
	} else {
		go currentSite.ReplaceAnchor(nil)
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("执行锚文本批量替换"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "锚文本替换任务已执行",
	})
}

func PluginAnchorDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginAnchorDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		//删一条
		anchor, err := currentSite.GetAnchorById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = currentSite.DeleteAnchor(anchor)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else if len(req.Ids) > 0 {
		//删除多条
		for _, id := range req.Ids {
			anchor, err := currentSite.GetAnchorById(id)
			if err != nil {
				continue
			}

			_ = currentSite.DeleteAnchor(anchor)
		}
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除锚文本：%d, %v", req.Id, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已执行删除操作",
	})
}

func PluginAnchorExport(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	anchors, err := currentSite.GetAllAnchors()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//header
	header := []string{"title", "link", "weight"}
	var content [][]interface{}
	//content
	for _, v := range anchors {
		content = append(content, []interface{}{v.Title, v.Link, v.Weight})
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("导出锚文本"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

func PluginAnchorImport(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	result, err := currentSite.ImportAnchors(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("导入锚文本：%s", result))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传完毕",
		"data": result,
	})
}

func PluginAnchorSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginAnchor := currentSite.PluginAnchor

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginAnchor,
	})
}

func PluginAnchorSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginAnchorConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.AnchorDensity < 10 {
		req.AnchorDensity = 100
	}

	currentSite.PluginAnchor.AnchorDensity = req.AnchorDensity
	currentSite.PluginAnchor.ReplaceWay = req.ReplaceWay
	currentSite.PluginAnchor.KeywordWay = req.KeywordWay

	err := currentSite.SaveSettingValue(provider.AnchorSettingKey, currentSite.PluginAnchor)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改锚文本设置信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
