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

func PluginOrderList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	orders, total := provider.GetOrderList(0, "", currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func PluginOrderDetail(ctx iris.Context) {
	orderId := ctx.URLParam("order_id")

	order, err := provider.GetOrderInfoByOrderId(orderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order.User,  _ = provider.GetUserInfoById(order.UserId)
	if order.ShareUserId > 0 {
		order.ShareUser,  _ = provider.GetUserInfoById(order.ShareUserId)
	}
	if order.ShareParentUserId > 0 {
		order.ParentUser,  _ = provider.GetUserInfoById(order.ShareParentUserId)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

func PluginOrderSetDeliver(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SetOrderDeliver(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已设置成功",
	})
}

func PluginOrderSetFinished(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = provider.SetOrderFinished(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已设置成功",
	})
}

func PluginOrderSetCanceled(ctx iris.Context) {
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = provider.SetOrderCanceled(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已设置成功",
	})
}

func PluginOrderSetRefund(ctx iris.Context) {
	var req request.OrderRefundRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	order, err := provider.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = provider.SetOrderRefund(order, req.Status)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已设置成功",
	})
}

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

	config.JsonData.PluginPay.WeixinAppId = req.WeixinAppId
	config.JsonData.PluginPay.WeixinAppSecret = req.WeixinAppSecret
	config.JsonData.PluginPay.WeixinMchId = req.WeixinMchId
	config.JsonData.PluginPay.WeixinApiKey = req.WeixinApiKey
	if req.WeixinCertPath != "" {
		config.JsonData.PluginPay.WeixinCertPath = req.WeixinCertPath
	}
	if req.WeixinKeyPath != "" {
		config.JsonData.PluginPay.WeixinCertPath = req.WeixinKeyPath
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
	if name != "weixin_cert_path" && name != "weixin_key_path" && name != "alipay_cert_path" {
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
	err = ioutil.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件保存失败",
		})
		return
	}

	fileName := strings.TrimPrefix(filePath, config.ExecPath)
	if name == "weixin_cert_path" {
		config.JsonData.PluginPay.WeixinCertPath = fileName
	} else if name == "weixin_key_path" {
		config.JsonData.PluginPay.WeixinKeyPath = fileName
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

func PluginOrderExport(ctx iris.Context) {
	var req request.OrderExportRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	header, content := provider.ExportOrders(&req)

	provider.AddAdminLog(ctx, fmt.Sprintf("导出订单"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header": header,
			"content": content,
		},
	})
}