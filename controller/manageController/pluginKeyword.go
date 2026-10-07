package manageController

import (
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginKeywordSetting 获取关键词插件的配置信息
func PluginKeywordSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.GetUserKeywordSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSaveKeywordSetting 保存关键词插件的配置信息。
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

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyKeywordConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

// PluginKeywordList 获取关键词列表，支持搜索和分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "title": 关键词搜索词。
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

// PluginKeywordDetailForm 新增或更新关键词：id 大于 0 时更新指定关键词，否则按标题批量新增（换行分隔，自动去重）。
//
// 参数说明：
//   - 请求体 "id": 关键词 ID，大于 0 表示更新。
//   - 请求体 "title": 关键词标题，新增时支持多行批量添加。
//   - 请求体 "category_id": 关键词分类 ID。
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
	// 新增分支支持批量，逐条收集保存成功的关键词；更新分支只有一条
	var saved []*model.Keyword

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
				"msg":  ctx.Tr("KeywordAlreadyExists", req.Title),
			})
			return
		}
		keyword.Title = req.Title
		keyword.CategoryId = req.CategoryId

		err = currentSite.SaveKeyword(keyword)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		saved = append(saved, keyword)
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
				if err := currentSite.SaveKeyword(keyword); err == nil {
					saved = append(saved, keyword)
				}
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateKeywordLog", req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("KeywordUpdated"),
		"data": saved,
	})
}

// PluginKeywordDelete 删除关键词，支持单个、批量或全部删除。
//
// 参数说明：
//   - 请求体 "id": 要删除的关键词 ID，大于 0 时删除单条。
//   - 请求体 "ids": 要删除的关键词 ID 列表。
//   - 请求体 "all": 是否删除全部关键词。
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
	} else if req.All {
		// 删除所有
		currentSite.DB.Where("`id` > 0").Delete(model.Keyword{})
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteKeywordLog", req.Id, req.Ids, req.All))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteOperationHasBeenPerformed"),
	})
}

// PluginKeywordExport 导出全部关键词，返回表头与数据内容。
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

	currentSite.AddAdminLog(ctx, ctx.Tr("ExportKeywords"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

// PluginKeywordImport 批量导入关键词。文件内容一行一个关键词，也支持指定分类如：title, category_id
//
// 参数说明：
//   - 请求体 "file": 导入的关键词文件。
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

	currentSite.AddAdminLog(ctx, ctx.Tr("ImportKeywords"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadCompleted"),
		"data": result,
	})
}
