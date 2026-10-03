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
// 默认可见性由「日常运营是否用得上」决定，而不是一刀切关闭：
// 站点内容/结构/SEO/互动/素材/会员订单这类日常操作默认出现在工具面上，
// 站点没配白名单时模型也能看到它们；
// 高危面（不可逆运维、主机级迁移、凭证与资金、对外发信、全站性配置）保持 DefaultOff，
// 必须由 ExposedIntents 显式点名。
//
// 可见 ≠ 可调：这些意图最终都经 api_invoke 打端点，还要求 G4 的 ApiExposure.mode
// 处于 read/read_write/all（零值仍是全部拒绝）。两道闸门各管一件事——
// 默认可见回答"这个站点有什么能力"，端点策略决定"这次部署允许它真的动哪些数据"。
//
// 写/删动作不因"默认开放"而失去保护：审批门（provider.PreviewCallApproval）按
// action 粒度判定风险，写操作仍会弹确认；端点层另有硬规则 + ApiExposure 两道闸门。
//
// 新增补齐域意图必须在两处之一留痕：默认开放（不写 DefaultOff），或登记进
// gatedDomainIntents 并写明理由。TestDomainIntentExposureIsDeliberate 会双向校验，
// 防止"忘记标记 DefaultOff 就顺手把高危能力放了出去"这类漂移。
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
// renames 是"意图参数名 → 端点参数名"的换算（叠加在通用分页换算之上）。
// cap 路由有 capEndpoints.Rename 做这件事，裸路由以前没有：参数名对不上时
// 端点 ReadJSON/URLParam 只会静默忽略，模型看到 code=0 却什么都没发生。
// 例如 commerce_order 的 id 要换算成端点的 order_id，seo 的 content 要换算成 robots。
// invokeRouteOverwriteFields 声明「同一端点的 POST 是全量覆盖」及其需保护字段。
//
// key 是 GET/POST 共享的端点路径（已规范化），value 是不能被清零的字段。
//
// ⚠️ **本表只收「逐字段无条件赋值」型端点，不收有 UpdateAll 开关的。**
//
// 全量覆盖有两种成因，修法不同（2026-10-03 踩过这个坑）：
//
//  ① provider 写 `if req.UpdateAll || req.X != ""` —— 有开关，
//     已在 request.* 加 `Partial` 修好（控制器 `if !req.Partial { req.UpdateAll = true }`，
//     意图层给 update 类 cap 注入 partial=true）。这类端点**不需要**本表补齐：
//     补齐反而多余且有害——它要回查旧值，而读端点可能返回派生值
//     （GetNavList 用 GetUrl 覆盖 link 就是先例）。
//
//  ② provider 直接 `material.Title = req.Title` —— **没有开关**，
//     partial 完全无效。这是本表要解决的：意图层回查旧值补齐，让「只传
//     要改的字段」真正成立。
//
// 判定方法（不要只搜 `req.UpdateAll = true`，那只是 ①）：
//
//	grep -c "req.UpdateAll" provider/<资源>.go     # 0 → 属 ②，收进本表
//	grep -n "req\.[A-Z][A-Za-z]* = req\." provider/<资源>.go   # 属 ②
var invokeRouteOverwriteFields = map[string][]string{
	// key 必须是 normalizeEndpointPath 之后的形态（带 /system/api 前缀），
	// 因为查表用的就是规范化后的路径——写成裸路径会永远查不到。
	//
	// 素材：provider/material.go:66-70 是逐字段无条件赋值（material.Title = req.Title…），
	// 没有 UpdateAll 但效果等价。实测（2026-10-03）素材 id=2「请输入验证码以便正常访问」
	// 只传 id+title 改名后 content 被清空（正文丢失），故保护 content。
	//
	// ⚠️ **status 不在保护列表**：material.go:68 写的是 `material.Status = 1` ——
	// 硬编码常量、根本不读 req.Status，即端点语义就是「保存即启用」。
	// 补齐传 0 也会被它覆盖成 1，登记了只是白登记（实测确认）。
	normalizeEndpointPath("/plugin/material/detail"): {"content", "category_id", "auto_update"},
	// 会员分组：commerce group_save。分级信息（level/price）被清零会让权限与定价失效。
	normalizeEndpointPath("/plugin/user/group/detail"): {"description", "level", "price", "favorable_price"},
	// 城市站：provider/place.go:93-105 逐字段无条件赋值。
	// 实测（2026-10-03，新建临时记录 id=1 后只传 id+title 改名）：
	// seo_title / keywords / description / template / logo / images / status / latitude
	// **8 个字段全被清空**（status 从 1 变 0 即站点被停用），波及面比会员那次更广。
	// 城市站是站点结构，模板/logo/坐标被清会直接影响前台渲染。
	normalizeEndpointPath("/plugin/place/detail"): {
		"seo_title", "keywords", "url_token", "description", "content",
		"parent_id", "sort", "template", "is_inherit", "images", "logo",
		"latitude", "longitude", "timezone", "status",
	},
}

// partialInvokePaths 登记「走 PATCH 语义、不需要补齐」的 POST 端点。
//
// 与 capEndpoints 的 partialUpdate 是两套并行机制，因为调用链不同：
//   - cap 类动作经 callCap → invokeCapEndpoint，那里统一注入 Fixed；
//   - invokeRoutes 声明式路由**绕过 capEndpoints**，直接调 api_invoke，
//     所以必须在调用点单独注入（见 invokeRoutes 里的分支）。
//
// 判定标准：provider 的 Save* 用的是 `if req.UpdateAll || req.X != ""`，
// 即存在 UpdateAll 开关。加了 partial 后端点自己就能只覆盖传入字段，
// 补齐（invokeRouteOverwriteFields）就多余了——而补齐要回查旧值，
// 读端点可能返回派生值（GetNavList 用 GetUrl 覆盖 link 是先例），
// 拿派生值写回等于用假数据覆盖真值。
//
// ⚠️ 这里是**路径**而不是 cap 名：同一个 POST 路径可能被多个意图共用。
var partialInvokePaths = map[string]bool{
	// 会员：provider/user.go 有 15 处 `if req.UpdateAll || req.X != ""`
	// （UserName/RealName/Birthday/Email/Phone/Status…），有开关。
	// 修复前它在 invokeRouteOverwriteFields 里（补齐兜底），
	// 加 partial 后已从那张表移除——两套机制叠加是多余的，且补齐有风险。
	normalizeEndpointPath("/plugin/user/detail"): true,
}

// preserveOnInvokeRoute 在调用全量覆盖的 POST 端点前，把未传的受保护字段用旧值补齐。
//
// 找旧值的方式：扫同一份 routes，找**指向同一路径的 GET 兄弟**（如
// user_update POST /plugin/user/detail ← user_get GET /plugin/user/detail）。
// 比按 cap 名配对更通用——routes 是声明式的，GET/POST 同路径是这里的普遍形态。
//
// 查不到就原样放行：让端点照旧处理，不猜值。
func preserveOnInvokeRoute(ctx context.Context, cap CapInvoker, action string,
	routes map[string]string, params map[string]any, args map[string]any) {
	if toInt64(params["id"]) <= 0 && toInt64(args["id"]) <= 0 {
		return // 没有 id 就不是更新，跳过
	}
	parts := strings.Fields(routes[action])
	if len(parts) != 2 {
		return
	}
	path := normalizeEndpointPath(parts[1])
	fields, ok := invokeRouteOverwriteFields[path]
	if !ok {
		return // 该端点不是全量覆盖（或未登记），不动
	}
	// 找同路径的 GET 兄弟
	var getTarget string
	for act, target := range routes {
		if act == action {
			continue
		}
		p := strings.Fields(target)
		if len(p) != 2 || !strings.EqualFold(p[0], "GET") {
			continue
		}
		if normalizeEndpointPath(p[1]) == path {
			getTarget = target
			break
		}
	}
	if getTarget == "" {
		return
	}
	out, err := cap(ctx, "api_invoke", map[string]any{
		"method": "GET",
		"path":   path,
		"params": map[string]any{"id": params["id"]},
	})
	if err != nil {
		return
	}
	old := parseArticle(out)
	if old == nil {
		return
	}
	for _, f := range fields {
		if _, provided := params[f]; provided {
			continue // 显式传了的一律尊重调用方
		}
		if v, ok := old[f]; ok && v != nil && v != "" {
			params[f] = v
		}
	}
}

func invokeRoutes(intentName string, routes map[string]string, renames ...map[string]string) Compose {
	registerIntentRoutes(intentName, routes)
	rename := mergeRenames(renames)
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
		params := endpointParams(args, rename)
		// 全量覆盖端点在写入前补齐未传字段（见 preserveOnInvokeRoute 的说明）。
		if strings.HasPrefix(strings.ToUpper(target), "POST") {
			// 走 PATCH 语义（partial）的端点不需要补齐——那是 UpdateAll 型，
			// 端点已能只覆盖显式传入的字段。两套机制不能叠加。
			//
			// partial 本身由 provider 的 injectPartialUpdate 统一注入
			// （那里是所有通道的必经之处，含通用 api 意图），此处只负责
			// 「跳过补齐」这个判断。
			if !partialInvokePaths[parts[1]] {
				preserveOnInvokeRoute(ctx, cap, action, routes, params, args)
			}
		}
		out, err := cap(ctx, "api_invoke", map[string]any{
			"method": parts[0],
			"path":   parts[1],
			"params": params,
		})
		if err != nil {
			return nil, err
		}
		// 解析 JSON 响应，提取 data 字段填充 Result.Data
		res := &Result{Text: out}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err == nil {
			if data, ok := envelope["data"]; ok && data != nil {
				// 端点自己回了 total 就说明它是分页列表，把分页信息一起带上。
				//
				// 背景（2026-10-03）：后台 34 个分页端点把 total 放在信封顶层，
				// 只取 data 会让 total 被静默丢弃。invokeRoutes 这条路径上的
				// action 命名五花八门（logs/texts/caches/statistic…），
				// 靠命名猜不可靠——直接看端点有没有回 total 最准。
				// 分页词用 args 里的原始意图参数（page/page_size），
				// endpointParams 之后才被换算成端点的 current/pageSize。
				res.Data = listWithTotal(data, envelope, args)
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
		Desc: "城市站（分站）的增删查与配置。action: list/detail/save/delete/setting_get/setting_save。",
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
		Desc: "SEO 收录相关操作。action: robots_get/robots_save(robots.txt 读写)/sitemap(重建 sitemap 并推送)/push_get/push_save/push_push/push_logs(链接推送配置与记录)。" +
			"robots_save 传的是 content（端点侧字段名 robots 已自动换算）；sitemap 不传参数时默认按 xml 重建。",
		Params: map[string]ParamSpec{
			"action":  {Type: "string", Desc: "操作", Required: true, Enum: []string{"robots_get", "robots_save", "sitemap", "push_get", "push_save", "push_push", "push_logs"}},
			"content": {Type: "string", Desc: "robots.txt 正文（robots_save 用）"},
			"values": {Type: "object", Desc: "按 action 传：\n" +
				"· push_save（**全量覆盖**）: baidu_api(百度推送地址)、bing_api、google_json、js_codes —— 改任一项都要四项传全\n" +
				"· sitemap: type（xml 或 txt，默认 xml）、auto_build（**整数 0/1，不是布尔值**，传 true/false 会报 json 解析失败）"},
			"urls":    {Type: "array", Items: "string", Desc: "要推送的 URL 列表（push_push 与 sitemap 的推送环节用）"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose:  seoCompose(),
	},
	{
		Name: "seo_jsonld", Title: "结构化数据配置", Domain: DomainSeo, Risk: RiskWrite,
		Desc: "JSON-LD 结构化数据的读写。action: get/save。",
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
		Desc: "面向生成式引擎的 llms.txt：读取配置、构建、查看状态。action: get/save/build/status。",
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
		Desc: "评论/留言的审核与清理。action: comment_list/comment_approve/comment_delete/guestbook_list/guestbook_detail/guestbook_setting_get/guestbook_setting_save/guestbook_status/guestbook_delete/guestbook_export。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"comment_list", "comment_approve", "comment_delete", "guestbook_list", "guestbook_detail", "guestbook_setting_get", "guestbook_setting_save", "guestbook_status", "guestbook_delete", "guestbook_export"}},
			"id":     {Type: "integer", Desc: "评论/留言 ID（comment_approve/guestbook_detail/guestbook_delete 必填）"},
			"page":   {Type: "integer", Desc: "页码", Default: 1},
			"ids":    {Type: "array", Items: "integer", Desc: "ID 列表（批量删除/批量更新状态时传，替代单个 id）"},
			"status": {Type: "integer", Desc: "状态。**整数不是布尔值**：0=待审，1=正常，2=垃圾（comment_approve/guestbook_status）"},
			"values": {Type: "object", Desc: "留言设置（guestbook_setting_save 时传入）：" +
				"return_message(提交成功后的提示语)、push_way(推送方式 整数：0=email/1=站点/2=API 接口)、" +
				"site_id(push_way=1 时必填)、api_url/api_method/header_key/header_value(push_way=2 的 API 推送配置，api_method 取 json/formdata/query)、" +
				"fields(留言字段列表)。**是全量覆盖**——先用 guestbook_setting_get 取全量，改完整体写回"},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose:  interactionCompose(),
	},

	// ───────────────────────── 内容生产域 ─────────────────────────
	{
		Name: "contentops_material", Title: "管理素材库", Domain: DomainContentOps, Risk: RiskWrite,
		Desc: "素材（待发布内容池）与其分类的管理、导入。action: list/detail/save/delete/import/category_list/category_save/category_delete。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "detail", "save", "delete", "import", "category_list", "category_save", "category_delete"}},
			"id":        {Type: "integer", Desc: "素材 ID"},
			"values":    {Type: "object", Desc: "素材字段（save 时传入）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
			"id":     {Type: "integer", Desc: "采集任务 ID（collect/combination 用）"},
			"title":  {Type: "string", Desc: "文章标题（collect/combination 用）"},
			"demand": {Type: "string", Desc: "采集要求/提示词（collect/combination 用）"},
			// replace 端点的 content_replace 是必填的 []config.ReplaceKeyword，
			// 缺了它整条 action 只会返回校验失败，模型无法自行猜到字段名。
			"replace":         {Type: "boolean", Desc: "是否启用内容替换（replace 用）"},
			"content_replace": {Type: "array", Items: "object", Desc: "替换规则列表（replace 必填，save 可选）。每项字段：from=待替换的词，to=替换后的词"},
			"values":          {Type: "object", Desc: "采集配置（save 时传入）：auto_collect、collect_mode、channels、error_times、language、insert_image、images、image_category_id、from_website、title_min_length、content_min_length、title_exclude、content_exclude、link_exclude、auto_pseudo、auto_translate、to_language、category_id、category_ids、save_type、start_hour、end_hour、daily_limit、custom_patten、proxy_config"},
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
			"action":      {Type: "string", Desc: "操作", Required: true, Enum: []string{"import", "status", "template", "setting_get", "setting_save", "timefactor_get", "timefactor_save"}},
			"category_id": {Type: "integer", Desc: "目标分类 ID（template 必填，import 建议传：决定导入文章落到哪个分类）"},
			"values":      {Type: "object", Desc: "导入参数/配置参数。import 传 title_type、plan_type、plan_start、days、check_duplicate、insert_image、images、image_category_id；setting_save 传 token、link_token；timefactor_save 传时间因子配置"},
			"file":        {Type: "string", Desc: "上传文件：data URI 或裸 base64"},
			"file_name":   {Type: "string", Desc: "文件名（配合 file）"},
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
		Desc: "翻译配置与翻译记录管理。action: get/save/logs/texts/text_save/text_delete。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"get", "save", "logs", "texts", "text_save", "text_delete"}},
			"id":        {Type: "integer", Desc: "记录 ID"},
			"values":    {Type: "object", Desc: "配置或译文内容"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
		Desc:       "自动生成标题图、图片水印。action: titleimage_get/titleimage_save/titleimage_upload/titleimage_generate/titleimage_preview/watermark_get/watermark_save/watermark_upload/watermark_generate/watermark_preview。上传字体/水印/底图文件用 *_upload（file 传 data URI 或裸 base64）。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"titleimage_get", "titleimage_save", "titleimage_upload", "titleimage_generate", "titleimage_preview",
				"watermark_get", "watermark_save", "watermark_upload", "watermark_generate", "watermark_preview"}},
			"values":    {Type: "object", Desc: "配置内容（*_save 时传入）"},
			"file":      {Type: "string", Desc: "字体/水印/底图文件：data URI 或裸 base64（*_upload）"},
			"file_name": {Type: "string", Desc: "文件名（配合 file，*_upload）"},
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
		Desc: "会员的增删改查与订单列表，会员分组的增删改查，分销商配置与审核。action: user_list/user_get/user_create/user_update/user_delete/user_orders/group_list/group_detail/group_save/group_delete/retailer_list/retailer_detail/retailer_setting_get/retailer_setting_save/retailer_apply/retailer_realname。" +
			"user_update 是**部分更新**：只传要改的字段即可，email/phone/group_id/status 等未传的会自动沿用原值" +
			"（意图层会先回查再补齐，2026-10-03 起不再需要手工 get 全量再整体写回）。" +
			"注意 group_save 与 retailer_setting_save 仍是全量覆盖语义，未登记的字段会被清成零值。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"user_list", "user_get", "user_create", "user_update", "user_delete", "user_orders", "group_list", "group_detail", "group_save", "group_delete", "retailer_list", "retailer_detail", "retailer_setting_get", "retailer_setting_save", "retailer_apply", "retailer_realname"}},
			"id":        {Type: "integer", Desc: "会员/分组/分销商 ID（retailer_apply / retailer_realname 也用会员的 id）"},
			"user_name": {Type: "string", Desc: "用户名（user_create 用）"},
			"password":  {Type: "string", Desc: "密码（user_create 必填；user_update 只在改密码时传）"},
			"email":     {Type: "string", Desc: "邮箱"},
			"phone":     {Type: "string", Desc: "手机号"},
			"status":    {Type: "integer", Desc: "会员状态，**整数不是布尔值**：0=待审核，1=正常，-1=禁用"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（*_list / user_orders）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（*_list / user_orders）", Default: 20},
			"values": {Type: "object", Desc: "按 action 传：" +
				"· user_update: **部分更新**，只传要改的字段；未传的 email/phone/group_id/status 等会自动沿用原值\n" +
				"· user_create: user_name、password(必填)、email、phone、group_id、status\n" +
				"· group_save: title、description、level(整数，越大权限越高)、price、favorable_price\n" +
				"· retailer_setting_save: allow_self(整数 0/1)、become_retailer(整数，0=需审核 1=自动成为分销员)\n" +
				"· retailer_apply: is_retailer(整数 0/1)\n" +
				"· retailer_realname: real_name(真实姓名)"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		Compose: invokeRoutes("commerce", map[string]string{
			"user_list":             "GET /plugin/user/list",
			"user_get":              "GET /plugin/user/detail",
			"user_create":           "POST /plugin/user/detail",
			"user_update":           "POST /plugin/user/detail",
			"user_delete":           "POST /plugin/user/delete",
			"user_orders":           "GET /plugin/order/list",
			"group_list":            "GET /plugin/user/group/list",
			"group_detail":          "GET /plugin/user/group/detail",
			"group_save":            "POST /plugin/user/group/detail",
			"group_delete":          "POST /plugin/user/group/delete",
			"retailer_list":         "GET /plugin/retailer/list",
			"retailer_detail":       "GET /plugin/user/detail",
			"retailer_setting_get":  "GET /plugin/retailer/config",
			"retailer_setting_save": "POST /plugin/retailer/config",
			"retailer_apply":        "POST /plugin/retailer/apply",
			"retailer_realname":     "POST /plugin/retailer/realname",
		}),
	},
	{
		Name: "commerce_order", Title: "管理订单", Domain: DomainCommerce, Risk: RiskWrite,
		Desc: "订单的查询与状态流转（发货/取消/完成/退款/支付）。action: list/detail/setting_get/setting_save/deliver/canceled/finished/refund/refund_apply/pay/export。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "detail", "setting_get", "setting_save", "deliver", "canceled", "finished",
				"refund", "refund_apply", "pay", "export"}},
			"id":        {Type: "string", Desc: "**订单号（业务单号，不是数据库自增 id）**，格式形如 wc2021101838889109642 / 202609300733050495，取自 list 返回项的 order_id 字段。detail/deliver/canceled/finished/refund/refund_apply/pay 必填（端点侧字段名是 order_id，已自动换算）"},
			"status":    {Type: "string", Desc: "订单状态过滤/目标状态：waiting,paid,delivery,finished,refunding,closed"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（list）", Default: 20},
			"values":    {Type: "object", Desc: "订单参数/配置参数。deliver 传 express_company、tracking_number；pay 传 user_id、pay_way；export 传 start_time、end_time；setting_save 传 no_process、auto_finish_day、auto_close_minute、seller_percent、no_need_login"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_invoke"},
		// 订单端点一律以 order_id 为主键（/plugin/order/detail、/deliver、/refund …），
		// 意图对外沿用通用的 id；不换算的话所有订单操作都打不到目标订单。
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
		}, map[string]string{"id": "order_id"}),
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
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
			"id":        {Type: "integer", Desc: "记录 ID"},
			"values":    {Type: "object", Desc: "申请/审批参数"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
		Desc: "公众号菜单、消息与自动回复规则。action: menu_list/menu_save/menu_delete/menu_sync/message_list/message_reply/message_delete/rule_list/rule_save/rule_delete。" +
			"注意三处返回形状差异：menu_list 只返回顶级菜单(父级 parent_id=0)，子菜单在各自的 children 字段里；" +
			"menu_list 空表时 data 为 null，而 message_list/rule_list 空表返回 []，判断结果前先看类型；" +
			"各 save 类 action 成功时 data 常为 null，判定是否写成功要回读 *_list 确认，不要只看 ok 字段。" +
			"menu_sync 会调微信官方接口，未配置 app_id/app_secret 时返回 errcode=41001 access_token missing 属预期。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"menu_list", "menu_save", "menu_delete", "menu_sync",
				"message_list", "message_reply", "message_delete",
				"rule_list", "rule_save", "rule_delete"}},
			"id": {Type: "integer", Desc: "记录 ID（menu_delete/message_reply/message_delete/rule_delete 必填；menu_save/rule_save 传了即为更新，不传则新建）"},
			"values": {Type: "object", Desc: "按 action 传对应字段：\n" +
				"· menu_save: name(必填)、type(click/view)、value(click 时是按钮 key，view 时是链接 URL)、sort(整数，小的在前)、parent_id(整数，0=顶级菜单，非 0=挂在该 id 下)。**是全量覆盖**——改任意字段都要把 name/type/value/sort/parent_id 一次传全，未传的会被置零值\n" +
				"· rule_save: keyword(触发关键词，留空字符串表示不按关键词触发)、content(回复内容)、**is_default(整数 0 或 1，不是布尔值，传 true/false 会报 json 解析失败)**。同样是全量覆盖\n" +
				"· message_reply: id(要回复的消息 ID)、reply(回复内容)"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（message_list/rule_list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（message_list/rule_list）", Default: 20},
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
		Desc: "公众号、小程序、Google 授权配置的读写。action: wechat_get/wechat_save/weapp_get/weapp_save/google_get/google_save。" +
			"⚠️ *_save 是**全量覆盖**而非合并：values 里没传的字段会被清成空串，" +
			"改单个字段必须先 *_get 取全量、改完整体写回。" +
			"凭证字段（app_secret/token/encoding_aes_key/client_secret）由 *_get **明文返回**，这是后台配置回显的既有行为。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"wechat_get", "wechat_save", "weapp_get", "weapp_save", "google_get", "google_save"}},
			"values": {Type: "object", Desc: "授权配置（*_save 时传入，**必须传全量**，未传的字段会被清空）。" +
				"wechat_save/weapp_save: app_id、app_secret、token、encoding_aes_key、verify_key、verify_msg；" +
				"google_save: client_id、client_secret、redirect_url。" +
				"判断 save 是否成功请用对应 *_get 回读校验——save 的响应 data 常为 null，看 ok 字段不可靠"},
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
		Desc: "邮件模板管理、服务配置与测试发送。action: list/detail/save/preview/setting_get/setting_save/logs/test。" +
			"返回统一为标准信封 {code,msg,ok,status,data}，业务载荷在 data 里（data 可能是对象或数组）。" +
			"⚠️ setting_save 是**全量覆盖**而非合并：values 里没传的字段会被清成零值，" +
			"务必先 setting_get 取全量、改完整体写回。" +
			"⚠️ setting_get 返回的 password 是掩码 ********，表示「授权码未改动」；" +
			"原样写回即可保持不变，只有真正换授权码时才传新值。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "detail", "save", "preview", "setting_get", "setting_save", "logs", "test"}},
			"key": {Type: "string", Desc: "模板 key。detail/save/preview 用它定位模板；" +
				"preview 只传 key 即可（服务端会自动补全模板内容再渲染）"},
			"values": {Type: "object", Desc: "配置或模板内容。" +
				"setting_save 传时必须是 setting_get 的完整结果（改若干字段后整体回传），只传部分字段会清空其余配置。" +
				"test 需传 {\"recipient\":\"\"} 走默认测试邮件；传 recipient 则须同时给 subject 与 message。"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
		Desc: "订阅用户与其分组的管理、群发。action: list/detail/save/delete/category_list/category_save/category_delete/send/send_status。" +
			"⚠️ send 是**异步长任务**（SMTP 单封耗时数秒，收件人多时同步会超时），只返回 job_id；" +
			"之后用 send_status 传 job_id 轮询，看 total/sent/failed/processed/running 与 errors 明细。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "save", "delete", "category_list", "category_save", "category_delete", "send", "send_status"}},
			"id":        {Type: "integer", Desc: "订阅用户 ID（save 更新时必填；delete 必填）"},
			"job_id":    {Type: "string", Desc: "群发任务 ID（send_status 必填），取自 send 返回的 job_id"},
			"values":    {Type: "object", Desc: "订阅用户字段（save）或群发内容（send）。send 必填字段：type（all/category/email）、subject（标题，注意不是 title）、content（正文）、emails（type=email 时的邮箱数组）、category_id（type=category 时）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
			"send_status":     "GET /plugin/subscriber/send/status",
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
			"values":    {Type: "object", Desc: "配置内容（save/setting_save 时传入）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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

	// ───────────────────────── 站点运维域 ─────────────────────────
	// 备份/升级/多站点属不可逆或全站级操作，保持 DefaultOff；
	// 缓存与全文索引维护是日常运营动作，默认开放（写动作仍走逐次确认）。
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
		Desc: "站点运维维护类操作。action: cache_get/cache_save/cache_build/cache_build_status/cache_build_index/cache_build_archive/cache_build_category/cache_build_tag/cache_clean/cache_push/cache_push_status/cache_push_logs/cache_upload/fulltext_get/fulltext_save/fulltext_status/fulltext_rebuild/replace。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"cache_get", "cache_save", "cache_build", "cache_build_status", "cache_build_index", "cache_build_archive", "cache_build_category", "cache_build_tag",
				"cache_clean", "cache_push", "cache_push_status", "cache_push_logs", "cache_upload",
				"fulltext_get", "fulltext_save", "fulltext_status", "fulltext_rebuild", "replace"}},
			"values":    {Type: "object", Desc: "缓存配置/索引配置/替换规则（*_save/replace 时传入）"},
			"file":      {Type: "string", Desc: "上传证书文件：data URI 或裸 base64（cache_upload）"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "detail", "save", "delete"}},
			"id":        {Type: "integer", Desc: "站点 ID"},
			"values":    {Type: "object", Desc: "站点配置（save 时传入）"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
		Desc: "只读：管理员列表与详情、分组列表与详情、后台菜单结构，以及管理员操作日志与登录日志。action: admin_list/admin_detail/admin_group_list/admin_group_detail/admin_menus/logs_action/logs_login。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"admin_list", "admin_detail", "admin_group_list", "admin_group_detail", "admin_menus", "logs_action", "logs_login"}},
			"id":        {Type: "integer", Desc: "管理员或分组 ID"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（列表类 action）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（列表类 action）", Default: 20},
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
		Desc: "只读：模板列表、详情与文件信息（修改模板请用 fs_write 通道）。action: list/detail/file_templates/file_info/helpers。" +
			"模板以 package（包名，取自 list 的结果）寻址，不是数字 ID。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{
				"list", "detail", "file_templates", "file_info", "helpers"}},
			"package": {Type: "string", Desc: "模板包名（detail/file_info 必填，取自 list 返回的 package 字段）"},
			"path":    {Type: "string", Desc: "模板内文件路径（file_info 用）"},
			"type":    {Type: "string", Desc: "文件类型（file_info 用）"},
			"values":  {Type: "object", Desc: "查询参数"},
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

// gatedDomainIntents 是补齐域中仍保持 DefaultOff 的意图及其理由。
//
// 判断标准只有一条：误操作的后果是否超出"改回来"的范围。默认开放的是日常运营
// （内容/结构/SEO/互动/素材/会员订单/缓存维护/只读查看），关闭的是
// 不可逆运维、主机级迁移、凭证与资金、对外发信、全站性配置。
//
// 这张表同时被 TestDomainIntentExposureIsDeliberate 用作另一半校验：
// 既不允许"默认开着却没登记"，也不允许"登记了却忘了标 DefaultOff"，
// 新增意图必须显式二选一，避免靠"记得加一行"来守住高危面。
var gatedDomainIntents = map[string]string{
	"contentops_collector": "批量采集：按配置持续写库并改采集策略，误配会批量产出垃圾内容",
	"contentops_import":    "批量导入 + 远程导入 Token 配置，一次失误影响整批文章",
	"contentops_transfer":  "跨站点数据迁移（主机级），覆盖目标站数据不可回滚",
	"contentops_imagedeco": "配图/水印：上传字体图片文件并批量改写附件",
	"commerce_pay":         "收款账户与支付证书，涉及资金凭证",
	"commerce_finance":     "财务流水与提现审批，直接涉及资金",
	"channel_wechat":       "公众号菜单与消息回复会同步到微信侧，对外可见",
	"channel_thirdparty":   "读写公众号/小程序/Google 授权配置（AppID、Secret）",
	"channel_sendmail":     "SMTP 配置与测试发信，会真实投递邮件给收件人",
	"channel_subscriber":   "订阅用户群发（send），对外触达真实邮箱",
	"system_security":      "访问限制/封禁/Akismet 等风控配置，误设可把站点或管理员挡在门外",
	"system_multilang":     "多语言子站的保存、删除与同步，影响其他站点",
	"system_rewrite":       "伪静态规则决定全站 URL 结构，写错即全站 404",
	"siteops_backup":       "数据库备份/导入/恢复/清理（主机级，破坏性）",
	"siteops_upgrade":      "版本检查与升级（主机级，破坏性）",
	"siteops_website":      "多站点增删（主机级）",
}

// RecommendedExposed 返回推荐的起步暴露清单：站点日常运营用到的意图。
//
// 三道闸门中 ExposedIntents 是第二道，决定「哪些意图出现在模型面」。
// 常用意图已不打 DefaultOff，留空即可得到这份清单描述的内容/结构/SEO/互动/素材/交易/运维能力。
// 清单刻意不含备份/升级/迁移/多站点/凭证与资金/对外发信这类高危面。
//
// 与留空的差异只在内置主机级工具（shell_exec、fs_*、system_plugin）：它们同为默认可见，
// 服务的是自我编写 Agent 的场景。纯运营站点显式填入本清单会把它们排除掉——
// 这正是本清单的实际价值：不是"打开能力"，而是"在不重启的前提下收口"。
func RecommendedExposed() []string {
	return []string{
		// 内容（核心写作链路）
		"content_article", "content_manage", "content_place",
		// 素材
		"media",
		// 结构
		"structure",
		// SEO 与数据
		"seo_keyword", "seo_anchor", "seo", "seo_jsonld", "seo_llms", "traffic_statistics",
		// 互动
		"interaction",
		// 内容生产（读多写少）
		"contentops_material", "contentops_translate",
		// 交易（会员与订单）
		"commerce", "commerce_order",
		// 站点配置与运维（保守：仅缓存/全文维护，不含备份/升级/迁移/多站点）
		"system_config", "siteops_maintain",
		// 只读查看
		"account", "design_manage",
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
