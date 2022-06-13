package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"strings"
	"time"
)

func SettingSystem(ctx iris.Context) {
	system := config.JsonData.System
	if system.SiteLogo != "" && !strings.HasPrefix(system.SiteLogo, "http") {
		system.SiteLogo = config.JsonData.System.BaseUrl + system.SiteLogo
	}

	// 读取language列表
	var languages []string
	readerInfos, err := ioutil.ReadDir(fmt.Sprintf("%slanguage", config.ExecPath))
	if err == nil {
		for _, info := range readerInfos {
			if strings.HasSuffix(info.Name(), ".yml") {
				languages = append(languages, strings.TrimSuffix(info.Name(), ".yml"))
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"system":         system,
			"languages":      languages,
		},
	})
}

func SettingSystemForm(ctx iris.Context) {
	var req request.SystemConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.AdminUrl = strings.TrimSpace(req.AdminUrl)
	if req.AdminUrl != "" && !strings.HasPrefix(req.AdminUrl, "http") {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "后台域名请填写正确的域名，并做好解析，否则可能会导致后台无法访问",
		})
		return
	}

	req.SiteLogo = strings.TrimPrefix(req.SiteLogo, config.JsonData.System.BaseUrl)

	////进行一些限制
	//if req.TemplateType == config.TemplateTypeSeparate {
	//	if req.MobileUrl == req.BaseUrl {
	//		ctx.JSON(iris.Map{
	//			"code": config.StatusFailed,
	//			"msg":  "手机端域名不能和电脑端域名一样",
	//		})
	//		return
	//	} else if req.MobileUrl == "" {
	//		ctx.JSON(iris.Map{
	//			"code": config.StatusFailed,
	//			"msg":  "你选择了电脑+手机模板类型，请填写手机端域名",
	//		})
	//		return
	//	}
	//}

	changed := false
	if config.JsonData.System.AdminUrl != req.AdminUrl {
		changed = true
	}
	if req.ExtraFields != nil {
		for i := range req.ExtraFields {
			req.ExtraFields[i].Name = library.Case2Camel(req.ExtraFields[i].Name)
		}
	}

	config.JsonData.System.SiteName = req.SiteName
	config.JsonData.System.SiteLogo = req.SiteLogo
	config.JsonData.System.SiteIcp = req.SiteIcp
	config.JsonData.System.SiteCopyright = req.SiteCopyright
	config.JsonData.System.AdminUrl = req.AdminUrl
	config.JsonData.System.SiteClose = req.SiteClose
	config.JsonData.System.SiteCloseTips = req.SiteCloseTips
	//config.JsonData.System.TemplateName = req.TemplateName
	//config.JsonData.System.TemplateType = req.TemplateType
	config.JsonData.System.BaseUrl = req.BaseUrl
	config.JsonData.System.MobileUrl = req.MobileUrl
	config.JsonData.System.Language = req.Language
	config.JsonData.System.ExtraFields = req.ExtraFields

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 重载 language
	config.LoadLanguage()

	// 如果切换了模板，则重载模板
	if changed {
		config.RestartChan <- true
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新系统配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingContent(ctx iris.Context) {
	system := config.JsonData.Content
	if system.DefaultThumb != "" && !strings.HasPrefix(system.DefaultThumb, "http") {
		system.DefaultThumb = config.JsonData.System.BaseUrl + system.DefaultThumb
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingContentForm(ctx iris.Context) {
	var req request.ContentConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.DefaultThumb = strings.TrimPrefix(req.DefaultThumb, config.JsonData.System.BaseUrl)

	config.JsonData.Content.RemoteDownload = req.RemoteDownload
	config.JsonData.Content.FilterOutlink = req.FilterOutlink
	config.JsonData.Content.ResizeImage = req.ResizeImage
	config.JsonData.Content.ResizeWidth = req.ResizeWidth
	config.JsonData.Content.ThumbCrop = req.ThumbCrop
	config.JsonData.Content.ThumbWidth = req.ThumbWidth
	config.JsonData.Content.ThumbHeight = req.ThumbHeight
	config.JsonData.Content.DefaultThumb = req.DefaultThumb

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新内容配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

//重建所有的thumb
func SettingThumbRebuild(ctx iris.Context) {
	go provider.ThumbRebuild()

	provider.AddAdminLog(ctx, fmt.Sprintf("重新生成所有缩略图"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "缩略图已更新",
	})
}

func SettingIndex(ctx iris.Context) {
	system := config.JsonData.Index

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingIndexForm(ctx iris.Context) {
	var req request.IndexConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.Index.SeoTitle = req.SeoTitle
	config.JsonData.Index.SeoKeywords = req.SeoKeywords
	config.JsonData.Index.SeoDescription = req.SeoDescription

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新首页TDK"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingNav(ctx iris.Context) {
	navList, _ := provider.GetNavList(false)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func SettingNavForm(ctx iris.Context) {
	var req request.NavConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var nav *model.Nav
	var err error
	if req.Id > 0 {
		nav, err = provider.GetNavById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		nav = &model.Nav{
			Status: 1,
		}
	}

	nav.Title = req.Title
	nav.SubTitle = req.SubTitle
	nav.Description = req.Description
	nav.ParentId = req.ParentId
	nav.NavType = req.NavType
	nav.PageId = req.PageId
	nav.Link = req.Link
	nav.Sort = req.Sort
	nav.Status = 1

	err = nav.Save(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新导航信息：%d => %s", nav.Id, nav.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingNavDelete(ctx iris.Context) {
	var req request.NavConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	nav, err := provider.GetNavById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = nav.Delete(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除导航信息：%d => %s", nav.Id, nav.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "导航已删除",
	})
}

func SettingContact(ctx iris.Context) {
	system := config.JsonData.Contact
	if system.Qrcode != "" && !strings.HasPrefix(system.Qrcode, "http") {
		system.Qrcode = config.JsonData.System.BaseUrl + system.Qrcode
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingContactForm(ctx iris.Context) {
	var req request.ContactConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.Qrcode = strings.TrimPrefix(req.Qrcode, config.JsonData.System.BaseUrl)

	if req.ExtraFields != nil {
		for i := range req.ExtraFields {
			req.ExtraFields[i].Name = library.Case2Camel(req.ExtraFields[i].Name)
		}
	}

	config.JsonData.Contact.UserName = req.UserName
	config.JsonData.Contact.Cellphone = req.Cellphone
	config.JsonData.Contact.Address = req.Address
	config.JsonData.Contact.Email = req.Email
	config.JsonData.Contact.Wechat = req.Wechat
	config.JsonData.Contact.Qrcode = req.Qrcode
	config.JsonData.Contact.ExtraFields = req.ExtraFields

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新联系人信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingCache(ctx iris.Context) {
	filePath := fmt.Sprintf("%scache/%s.log", config.ExecPath, "cache_clear")
	info, err := os.Stat(filePath)
	var lastUpdate int64
	if err == nil {
		lastUpdate = info.ModTime().Unix()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"last_update": lastUpdate,
		},
	})
}

func SettingCacheForm(ctx iris.Context) {
	// todo, 清理缓存
	provider.DeleteCacheCategories()
	provider.DeleteCacheFixedLinks()
	provider.DeleteCacheModules()
	provider.DeleteCacheRedirects()
	// 记录
	filePath := fmt.Sprintf("%scache/%s.log", config.ExecPath, "cache_clear")
	ioutil.WriteFile(filePath, []byte(fmt.Sprintf("%d", time.Now().Unix())), os.ModePerm)

	provider.AddAdminLog(ctx, fmt.Sprintf("手动更新缓存"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "缓存已更新",
	})
}
