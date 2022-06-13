package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginKeywordList(ctx iris.Context) {
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("title")

	keywordList, total, err := provider.GetKeywordList(keyword, currentPage, pageSize)
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
		"total": total,
		"data": keywordList,
	})
}

func PluginKeywordDetailForm(ctx iris.Context) {
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
		keyword, err = provider.GetKeywordById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//去重
		exists, err := provider.GetKeywordByTitle(req.Title)
		if err == nil && exists.Id != keyword.Id {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  fmt.Errorf("已存在关键词%s，修改失败", req.Title),
			})
			return
		}
		keyword.Title = req.Title
		keyword.CategoryId = req.CategoryId

		err = keyword.Save(dao.DB)
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
				_, err := provider.GetKeywordByTitle(v)
				if err == nil {
					//已存在，跳过
					continue
				}
				keyword = &model.Keyword{
					Title:  v,
					CategoryId: req.CategoryId,
					Status: 1,
				}
				keyword.Save(dao.DB)
			}
		}
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新关键词：%s", req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词已更新",
	})
}

func PluginKeywordDelete(ctx iris.Context) {
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
		keyword, err := provider.GetKeywordById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = provider.DeleteKeyword(keyword)
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
			keyword, err := provider.GetKeywordById(id)
			if err != nil {
				continue
			}

			_ = provider.DeleteKeyword(keyword)
		}
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除关键词：%d, %v", req.Id, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已执行删除操作",
	})
}

func PluginKeywordExport(ctx iris.Context) {
	keywords, err := provider.GetAllKeywords()
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

	provider.AddAdminLog(ctx, fmt.Sprintf("导出关键词"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header": header,
			"content": content,
		},
	})
}

func PluginKeywordImport(ctx iris.Context) {
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	result, err := provider.ImportKeywords(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("导入关键词"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传完毕",
		"data": result,
	})
}
