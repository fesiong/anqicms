package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"path/filepath"
	"strings"
)

func PluginPayConfig(ctx iris.Context) {
	pluginRewrite := config.JsonData.PluginPay

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginRewrite,
	})
}

func PluginPayConfigForm(ctx iris.Context) {
	var req request.PluginPayConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginPay.AlipayAppId = req.AlipayAppId
	config.JsonData.PluginPay.AlipayPrivateKey = req.AlipayPrivateKey
	if req.AlipayCertPath != "" {
		config.JsonData.PluginPay.AlipayCertPath = req.AlipayCertPath
	}
	if req.AlipayRootCertPath != "" {
		config.JsonData.PluginPay.AlipayRootCertPath = req.AlipayRootCertPath
	}
	if req.AlipayPublicCertPath != "" {
		config.JsonData.PluginPay.AlipayPublicCertPath = req.AlipayPublicCertPath
	}

	config.JsonData.PluginPay.WechatAppId = req.WechatAppId
	config.JsonData.PluginPay.WechatAppSecret = req.WechatAppSecret
	config.JsonData.PluginPay.WeappAppId = req.WeappAppId
	config.JsonData.PluginPay.WeappAppSecret = req.WeappAppSecret

	config.JsonData.PluginPay.WechatMchId = req.WechatMchId
	config.JsonData.PluginPay.WechatApiKey = req.WechatApiKey
	if req.WechatCertPath != "" {
		config.JsonData.PluginPay.WechatCertPath = req.WechatCertPath
	}
	if req.WechatKeyPath != "" {
		config.JsonData.PluginPay.WechatKeyPath = req.WechatKeyPath
	}

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新支付配置信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginPayUploadFile(ctx iris.Context) {
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

	filePath := fmt.Sprintf("%sdata/cert/%s", config.ExecPath, name+".pem")
	buff, err := ioutil.ReadAll(file)
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

	fileName := strings.TrimPrefix(filePath, config.ExecPath)
	if name == "wechat_cert_path" {
		config.JsonData.PluginPay.WechatCertPath = fileName
	} else if name == "wechat_key_path" {
		config.JsonData.PluginPay.WechatKeyPath = fileName
	} else if name == "alipay_cert_path" {
		config.JsonData.PluginPay.AlipayCertPath = fileName
	}

	err = config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("上传支付Cert文件：%s", name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文件已上传完成",
		"data": fileName,
	})
}
