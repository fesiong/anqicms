package config

const Version = "1.0.0"
const Trial = false
const VersionType = ""

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
	CustomFieldTypeEditor   = "editor"
	CustomFieldTypeRadio    = "radio"
	CustomFieldTypeCheckbox = "checkbox"
	CustomFieldTypeSelect   = "select"
	CustomFieldTypeImage    = "image"
	CustomFieldTypeFile     = "file"
	CustomFieldTypeImages   = "images"
	CustomFieldTypeTexts    = "texts"
	CustomFieldTypeArchive  = "archive"
	CustomFieldTypeCategory = "category"
	CustomFieldTypeDate     = "date"
	CustomFieldTypeTime     = "time"
	CustomFieldTypeDatetime = "datetime"
	CustomFieldTypeColor    = "color"
	CustomFieldTypeTimeline = "timeline"
)

const (
	CategoryTypeArchive = 1
	CategoryTypePage    = 3

	CategoryShowTypeTree = 0
	CategoryShowTypeNode = 1
	CategoryShowTypeList = 2
)

const (
	ContentStatusDraft  = 0  // 草稿
	ContentStatusOK     = 1  // 正式内容
	ContentStatusPlan   = 2  // 计划内容，等待发布
	ContentStatusDelete = 99 // 已删除
)

const (
	UrlTokenTypeFull = 0
	UrlTokenTypeSort = 1
)

const DefaultQuality = 90

const (
	StorageTypeLocal   = "local" // or empty
	StorageTypeAliyun  = "aliyun"
	StorageTypeTencent = "tencent"
	StorageTypeQiniu   = "qiniu"
	StorageTypeUpyun   = "upyun"
	StorageTypeGoogle  = "google"
	StorageTypeAws     = "awss3"
	StorageTypeR2      = "r2" // Cloudflare R2
	StorageTypeFTP     = "ftp"
	StorageTypeSSH     = "ssh"
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

	PayWayWechat     = "wechat"     // 微信网页支付
	PayWayWeapp      = "weapp"      // 微信小程序支付
	PayWayAlipay     = "alipay"     // 支付宝支付
	PayWayOffline    = "offline"    // 线下支付
	PayWayBalance    = "balance"    // 余额支付
	PayWayPaypal     = "paypal"     // paypal
	PayWayStripe     = "stripe"     // stripe
	PayWayNowpayment = "nowpayment" // nowpayment
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
	CommunicationStatusWaiting  = 0
	CommunicationStatusDoing    = 1 // 沟通中
	CommunicationStatusQuote    = 2
	CommunicationStatusAccepted = 3
	CommunicationStatusCancel   = -1
	CommunicationStatusFinished = 4
	CommunicationStatusRejected = 5
	CommunicationStatusRefund   = 6
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

	LanguageZh = "zh-CN"
	LanguageEn = "en"

	CollectModeCollect = 0
	CollectModeCombine = 1

	CollectImageRemove   = 0 // 移除
	CollectImageRetain   = 1 // 保留
	CollectImageInsert   = 2 // 自定义插入
	CollectImageCategory = 3 // 插入指定分类图片
)

// login platform
const (
	PlatformTT       = "tt"       // toutiao
	PlatformSwan     = "swan"     // baidu
	PlatformAlipay   = "alipay"   // alipay
	PlatformQQ       = "qq"       // qq
	PlatformWeapp    = "weapp"    // weapp mini program
	PlatformWechat   = "wechat"   // WeChat official account
	PlatformGoogle   = "google"   // google +
	PlatformInWechat = "inwechat" // in the wechat browser
	PlatformWebsite  = "website"  // normal login way, like email/cellphone/username
)

const (
	OrderTypeGoods         = "goods"
	OrderTypeVip           = "vip"
	OrderTypeCommunication = "communication"
)

const (
	ProductTypeNormal       = 1 // 普通商品
	ProductTypeWholesale    = 2 //批发商品
	ProductTypeCustom       = 3 // 自定义商品
	ProductTypeVirtual      = 4 // 虚拟商品
	ProductTypeService      = 5 // 服务商品
	ProductTypeSubscription = 6 // 订阅商品
)

const (
	PasswordFindWayFile = "file"
	PasswordFindWayDNS  = "dns"
)

const (
	ArchiveFromDefault = 0
	ArchiveFromCollect = 1 // 采集的文章
	ArchiveFromAi      = 2 // AI生成的文章
)

const (
	AiArticleTypeDefault   = 0
	AiArticleTypeGenerate  = 1
	AiArticleTypeTranslate = 2
	AiArticleTypeAiPseudo  = 3
	AiArticleTypeSelfMedia = 4
)

// SingleArticleCount AI 改写使用， 1000 字一篇
const SingleArticleCount = 1000

const (
	AiArticleStatusCanceled  = -1
	AiArticleStatusWaiting   = 0
	AiArticleStatusDoing     = 1
	AiArticleStatusCompleted = 2
	AiArticleStatusError     = 4
)

const (
	InterferenceModeClass = 0 // 添加随机class
	InterferenceModeText  = 1 // 添加随机隐藏文字
)

const (
	MultiLangTypeDomain    = "domain"
	MultiLangTypeDirectory = "directory"
	MultiLangTypeSame      = "same"

	MultiLangSiteTypeMulti  = "multi"
	MultiLangSiteTypeSingle = "single"
)

const (
	CouponTypeRegister  = 1 // 注册送优惠券,每人一张
	CouponTypeLimit     = 2 // 限量优惠券,每人可领一张
	CouponTypeBuy       = 3 // 购买成功赠送,一次一张
	CouponTypePromotion = 4 // 促销优惠券, 满额直减,不需领取
	CouponTypeFree      = 5 // 自由规则, 请求接口即可获得
	CouponTypeIntegral  = 6 // 积分兑换券, 可以使用对应的积分兑换
	CouponTypeActivity  = 7 // 活动奖品(小游戏奖品)
)

const (
	CouponMoneyTypeFixed    = 0 // 固定金额
	CouponMoneyTypeRandom   = 1 // 随机金额
	CouponMoneyTypeDiscount = 2 // 固定折扣

	CouponRangeTypeAll        = 0 // 全部可用
	CouponRangeTypeProduct    = 1 // 指定商品可用
	CouponRangeTypeNotProduct = 2 // 指定商品不可用

	CouponStatusUnused  = 0 // 未使用
	CouponStatusUsing   = 1 // 使用中
	CouponStatusUsed    = 2 // 已使用
	CouponStatusExpired = 3 // 已过期

	OrderServiceTypeFixed      = 0 // 固定金额
	OrderServiceTypePercentage = 2 // 订单金额的百分比
)

const (
	UserStatusActive  = 1  // 已激活
	UserStatusPending = 0  // 待审核
	UserStatusBlocked = -1 // 已封禁
)

const (
	ShippingTypeFixed  = 1 // 固定运费
	ShippingTypeWeight = 2 // 按重量计费
	ShippingTypeVolume = 3 // 按体积计费
	ShippingTypeAmount = 4 // 按金额计费
)
