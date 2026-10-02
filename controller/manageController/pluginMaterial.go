package manageController

import (
	"io"
	"strings"

	"github.com/kataras/iris/v12"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginMaterialList 获取素材列表，支持分类、关键词过滤和分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "keyword": 素材关键词搜索。
//   - 查询参数 "category_id": 素材分类 ID，默认为 0（全部分类）。
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

// PluginMaterialCategoryList 获取全部素材分类列表。
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

// PluginMaterialDetail 根据 ID 获取素材详情。
//
// 参数说明：
//   - 查询参数 "id": 素材 ID。
func PluginMaterialDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParamIntDefault("id", 0)

	detail, err := currentSite.GetMaterialById(uint(id))
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
		"data": detail,
	})
}

// PluginMaterialDetailForm 新增或更新素材。
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

// PluginMaterialDelete 删除指定素材。
//
// 参数说明：
//   - 请求体 "id": 要删除的素材 ID。
func PluginMaterialDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterialDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	material, err := currentSite.GetMaterialById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteMaterial(material.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteMaterialLog", material.Id, material.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("MaterialDeleted"),
	})
}

// PluginMaterialCategoryDetailForm 新增或更新素材分类。
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

// PluginMaterialCategoryDelete 删除指定素材分类。
//
// 参数说明：
//   - 请求体 "id": 要删除的素材分类 ID。
func PluginMaterialCategoryDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMaterialCategoryDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	category, err := currentSite.GetMaterialCategoryById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteMaterialCategory(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteMaterialLog", req.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryDeleted"),
	})
}

// PluginMaterialImport 批量导入素材。
//
// 参数说明：
//   - 请求体 "materials": 要导入的素材列表。
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

// ConvertFileToUtf8 将上传的文件转码为 UTF-8 编码，可选移除 HTML 标签与多余空白，返回转换后的文本内容。
//
// 参数说明：
//   - 表单文件 "file": 待转码的文件。
//   - 表单值 "remove_tag": 是否移除 HTML 标签，默认为 false。
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
