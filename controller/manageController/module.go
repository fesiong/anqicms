package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"regexp"
)

func ModuleList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	modules, err := currentSite.GetModules()
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
		"data": modules,
	})
}

func ModuleDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	module, err := currentSite.GetModuleById(id)
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
		"data": module,
	})
}

func ModuleDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ModuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	matched, err := regexp.MatchString(`^[a-z][a-z0-9_]*$`, req.TableName)
	if req.TableName == "" || !matched {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请正确填写模型表名",
		})
		return
	}

	matched, _ = regexp.MatchString(`^[a-z][a-z0-9_]*$`, req.UrlToken)
	if req.UrlToken == "" || !matched {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请正确填写URL别名",
		})
		return
	}

	module, err := currentSite.SaveModule(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 更新缓存
	go func() {
		currentSite.BuildModuleCache(ctx)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改文档模型：%d => %s", module.Id, module.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
		"data": module,
	})
}

func ModuleFieldsDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ModuleFieldRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteModuleField(req.Id, req.FieldName)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模型字段：%d => %s", req.Id, req.FieldName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "字段已删除",
	})
}

func ModuleDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ModuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	module, err := currentSite.GetModuleById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if module.IsSystem == 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "内置模型不能删除",
		})
		return
	}

	err = currentSite.DeleteModule(module)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除文档模型：%d => %s", module.Id, module.Title))

	currentSite.DeleteCacheModules()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "模型已删除",
	})
}
