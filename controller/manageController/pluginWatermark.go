package manageController

import (
	"github.com/kataras/iris/v12"
	"golang.org/x/image/webp"
	"image"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"os"
	"path/filepath"
	"strings"
)

func PluginWatermarkConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginWatermark

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginWatermarkConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginWatermark
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Size == 0 {
		req.Size = 20
	}
	if req.Position == 0 {
		req.Position = 9
	}
	if req.Opacity == 0 {
		req.Opacity = 100
	}
	if req.MinSize == 0 {
		req.MinSize = 400
	}
	if req.Color == "" {
		req.Color = "#ffffff"
	}

	currentSite.PluginWatermark.Open = req.Open
	currentSite.PluginWatermark.Type = req.Type
	currentSite.PluginWatermark.ImagePath = req.ImagePath
	currentSite.PluginWatermark.Text = req.Text
	currentSite.PluginWatermark.FontPath = req.FontPath
	currentSite.PluginWatermark.Size = req.Size
	currentSite.PluginWatermark.Color = req.Color
	currentSite.PluginWatermark.Position = req.Position
	currentSite.PluginWatermark.Opacity = req.Opacity

	err := currentSite.SaveSettingValue(provider.WatermarkSettingKey, currentSite.PluginWatermark)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateWatermarkConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginWatermarkPreview(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	cfg := currentSite.PluginWatermark
	cfg.Open = true
	wm := currentSite.NewWatermark(cfg)
	if wm == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "watermark error",
		})
		return
	}
	str := wm.DrawWatermarkPreview()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": str,
	})
}

func PluginWatermarkUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	name := ctx.PostValue("name")
	// allow upload font and image
	if name != "font_path" && name != "image_path" {
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
		fileName = "uploads/watermark/watermark_font.ttf"
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
		fileName = "uploads/watermark/watermark_image" + filepath.Ext(info.Filename)
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
		currentSite.PluginWatermark.FontPath = fileName
	} else {
		currentSite.PluginWatermark.ImagePath = fileName
	}
	err = currentSite.SaveSettingValue(provider.TitleImageSettingKey, currentSite.PluginTitleImage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadWatermarkResourceLog", name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
		"data": fileName,
	})
}

func PluginWatermarkGenerate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	currentSite.GenerateAllWatermark()

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchGenerateWatermarkImages"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}
