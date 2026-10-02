package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginRewrite 获取当前站点的伪静态重写配置。
func PluginRewrite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginRewrite := currentSite.PluginRewrite

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginRewrite,
	})
}

// PluginRewriteForm 保存当前站点的伪静态重写配置。
// 伪静态重写规则完整有11行，默认常用配置6行,分别是：archive=文档详情、category=分类文档列表、archiveIndex=模型首页、page=单页面、tagIndex=标签列表、tag=标签详情。
// 其他值：people=用户主页，peopleIndex=用户列表页，search=搜索页，place=城市页，placeIndex=城市列表页。
// 没条规则使用三个等号“===”做标记分割。
// 变量由花括号包裹 `{}` ,如 `{id}` 。可用的变量有:数据ID `{id}` ； 文档自定义链接名 `{filename}` ； 分类自定义链接名 `{catname}` ， 多级分类自定义链接名 `{multicatname}` , `{multicatname}` 和 `{catname}` 只能使用一个； 分类ID `{catid}` ； 模型表名 `{module}` ；年 `{year}` ， 月 `{month}` ， 日 `{day}` ， 时 `{hour}` ， 分 `{minute}` ， 秒 `{second}` ，年月日时分秒只有文档(archive)可用； 分页页码 `{page}` ,分页需放在小括号内, 如: `(/{page})` 。
// 文档详情还支持更灵活的自定义模式，可以单独给某个内容模型设置不同的伪静态规则。设置方法：{内容模型的URL别名}:archive==={规则}，如产品模型的URL别名后台设置的是：products，如需单独设置产品模型详情的伪静态规则为/{catname}/{filename}.html，则规则为：products:archive===/{catname}/{filename}.html
// 示例：
//
//	archive===/{module}/{filename}.html
//	category===/{module}/{catname}(/{page})
//	archiveIndex===/{module}.html
//	page===/{filename}.html
//	tagIndex===/tags(/{page})
//	tag===/tag/{filename}(/{page})
func PluginRewriteForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginRewriteConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.PluginRewrite.Mode != req.Mode || currentSite.PluginRewrite.Patten != req.Patten {
		currentSite.PluginRewrite.Mode = req.Mode
		currentSite.PluginRewrite.Patten = req.Patten
		err := currentSite.SaveSettingValue(provider.RewriteSettingKey, currentSite.PluginRewrite)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		currentSite.ParsePattern(true)
		currentSite.RemoveHtmlCache()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("AdjustPseudoStaticConfigurationLog", req.Mode))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}
