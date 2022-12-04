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
	currentSite := provider.CurrentSite(ctx)
	// 读取 设计列表
	designList := currentSite.GetDesignList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": designList,
	})
}

func GetDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	packageName := ctx.URLParam("package")

	designInfo, err := currentSite.GetDesignInfo(packageName, true)
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
	currentSite := provider.CurrentSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveDesignInfo(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.System.TemplateName == req.Package {
		// 更改当前
		if currentSite.System.TemplateType != req.TemplateType {
			currentSite.System.TemplateType = req.TemplateType
			err = currentSite.SaveSettingValue(provider.SystemSettingKey, currentSite.System)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		}
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改模板信息：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "修改成功",
	})
}

func UseDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	_, err := currentSite.GetDesignInfo(req.Package, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.System.TemplateName != req.Package {
		currentSite.System.TemplateName = req.Package
		currentSite.System.TemplateType = req.TemplateType
		err = currentSite.SaveSettingValue(provider.SystemSettingKey, currentSite.System)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			// 如果切换了模板，则重载模板
			config.RestartChan <- true

			time.Sleep(2 * time.Second)
			currentSite.DeleteCacheIndex()
		}()
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("启用新模板：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "切换成功",
	})
}

func DeleteDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteDesignInfo(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模板：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func DownloadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	data, err := currentSite.CreateDesignZip(req.Package)
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

	err = currentSite.UploadDesignZip(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传模板：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传成功",
	})
}

func BackupDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.DesignDataRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.BackupDesignData(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("备份模板数据：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "数据备份成功",
	})
}

func RestoreDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.DesignDataRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.AutoBackup {
		// 如果用户勾选了自动备份
		err := currentSite.BackupData()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	err := currentSite.RestoreDesignData(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("初始化模板数据：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "数据初始化成功",
	})
}

func UploadDesignFile(ctx iris.Context) {
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

	packageName := ctx.PostValue("package")
	filePath := ctx.PostValue("path")
	fileType := ctx.PostValue("type")

	err = currentSite.UploadDesignFile(file, info, packageName, fileType, filePath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteCacheIndex()
	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传模板文件：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传成功",
	})
}

func GetDesignFileDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")

	fileInfo, err := currentSite.GetDesignFileDetail(packageName, fileName, fileType, true)
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
	currentSite := provider.CurrentSite(ctx)
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")

	histories := currentSite.GetDesignFileHistories(packageName, fileName, fileType)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": histories,
	})
}

func DeleteDesignFileHistories(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.RestoreDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteDesignHistoryFile(req.Package, req.Filepath, req.Hash, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模板文件历史：%s => %s", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func RestoreDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.RestoreDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.RestoreDesignFile(req.Package, req.Filepath, req.Hash, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	fileInfo, _ := currentSite.GetDesignFileDetail(req.Package, req.Filepath, req.Type, true)

	currentSite.DeleteCacheIndex()
	currentSite.AddAdminLog(ctx, fmt.Sprintf("从历史恢复模板文件：%s => %s", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "替换成功",
		"data": fileInfo,
	})
}

func SaveDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.SaveDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "修改成功",
	})
}

func CopyDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CopyDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.CopyDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("复制模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "复制成功",
	})
}

func DeleteDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.SaveDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteDesignFile(req.Package, req.Path, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func GetDesignTemplateFiles(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	packageName := currentSite.System.TemplateName
	templates, err := currentSite.GetDesignTemplateFiles(packageName)
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
		"data": templates,
	})
}

func GetDesignDocs(ctx iris.Context) {
	docs := []response.DesignDocGroup{
		{
			Title: "模板制作帮助",
			Docs: []response.DesignDoc{
				{
					Title: "一些基本约定",
					Link:  "https://www.anqicms.com/help-design/116.html",
				},
				{
					Title: "目录和模板",
					Link:  "https://www.anqicms.com/help-design/117.html",
				},
				{
					Title: "标签和使用方法",
					Link:  "https://www.anqicms.com/help-design/118.html",
				},
			},
		},
		{
			Title: "常用标签",
			Docs: []response.DesignDoc{
				{
					Title: "系统设置标签",
					Link:  "https://www.anqicms.com/manual-normal/73.html",
				},
				{
					Title: "联系方式标签",
					Link:  "https://www.anqicms.com/manual-normal/74.html",
				},
				{
					Title: "万能TDK标签",
					Link:  "https://www.anqicms.com/manual-normal/75.html",
				},
				{
					Title: "导航列表标签",
					Link:  "https://www.anqicms.com/manual-normal/76.html",
				},
				{
					Title: "面包屑导航标签",
					Link:  "https://www.anqicms.com/manual-normal/87.html",
				},
				{
					Title: "统计代码标签",
					Link:  "https://www.anqicms.com/manual-normal/91.html",
				},
			},
		},
		{
			Title: "分类页面标签",
			Docs: []response.DesignDoc{
				{
					Title: "分类列表标签",
					Link:  "https://www.anqicms.com/manual-category/77.html",
				},
				{
					Title: "分类详情标签",
					Link:  "https://www.anqicms.com/manual-category/78.html",
				},
				{
					Title: "单页列表标签",
					Link:  "https://www.anqicms.com/manual-category/83.html",
				},
				{
					Title: "单页详情标签",
					Link:  "https://www.anqicms.com/manual-category/84.html",
				},
			},
		},
		{
			Title: "文档标签",
			Docs: []response.DesignDoc{
				{
					Title: "文档列表标签",
					Link:  "https://www.anqicms.com/manual-archive/79.html",
				},
				{
					Title: "文档详情标签",
					Link:  "https://www.anqicms.com/manual-archive/80.html",
				},
				{
					Title: "上一篇文档标签",
					Link:  "https://www.anqicms.com/manual-archive/88.html",
				},
				{
					Title: "下一篇文档标签",
					Link:  "https://www.anqicms.com/manual-archive/89.html",
				},
				{
					Title: "相关文档标签",
					Link:  "https://www.anqicms.com/manual-archive/92.html",
				},
				{
					Title: "文档参数标签",
					Link:  "https://www.anqicms.com/manual-archive/95.html",
				},
				{
					Title: "文档参数筛选标签",
					Link:  "https://www.anqicms.com/manual-archive/96.html",
				},
			},
		},
		{
			Title: "文档Tag标签",
			Docs: []response.DesignDoc{
				{
					Title: "文档Tag列表标签",
					Link:  "https://www.anqicms.com/manual-tag/81.html",
				},
				{
					Title: "Tag文档列表标签",
					Link:  "https://www.anqicms.com/manual-tag/82.html",
				},
				{
					Title: "Tag详情标签",
					Link:  "https://www.anqicms.com/manual-tag/90.html",
				},
			},
		},
		{
			Title: "其他标签",
			Docs: []response.DesignDoc{
				{
					Title: "评论标列表签",
					Link:  "https://www.anqicms.com/manual-other/85.html",
				},
				{
					Title: "留言表单标签",
					Link:  "https://www.anqicms.com/manual-other/86.html",
				},
				{
					Title: "分页标签",
					Link:  "https://www.anqicms.com/manual-other/94.html",
				},
				{
					Title: "友情链接标签",
					Link:  "https://www.anqicms.com/manual-other/97.html",
				},
				{
					Title: "留言验证码使用标签",
					Link:  "https://www.anqicms.com/manual-other/139.html",
				},
			},
		},
		{
			Title: "通用模板标签",
			Docs: []response.DesignDoc{
				{
					Title: "其他辅助标签",
					Link:  "https://www.anqicms.com/manual-common/93.html",
				},
				{
					Title: "更多过滤器",
					Link:  "https://www.anqicms.com/manual-common/98.html",
				},
				{
					Title: "定义变量赋值标签",
					Link:  "https://www.anqicms.com/manual-common/99.html",
				},
				{
					Title: "格式化时间戳标签",
					Link:  "https://www.anqicms.com/manual-common/100.html",
				},
				{
					Title: "for循环遍历标签",
					Link:  "https://www.anqicms.com/manual-common/101.html",
				},
				{
					Title: "移除逻辑标签占用行",
					Link:  "https://www.anqicms.com/manual-common/102.html",
				},
				{
					Title: "算术运算标签",
					Link:  "https://www.anqicms.com/manual-common/103.html",
				},
				{
					Title: "if逻辑判断标签",
					Link:  "https://www.anqicms.com/manual-common/104.html",
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
