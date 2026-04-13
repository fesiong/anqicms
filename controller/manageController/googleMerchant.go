package manageController

import (
	"context"
	"github.com/kataras/iris/v12"
	"google.golang.org/api/content/v2.1"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginGoogleMerchantConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetGoogleMerchantSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginGoogleMerchantConfigSave(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginGoogleMerchantConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	setting := currentSite.GetGoogleMerchantSetting()
	setting.Open = req.Open
	setting.ClientId = req.ClientId
	setting.ClientSecret = req.ClientSecret
	setting.PlacesApiKey = req.PlacesApiKey

	err := currentSite.SaveSettingValue(provider.GoogleMerchantSettingKey, req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateGoogleMerchantConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginGoogleMerchantAuthUrl(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	googleCfg := currentSite.GetGoogleMerchantConfig(true)
	if googleCfg == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("GoogleAuthDisable"),
		})
		return
	}

	state := ctx.URLParam("state")
	if state == "" {
		state = library.GenerateRandString(32)
	}
	link := googleCfg.AuthCodeURL(state)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": iris.Map{
			"state": state,
			"url":   link,
		},
	})
}

func PluginGoogleMerchantCallback(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginGoogleAuthRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Code == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Code",
		})
		return
	}

	googleCfg := currentSite.GetGoogleMerchantConfig(true)
	if googleCfg == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("GoogleAuthDisable"),
		})
		return
	}

	token, err := googleCfg.Exchange(ctx, req.Code)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Failed to exchange token: " + err.Error(),
		})
		return
	}

	// 4. 此时 token 包含了 AccessToken, RefreshToken, Expiry 等
	// 重要：检查 RefreshToken 是否存在。首次授权通常会有，后续可能为空。
	if token.RefreshToken == "" {
		// 这可能意味着之前已经授权过，且刷新令牌仍有效。
		// 你需要从数据库中查找该用户已有的刷新令牌来使用。
	}

	// 5. (可选) 使用获取到的令牌初始化 API 客户端，并获取商户的 Merchant Center ID
	client := googleCfg.Client(ctx, token)
	oauth2Service, err := oauth2.NewService(context.Background(), option.WithHTTPClient(client))
	userInfo, err := oauth2Service.Userinfo.Get().Do()

	setting := currentSite.GetGoogleMerchantSetting()
	setting.Email = userInfo.Email
	setting.Name = userInfo.Name
	setting.GivenName = userInfo.GivenName
	setting.FamilyName = userInfo.FamilyName
	if token.RefreshToken == "" {
		token.RefreshToken = setting.RefreshToken
	}
	setting.Token = token
	err = currentSite.SaveSettingValue(provider.GoogleMerchantSettingKey, setting)

	// save merchant info
	contentSvc, err := content.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Failed to get Merchant Center ID: " + err.Error(),
		})
		return
	}
	// 获取商户信息，以验证权限并存储 merchant_id
	authMerchantInfo, err := contentSvc.Accounts.Authinfo().Do()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Failed to get Merchant Center ID: " + err.Error(),
		})
		return
	}
	if len(authMerchantInfo.AccountIdentifiers) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Failed to get Merchant Center ID: " + err.Error(),
		})
		return
	}
	setting.MerchantId = authMerchantInfo.AccountIdentifiers[0].MerchantId
	setting.AggregatorId = authMerchantInfo.AccountIdentifiers[0].AggregatorId
	setting.Accounts = authMerchantInfo.AccountIdentifiers

	err = currentSite.SaveSettingValue(provider.GoogleMerchantSettingKey, setting)

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateGoogleMerchantConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "Success",
		"data": setting,
	})
}

func PluginGoogleMerchantAccountSelect(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetGoogleMerchantSetting()
	var req request.PluginGoogleMerchantAccountRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.MerchantId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Merchant ID",
		})
		return
	}

	var exists bool
	for _, item := range setting.Accounts {
		if item.MerchantId == req.MerchantId {
			exists = true
			setting.MerchantId = item.MerchantId
			setting.AggregatorId = item.AggregatorId
			break
		}
	}
	if !exists {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Invalid Merchant ID",
		})
		return
	}
	_ = currentSite.SaveSettingValue(provider.GoogleMerchantSettingKey, setting)

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateGoogleMerchantConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "Success",
		"data": setting,
	})
}

func PluginGoogleMerchantSyncLogs(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	list, total := currentSite.GetGoogleMerchantSyncLogs(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

func PluginGoogleMerchantSyncStatusList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	list, total := currentSite.GetGoogleMerchantSyncStatuses(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

func PluginGoogleMerchantSync(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 开始同步
	var req request.PluginGoogleMerchantSyncRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	go currentSite.StartSyncGoogleMerchantProducts(req.Full, nil)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

func PluginGoogleMerchantSyncDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 开始同步
	var req request.PluginGoogleMerchantSyncRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err := currentSite.DeleteSyncGoogleMerchantProducts(req.Ids)
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
	})
}
