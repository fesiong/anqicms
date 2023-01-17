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

// PluginKeywordSetting 全局配置
func PluginKeywordSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetUserKeywordSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveKeywordSetting 全局配置保存
func PluginSaveKeywordSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.KeywordJson
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//将现有配置写回文件
	err := currentSite.SaveUserKeywordSetting(req, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改关键词配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
	})
}

func PluginKeywordList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("title")

	keywordList, total, err := currentSite.GetKeywordList(keyword, currentPage, pageSize)
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
		"data":  keywordList,
	})
}

func PluginKeywordDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginKeyword
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var keyword *model.Keyword
	var err error

	if req.Id > 0 {
		keyword, err = currentSite.GetKeywordById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//去重
		exists, err := currentSite.GetKeywordByTitle(req.Title)
		if err == nil && exists.Id != keyword.Id {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  fmt.Errorf("已存在关键词%s，修改失败", req.Title),
			})
			return
		}
		keyword.Title = req.Title
		keyword.CategoryId = req.CategoryId

		err = keyword.Save(currentSite.DB)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		//新增支持批量插入
		keywords := strings.Split(req.Title, "\n")
		for _, v := range keywords {
			v = strings.TrimSpace(v)
			if v != "" {
				_, err := currentSite.GetKeywordByTitle(v)
				if err == nil {
					//已存在，跳过
					continue
				}
				keyword = &model.Keyword{
					Title:      v,
					CategoryId: req.CategoryId,
					Status:     1,
				}
				keyword.Save(currentSite.DB)
			}
		}
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新关键词：%s", req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词已更新",
	})
}

func PluginKeywordDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginKeywordDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		//删一条
		keyword, err := currentSite.GetKeywordById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = currentSite.DeleteKeyword(keyword)
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
			keyword, err := currentSite.GetKeywordById(id)
			if err != nil {
				continue
			}

			_ = currentSite.DeleteKeyword(keyword)
		}
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除关键词：%d, %v", req.Id, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已执行删除操作",
	})
}

func PluginKeywordExport(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	keywords, err := currentSite.GetAllKeywords()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//header
	header := []string{"title", "category_id"}
	var content [][]interface{}
	//content
	for _, v := range keywords {
		content = append(content, []interface{}{v.Title, v.CategoryId})
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("导出关键词"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

func PluginKeywordImport(ctx iris.Context) {
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

	result, err := currentSite.ImportKeywords(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("导入关键词"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传完毕",
		"data": result,
	})
}
