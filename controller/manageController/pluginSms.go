package manageController

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"

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
		// R146③：请求体不合法是「参数错误」而不是「保存失败」，回 SmsParamInvalid；
		// R114/R144：解析器原文只进服务端日志（只记 error 对象，绝不记请求体）。
		log.Println("后台短信配置保存失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SmsParamInvalid"),
		})
		return
	}
	// 部分更新：以现值为基底，仅覆盖请求体中显式出现的字段，未传字段保持原配置。
	req = *currentSite.PluginSms
	body, err := ctx.GetBody()
	if err != nil {
		log.Println("后台短信配置保存失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SmsParamInvalid"),
		})
		return
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Println("后台短信配置保存失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SmsParamInvalid"),
		})
		return
	}
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		log.Println("后台短信配置保存失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SmsParamInvalid"),
		})
		return
	}
	// 12 个凭据逐个哨兵回写保护（值为 ******** 则取内存真值，空串真的清空）。
	restoreSmsSecretSentinels(&req, currentSite.PluginSms)
	// 阿里云 endpoint 入站归一化（裁决 R36）：站长最常见的填法是带 scheme 的
	// 「https://dysmsapi.aliyuncs.com」，而适配器裸拼 "https://"+endpoint+"/?" →
	// 拼出 https://https//… 的畸形 URL，要到发请求才炸。保存时归一化，
	// 库里既有值另有 ApplyDefaults 的防御性归一化兜底。
	req.Aliyun.Domestic.Endpoint = sms.NormalizeEndpointHost(req.Aliyun.Domestic.Endpoint)
	req.Aliyun.International.Endpoint = sms.NormalizeEndpointHost(req.Aliyun.International.Endpoint)
	// 保持指针身份：其它代码（provider/smsSend.go 等发码流程）持有该指针，
	// 不能写成 currentSite.PluginSms = &req。
	*currentSite.PluginSms = req

	if err := currentSite.SaveSettingValue(provider.SmsSettingKey, currentSite.PluginSms); err != nil {
		// DB 失败不是「参数格式不正确」，语义上是保存失败；原文只进服务端日志。
		log.Println("后台短信配置保存失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SaveFailed"),
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

// respondSmsOpsError 是后台短信端点（test / logs）的统一错误出口（R114/R146）。
// code 恒为 config.StatusFailed；msg 只暴露可翻译文案，绝不回显解析器或驱动的内部措辞。
// 判定顺序与前台 respondSmsError 同一机制（R144/R145，字典即白名单）：
//  1. *WaitError / *AttemptsError 按类型分支（唯一需要传参的两类）；
//  2. errors.Is(err, sms.ErrSmsSendFailed) 直接归约（*SendError 经 Unwrap 落这里）；
//  3. 其余：Tr(raw) != raw 即说明 raw 是一枚真实业务键（Tr 兜底同样是原样返回）；
//     含 % 的文本经兜底格式化会被改写、从而被误判成键，先短路归约。
//
// 原先手抄的 smsOpsKnownBusinessErrors 已删除（漏了 SmsCodeLocked，且与字典漂移）。
func respondSmsOpsError(ctx iris.Context, currentSite *provider.Website, err error) {
	var wait *sms.WaitError
	var attempts *sms.AttemptsError
	// 方法值：统一 currentSite.Tr(key, args...) 的翻译出口。
	translate := currentSite.Tr
	var msg string
	switch {
	case errors.As(err, &wait):
		msg = translate(err.Error(), wait.Seconds)
	case errors.As(err, &attempts):
		msg = translate(err.Error(), attempts.Remaining)
	case errors.Is(err, sms.ErrSmsSendFailed):
		msg = translate(sms.ErrSmsSendFailed.Error())
	default:
		// 判据与前台统一收敛到 provider.SmsBusinessMessage（内部固定 TplTr）。
		// 不能再用 currentSite.Tr：Tr = Brandize(tr(x))，OEM 构建下含官方品牌名/
		// 域名的普通错误文本会被改写，从而被误判成语言键、把原文送出门。
		if translated, ok := provider.SmsBusinessMessage(currentSite, err); ok {
			msg = translated
		} else {
			// R146②：原始错误只进服务端日志（只记错误对象，不记请求体），对外归约（§10.4）。
			log.Println("后台短信端点内部错误：", err)
			msg = translate(sms.ErrSmsSendFailed.Error())
		}
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
		// R146③：请求体不合法，语义是「参数错误」而不是「发送失败」，
		// 故不经 ops 归约，直接回 SmsParamInvalid（同样不回显解析器文本）。
		// 服务端必须留一行日志，否则线上只能看到「参数格式不正确」而没有任何线索；
		// 只记 error 对象，绝不记请求体（可能含手机号）。
		log.Println("后台短信测试发送失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SmsParamInvalid"),
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
		Ip:           provider.SmsClientIP(ctx, currentSite),
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

// smsLogFilterParams 是 logs 查询的强类型筛选参数（R119② / R120③）。
// status 的 0 是合法筛选值，因此必须用 hasStatus 区分「未传」与「传 0」。
type smsLogFilterParams struct {
	status    int
	hasStatus bool
	startTS   int64
	endTS     int64
}

// smsParseLogFilters 校验并解析 status / start_time / end_time。
//
// 口径（R119② / R120③）：status 只允许 ""（不筛选）/ "0" / "1" / "-1"；
// start_time / end_time 只允许 "" 或正整数 Unix 秒；两者都传且 start > end 非法。
// ok = false 时 handler 必须返回 SmsParamInvalid，绝不静默当成未传 —— 静默忽略会
// 让非法的筛选条件退化成「全量」，结果集被放大且界面看不出原因。
func smsParseLogFilters(status, startTime, endTime string) (smsLogFilterParams, bool) {
	var p smsLogFilterParams
	switch status {
	case "":
	case "0":
		p.status, p.hasStatus = 0, true
	case "1":
		p.status, p.hasStatus = 1, true
	case "-1":
		p.status, p.hasStatus = -1, true
	default:
		return p, false
	}
	parseTS := func(raw string) (int64, bool) {
		if raw == "" {
			return 0, true
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return 0, false
		}
		return v, true
	}
	var ok bool
	if p.startTS, ok = parseTS(startTime); !ok {
		return p, false
	}
	if p.endTS, ok = parseTS(endTime); !ok {
		return p, false
	}
	if startTime != "" && endTime != "" && p.startTS > p.endTS {
		return p, false
	}
	return p, true
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
	// R120③：非法筛选参数明确报错（SmsParamInvalid），不得静默当成未传。
	filters, ok := smsParseLogFilters(status, startTime, endTime)
	if !ok {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SmsParamInvalid"),
		})
		return
	}

	list, total := currentSite.GetSmsLogList(func(tx *gorm.DB) *gorm.DB {
		if scene != "" {
			tx = tx.Where("scene = ?", scene)
		}
		if providerName != "" {
			tx = tx.Where("provider = ?", providerName)
		}
		if filters.hasStatus {
			tx = tx.Where("status = ?", filters.status)
		}
		if phone != "" {
			tx = tx.Where("phone = ?", phone)
		}
		if filters.startTS > 0 {
			tx = tx.Where("created_time >= ?", filters.startTS)
		}
		if filters.endTS > 0 {
			tx = tx.Where("created_time <= ?", filters.endTS)
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
