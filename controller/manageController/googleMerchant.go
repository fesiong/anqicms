package manageController

import (
	"context"

	"github.com/kataras/iris/v12"
	content "google.golang.org/api/content/v2.1"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGoogleMerchantConfig 获取 Google 商品同步配置（无参数）。
func PluginGoogleMerchantConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetGoogleMerchantSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginGoogleMerchantConfigSave 保存 Google 商品同步配置；整体落库，未传的字段（含 token 与 accounts）会被清成零值。
func PluginGoogleMerchantConfigSave(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginGoogleMerchantConfig
	if !readJSONPayload(ctx, &req) {
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

// PluginGoogleMerchantAuthUrl 生成 Google 商家授权的跳转链接，供管理员点击完成授权。
//
// 参数说明：
//   - 查询参数 "state": 防 CSRF 的随机串，可选；不传时服务端生成 32 位随机串，并在返回结果里一并给出，回调时需原样带回。
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
		state = config.GenerateRandString(32)
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

// PluginGoogleMerchantCallback 接收 Google 授权回调：用 code 换令牌并落库，state 需与授权时一致。
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

// PluginGoogleMerchantAccountSelect 选择要同步商品的 Merchant Center 账号。
func PluginGoogleMerchantAccountSelect(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.GetGoogleMerchantSetting()
	var req request.PluginGoogleMerchantAccountRequest
	if !readJSONPayload(ctx, &req) {
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

// PluginGoogleMerchantSyncLogs 分页查询商品同步日志。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
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

// PluginGoogleMerchantSyncStatusList 分页查询各商品的 Google 同步状态。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
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

// PluginGoogleMerchantSync 把商品推送到 Google Merchant Center。
// 异步执行：接口立即返回，实际同步在后台跑，进度看 sync/statuses 与 sync/logs。
// full=true 表示全量重建，否则按变动增量同步；ids 字段在本端点不被使用。
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

// PluginGoogleMerchantSyncDelete 从 Google Merchant Center 删除 ids 指定的商品。
func PluginGoogleMerchantSyncDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 开始同步
	var req request.PluginGoogleMerchantSyncRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requireAnyID(ctx, 0, req.Ids) {
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
