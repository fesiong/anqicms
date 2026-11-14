package manageController

import (
	"encoding/json"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// SmsSecretMask 是短信渠道凭据的掩码占位符，与 SendmailPasswordMask 同串但**独立声明**
// （参考 provider/channelAdmin.go:29 的既有理由：复用同一常量会让「改发信授权码掩码」
// 意外改到短信）。读端点把非空凭据替换成它；写端点把它识别为「未改动」哨兵。
const SmsSecretMask = "********"

// maskSmsSecrets 对一份**值拷贝**的短信配置逐字段脱敏：非空凭据置哨兵，空凭据
// 保持空串（站长要能分清配没配，不给空值打掩码）。只改 string 凭据字段，不碰
// Templates / Texts 两个 map；读端点用的是解引用值拷贝，故不会污染内存里的真实 AK/SK。
func maskSmsSecrets(setting *config.PluginSmsConfig) {
	if setting.Aliyun.Domestic.AccessKeyId != "" {
		setting.Aliyun.Domestic.AccessKeyId = SmsSecretMask
	}
	if setting.Aliyun.Domestic.AccessKeySecret != "" {
		setting.Aliyun.Domestic.AccessKeySecret = SmsSecretMask
	}
	if setting.Aliyun.International.AccessKeyId != "" {
		setting.Aliyun.International.AccessKeyId = SmsSecretMask
	}
	if setting.Aliyun.International.AccessKeySecret != "" {
		setting.Aliyun.International.AccessKeySecret = SmsSecretMask
	}
	if setting.Tencent.Domestic.SecretId != "" {
		setting.Tencent.Domestic.SecretId = SmsSecretMask
	}
	if setting.Tencent.Domestic.SecretKey != "" {
		setting.Tencent.Domestic.SecretKey = SmsSecretMask
	}
	if setting.Tencent.International.SecretId != "" {
		setting.Tencent.International.SecretId = SmsSecretMask
	}
	if setting.Tencent.International.SecretKey != "" {
		setting.Tencent.International.SecretKey = SmsSecretMask
	}
	if setting.Twilio.International.AccountSid != "" {
		setting.Twilio.International.AccountSid = SmsSecretMask
	}
	if setting.Twilio.International.AuthToken != "" {
		setting.Twilio.International.AuthToken = SmsSecretMask
	}
	if setting.Twilio.International.ApiKeySid != "" {
		setting.Twilio.International.ApiKeySid = SmsSecretMask
	}
	if setting.Twilio.International.ApiKeySecret != "" {
		setting.Twilio.International.ApiKeySecret = SmsSecretMask
	}
}

// restoreSmsSecretSentinels 哨兵回写保护：值**恰好**等于 SmsSecretMask 时取 current
// 里对应的真值；新值原样保留（更新）；空串真的清空 —— 哨兵只精确匹配 "********"，
// 空串是主动清空而不是「未改动」。12 个凭据字段逐个处理，漏一个就会把哨兵串
// 当明文凭据存进库（站点随即发不出短信，且无任何报错）。
func restoreSmsSecretSentinels(req *config.PluginSmsConfig, current *config.PluginSmsConfig) {
	if req.Aliyun.Domestic.AccessKeyId == SmsSecretMask {
		req.Aliyun.Domestic.AccessKeyId = current.Aliyun.Domestic.AccessKeyId
	}
	if req.Aliyun.Domestic.AccessKeySecret == SmsSecretMask {
		req.Aliyun.Domestic.AccessKeySecret = current.Aliyun.Domestic.AccessKeySecret
	}
	if req.Aliyun.International.AccessKeyId == SmsSecretMask {
		req.Aliyun.International.AccessKeyId = current.Aliyun.International.AccessKeyId
	}
	if req.Aliyun.International.AccessKeySecret == SmsSecretMask {
		req.Aliyun.International.AccessKeySecret = current.Aliyun.International.AccessKeySecret
	}
	if req.Tencent.Domestic.SecretId == SmsSecretMask {
		req.Tencent.Domestic.SecretId = current.Tencent.Domestic.SecretId
	}
	if req.Tencent.Domestic.SecretKey == SmsSecretMask {
		req.Tencent.Domestic.SecretKey = current.Tencent.Domestic.SecretKey
	}
	if req.Tencent.International.SecretId == SmsSecretMask {
		req.Tencent.International.SecretId = current.Tencent.International.SecretId
	}
	if req.Tencent.International.SecretKey == SmsSecretMask {
		req.Tencent.International.SecretKey = current.Tencent.International.SecretKey
	}
	if req.Twilio.International.AccountSid == SmsSecretMask {
		req.Twilio.International.AccountSid = current.Twilio.International.AccountSid
	}
	if req.Twilio.International.AuthToken == SmsSecretMask {
		req.Twilio.International.AuthToken = current.Twilio.International.AuthToken
	}
	if req.Twilio.International.ApiKeySid == SmsSecretMask {
		req.Twilio.International.ApiKeySid = current.Twilio.International.ApiKeySid
	}
	if req.Twilio.International.ApiKeySecret == SmsSecretMask {
		req.Twilio.International.ApiKeySecret = current.Twilio.International.ApiKeySecret
	}
}

// PluginSmsSetting 获取当前站点的短信渠道配置（读掩码）。
func PluginSmsSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 必须解引用做**值拷贝**再脱敏。
	//
	// PluginSms 的类型是 *config.PluginSmsConfig（provider/website.go:88），
	// 所以 `setting := currentSite.PluginSms` 只是拷贝指针，随后
	// `setting.Aliyun... = mask` 改的就是内存里的全局对象 —— 一次 setting_get
	// 调用就会把运行中的真实 AK/SK 抹成哨兵串，之后所有发信失败且没有任何
	// 报错（2026-10-03 实际踩过，与 PluginSendmailSetting 同源）。
	//
	// 嵌套 region 结构体是值类型，随外层值拷贝一起复制；Templates / Texts 是
	// map（引用类型），值拷贝会共享底层，但脱敏**只读不改** map，故安全 ——
	// 将来若要修改这两个 map 必须显式深拷贝（规格 §3.4）。
	setting := *currentSite.PluginSms
	maskSmsSecrets(&setting)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSmsSettingForm 保存短信渠道配置（部分更新 + 哨兵回写保护）。
//
// null 的两种行为不同（library/merge.go:40 直接调 json.Unmarshal，规格 §3.5）：
//   - 顶层 `"aliyun": null` → **no-op**：null 解进结构体值是 no-op，基底原值保留；
//   - 嵌套 `"aliyun": {"domestic": {"templates": null}}` → **清空**：null 解进
//     map 会把它设成 nil，该区域所有场景模板静默丢失、界面上看不出来。
//
// 因此前端**不得**提交 templates / texts 的 null map：要么带完整 map，要么整个
// 厂商对象不出现在 payload 里。前端 payload 护栏见任务 14。
func PluginSmsSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 具名 ReadJSON(&req) 必须保留且恰好一次：cmd/apicatalog 靠它的具名结构体
	// 类型生成字段级 params，换成 map 或第二次 ReadJSON 会让 params 归零（R103/R105）。
	var req config.PluginSmsConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 部分更新：以现值为基底，仅覆盖请求体中显式出现的字段，未传字段保持原配置。
	req = *currentSite.PluginSms
	body, err := ctx.GetBody()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 12 个凭据逐个哨兵回写保护（值为 ******** 则取内存真值，空串真的清空）。
	restoreSmsSecretSentinels(&req, currentSite.PluginSms)
	// 保持指针身份：其它代码（provider/smsSend.go 等发码流程）持有该指针，
	// 不能写成 currentSite.PluginSms = &req。
	*currentSite.PluginSms = req

	if err := currentSite.SaveSettingValue(provider.SmsSettingKey, currentSite.PluginSms); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ConfigurationUpdated"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}
