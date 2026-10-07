package manageController

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/request"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginGetPaymentAccounts 获取当前站点的全部支付账户列表。
func PluginGetPaymentAccounts(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	accounts := currentSite.GetPaymentAccounts(func(tx *gorm.DB) *gorm.DB {
		return tx
	})

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": accounts,
	})
}

// PluginPayStatistic 获取支付账户的收款统计列表，支持按账户筛选和分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "account_id": 支付账户 ID，默认为 0 表示全部账户。
func PluginPayStatistic(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	accountId := ctx.URLParamInt64Default("account_id", 0)
	result, total := currentSite.GetPaymentAccountStatistic(accountId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  result,
	})
}

// PluginGetPaymentAccountDetail 获取指定 ID 的支付账户详情。
//
// 参数说明：
//   - 查询参数 "id": 支付账户 ID，必填。
func PluginGetPaymentAccountDetail(ctx iris.Context) {
	id := ctx.URLParamInt64Default("id", 0)
	if id == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid ID",
		})
		return
	}
	currentSite := provider.CurrentSite(ctx)
	account, err := currentSite.GetPaymentAccountById(id)
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
		"data": account,
	})
}

// PluginSavePaymentAccount 保存（新增或更新）支付账户配置。
func PluginSavePaymentAccount(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PaymentAccountRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	account, err := currentSite.SavePaymentAccount(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdatePaymentConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
		"data": account,
	})
}

// PluginDeletePaymentAccount 删除指定 ID 的支付账户。
func PluginDeletePaymentAccount(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PaymentAccountRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeletePaymentAccount(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdatePaymentConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginPayUploadFile 上传支付账户所需的证书
//
// 参数说明：
//   - 表单参数 "name"：证书文件名，支持 .pem|.crt|.key 后缀。
//   - 表单参数 "file"：证书文件
func PluginPayUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	name := ctx.PostValue("name")
	if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".key") {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileNameInvalid"),
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
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ReadFailed"),
		})
		return
	}

	newName := library.Md5Bytes(buff)
	fileName := newName + ".pem"
	filePath := fmt.Sprint(currentSite.DataPath + "cert/" + fileName)

	err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("DirectoryCreationFailed"),
		})
		return
	}
	err = os.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileSaveFailed"),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadPaymentCertificateLog", name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
		"data": fileName,
	})
}
