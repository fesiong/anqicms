package config

const Version = "2.5.2"

const (
	StatusOK         = 0
	StatusFailed     = -1
	StatusNoLogin    = 1001
	StatusNoAccess   = 1002
	StatusApiSuccess = 200
)

const (
	CustomFieldTypeText     = "text"
	CustomFieldTypeNumber   = "number"
	CustomFieldTypeTextarea = "textarea"
	CustomFieldTypeRadio    = "radio"
	CustomFieldTypeCheckbox = "checkbox"
	CustomFieldTypeSelect   = "select"
	CustomFieldTypeImage    = "image"
	CustomFieldTypeFile     = "file"
)

const (
	CategoryTypeArchive = 1
	CategoryTypePage    = 3
)

const (
	ContentStatusDraft = 0 // 草稿
	ContentStatusOK    = 1 // 正式内容
	ContentStatusPlan  = 2 // 计划内容，等待发布
)

const (
	UrlTokenTypeFull = 0
	UrlTokenTypeSort = 1
)

const (
	StorageTypeLocal   = "local" // or empty
	StorageTypeAliyun  = "aliyun"
	StorageTypeTencent = "tencent"
	StorageTypeQiniu   = "qiniu"
	StorageTypeUpyun   = "upyun"
)

// 支付状态， 0 待支付，1 已支付待发货，2 已发货待收货，3 已收货，8 申请退款中，9 已退款，-1 订单已关闭
const (
	OrderStatusCanceled   = -1
	OrderStatusWaiting    = 0
	OrderStatusPaid       = 1
	OrderStatusDelivering = 2
	OrderStatusCompleted  = 3

	OrderStatusRefunding = 8
	OrderStatusRefunded  = 9

	OrderRefundStatusWaiting = 0
	OrderRefundStatusDone    = 1
	OrderRefundStatusFailed  = -1 //退款失败

	CommissionStatusWait   = 0 //未提现
	CommissionStatusPaid   = 1 //已提现
	CommissionStatusCancel = -1

	PayWayWechat  = "wechat"  // 微信网页支付
	PayWayWeapp   = "weapp"   // 微信小程序支付
	PayWayAlipay  = "alipay"  // 支付宝支付
	PayWayOffline = "offline" // 线下支付
)

const (
	FinanceIncome = 1
	FinanceOutput = 2

	//资金类型
	FinanceActionSale       = 1
	FinanceActionBuy        = 2
	FinanceActionRefund     = 3
	FinanceActionCharge     = 4
	FinanceActionWithdraw   = 5
	FinanceActionSpread     = 6
	FinanceActionCashBack   = 7
	FinanceActionCommission = 8
)

const (
	WithdrawStatusWaiting  = 0
	WithdrawStatusAgree    = 1
	WithdrawStatusFinished = 2
	WithdrawStatusCanceled = -1
)

const (
	EnginBaidu  = "baidu" // or empty
	Engin360    = "360"
	EnginSogou  = "sogou"
	EnginGoogle = "google"
	EnginBingCn = "bingcn"
	EnginBing   = "bing"
	EnginOther  = "other"

	LanguageZh = "zh"
	LanguageEn = "en"

	CollectModeCollect = 0
	CollectModeCombine = 1
)

// login platform
const (
	PlatformTT       = "tt"       // toutiao
	PlatformSwan     = "swan"     // baidu
	PlatformAlipay   = "alipay"   // alipay
	PlatformQQ       = "qq"       // qq
	PlatformWeapp    = "weapp"    // weapp mini program
	PlatformWechat   = "wechat"   // WeChat official account
	PlatformInWechat = "inwechat" // in the wechat browser
	PlatformWebsite  = "website"  // normal login way, like email/cellphone/username
)

const (
	OrderTypeGoods = "goods"
	OrderTypeVip   = "vip"
)
