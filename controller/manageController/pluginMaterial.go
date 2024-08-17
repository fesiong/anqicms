package manageController

import (
	"github.com/kataras/iris/v12"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginMaterialList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("keyword")
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))

	materialList, total, err := currentSite.GetMaterialList(categoryId, keyword, currentPage, pageSize)
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
		"data":  materialList,
	})
}

func PluginMaterialCategoryList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	categories, err := currentSite.GetMaterialCategories()
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
		"data": categories,
	})
}

func PluginMaterialDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterial
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := currentSite.SaveMaterial(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMaterialLog", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("MaterialUpdated"),
		"data": category,
	})
}

func PluginMaterialDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterial
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteMaterial(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteMaterialLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("MaterialDeleted"),
	})
}

func PluginMaterialCategoryDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterialCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := currentSite.SaveMaterialCategory(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMaterialCategoryLog", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryUpdated"),
		"data": category,
	})
}

func PluginMaterialCategoryDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterialCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteMaterialCategory(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteMaterialLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryDeleted"),
	})
}

func PluginMaterialImport(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterialImportRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.SaveMaterials(req.Materials)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ImportMaterial"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ImportSuccessful"),
	})
}

func ConvertFileToUtf8(ctx iris.Context) {
	file, _, err := ctx.FormFile("file")
	removeTag, _ := ctx.PostValueBool("remove_tag")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
			"data": "",
		})
		return
	}

	defer file.Close()

	//写入文件
	bufBytes, _ := io.ReadAll(file)

	_, contentType, _ := charset.DetermineEncoding(bufBytes, "")
	if contentType != "utf-8" {
		str, err := library.DecodeToUTF8(bufBytes, simplifiedchinese.GB18030)
		if err == nil {
			bufBytes = str
		}
	}

	content := string(bufBytes)
	if removeTag {
		content = provider.CleanTagsAndSpaces(content)
	}
	content = strings.TrimSpace(content)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": content,
	})
}
