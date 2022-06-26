package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"time"
)

func GetDesignList(ctx iris.Context) {
	// 读取 设计列表
	designList := provider.GetDesignList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": designList,
	})
}

func GetDesignInfo(ctx iris.Context) {
	packageName := ctx.URLParam("package")

	designInfo, err := provider.GetDesignInfo(packageName, true)
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
		"data": designInfo,
	})
}

func SaveDesignInfo(ctx iris.Context) {
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SaveDesignInfo(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("修改模板信息：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "修改成功",
	})
}

func UseDesignInfo(ctx iris.Context) {
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	_, err := provider.GetDesignInfo(req.Package, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if config.JsonData.System.TemplateName != req.Package {
		config.JsonData.System.TemplateName = req.Package

		err = config.WriteConfig()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		// 如果切换了模板，则重载模板
		config.RestartChan <- true

		time.Sleep(2 * time.Second)

		provider.DeleteCacheIndex()
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("启用新模板：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "切换成功",
	})
}

func DeleteDesignInfo(ctx iris.Context) {
	packageName := ctx.URLParam("package")

	err := provider.DeleteDesignInfo(packageName)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除模板：%s", packageName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func DownloadDesignInfo(ctx iris.Context) {
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	data, err := provider.CreateDesignZip(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//读取文件
	ctx.ResponseWriter().Header().Set(context.ContentDispositionHeaderKey, fmt.Sprintf("attachment;filename=%s.zip", req.Package))
	ctx.Binary(data.Bytes())
}

func UploadDesignInfo(ctx iris.Context) {
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	err = provider.UploadDesignZip(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":    "上传成功",
	})
}

func UploadDesignFile(ctx iris.Context) {
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	packageName := ctx.PostValue("package")
	filePath := ctx.PostValue("path")
	fileType := ctx.PostValue("type")

	err = provider.UploadDesignFile(file, info, packageName, fileType, filePath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":    "上传成功",
	})
}

func GetDesignFileDetail(ctx iris.Context) {
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")

	fileInfo, err := provider.GetDesignFileDetail(packageName, fileName, fileType, true)
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
		"data": fileInfo,
	})
}

func GetDesignFileHistories(ctx iris.Context) {
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")

	histories := provider.GetDesignFileHistories(packageName, fileName, fileType)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": histories,
	})
}

func DeleteDesignFileHistories(ctx iris.Context) {
	var req request.RestoreDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteDesignHistoryFile(req.Package, req.Filepath, req.Hash, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除模板文件历史：%s => %s", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func RestoreDesignFile(ctx iris.Context) {
	var req request.RestoreDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.RestoreDesignFile(req.Package, req.Filepath, req.Hash, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	fileInfo, _ := provider.GetDesignFileDetail(req.Package, req.Filepath, req.Type, true)

	provider.AddAdminLog(ctx, fmt.Sprintf("从历史恢复模板文件：%s => %s", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "替换成功",
		"data": fileInfo,
	})
}

func SaveDesignFile(ctx iris.Context) {
	var req request.SaveDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SaveDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("修改模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "修改成功",
	})
}

func DeleteDesignFile(ctx iris.Context) {
	var req request.SaveDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteDesignFile(req.Package, req.Path, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func GetDesignDocs(ctx iris.Context) {
	docs := []response.DesignDocGroup{
		{
			Title: "模板制作帮助",
			Docs: []response.DesignDoc{
				{
					Title: "一些基本约定",
					Link: "https://www.kandaoni.com/article/116",
				},
				{
					Title: "目录和模板",
					Link: "https://www.kandaoni.com/article/117",
				},
				{
					Title: "标签和使用方法",
					Link: "https://www.kandaoni.com/article/118",
				},
			},
		},
		{
			Title: "常用标签",
			Docs: []response.DesignDoc{
				{
					Title: "系统设置标签",
					Link: "https://www.kandaoni.com/article/73",
				},
				{
					Title: "联系方式标签",
					Link: "https://www.kandaoni.com/article/74",
				},
				{
					Title: "万能TDK标签",
					Link: "https://www.kandaoni.com/article/75",
				},
				{
					Title: "导航列表标签",
					Link: "https://www.kandaoni.com/article/76",
				},
				{
					Title: "面包屑导航标签",
					Link: "https://www.kandaoni.com/article/87",
				},
				{
					Title: "统计代码标签",
					Link: "https://www.kandaoni.com/article/91",
				},
			},
		},
		{
			Title: "分类页面标签",
			Docs: []response.DesignDoc{
				{
					Title: "分类列表标签",
					Link: "https://www.kandaoni.com/article/77",
				},
				{
					Title: "分类详情标签",
					Link: "https://www.kandaoni.com/article/78",
				},
				{
					Title: "单页列表标签",
					Link: "https://www.kandaoni.com/article/83",
				},
				{
					Title: "单页详情标签",
					Link: "https://www.kandaoni.com/article/84",
				},
			},
		},
		{
			Title: "文档标签",
			Docs: []response.DesignDoc{
				{
					Title: "文档列表标签",
					Link: "https://www.kandaoni.com/article/79",
				},
				{
					Title: "文档详情标签",
					Link: "https://www.kandaoni.com/article/80",
				},
				{
					Title: "上一篇文档标签",
					Link: "https://www.kandaoni.com/article/88",
				},
				{
					Title: "下一篇文档标签",
					Link: "https://www.kandaoni.com/article/89",
				},
				{
					Title: "相关文档标签",
					Link: "https://www.kandaoni.com/article/92",
				},
				{
					Title: "文档参数标签",
					Link: "https://www.kandaoni.com/article/95",
				},
				{
					Title: "文档参数筛选标签",
					Link: "https://www.kandaoni.com/article/96",
				},
			},
		},
		{
			Title: "文档Tag标签",
			Docs: []response.DesignDoc{
				{
					Title: "文档Tag列表标签",
					Link: "https://www.kandaoni.com/article/81",
				},
				{
					Title: "Tag文档列表标签",
					Link: "https://www.kandaoni.com/article/82",
				},
				{
					Title: "Tag详情标签",
					Link: "https://www.kandaoni.com/article/90",
				},
			},
		},
		{
			Title: "其他标签",
			Docs: []response.DesignDoc{
				{
					Title: "评论标列表签",
					Link: "https://www.kandaoni.com/article/85",
				},
				{
					Title: "留言表单标签",
					Link: "https://www.kandaoni.com/article/86",
				},
				{
					Title: "分页标签",
					Link: "https://www.kandaoni.com/article/94",
				},
				{
					Title: "友情链接标签",
					Link: "https://www.kandaoni.com/article/97",
				},
			},
		},
		{
			Title: "通用模板标签",
			Docs: []response.DesignDoc{
				{
					Title: "其他辅助标签",
					Link: "https://www.kandaoni.com/article/93",
				},
				{
					Title: "更多过滤器",
					Link: "https://www.kandaoni.com/article/98",
				},
				{
					Title: "定义变量赋值标签",
					Link: "https://www.kandaoni.com/article/99",
				},
				{
					Title: "格式化时间戳标签",
					Link: "https://www.kandaoni.com/article/100",
				},
				{
					Title: "for循环遍历标签",
					Link: "https://www.kandaoni.com/article/101",
				},
				{
					Title: "移除逻辑标签占用行",
					Link: "https://www.kandaoni.com/article/102",
				},
				{
					Title: "算术运算标签",
					Link: "https://www.kandaoni.com/article/103",
				},
				{
					Title: "if逻辑判断标签",
					Link: "https://www.kandaoni.com/article/104",
				},
			},
		},
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}