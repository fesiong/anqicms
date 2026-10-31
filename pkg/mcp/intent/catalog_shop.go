package intent

// catalog_shop.go —— 商城（AnqiShop 合并）新增端点的语义化入口。
//
// 背景：商城分支带来 45 个新端点，合并时只补了 domain.go 的「模块 → 域」映射，
// 没有任何意图声明它们。后果不是"少一个 convenience 封装"，而是这些端点在
// 模型面里**完全不存在**：唯一的通用通道 api 是 DefaultOff，站点没配白名单时
// 连 api 工具都不出现，45 个端点等于 0 个可达。
//
// 分组按「运营动作」而不是「路由模块」，因为模型选工具时想的是任务：
//
//	commerce_marketing   发券、做活动（coupon / activity）
//	commerce_fulfillment 运费规则、询盘报价与站内消息（shipping / communication）
//	commerce_review      商品评价与增值服务项（order/review、order/service）
//	seo_merchant         Google Merchant Center 商品同步
//
// 愿望清单只有一个只读列表端点，挂在既有 commerce（会员）意图下，不单开工具。
//
// 全部 DefaultOff：交易域未随站点发行，且这几组能力都动真金白银
// （券面额、活动价、运费、报价）或对外可见（商品 Feed、发给客户的邮件/消息），
// 误操作的代价超出"改回来"的范围。站点要用时在 ExposedIntents 里显式点名。
//
// ⚠️ 金额字段（money/max_money/need_money/price/fee/quote_price/first_fee…）
// 单位一律是**分**，与后台表单的元不同——写错一位就是 100 倍误差。
var shopIntentCatalog = []*IntentSpec{
	// ───────────────────── 优惠券与营销活动 ─────────────────────
	{
		Name: "commerce_marketing", Title: "优惠券与营销活动", Domain: DomainCommerce, Risk: RiskWrite,
		Desc: "商城营销：优惠券（含券码、适用商品）与限时营销活动的增删改查。**默认关闭**，需 ExposedIntents 显式开启。" +
			"action: coupon_list/coupon_get/coupon_save/coupon_delete/coupon_archives/coupon_archives_add/coupon_archives_delete/" +
			"coupon_code_list/coupon_code_save/coupon_code_delete/coupon_code_make/activity_list/activity_get/activity_save/activity_delete。" +
			"金额字段（money/max_money/need_money/price）单位是**分**。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"coupon_list", "coupon_get", "coupon_save", "coupon_delete",
				"coupon_archives", "coupon_archives_add", "coupon_archives_delete",
				"coupon_code_list", "coupon_code_save", "coupon_code_delete", "coupon_code_make",
				"activity_list", "activity_get", "activity_save", "activity_delete",
			}},
			"id": {Type: "integer", Desc: "主键，含义随 action 变：" +
				"优惠券 ID（coupon_get/coupon_save/coupon_delete）、" +
				"营销活动 ID（activity_get/activity_save/activity_delete）、" +
				"**券码 ID**（coupon_code_save/coupon_code_delete）。" +
				"查询券码列表用 coupon_id，不要用 id 过滤。"},
			"coupon_id": {Type: "integer", Desc: "优惠券 ID：coupon_archives（列出该券适用商品）、coupon_archives_add/delete、" +
				"coupon_code_list、coupon_code_make 必填；与 id 是两回事（这些 action 里的 id 指券码）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（coupon_list/coupon_archives/coupon_code_list/activity_list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（同上）", Default: 20},
			"values": {Type: "object", Desc: "按 action 传字段（未列出的 action 只需 id/coupon_id，不用 values）：\n" +
				"· coupon_save：title 标题、money 面额(分)、money_type 金额类型(0 固定金额/1 随机金额/2 百分比)、" +
				"max_money 随机或百分比券的最高抵扣(分)、need_money 使用门槛(分,0=无门槛)、need_integral 兑换所需积分、" +
				"type 券类型(1 注册送/2 限量领取/3 下单赠送/4 促销满减无需领取/5 自由规则/6 积分兑换/7 活动奖品)、" +
				"start_time 与 end_time 生效区间(unix 秒,0=不限)、expire 领取后有效天数、status(1 启用/0 停用)、" +
				"range_type 适用范围(0 全部商品/1 仅指定商品可用/2 指定商品不可用)、description 说明。" +
				"**未传字段保持原值**（2026-10-29 起端点已是 PATCH 语义，只需传要改的字段，不再需要先 coupon_get）；" +
				"要停用/清空某个字段就**显式传 0**（status=0 停用、need_money=0 无门槛、expire=0 跟随固定起止时间），0 会真的写进库\n" +
				"· coupon_delete：只需 id，values 不用传\n" +
				"· coupon_archives_add / coupon_archives_delete：coupon_id 必填；archive_ids 商品 ID 数组（单条也可用 archive_id）\n" +
				"· coupon_code_make：coupon_id 必填、count 生成张数(>0)、user_id 可选（绑定到指定会员）\n" +
				"· coupon_code_save：id 为券码 ID(>0 改，0 新建)、user_id 归属会员、coupon_id 所属券、code 券码串(空则自动生成)、" +
				"status(1 可用/0 已用或失效)、money 覆盖该券码面额(分)、start_time/end_time 券码级有效期" +
				"——**未传字段保持原值**，只需传要改的那几个（2026-10-29 起不再是全量覆盖）；" +
				"0 会真的写入（money=0 零元码、status=0 标记失效），不会落入「没传」\n" +
				"· coupon_code_delete：ids 券码 ID 数组（必填；**没有单条删除**，单条也要包成数组）\n" +
				"· activity_save：title 活动名、description 介绍、goods_id 关联商品 ID、goods_type 商品类型标记(≤16 字符,通常留空)、" +
				"price 活动价(分)、stock 活动库存、start_time/end_time 活动时间(unix 秒)、user_limit 每人限购(0 不限)、" +
				"images 宣传图 URL 数组、can_use_coupon 是否允许叠加优惠券、extra 附加设置对象、status(1 生效/0 失效)、" +
				"skus 规格数组[{id,sku_id,goods_id,price,stock}]。**未传字段保持原值**（2026-10-29 起 PATCH 语义，" +
				"只需传要改的字段）；skus 三态：不传=不动规格、传空数组=清空全部、非空=按列表同步\n" +
				"· activity_delete：只需 id，values 不用传",
			},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce_marketing", map[string]string{
			"coupon_list":            "GET /plugin/coupon/list",
			"coupon_get":             "GET /plugin/coupon/detail",
			"coupon_save":            "POST /plugin/coupon/detail",
			"coupon_delete":          "POST /plugin/coupon/delete",
			"coupon_archives":        "GET /plugin/coupon/archives",
			"coupon_archives_add":    "POST /plugin/coupon/archives/add",
			"coupon_archives_delete": "POST /plugin/coupon/archives/delete",
			"coupon_code_list":       "GET /plugin/coupon/code/list",
			"coupon_code_save":       "POST /plugin/coupon/code/detail",
			"coupon_code_delete":     "POST /plugin/coupon/code/delete",
			"coupon_code_make":       "POST /plugin/coupon/code/make",
			"activity_list":          "GET /plugin/activity/list",
			"activity_get":           "GET /plugin/activity/detail",
			"activity_save":          "POST /plugin/activity/detail",
			"activity_delete":        "POST /plugin/activity/delete",
		}),
	},

	// ───────────────────── 运费、询盘与站内消息 ─────────────────────
	{
		Name: "commerce_fulfillment", Title: "运费与询盘客服", Domain: DomainCommerce, Risk: RiskWrite,
		Desc: "商城履约链路：运费规则配置、询盘订单（报价/关闭/图片）、站内消息与客服邮件模板。**默认关闭**，需 ExposedIntents 显式开启。" +
			"action: shipping_get/shipping_save/order_list/order_get/order_close/order_images_save/" +
			"message_list/message_send/message_delete/quote_send/setting_get/setting_save。" +
			"发送类 action 的 sender_id/sender_name/sender_type 由服务端按当前管理员身份覆盖，**不要传**。" +
			"金额字段（quote_price/first_fee/continue_fee）单位是**分**。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"shipping_get", "shipping_save",
				"order_list", "order_get", "order_close", "order_images_save",
				"message_list", "message_send", "message_delete", "quote_send",
				"setting_get", "setting_save",
			}},
			"id": {Type: "integer", Desc: "询盘订单 ID（order_get/order_close/order_images_save/quote_send）或消息 ID（message_delete）；" +
				"message_list **必须**带上所属订单号：写 values.comm_order_id（或顶层 comm_order_id），不传会被端点拒绝（Invalid Params）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（order_list/message_list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（order_list/message_list）", Default: 20},
			"values": {Type: "object", Desc: "按 action 传字段（未列出的 action 只需 id，不用 values）：\n" +
				"· shipping_save：**全量覆盖，先 shipping_get 取全量再改**。" +
				"type 计费方式(1 固定运费/2 按重量/3 按体积/4 按数量/5 按金额)、" +
				"units 计量单位列表、weight_units 重量单位列表、volumetric_divisor 体积换算除数(如 5000)、" +
				"is_free_shipping 是否全站包邮(true 时下列规则全部失效)、" +
				"delivery_place 发货地{country_code,province_code,country,…}、" +
				"free_shipping_rule 满额包邮{type 0 不启用/1 满金额/2 满重量/3 满体积/4 满件数, threshold 阈值(保留元)}、" +
				"default_rule 默认运费规则{first_unit 首单位, first_fee 首费(分), continue_unit 续单位, continue_fee 续费(分), regions 适用地区}、" +
				"region_rules 地区自定义规则数组（结构同 default_rule）\n" +
				"· order_close：只需 id，values 不用传（把询盘订单置为已关闭）\n" +
				"· order_images_save：id 必填、field 图片字段名（images/sketch_images/finished_images）、images 图片 URL 数组" +
				"（只改图片，其余字段不动）\n" +
				"· message_send：comm_order_id 必填、content 消息内容、message_type 消息类型(空则 text)；" +
				"发送后订单若是「待处理」会自动转为「处理中」\n" +
				"· message_delete：只需 id，values 不用传（不可恢复）\n" +
				"· quote_send：id 必填、quote_price 报价金额(分，**必须 >0** 否则端点直接报 Invalid Params)、quote_content 报价说明、" +
				"images 附图数组；可选 content/order_extra\n" +
				"· setting_save：**全量覆盖**。summited_template 提交成功邮件模板、" +
				"quote_template 报价邮件模板、reply_template 回复邮件模板（均含 title/content 等邮件模板字段）、" +
				"order_fields 询盘表单自定义字段数组",
			},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce_fulfillment", map[string]string{
			"shipping_get":      "GET /plugin/shipping/setting",
			"shipping_save":     "POST /plugin/shipping/setting",
			"order_list":        "GET /plugin/communication/order/list",
			"order_get":         "GET /plugin/communication/order/detail",
			"order_close":       "POST /plugin/communication/order/close",
			"order_images_save": "POST /plugin/communication/order/images/save",
			"message_list":      "GET /plugin/communication/message/list",
			"message_send":      "POST /plugin/communication/message/send",
			"message_delete":    "POST /plugin/communication/message/delete",
			"quote_send":        "POST /plugin/communication/quote/send",
			"setting_get":       "GET /plugin/communication/setting",
			"setting_save":      "POST /plugin/communication/setting",
		}),
	},

	// ───────────────────── 商品评价与增值服务 ─────────────────────
	{
		Name: "commerce_review", Title: "商品评价与增值服务", Domain: DomainCommerce, Risk: RiskWrite,
		Desc: "商城口碑与增值服务：订单商品评价的查询/审核/删除，以及结算页可选的增值服务项（安装、延保等）配置。" +
			"**默认关闭**，需 ExposedIntents 显式开启。action: review_list/review_get/review_save/review_delete/" +
			"service_list/service_get/service_save/service_delete。金额字段（fee/max_fee）单位是**分**。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"review_list", "review_get", "review_save", "review_delete",
				"service_list", "service_get", "service_save", "service_delete",
			}},
			"id":        {Type: "integer", Desc: "评价 ID（review_*）或增值服务项 ID（service_*）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（review_list/service_list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（review_list/service_list）", Default: 20},
			"values": {Type: "object", Desc: "按 action 传字段（未列出的 action 只需 id/page，不用 values）：\n" +
				"· review_list 过滤条件：order_id 订单号、archive_id 商品 ID\n" +
				"· review_save：**纯全量覆盖，没有 GET 同路径兄弟可回查**，改一个字段也要带齐：" +
				"archive_id 商品 ID、archive_sku_id 规格 ID、order_id 订单号、user_id 所属会员、" +
				"user_name 昵称、email、ip、score 评分、content 评价内容、status 审核状态、images 图片数组。" +
				"⚠️ user_id 必须与原评价一致，否则端点返回 Not Found（这是它的归属校验）；" +
				"同一订单同一商品重复提交时端点直接返回成功但不落库\n" +
				"· review_delete：只需 id，values 不用传（不可恢复）\n" +
				"· service_save：service_code 服务编码(唯一标识)、service_name 名称、service_type 类型、" +
				"description 说明、enabled 是否启用、sort 排序、fee 服务费(分)、fee_type 计费方式、" +
				"max_fee 费用上限(分)、extra 附加配置对象——**全量覆盖**，改一个字段也要带齐\n" +
				"· service_delete：只需 id，values 不用传（不可恢复）",
			},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce_review", map[string]string{
			"review_list":    "GET /plugin/order/reviews",
			"review_get":     "GET /plugin/order/review",
			"review_save":    "POST /plugin/order/review/save",
			"review_delete":  "POST /plugin/order/review/delete",
			"service_list":   "GET /plugin/order/services",
			"service_get":    "GET /plugin/order/service",
			"service_save":   "POST /plugin/order/service/save",
			"service_delete": "POST /plugin/order/service/delete",
		}),
	},

	// ───────────────────── Google Merchant 商品同步 ─────────────────────
	{
		Name: "seo_merchant", Title: "Google 商品同步", Domain: DomainSeo, Risk: RiskWrite,
		Desc: "把商品同步到 Google Merchant Center：授权配置、选商户、推送与删除、同步日志。" +
			"**默认关闭**，需 ExposedIntents 显式开启（配置含 client_secret 与 access/refresh token）。" +
			"action: config_get/config_save/auth_url/callback/account_select/sync/sync_delete/logs/statuses。" +
			"典型流程：config_get → config_save 填 client_id/client_secret → auth_url 取授权链接 → 用户授权后 callback 换 token → " +
			"account_select 选商户 → sync 推送 → logs/statuses 查结果。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"config_get", "config_save", "auth_url", "callback",
				"account_select", "sync", "sync_delete", "logs", "statuses",
			}},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（logs/statuses）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（logs/statuses）", Default: 20},
			"values": {Type: "object", Desc: "按 action 传字段（未列出的 action 不用 values）：\n" +
				"· config_save：**全量覆盖且会连带清空 token**——端点把整个配置对象原样落库，" +
				"只传 open/client_id 会把 token/accounts/merchant_id 一起清掉。必须先 config_get 取全量、改完整体写回。" +
				"字段：open 是否启用、client_id、client_secret、places_api_key、" +
				"token(object,含 access_token/refresh_token/token_type/expiry)、" +
				"accounts(array,已授权商户列表)、merchant_id、aggregator_id、" +
				"email/name/given_name/family_name、redirect_url（OAuth 回调地址，需与 Google 后台一致）\n" +
				"· auth_url：state 可选（不传则服务端随机生成并回显）。返回 data.state 与 data.url，" +
				"把 url 给用户去授权，并要求回调时带回同一个 state\n" +
				"· callback：code 必填（Google 回调的授权码）、state 原样带回；成功后写入 token\n" +
				"· account_select：merchant_id 必填（从 config_get 的 accounts 里选一个作为默认商户）\n" +
				"· sync：full 布尔，true=全量重推、false=增量。⚠️ 该端点**忽略 ids**（服务端固定走全量/增量），" +
				"且是**异步**的——立刻返回成功，真实结果要过一会儿查 logs/statuses\n" +
				"· sync_delete：ids 要下架的商品 ID 数组（必填，不可恢复；只删远端 Feed 记录，不动本站商品）\n" +
				"· logs / statuses：分页查同步日志与逐商品同步状态，判定是否成功看这里而不是 sync 的回执",
			},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("seo_merchant", map[string]string{
			"config_get":     "GET /plugin/googlemerchant/config",
			"config_save":    "POST /plugin/googlemerchant/config",
			"auth_url":       "GET /plugin/googlemerchant/auth/url",
			"callback":       "POST /plugin/googlemerchant/callback",
			"account_select": "POST /plugin/googlemerchant/account/select",
			"sync":           "POST /plugin/googlemerchant/sync",
			"sync_delete":    "POST /plugin/googlemerchant/sync/delete",
			"logs":           "GET /plugin/googlemerchant/sync/logs",
			"statuses":       "GET /plugin/googlemerchant/sync/statuses",
		}),
	},
}
