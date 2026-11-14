package manageController

import (
	"encoding/json"
	"errors"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/provider/sms"
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

// pluginSmsTestRequest 是后台测试发送的请求体（apicatalog 靠具名 ReadJSON 生成字段级 params）。
type pluginSmsTestRequest struct {
	PhoneCountry string `json:"phone_country"` // 区号，空则默认 86
	Phone        string `json:"phone"`         // 本地号码
}

// respondSmsOpsError 是后台短信端点（test / logs）的统一错误出口（R114）。
// code 恒为 config.StatusFailed；msg 只暴露可翻译键，绝不回显平台原始错误码/消息。
// 平台 *SendError 经其 Unwrap 归约到 sms.ErrSmsSendFailed；仅 *WaitError 传 Seconds、
// *AttemptsError 传 Remaining，其余错误一律不传参（Tr 兜底是字符串拼接，多传参会污染文案）。
func respondSmsOpsError(ctx iris.Context, currentSite *provider.Website, err error) {
	if errors.Is(err, sms.ErrSmsSendFailed) {
		err = sms.ErrSmsSendFailed
	}
	var wait *sms.WaitError
	var attempts *sms.AttemptsError
	// 方法值：与前台 respondSmsError 口径一致（统一 currentSite.Tr(err.Error(), args...)）。
	translate := currentSite.Tr
	var msg string
	switch {
	case errors.As(err, &wait):
		msg = translate(err.Error(), wait.Seconds)
	case errors.As(err, &attempts):
		msg = translate(err.Error(), attempts.Remaining)
	default:
		msg = translate(err.Error())
	}
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  msg,
	})
}

// PluginSmsTest 向指定号码发一条测试短信（POST /plugin/sms/test，规格 §7.2）。
// test 场景不受 Scenes 白名单限制，但仍走同一条 SendSmsCode 链路，IP 与号码日限照常生效。
// 模板取该 region 下 login 场景的模板；login 为空明确报错 ErrSmsTemplateMissing，
// 绝不静默套用别的场景，也绝不改 currentSite.PluginSms 内存真值去塞 test 模板（R113）。
func PluginSmsTest(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req pluginSmsTestRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 归一化定 region（与 SendSmsCode 第 3 步同口径）。
	phone, err := sms.NormalizePhone(req.PhoneCountry, req.Phone)
	if err != nil {
		respondSmsOpsError(ctx, currentSite, err)
		return
	}
	region := phone.Region()
	// 前置检查该 region 的 login 模板：为空则明确报错 sms.ErrSmsTemplateMissing，
	// 不静默套用别的场景；其它 Resolve 错误（缺凭据/签名/区域不支持）也照常回错误文案。
	if _, err := sms.Resolve(currentSite.PluginSms, region, sms.SceneLogin); err != nil {
		if errors.Is(err, sms.ErrSmsTemplateMissing) {
			respondSmsOpsError(ctx, currentSite, sms.ErrSmsTemplateMissing)
			return
		}
		respondSmsOpsError(ctx, currentSite, err)
		return
	}
	// 走同一条生产入口：Scene=test（第 2 步跳过白名单、仍受 IP/号码限额），
	// UserId=0（管理员不是前台用户，sms_logs.user_id 只对前台发码有意义）。
	result, err := currentSite.SendSmsCode(provider.SmsSendInput{
		PhoneCountry: req.PhoneCountry,
		Phone:        req.Phone,
		Scene:        sms.SceneTest,
		Ip:           ctx.RemoteAddr(),
		UserId:       0,
	})
	if err != nil {
		respondSmsOpsError(ctx, currentSite, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// PluginSmsLogs 分页查询短信发送记录（GET /plugin/sms/logs，规格 §7.2）。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - pageSize: 每页条数，默认为 20。
//   - 查询参数 "scene": 按场景精确筛选。
//   - 查询参数 "provider": 按服务商精确筛选。
//   - 查询参数 "status": 按发送状态精确筛选。
//   - 查询参数 "phone": 按号码精确筛选。
//   - 查询参数 "start_time": created_time 下界（含）。
//   - 查询参数 "end_time": created_time 上界（含）。
//
// ⚠️ 展示须知：sms_logs.template_id 对 Twilio 存的是含 {{code}} 占位符的
// 未渲染模板文本（符合 §4.3「不存渲染后正文」），展示时不得当作已渲染文案呈现（任务 14）。
func PluginSmsLogs(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	page := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	scene := ctx.URLParam("scene")
	providerName := ctx.URLParam("provider")
	status := ctx.URLParam("status")
	phone := ctx.URLParam("phone")
	startTime := ctx.URLParam("start_time")
	endTime := ctx.URLParam("end_time")

	list, total := currentSite.GetSmsLogList(func(tx *gorm.DB) *gorm.DB {
		if scene != "" {
			tx = tx.Where("scene = ?", scene)
		}
		if providerName != "" {
			tx = tx.Where("provider = ?", providerName)
		}
		if status != "" {
			tx = tx.Where("status = ?", status)
		}
		if phone != "" {
			tx = tx.Where("phone = ?", phone)
		}
		if startTime != "" {
			tx = tx.Where("created_time >= ?", startTime)
		}
		if endTime != "" {
			tx = tx.Where("created_time <= ?", endTime)
		}
		return tx
	}, page, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"data":  list,
		"total": total,
	})
}
