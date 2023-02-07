package manageController

import (
	"fmt"
	"github.com/chai2010/webp"
	"github.com/kataras/iris/v12"
	"image"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"os"
	"path/filepath"
	"strings"
)

func PluginTitleImageConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginTitleImage

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginTitleImageConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginTitleImageConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Width == 0 {
		req.Width = 800
	}
	if req.Height == 0 {
		req.Width = 600
	}
	if req.FontSize == 0 {
		req.FontSize = 28
	}

	currentSite.PluginTitleImage.Open = req.Open
	currentSite.PluginTitleImage.BgImage = req.BgImage
	currentSite.PluginTitleImage.FontPath = req.FontPath
	currentSite.PluginTitleImage.FontSize = req.FontSize
	currentSite.PluginTitleImage.Width = req.Width
	currentSite.PluginTitleImage.Height = req.Height
	currentSite.PluginTitleImage.Noise = req.Noise
	currentSite.PluginTitleImage.FontColor = req.FontColor

	err := currentSite.SaveSettingValue(provider.TitleImageSettingKey, currentSite.PluginTitleImage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新标题图片配置信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginTitleImagePreview(ctx iris.Context) {
	text := ctx.URLParamDefault("text", "欢迎使用安企内容管理系统")
	currentSite := provider.CurrentSite(ctx)
	str := currentSite.NewTitleImage(text).EncodeB64string()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": str,
	})
}

func PluginTitleImageUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	name := ctx.PostValue("name")
	// allow upload font and image
	if name != "font_path" && name != "bg_image" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件名无效",
		})
		return
	}

	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()
	var fileName string
	if name == "font_path" {
		// only support .ttf font
		if !strings.HasSuffix(info.Filename, ".ttf") {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "仅支持 .ttf 格式的字体文件",
			})
			return
		}
		fileName = "uploads/titleimage/title_font.ttf"
	} else {
		// image
		_, _, err := image.Decode(file)
		if err != nil {
			file.Seek(0, 0)
			if strings.HasSuffix(info.Filename, "webp") {
				_, err = webp.Decode(file)
			}
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "不支持的图片格式",
				})
				return
			}
		}
		file.Seek(0, 0)
		fileName = "uploads/titleimage/bg_image" + filepath.Ext(info.Filename)
	}
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "读取失败",
		})
		return
	}

	err = os.MkdirAll(filepath.Dir(currentSite.PublicPath+fileName), os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "目录创建失败",
		})
		return
	}
	err = os.WriteFile(currentSite.PublicPath+fileName, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件保存失败",
		})
		return
	}

	if name == "font_path" {
		currentSite.PluginTitleImage.FontPath = fileName
	} else {
		currentSite.PluginTitleImage.BgImage = fileName
	}
	err = currentSite.SaveSettingValue(provider.TitleImageSettingKey, currentSite.PluginTitleImage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传标题图片资源：%s", name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文件已上传完成",
		"data": fileName,
	})
}
