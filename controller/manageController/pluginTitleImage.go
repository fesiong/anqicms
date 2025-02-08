package manageController

import (
	"github.com/kataras/iris/v12"
	"golang.org/x/image/webp"
	"image"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"os"
	"path/filepath"
	"strings"
)

func PluginTitleImageConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginTitleImage

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginTitleImageConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
	currentSite.PluginTitleImage.DrawSub = req.DrawSub
	currentSite.PluginTitleImage.BgImages = req.BgImages
	currentSite.PluginTitleImage.FontPath = req.FontPath
	currentSite.PluginTitleImage.FontSize = req.FontSize
	currentSite.PluginTitleImage.FontBgColor = req.FontBgColor
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

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateTitleImageConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginTitleImagePreview(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	text := ctx.URLParamDefault("text", ctx.Tr("WelcomeToAnqiCMS"))
	str := currentSite.NewTitleImage().DrawPreview(text)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": str,
	})
}

func PluginTitleImageUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	name := ctx.PostValue("name")
	// allow upload font and image
	if name != "font_path" && name != "bg_image" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileNameInvalid"),
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
				"msg":  ctx.Tr("OnlySupportsTtfFormat"),
			})
			return
		}
		fileName = "uploads/titleimage/title_font.ttf"
	} else {
		// image
		_, _, err = image.Decode(file)
		if err != nil {
			file.Seek(0, 0)
			if strings.HasSuffix(info.Filename, "webp") {
				_, err = webp.Decode(file)
			}
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("UnsupportedImageFormat"),
				})
				return
			}
		}
		file.Seek(0, 0)
		fileName = "uploads/titleimage/" + library.Md5(info.Filename) + filepath.Ext(info.Filename)
	}
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ReadFailed"),
		})
		return
	}

	err = os.MkdirAll(filepath.Dir(currentSite.PublicPath+fileName), os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("DirectoryCreationFailed"),
		})
		return
	}
	err = os.WriteFile(currentSite.PublicPath+fileName, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileSaveFailed"),
		})
		return
	}

	if name == "font_path" {
		currentSite.PluginTitleImage.FontPath = fileName
		err = currentSite.SaveSettingValue(provider.TitleImageSettingKey, currentSite.PluginTitleImage)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadTitleImageResourcesLog", name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
		"data": fileName,
	})
}

func PluginTitleImageGenerate(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	currentSite.GenerateAllTitleImages()

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchGenerateWatermarkImages"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}
