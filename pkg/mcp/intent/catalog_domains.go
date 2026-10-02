package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// catalog_domains.go 补齐「有端点、无专属 cap」的域的意图（方案第 3 步）。
//
// 背景：F 阶段实测 394 个后台端点里，plugin 命名空间独占 223 个却几乎没有专属 cap，
// 靠手写 cap 去追平这套平行实现既不现实也没必要——G1–G3 已经铺好了
// 「元数据发现（api_list/api_schema）+ 通用执行（api_invoke）」的通路。
//
// 因此这里的意图不再委托专属 cap，而是通过 invokeRoutes 直接路由到后台端点，
// 由 api_invoke 统一执行。好处：
//  1. 一次覆盖全部端点，不需要为每个域写一批 cap；
//  2. 天然继承 G4 的两道闸门（硬规则 + ApiExposure 白名单），
//     新增意图不会绕过任何既有安全边界；
//  3. 身份仍由 McpConfig.InvokeAdminId 显式指定，模型无法自选身份。
//
// 全部标记 DefaultOff：默认不进入任何工具清单，需由 ExposedIntents 显式开启。
// 推荐起步清单见 RecommendedExposed()。

// invokeRoutes 构造"按 action 路由到后台端点"的组合工具。
//
// 与 switchCompose 的区别：switchCompose 路由到底层 cap（需要有专属 cap），
// 本函数路由到 REST 端点，经 api_invoke 通用执行器调用。
//
// routes 的值格式为 "METHOD PATH"，例如 "GET /plugin/push"。
// 自动补全 /system/api
//
// intentName 仅用于登记声明来源（见 coverage.go 的 DeclaredEndpointTargets）：
// 覆盖审计要回答"哪些端点被 AI 直连、经由哪个意图"，光拿到闭包里的 routes
// 无法反查归属。名字写错由 TestDeclaredEndpointTargetsRegistered 穷举校验拦下。
func invokeRoutes(intentName string, routes map[string]string) Compose {
	registerIntentRoutes(intentName, routes)
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		target, ok := routes[action]
		if !ok {
			return nil, fmt.Errorf("不支持的操作: %s（可选: %v）", action, keysOf(routes))
		}
		parts := strings.Fields(target)
		if len(parts) != 2 {
			return nil, fmt.Errorf("意图路由声明格式错误（应为 \"METHOD PATH\"）: %q", target)
		}
		// 自动补全 /system/api（与覆盖审计共用同一套规范化规则，避免两处口径打架）
		parts[1] = normalizeEndpointPath(parts[1])
		// 除 action 外的全部字段作为端点参数透传。
		// InputSchema 未设 additionalProperties:false，因此模型可传未声明字段；
		// 具体字段定义请用 api_schema 查询，避免凭描述猜测。
		sub := make(map[string]any, len(args))
		for k, v := range args {
			if k == "action" {
				continue
			}
			// 如果存在 values 对象，将其字段扁平化到 sub 中（而非嵌套传递）
			if k == "values" {
				if valuesMap, ok := v.(map[string]any); ok {
					for vk, vv := range valuesMap {
						sub[vk] = vv
					}
				}
				continue
			}
			sub[k] = v
		}
		out, err := cap(ctx, "api_invoke", map[string]any{
			"method": parts[0],
			"path":   parts[1],
			"params": sub,
		})
		if err != nil {
			return nil, err
		}
		// 解析 JSON 响应，提取 data 字段填充 Result.Data
		res := &Result{Text: out}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err == nil {
			if data, ok := envelope["data"]; ok && data != nil {
				res.Data = data
			}
		}
		return res, nil
	}
}

// domainIntentCatalog 是补齐域的意图清单，由 init 合并进总的 IntentCatalog。
// action 的归一化 list/detail/save/delete/setting_get/setting_save ...
var domainIntentCatalog = []*IntentSpec{
	// ───────────────────────── 内容域 ─────────────────────────
	{
		Name: "content_place", Title: "管理城市站", Domain: DomainContent, Risk: RiskWrite,
		Desc:       "城市站（分站）的增删查与配置。action: list/detail/save/delete/setting_get/setting_save。",
		// 与其它补齐域意图一致：含写操作且能改站点级配置，默认关闭，需显式开白。
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "detail", "save", "delete", "setting_get", "setting_save"}},
			"id":     {Type: "integer", Desc: "城市站 ID"},
			"values": {Type: "object", Desc: "城市站内容（save 写操作）/配置内容（setting 写操作）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("content_place", map[string]string{
			"list":         "GET /plugin/place/list",
			"detail":       "GET /plugin/place/detail",
			"save":         "POST /plugin/place/detail",
			"delete":       "POST /plugin/place/delete",
			"setting_get":  "GET /plugin/place/setting",
			"setting_save": "POST /plugin/place/setting",
		}),
	},

	// ───────────────────────── SEO 域 ─────────────────────────
	// 合并 seo_sitemap(原 catalog.go) + seo_robots + seo_push 为单一 seo 工具。
	// sitemap 走 cap(sitemap_rebuild+url_push)，robots/push 走端点；均经 api_invoke 执行。
	{
		Name: "seo", Title: "robots/sitemap/推送", Domain: DomainSeo, Risk: RiskWrite,
		Desc:       "SEO 收录相关操作。action: robots_get/robots_save(robots.txt 读写)/sitemap(重建 sitemap 并推送)/push_get/push_save/push_push/push_logs(链接推送配置与记录)。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"robots_get", "robots_save", "sitemap", "push_get", "push_save", "push_push", "push_logs"}},
			"content": {Type: "string", Desc: "robots.txt 内容（robots_save 用）"},
			"values":  {Type: "object", Desc: "链接推送配置（push_save 时传入）"},
			"urls":    {Type: "array", Items: "string", Desc: "要推送的 URL 列表（push_push 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose:  seoCompose(),
	},
	{
		Name: "seo_jsonld", Title: "结构化数据配置", Domain: DomainSeo, Risk: RiskWrite,
		Desc:       "JSON-LD 结构化数据的读写。action: get/save。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"get", "save"}},
			"values": {Type: "object", Desc: "结构化数据配置（save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("seo_jsonld", map[string]string{
			"get":  "GET /plugin/jsonld/config",
			"save": "POST /plugin/jsonld/config",
		}),
	},
	{
		Name: "seo_llms", Title: "管理 llms.txt", Domain: DomainSeo, Risk: RiskWrite,
		Desc:       "面向生成式引擎的 llms.txt：读取配置、构建、查看状态。action: get/save/build/status。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"get", "save", "build", "status"}},
			"values": {Type: "object", Desc: "配置内容（save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("seo_llms", map[string]string{
			"get":    "GET /plugin/llms/setting",
			"save":   "POST /plugin/llms/setting",
			"build":  "POST /plugin/llms/build",
			"status": "GET /plugin/llms/status",
		}),
	},

	// ───────────────────────── 互动域 ─────────────────────────
	// 合并 interaction_comment(原 catalog.go) + interaction_guestbook 为单一 interaction 工具。
	// comment_* 走 cap（comment_list/approve/delete/guestbook_list 均有等价端点 → api_invoke），
	// guestbook_* 详情/设置/状态/删除/导出走端点。
	{
		Name: "interaction", Title: "评论与留言", Domain: DomainInteraction, Risk: RiskWrite,
		Desc:       "评论/留言的审核与清理。action: comment_list/comment_approve/comment_delete/guestbook_list/guestbook_detail/guestbook_setting_get/guestbook_setting_save/guestbook_status/guestbook_delete/guestbook_export。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"comment_list", "comment_approve", "comment_delete", "guestbook_list", "guestbook_detail", "guestbook_setting_get", "guestbook_setting_save", "guestbook_status", "guestbook_delete", "guestbook_export"}},
			"id":     {Type: "integer", Desc: "评论/留言 ID"},
			"page":   {Type: "integer", Desc: "页码", Default: 1},
			"status": {Type: "integer", Desc: "状态过滤/设置（comment_approve/guestbook_status）"},
			"values": {Type: "object", Desc: "留言设置（guestbook_setting_save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose:  interactionCompose(),
	},

	// ───────────────────────── 内容生产域 ─────────────────────────
	{
		Name: "contentops_material", Title: "管理素材库", Domain: DomainContentOps, Risk: RiskWrite,
		Desc:       "素材（待发布内容池）与其分类的管理、导入。action: list/detail/save/delete/import/category_list/category_save/category_delete。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "detail", "save", "delete", "import", "category_list", "category_save", "category_delete"}},
			"id":     {Type: "integer", Desc: "素材 ID"},
			"values": {Type: "object", Desc: "素材字段（save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("contentops_material", map[string]string{
			"list":            "GET /plugin/material/list",
			"detail":          "GET /plugin/material/detail",
			"save":            "POST /plugin/material/detail",
			"import":          "POST /plugin/material/import",
			"delete":          "POST /plugin/material/delete",
			"category_list":   "GET /plugin/material/category/list",
			"category_save":   "POST /plugin/material/category/detail",
			"category_delete": "POST /plugin/material/category/delete",
		}),
	},
	{
		Name: "contentops_collector", Title: "采集配置与执行", Domain: DomainContentOps, Risk: RiskWrite,
		Desc:       "文章采集：配置读写、启动采集、关键词挖掘、内容替换。action: get/save/start/collect/combination/replace/dig。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"get", "save", "start", "collect", "combination", "replace", "dig"}},
			"values": {Type: "object", Desc: "采集配置（save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("contentops_collector", map[string]string{
			"get":         "GET /collector/setting",
			"save":        "POST /collector/setting",
			"start":       "POST /collector/article/start",
			"collect":     "POST /collector/article/collect",
			"combination": "POST /collector/article/combination/get",
			"replace":     "POST /collector/article/replace",
			"dig":         "POST /collector/keyword/dig",
		}),
	},
	{
		Name: "contentops_import", Title: "批量导入/时间因子配置", Domain: DomainContentOps, Risk: RiskWrite,
		Desc:       "文章批量导入（含 Excel 模板），远程接口导入Token配置，时间因子（更新文章时间）配置。action: import/status/template/setting_get/setting_save/timefactor_get/timefactor_save。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"import", "status", "template", "setting_get", "setting_save", "timefactor_get", "timefactor_save"}},
			"values":    {Type: "object", Desc: "导入参数/配置参数"},
			"file":      {Type: "string", Desc: "上传文件：data URI 或裸 base64"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("contentops_import", map[string]string{
			"import":          "POST /archive/import",
			"status":          "GET /archive/import/status",
			"template":        "POST /archive/import/exceltemplate",
			"setting_get":     "GET /plugin/import/api",
			"setting_save":    "POST /plugin/import/token",
			"timefactor_get":  "GET /plugin/timefactor/setting",
			"timefactor_save": "POST /plugin/timefactor/setting",
		}),
	},
	{
		Name: "contentops_transfer", Title: "数据迁移", Domain: DomainContentOps, Risk: RiskSystem,
		Desc:       "跨站点数据迁移。**破坏性操作，默认关闭**，仅在人工确认后开启。action: task/modules/create/download/start。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"task", "modules", "create", "download", "start"}},
			"values": {Type: "object", Desc: "迁移参数"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("contentops_transfer", map[string]string{
			"task":     "GET /plugin/transfer/task",
			"modules":  "POST /plugin/transfer/modules",
			"create":   "POST /plugin/transfer/create",
			"download": "POST /plugin/transfer/download",
			"start":    "POST /plugin/transfer/start",
		}),
	},
	{
		Name: "contentops_translate", Title: "多语言翻译", Domain: DomainContentOps, Risk: RiskWrite,
		Desc:       "翻译配置与翻译记录管理。action: get/save/logs/texts/text_save/text_delete。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"get", "save", "logs", "texts", "text_save", "text_delete"}},
			"id":     {Type: "integer", Desc: "记录 ID"},
			"values": {Type: "object", Desc: "配置或译文内容"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("contentops_translate", map[string]string{
			"get":         "GET /plugin/translate/config",
			"save":        "POST /plugin/translate/config",
			"logs":        "GET /plugin/translate/logs",
			"texts":       "GET /plugin/translate/log/texts",
			"text_save":   "POST /plugin/translate/log/text/save",
			"text_delete": "POST /plugin/translate/log/text/remove",
		}),
	},
	{
		Name: "contentops_imagedeco", Title: "配图与水印", Domain: DomainContentOps, Risk: RiskWrite,
		Desc:       "自动生成标题图、图片水印。action: titleimage_get/titleimage_save/titleimage_generate/titleimage_preview/watermark_get/watermark_save/watermark_generate/watermark_preview。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"titleimage_get", "titleimage_save", "titleimage_generate", "titleimage_preview",
				"watermark_get", "watermark_save", "watermark_generate", "watermark_preview"}},
			"values":    {Type: "object", Desc: "配置内容（*_save 时传入）"},
			"file":      {Type: "string", Desc: "字体/水印/底图文件：data URI 或裸 base64"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("contentops_imagedeco", map[string]string{
			"titleimage_get":      "GET /plugin/titleimage/config",
			"titleimage_save":     "POST /plugin/titleimage/config",
			"titleimage_upload":   "POST /plugin/titleimage/upload",
			"titleimage_generate": "POST /plugin/titleimage/generate",
			"titleimage_preview":  "GET /plugin/titleimage/preview",
			"watermark_get":       "GET /plugin/watermark/config",
			"watermark_save":      "POST /plugin/watermark/config",
			"watermark_upload":    "POST /plugin/watermark/upload",
			"watermark_generate":  "POST /plugin/watermark/generate",
			"watermark_preview":   "GET /plugin/watermark/preview",
		}),
	},

	// ───────────────────────── 交易域 ─────────────────────────
	// 合并 commerce_user(原 catalog.go) + commerce_group + commerce_retailer 为单一 commerce 工具。
	{
		Name: "commerce", Title: "会员/分组/分销商", Domain: DomainCommerce, Risk: RiskWrite,
		Desc:       "会员的增删改查与订单列表，会员分组的增删查，分销商配置与审核。action: user_list/user_get/user_create/user_update/user_delete/user_orders/group_list/group_detail/group_save/group_delete/retailer_list/retailer_detail/retailer_setting_get/retailer_setting_save/retailer_apply/retailer_realname。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"user_list", "user_get", "user_create", "user_update", "user_delete", "user_orders", "group_list", "group_detail", "group_save", "group_delete", "retailer_list", "retailer_detail", "retailer_setting_get", "retailer_setting_save", "retailer_apply", "retailer_realname"}},
			"id":        {Type: "integer", Desc: "会员/分组/分销商 ID"},
			"user_name": {Type: "string", Desc: "用户名"},
			"password":  {Type: "string", Desc: "密码"},
			"email":     {Type: "string", Desc: "邮箱"},
			"phone":     {Type: "string", Desc: "手机号"},
			"status":    {Type: "integer", Desc: "状态 1/0"},
			"page":      {Type: "integer", Desc: "页码", Default: 1},
			"values":    {Type: "object", Desc: "分组字段/分销商配置（*_save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce", map[string]string{
			"user_list":           "GET /plugin/user/list",
			"user_get":            "GET /plugin/user/detail",
			"user_create":         "POST /plugin/user/detail",
			"user_update":         "POST /plugin/user/detail",
			"user_delete":         "POST /plugin/user/delete",
			"user_orders":         "GET /plugin/order/list",
			"group_list":          "GET /plugin/user/group/list",
			"group_detail":        "GET /plugin/user/group/detail",
			"group_save":          "POST /plugin/user/group/detail",
			"group_delete":        "POST /plugin/user/group/delete",
			"retailer_list":       "GET /plugin/retailer/list",
			"retailer_detail":     "GET /plugin/user/detail",
			"retailer_setting_get":  "GET /plugin/retailer/config",
			"retailer_setting_save": "POST /plugin/retailer/config",
			"retailer_apply":        "POST /plugin/retailer/apply",
			"retailer_realname":     "POST /plugin/retailer/realname",
		}),
	},
	{
		Name: "commerce_order", Title: "管理订单", Domain: DomainCommerce, Risk: RiskWrite,
		Desc:       "订单的查询与状态流转（发货/取消/完成/退款/支付）。action: list/detail/setting_get/setting_save/deliver/canceled/finished/refund/refund_apply/pay/export。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "detail", "setting_get", "setting_save", "deliver", "canceled", "finished",
				"refund", "refund_apply", "pay", "export"}},
			"id":     {Type: "integer", Desc: "订单 ID"},
			"values": {Type: "object", Desc: "订单参数/配置参数"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce_order", map[string]string{
			"list":         "GET /plugin/order/list",
			"detail":       "GET /plugin/order/detail",
			"setting_get":  "GET /plugin/order/config",
			"setting_save": "POST /plugin/order/config",
			"deliver":      "POST /plugin/order/deliver",
			"canceled":     "POST /plugin/order/canceled",
			"finished":     "POST /plugin/order/finished",
			"refund":       "POST /plugin/order/refund",
			"refund_apply": "POST /plugin/order/refund/apply",
			"pay":          "POST /plugin/order/pay",
			"export":       "POST /plugin/order/export",
		}),
	},
	{
		// Risk 取 write 而非 read：本意图除查询外还路由 save/delete/upload 三个写端点。
		// 声明成 read 会让 MCP 的工具标注与"只读意图"的批量放行口径一起失真
		// （由 TestActionRisk_AuditAllIntents 拦下）。
		Name: "commerce_pay", Title: "支付账户与记录", Domain: DomainCommerce, Risk: RiskWrite,
		Desc:       "收款账户与支付记录。action: list/detail/statistic(查询)/save/delete/upload(写)。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "detail", "save", "delete", "upload", "statistic"}},
			"id":        {Type: "integer", Desc: "支付记录 ID"},
			"values":    {Type: "object", Desc: "账户参数"},
			"file":      {Type: "string", Desc: "证书文件：data URI 或裸 base64"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce_pay", map[string]string{
			"list":      "GET /plugin/pay/accounts",
			"detail":    "GET /plugin/pay/detail",
			"save":      "POST /plugin/pay/detail",
			"delete":    "POST /plugin/pay/delete",
			"upload":    "POST /plugin/pay/upload",
			"statistic": "GET /plugin/pay/statistic",
		}),
	},
	{
		Name: "commerce_finance", Title: "财务与佣金提现", Domain: DomainCommerce, Risk: RiskWrite,
		Desc:       "财务流水、佣金、提现申请与审批。action: finance_list/finance_detail/commission_list/commission_detail/withdraw_list/withdraw_detail/withdraw_apply/withdraw_approval/withdraw_finished。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"finance_list", "finance_detail", "commission_list", "commission_detail",
				"withdraw_list", "withdraw_detail", "withdraw_apply", "withdraw_approval", "withdraw_finished"}},
			"id":     {Type: "integer", Desc: "记录 ID"},
			"values": {Type: "object", Desc: "申请/审批参数"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce_finance", map[string]string{
			"finance_list":      "GET /plugin/finance/list",
			"finance_detail":    "GET /plugin/finance/detail",
			"commission_list":   "GET /plugin/commission/list",
			"commission_detail": "GET /plugin/commission/detail",
			"withdraw_list":     "GET /plugin/withdraw/list",
			"withdraw_detail":   "GET /plugin/withdraw/detail",
			"withdraw_apply":    "POST /plugin/withdraw/apply",
			"withdraw_approval": "POST /plugin/withdraw/approval",
			"withdraw_finished": "POST /plugin/withdraw/finished",
		}),
	},
	// ───────────────────────── 渠道域 ─────────────────────────
	{
		Name: "channel_wechat", Title: "公众号运营", Domain: DomainChannel, Risk: RiskWrite,
		Desc:       "公众号菜单、消息与自动回复规则。action: menu_list/menu_save/menu_delete/menu_sync/message_list/message_reply/message_delete/rule_list/rule_save/rule_delete。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"menu_list", "menu_save", "menu_delete", "menu_sync",
				"message_list", "message_reply", "message_delete",
				"rule_list", "rule_save", "rule_delete"}},
			"id":     {Type: "integer", Desc: "记录 ID"},
			"values": {Type: "object", Desc: "配置或内容参数"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("channel_wechat", map[string]string{
			"menu_list":      "GET /plugin/wechat/menu/list",
			"menu_save":      "POST /plugin/wechat/menu/save",
			"menu_delete":    "POST /plugin/wechat/menu/delete",
			"menu_sync":      "POST /plugin/wechat/menu/sync",
			"message_list":   "GET /plugin/wechat/message/list",
			"message_reply":  "POST /plugin/wechat/message/reply",
			"message_delete": "POST /plugin/wechat/message/delete",
			"rule_list":      "GET /plugin/wechat/reply/rule/list",
			"rule_save":      "POST /plugin/wechat/reply/rule/save",
			"rule_delete":    "POST /plugin/wechat/reply/rule/delete",
		}),
	},
	{
		Name: "channel_thirdparty", Title: "第三方授权账号配置", Domain: DomainChannel, Risk: RiskWrite,
		Desc:       "公众号、小程序、Google 授权配置的读写。action: wechat_get/wechat_save/weapp_get/weapp_save/google_get/google_save。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"wechat_get", "wechat_save", "weapp_get", "weapp_save", "google_get", "google_save"}},
			"id":     {Type: "integer", Desc: "订阅用户 ID"},
			"values": {Type: "object", Desc: "订阅用户字段或群发内容"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("channel_thirdparty", map[string]string{
			"wechat_get":  "GET /plugin/wechat/config",
			"wechat_save": "POST /plugin/wechat/config",
			"weapp_get":   "GET /plugin/weapp/config",
			"weapp_save":  "POST /plugin/weapp/config",
			"google_get":  "GET /plugin/google/setting",
			"google_save": "POST /plugin/google/setting",
		}),
	},

	{
		Name: "channel_sendmail", Title: "邮件发送", Domain: DomainChannel, Risk: RiskWrite,
		Desc:       "邮件模板管理、服务配置与测试发送。action: list/detail/save/preview/setting_get/setting_save/logs/test。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "detail", "save", "preview", "setting_get", "setting_save", "logs", "test"}},
			"key":    {Type: "string", Desc: "模板 key"},
			"values": {Type: "object", Desc: "配置或模板内容"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("channel_sendmail", map[string]string{
			"list":         "GET /plugin/sendmail/templates",
			"detail":       "GET /plugin/sendmail/template",
			"save":         "POST /plugin/sendmail/template",
			"preview":      "POST /plugin/sendmail/template/preview",
			"setting_get":  "GET /plugin/sendmail/setting",
			"setting_save": "POST /plugin/sendmail/setting",
			"logs":         "GET /plugin/sendmail/list",
			"test":         "POST /plugin/sendmail/test",
		}),
	},
	{
		Name: "channel_subscriber", Title: "订阅用户管理", Domain: DomainChannel, Risk: RiskWrite,
		Desc:       "订阅用户与其分组的管理、群发。action: list/detail/save/delete/category_list/category_save/category_delete/send。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "save", "delete", "category_list", "category_save", "category_delete", "send"}},
			"id":     {Type: "integer", Desc: "订阅用户 ID"},
			"values": {Type: "object", Desc: "订阅用户字段或群发内容"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("channel_subscriber", map[string]string{
			"list":            "GET /plugin/subscriber/list",
			"detail":          "GET /plugin/subscriber/detail",
			"save":            "POST /plugin/subscriber/save",
			"delete":          "POST /plugin/subscriber/delete",
			"category_list":   "GET /plugin/subscriber/category/list",
			"category_save":   "POST /plugin/subscriber/category/save",
			"category_delete": "POST /plugin/subscriber/category/delete",
			"send":            "POST /plugin/subscriber/send",
		}),
	},

	// ───────────────────────── 系统域 ─────────────────────────
	{
		Name: "system_security", Title: "安全与风控配置", Domain: DomainSystem, Risk: RiskWrite,
		Desc:       "访问限制、防采集干扰、Akismet/reCAPTCHA 校验等安全配置。action: limiter_get/limiter_save/blockedips/blockedip_delete/interference_get/interference_save/akismet_get/akismet_save。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"limiter_get", "limiter_save", "blockedips", "blockedip_delete",
				"interference_get", "interference_save",
				"akismet_get", "akismet_save"}},
			"values": {Type: "object", Desc: "配置内容（*_save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("system_security", map[string]string{
			"limiter_get":       "GET /plugin/limiter/setting",
			"limiter_save":      "POST /plugin/limiter/setting",
			"blockedips":        "GET /plugin/limiter/blockedips",
			"blockedip_delete":  "POST /plugin/limiter/blockedip/remove",
			"interference_get":  "GET /plugin/interference/config",
			"interference_save": "POST /plugin/interference/config",
			"akismet_get":       "GET /plugin/akismet/setting",
			"akismet_save":      "POST /plugin/akismet/setting",
		}),
	},
	{
		Name: "system_multilang", Title: "多语言站点", Domain: DomainSystem, Risk: RiskWrite,
		Desc:       "多语言站点配置、子站同步与静态页缓存。action: setting_get/setting_save/list/validsites/save/delete/sync/status/caches/logs/cache_delete。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"setting_get", "setting_save", "list", "validsites", "save", "delete",
				"sync", "status", "caches", "logs", "cache_delete"}},
			"values": {Type: "object", Desc: "配置内容（save/setting_save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("system_multilang", map[string]string{
			"setting_get":  "GET /plugin/multilang/config",
			"setting_save": "POST /plugin/multilang/config",
			"list":         "GET /plugin/multilang/sites",
			"validsites":   "GET /plugin/multilang/validsites",
			"save":         "POST /plugin/multilang/site/save",
			"delete":       "POST /plugin/multilang/site/remove",
			"sync":         "POST /plugin/multilang/site/sync",
			"status":       "GET /plugin/multilang/site/sync/status", // 真实路由带 sync 段，勿简写
			"caches":       "GET /plugin/multilang/site/html/caches",
			"cache_delete": "POST /plugin/multilang/site/html/cache/remove",
			"logs":         "GET /plugin/multilang/site/html/logs",
		}),
	},
	{
		Name: "system_rewrite", Title: "伪静态规则", Domain: DomainSystem, Risk: RiskWrite,
		Desc:       "伪静态（rewrite）规则的读写。action: get/save。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"get", "save"}},
			"values": {Type: "object", Desc: "规则内容（save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("system_rewrite", map[string]string{
			"get":  "GET /plugin/rewrite",
			"save": "POST /plugin/rewrite",
		}),
	},

	// ───────────────────────── 站点运维域（默认全关） ─────────────────────────
	{
		Name: "siteops_backup", Title: "备份与恢复", Domain: DomainSiteOps, Risk: RiskSystem,
		Desc:       "数据库备份、导入、删除与清理。**破坏性操作，默认关闭**。action: list/dump/status/import/restore/delete/remark/export/cleanup。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "dump", "status", "import", "restore", "delete", "remark", "export", "cleanup"}},
			"values":    {Type: "object", Desc: "备份参数"},
			"file":      {Type: "string", Desc: "导入的备份文件：data URI 或裸 base64"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("siteops_backup", map[string]string{
			"list":    "GET /plugin/backup/list",
			"dump":    "POST /plugin/backup/dump",
			"status":  "GET /plugin/backup/status",
			"import":  "POST /plugin/backup/import",
			"restore": "POST /plugin/backup/restore",
			"delete":  "POST /plugin/backup/delete",
			"remark":  "POST /plugin/backup/remark",
			"export":  "GET /plugin/backup/export",
			"cleanup": "POST /plugin/backup/cleanup",
		}),
	},
	// 合并 siteops_cache + siteops_fulltext + siteops_replace 为单一 siteops_maintain 工具。
	{
		Name: "siteops_maintain", Title: "缓存/全文/替换", Domain: DomainSiteOps, Risk: RiskWrite,
		Desc:       "站点运维维护类操作。action: cache_get/cache_save/cache_build/cache_build_status/cache_build_index/cache_build_archive/cache_build_category/cache_build_tag/cache_clean/cache_push/cache_push_status/cache_push_logs/cache_upload/fulltext_get/fulltext_save/fulltext_status/fulltext_rebuild/replace。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"cache_get", "cache_save", "cache_build", "cache_build_status", "cache_build_index", "cache_build_archive", "cache_build_category", "cache_build_tag",
				"cache_clean", "cache_push", "cache_push_status", "cache_push_logs", "cache_upload",
				"fulltext_get", "fulltext_save", "fulltext_status", "fulltext_rebuild", "replace"}},
			"values":    {Type: "object", Desc: "缓存配置/索引配置/替换规则（*_save/replace 时传入）"},
			"file":      {Type: "string", Desc: "上传证书文件：data URI 或裸 base64（cache_upload）"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("siteops_maintain", map[string]string{
			"cache_get":            "GET /plugin/htmlcache/config",
			"cache_save":           "POST /plugin/htmlcache/config",
			"cache_build":          "POST /plugin/htmlcache/build",
			"cache_build_status":   "GET /plugin/htmlcache/build/status",
			"cache_build_index":    "POST /plugin/htmlcache/build/index",
			"cache_build_archive":  "POST /plugin/htmlcache/build/archive",
			"cache_build_category": "POST /plugin/htmlcache/build/category",
			"cache_build_tag":      "POST /plugin/htmlcache/build/tag",
			"cache_clean":          "POST /plugin/htmlcache/clean",
			"cache_push":           "POST /plugin/htmlcache/push",
			"cache_push_status":    "GET /plugin/htmlcache/push/status",
			"cache_push_logs":      "GET /plugin/htmlcache/push/logs",
			"cache_upload":         "POST /plugin/htmlcache/upload",
			"fulltext_get":         "GET /plugin/fulltext/config",
			"fulltext_save":        "POST /plugin/fulltext/config",
			"fulltext_status":      "GET /plugin/fulltext/status",
			"fulltext_rebuild":     "POST /plugin/fulltext/rebuild",
			"replace":              "POST /plugin/replace/values",
		}),
	},
	{
		Name: "siteops_upgrade", Title: "版本检查与升级", Domain: DomainSiteOps, Risk: RiskSystem,
		Desc:       "检查新版本并执行升级。**破坏性操作，默认关闭**。action: check/info/upgrade。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"check", "info", "upgrade"}},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("siteops_upgrade", map[string]string{
			"check":   "GET /version/check",
			"info":    "GET /version/info",
			"upgrade": "POST /version/upgrade",
		}),
	},
	{
		Name: "siteops_website", Title: "多站点管理", Domain: DomainSiteOps, Risk: RiskSystem,
		Desc:       "多站点的增删查。**默认关闭**。action: list/detail/save/delete。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "detail", "save", "delete"}},
			"id":     {Type: "integer", Desc: "站点 ID"},
			"values": {Type: "object", Desc: "站点配置（save 时传入）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("siteops_website", map[string]string{
			"list":   "GET /website/list",
			"detail": "GET /website/info",
			"save":   "POST /website/save",
			"delete": "POST /website/delete",
		}),
	},

	// ───────────────────────── 账号域（仅只读） ─────────────────────────
	// 注意：管理员的增改删**不提供意图**。G4 硬规则只允许 admin 的只读端点，
	// 且该模块存在已知的横向越权面（见 doc/安全审计综合报告.md）。
	// 合并 account_admin_read + account_logs_read 为单一 account 工具（仅只读）。
	{
		Name: "account", Title: "查看管理员与日志", Domain: DomainAccount, Risk: RiskRead,
		Desc:       "只读：管理员列表与详情、分组列表与详情、后台菜单结构，以及管理员操作日志与登录日志。action: admin_list/admin_detail/admin_group_list/admin_group_detail/admin_menus/logs_action/logs_login。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"admin_list", "admin_detail", "admin_group_list", "admin_group_detail", "admin_menus", "logs_action", "logs_login"}},
			"id":     {Type: "integer", Desc: "管理员或分组 ID"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("account", map[string]string{
			"admin_list":         "GET /admin/list",
			"admin_detail":       "GET /admin/detail",
			"admin_group_list":   "GET /admin/group/list",
			"admin_group_detail": "GET /admin/group/detail",
			"admin_menus":        "GET /admin/menus",
			"logs_action":        "GET /admin/logs/action",
			"logs_login":         "GET /admin/logs/login",
		}),
	},

	// ───────────────────────── 设计域（默认只读） ─────────────────────────
	// 模板文件的写入不提供意图：已有 fs_write 通道，且在线改模板不可逆。
	{
		Name: "design_manage", Title: "查看模板", Domain: DomainDesign, Risk: RiskRead,
		Desc:       "只读：模板列表、详情与文件信息（修改模板请用fs_write 通道）。action: list/detail/file_templates/file_info/helpers。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "detail", "file_templates", "file_info", "helpers"}},
			"id":     {Type: "integer", Desc: "模板 ID"},
			"values": {Type: "object", Desc: "查询参数"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("design_manage", map[string]string{
			"list":           "GET /design/list",
			"detail":         "GET /design/info",
			"file_templates": "GET /design/file/templates",
			"file_info":      "GET /design/file/info",
			"helpers":        "GET /design/helpers",
		}),
	},
}

// RecommendedExposed 返回推荐的起步暴露清单（约 24 个）。
//
// 三道闸门中 ExposedIntents 是第二道，决定「哪些意图出现在模型面」。
// 全部新补齐的意图都标记了 DefaultOff，若不配置清单则一个都不开放；
// 这里给出一份保守且够用的起步组合：只读为主 + 常见写操作，
// 不含备份/升级/迁移/多站点/管理员写操作。
func RecommendedExposed() []string {
	return []string{
		// 内容（核心写作链路）
		"content_article", "content_manage",
		// 素材
		"media",
		// 结构
		"structure",
		// SEO 与数据
		"seo_keyword", "seo_anchor", "seo", "traffic_statistics",
		// 互动
		"interaction",
		// 内容生产（读多写少）
		"contentops_material", "contentops_translate",
		// 交易（只读向）
		"commerce", "commerce_order",
		// 站点配置与运维（保守：仅缓存/全文/替换，不含备份/升级/迁移）
		"system_config", "siteops_maintain",
		// 账户只读
		"account",
		// Agent 调度
		"agent",
		// 联网获取与搜索（内置低危只读工具，原 web_fetch/web_search 默认暴露）
		"web",
		// 通用调用入口（合并为单一 api 工具，含 invoke，故不列入保守默认；按需显式开启）
	}
}

func init() {
	IntentCatalog = append(IntentCatalog, domainIntentCatalog...)
}
