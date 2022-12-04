package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"os"
	"path/filepath"
)

func PluginPayConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginRewrite := currentSite.PluginPay

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginRewrite,
	})
}

func PluginPayConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginPayConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginPay.AlipayAppId = req.AlipayAppId
	currentSite.PluginPay.AlipayPrivateKey = req.AlipayPrivateKey
	if req.AlipayCertPath != "" {
		currentSite.PluginPay.AlipayCertPath = req.AlipayCertPath
	}
	if req.AlipayRootCertPath != "" {
		currentSite.PluginPay.AlipayRootCertPath = req.AlipayRootCertPath
	}
	if req.AlipayPublicCertPath != "" {
		currentSite.PluginPay.AlipayPublicCertPath = req.AlipayPublicCertPath
	}

	currentSite.PluginPay.WechatAppId = req.WechatAppId
	currentSite.PluginPay.WechatAppSecret = req.WechatAppSecret
	currentSite.PluginPay.WeappAppId = req.WeappAppId
	currentSite.PluginPay.WeappAppSecret = req.WeappAppSecret

	currentSite.PluginPay.WechatMchId = req.WechatMchId
	currentSite.PluginPay.WechatApiKey = req.WechatApiKey
	if req.WechatCertPath != "" {
		currentSite.PluginPay.WechatCertPath = req.WechatCertPath
	}
	if req.WechatKeyPath != "" {
		currentSite.PluginPay.WechatKeyPath = req.WechatKeyPath
	}

	err := currentSite.SaveSettingValue(provider.PaySettingKey, currentSite.PluginPay)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新支付配置信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginPayUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	name := ctx.PostValue("name")
	if name != "wechat_cert_path" && name != "wechat_key_path" && name != "alipay_cert_path" && name != "alipay_root_cert_path" && name != "alipay_public_cert_path" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件名无效",
		})
		return
	}

	file, _, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()
	fileName := name + ".pem"
	filePath := fmt.Sprintf(currentSite.DataPath + "cert/" + fileName)
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "读取失败",
		})
		return
	}

	err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "目录创建失败",
		})
		return
	}
	err = os.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件保存失败",
		})
		return
	}

	if name == "wechat_cert_path" {
		currentSite.PluginPay.WechatCertPath = fileName
	} else if name == "wechat_key_path" {
		currentSite.PluginPay.WechatKeyPath = fileName
	} else if name == "alipay_cert_path" {
		currentSite.PluginPay.AlipayCertPath = fileName
	} else if name == "alipay_root_cert_path" {
		currentSite.PluginPay.AlipayRootCertPath = fileName
	} else if name == "alipay_public_cert_path" {
		currentSite.PluginPay.AlipayPublicCertPath = fileName
	}

	err = currentSite.SaveSettingValue(provider.PaySettingKey, currentSite.PluginPay)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传支付证书文件：%s", name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文件已上传完成",
		"data": fileName,
	})
}
