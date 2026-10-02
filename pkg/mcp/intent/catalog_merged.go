package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// catalog_merged.go 把多个语义相近的意图合并为少量组合工具，降低暴露面（约 82 → 51）。
//
// 合并后的工具一律用 action 字段区分子操作：
//   - 能力类子操作经 switchCompose 委托给对应 cap（cap 再经 cap_routes.go 解析成真实端点）；
//   - 端点类子操作经 invokeRoutes / 直接 api_invoke 执行。
//
// 风险等级取并集（最高风险），DefaultOff 继承原工具（原 cap 类域工具多为 DefaultOff）。
// 覆盖审计：端点类子操作在 Compose 构造期调用 registerIntentRoutes 登记，
// 由 TestDeclaredEndpointTargetsRegistered 穷举校验。

var mergedIntentCatalog = []*IntentSpec{
	// ───────────────────────── 内容域：文档 ─────────────────────────
	{
		Name: "content_article", Title: "文档全生命周期", Domain: DomainContent, Risk: RiskWrite,
		Desc: "文档的列出/获取/保存(创建或更新)/发布下架/删除。action: list/get/save/publish/delete。save 时 id 为空则新建，否则按 id 更新（仅覆盖传入字段）。",
		Params: map[string]ParamSpec{
			"action":       {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "get", "save", "publish", "delete"}},
			"page":         {Type: "integer", Desc: "页码，从 1 开始", Default: 1},
			"page_size":    {Type: "integer", Desc: "每页条数", Default: 20},
			"category_id":  {Type: "integer", Desc: "分类 ID 过滤"},
			"module_id":    {Type: "integer", Desc: "模型 ID 过滤"},
			"parent_id":    {Type: "integer", Desc: "父级 ID 过滤"},
			"flag":         {Type: "string", Desc: "标记过滤（h=头条，c=推荐，f=幻灯，a=特荐，s=滚动，b=加粗，p=图片，j=跳转）"},
			"keyword":      {Type: "string", Desc: "标题/关键词模糊搜索"},
			"status":       {Type: "string", Desc: "状态：ok=正式文档，draft=草稿，plan=待发布", Enum: []string{"ok", "draft", "plan"}},
			"order_by":     {Type: "string", Desc: "排序字段，如 created_time"},
			"order_dir":    {Type: "string", Desc: "排序方向 asc/desc"},
			"id":           {Type: "integer", Desc: "文档 ID（get/save 更新/publish/delete 必填）"},
			"title":        {Type: "string", Desc: "标题（save 必填）"},
			"content":      {Type: "string", Desc: "正文（Markdown）（save 必填）"},
			"logo":         {Type: "string", Desc: "封面图 URL"},
			"draft":        {Type: "boolean", Desc: "true=草稿，false=发布"},
			"tags":         {Type: "array", Items: "string", Desc: "标签列表"},
			"created_time": {Type: "integer", Desc: "创建时间 Unix 时间戳"},
		},
		Required: []string{"action"},
		Caps:     []string{"archive_list", "archive_get", "archive_create", "archive_update", "archive_publish", "archive_delete"},
		Compose:  contentArticleCompose,
		Output:   ArticleOut{},
	},
	// ───────────────────────── 内容域：分类/标签/单页/模型 ─────────────────────────
	{
		Name: "content_manage", Title: "管理分类/标签/单页/模型", Domain: DomainContent, Risk: RiskWrite,
		Desc: "内容结构对象的增删改查。action: category_list/category_get/category_create/category_update/category_delete/tag_list/tag_get/tag_create/tag_update/tag_delete/page_list/page_get/page_create/page_update/page_delete/module_list/module_get/module_create/module_update/module_delete。",
		Params: map[string]ParamSpec{
			"action":      {Type: "string", Desc: "操作", Required: true, Enum: []string{"category_list", "category_get", "category_create", "category_update", "category_delete", "tag_list", "tag_get", "tag_create", "tag_update", "tag_delete", "page_list", "page_get", "page_create", "page_update", "page_delete", "module_list", "module_get", "module_create", "module_update", "module_delete"}},
			"id":          {Type: "integer", Desc: "对象 ID（get/update/delete 必填）"},
			"title":       {Type: "string", Desc: "名称（create/update）"},
			"parent_id":   {Type: "integer", Desc: "父级 ID"},
			"module_id":   {Type: "integer", Desc: "模型 ID，默认 1"},
			"description": {Type: "string", Desc: "描述"},
			"keywords":    {Type: "string", Desc: "关键词"},
			"content":     {Type: "string", Desc: "单页正文"},
			"status":      {Type: "integer", Desc: "单页状态 1/0"},
		},
		Required: []string{"action"},
		Caps: []string{
			"category_list", "category_get", "category_create", "category_update", "category_delete",
			"tag_list", "tag_get", "tag_create", "tag_update", "tag_delete",
			"page_list", "page_get", "page_create", "page_update", "page_delete",
			"module_list", "module_get", "module_create", "module_update", "module_delete",
		},
		Compose: switchCompose(map[string]string{
			"category_list": "category_list", "category_get": "category_get", "category_create": "category_create", "category_update": "category_update", "category_delete": "category_delete",
			"tag_list": "tag_list", "tag_get": "tag_get", "tag_create": "tag_create", "tag_update": "tag_update", "tag_delete": "tag_delete",
			"page_list": "page_list", "page_get": "page_get", "page_create": "page_create", "page_update": "page_update", "page_delete": "page_delete",
			"module_list": "module_list", "module_get": "module_get", "module_create": "module_create", "module_update": "module_update", "module_delete": "module_delete",
		}),
	},
	// ───────────────────────── 素材域 ─────────────────────────
	{
		Name: "media", Title: "素材管理", Domain: DomainMedia, Risk: RiskWrite,
		Desc: "附件/图片的上传、列出、获取、删除。action: upload/list/get/delete。",
		Params: map[string]ParamSpec{
			"action":      {Type: "string", Desc: "操作", Required: true, Enum: []string{"upload", "list", "get", "delete"}},
			"base64":      {Type: "string", Desc: "文件 base64 内容（upload）"},
			"url":         {Type: "string", Desc: "远程文件 URL（服务端下载，upload）"},
			"file_name":   {Type: "string", Desc: "文件名（可选）"},
			"id":          {Type: "integer", Desc: "附件 ID（get/delete 必填）"},
			"page":        {Type: "integer", Desc: "页码", Default: 1},
			"page_size":   {Type: "integer", Desc: "每页条数", Default: 20},
			"category_id": {Type: "integer", Desc: "分类 ID"},
		},
		Required: []string{"action"},
		Caps:     []string{"attachment_upload", "attachment_list", "attachment_get", "attachment_delete"},
		Compose: mediaCompose(map[string]string{
			"upload": "attachment_upload", "list": "attachment_list", "get": "attachment_get", "delete": "attachment_delete",
		}),
	},
	// ───────────────────────── 结构域 ─────────────────────────
	{
		Name: "structure", Title: "管理导航/友链/重定向", Domain: DomainStructure, Risk: RiskWrite,
		Desc: "站点结构的增删查。action: nav_list/nav_create/nav_update/nav_delete/friendlink_list/friendlink_create/friendlink_delete/redirect_list/redirect_create。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"nav_list", "nav_create", "nav_update", "nav_delete", "friendlink_list", "friendlink_create", "friendlink_delete", "redirect_list", "redirect_create"}},
			"id":        {Type: "integer", Desc: "对象 ID"},
			"title":     {Type: "string", Desc: "标题/站点名"},
			"link":      {Type: "string", Desc: "链接"},
			"logo":      {Type: "string", Desc: "Logo URL"},
			"parent_id": {Type: "integer", Desc: "父级 ID"},
			"from":      {Type: "string", Desc: "重定向源路径（访问地址 URI）"},
			"to":        {Type: "string", Desc: "重定向目标路径"},
		},
		Required: []string{"action"},
		Caps:     []string{"nav_list", "nav_create", "nav_update", "nav_delete", "friendlink_list", "friendlink_create", "friendlink_delete", "redirect_list", "redirect_create"},
		Compose: switchCompose(map[string]string{
			"nav_list": "nav_list", "nav_create": "nav_create", "nav_update": "nav_update", "nav_delete": "nav_delete",
			"friendlink_list": "friendlink_list", "friendlink_create": "friendlink_create", "friendlink_delete": "friendlink_delete",
			"redirect_list": "redirect_list", "redirect_create": "redirect_create",
		}),
	},
	// ───────────────────────── 智能体域：技能（本地，DefaultOff）─────────────────────────
	{
		Name: "skill", Title: "管理本地技能", Domain: DomainAgent, Risk: RiskWrite,
		Desc: "本地技能（SKILL）的列出/加载/重载/保存。action: list/get/reload/save。涉及文件系统，默认关闭，需 ExposedIntents 显式开启。",
		Params: map[string]ParamSpec{
			"action":      {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "get", "reload", "save"}},
			"name":        {Type: "string", Desc: "技能名称（get/save，英文小写连字符，如 seo-analyzer）", Required: true},
			"arguments":   {Type: "string", Desc: "传递给技能的参数（get 可选）"},
			"content":     {Type: "string", Desc: "技能正文 Markdown（save 必填）"},
			"description": {Type: "string", Desc: "一句话描述"},
			"category":    {Type: "string", Desc: "分类，如 SEO、写作、运维"},
			"tags":        {Type: "string", Desc: "标签，逗号分隔"},
		},
		Required:   []string{"action"},
		Caps:       []string{"skill_list", "skill_get", "skill_reload", "skill_save"},
		Compose:    switchCompose(map[string]string{"list": "skill_list", "get": "skill_get", "reload": "skill_reload", "save": "skill_save"}),
		DefaultOff: true,
	},
	// ───────────────────────── 智能体域：Agent 调度 ─────────────────────────
	{
		Name: "agent", Title: "管理 AI Agent", Domain: DomainAgent, Risk: RiskWrite,
		Desc: "Agent 与子任务的增删查/运行，及远程技能检索安装。action: manage_create/manage_list/manage_delete/manage_toggle/manage_run/manage_chat/skill_search/skill_install/task。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"manage_create", "manage_list", "manage_delete", "manage_toggle", "manage_run", "manage_chat", "skill_search", "skill_install", "task"}},
			"id":     {Type: "integer", Desc: "Agent ID"},
			"title":  {Type: "string", Desc: "名称"},
			"prompt": {Type: "string", Desc: "提示词/对话内容"},
			"status": {Type: "integer", Desc: "状态 1/0（toggle）"},
			"query":  {Type: "string", Desc: "技能搜索关键词（skill_search）"},
			"limit":  {Type: "integer", Desc: "搜索结果上限 1-50，默认 10（skill_search）"},
			"slug":   {Type: "string", Desc: "SkillHub 技能 slug，如 find-skills（skill_install）"},
			"force":  {Type: "boolean", Desc: "是否覆盖已安装同名技能（skill_install）"},
			"tasks":  {Type: "array", Items: "object", Desc: "子任务列表（task）。每项字段：description(必填,3-5词标签)、prompt(必填,完整指令)、type(必填,explore只读/worker可写)、scope(可选,worker允许写的文件glob数组)"},
		},
		Required: []string{"action"},
		Caps:     []string{"agent_create", "agent_list", "agent_delete", "agent_toggle", "agent_run", "agent_chat", "skill_search", "skill_install", "task"},
		Compose: switchCompose(map[string]string{
			"manage_create": "agent_create", "manage_list": "agent_list", "manage_delete": "agent_delete", "manage_toggle": "agent_toggle", "manage_run": "agent_run", "manage_chat": "agent_chat",
			"skill_search": "skill_search", "skill_install": "skill_install",
			"task": "task",
		}),
	},
	// ───────────────────────── 系统域：4 个配置读写（合并）─────────────────────────
	{
		Name: "system_config", Title: "站点配置读写", Domain: DomainSystem, Risk: RiskWrite,
		Desc: "站点设置的读写收敛：action: setting(按 section 读写 system/content/contact/diy_field/index/safe/rewrite)/site_info/site_version/site_anqi(站点与版本信息)/storage_get/storage_save/storage_upload(存储配置)/template_info/template_reload(模板)。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"setting", "site_info", "site_version", "site_anqi", "storage_get", "storage_save", "storage_upload", "template_info", "template_reload"}},
			"section":   {Type: "string", Desc: "设置分区（setting 用）", Enum: []string{"system", "content", "contact", "diy_field", "index", "safe", "rewrite"}},
			"values":    {Type: "object", Desc: "要更新的字段（setting 写 / storage_save 用）"},
			"content":   {Type: "string", Desc: "robots 等文本内容（storage_upload 用）"},
			"file":      {Type: "string", Desc: "上传证书文件：data URI 或裸 base64（storage_upload 用）"},
			"file_name": {Type: "string", Desc: "文件名（配合 file）"},
		},
		Required: []string{"action"},
		Caps: []string{
			"setting_system", "setting_system_form", "setting_content", "setting_content_form", "setting_contact", "setting_contact_form",
			"setting_diy_field", "setting_diy_field_form", "setting_index", "setting_index_form", "setting_safe", "setting_safe_form",
			"rewrite_get", "rewrite_form", "website_info", "version", "anqi_info", "template_get_info", "template_reload", "api_invoke",
		},
		Compose: systemConfigCompose,
	},
	// ───────────────────────── 内置域：联网获取与搜索 ─────────────────────────
	{
		Name: "web", Title: "联网获取与搜索", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "通过 HTTP 获取网页内容(fetch) 或在线搜索(search)。action: fetch/search。fetch 需 url，search 需 query。",
		Params: map[string]ParamSpec{
			"action":  {Type: "string", Desc: "操作", Required: true, Enum: []string{"fetch", "search"}},
			"url":     {Type: "string", Desc: "目标网页 URL（fetch 必填）"},
			"query":   {Type: "string", Desc: "搜索关键词（search 必填）"},
			"limit":   {Type: "integer", Desc: "返回结果条数上限（search 用）"},
			"timeout": {Type: "integer", Desc: "超时秒数（fetch 用）"},
			"headers": {Type: "object", Desc: "自定义请求头（fetch 用）"},
		},
		Required: []string{"action"},
		Caps:     []string{"web_fetch", "web_search"},
		Compose:  switchCompose(map[string]string{"fetch": "web_fetch", "search": "web_search"}),
	},
}

// contentArticleCompose 文档组合工具：list/get/publish/delete 直委托 cap，
// save 走 archive_create/update 并按返回回填结构化回执（含 id）。
func contentArticleCompose(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
	action, _ := args["action"].(string)
	switch action {
	case "list", "get", "publish", "delete":
		capName := map[string]string{
			"list": "archive_list", "get": "archive_get",
			"publish": "archive_publish", "delete": "archive_delete",
		}[action]
		out, err := callCap(ctx, cap, capName, withoutAction(args))
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
	case "save":
		// 两个能力都映射到 POST /archive/detail（端点靠 id 是否为空区分新建与更新）。
		id := toInt64(args["id"])
		capName := "archive_update"
		if id == 0 {
			capName = "archive_create"
		}
		out, err := callCap(ctx, cap, capName, withoutAction(args))
		if err != nil {
			return nil, err
		}
		// 解析端点返回的文档对象，回填结构化回执（与 ArticleOut 对齐）。
		// api_invoke 的 out 形如 {"ok":..,"data":<控制器响应>}，控制器再包一层
		// {"code","msg","data":<文档>}，故文档在 data.data 里；extractID 同样优先取该路径。
		art := parseArticle(out)
		rid := extractID(out)
		data := map[string]any{"action": capName}
		switch {
		case id != 0:
			data["id"] = id
		case art != nil:
			if v, ok := art["id"].(float64); ok && v != 0 {
				data["id"] = int64(v)
			} else {
				data["id"] = rid
			}
		default:
			data["id"] = rid
		}
		if art != nil {
			if v, ok := art["title"].(string); ok {
				data["title"] = v
			}
			if v, ok := art["link"].(string); ok && v != "" {
				data["url"] = v
			}
			if v, ok := art["status"]; ok {
				data["status"] = v
			}
		}
		return &Result{Text: out, Data: data}, nil
	default:
		return nil, fmt.Errorf("不支持的操作: %s（可选: list/get/save/publish/delete）", action)
	}
}

// mediaCompose 直接委托 switchCompose。
// attachment_upload 现已返回标准 JSON 信封（含 data 字段），switchCompose 会自动提取，
// 不再需要从纯文本中抠 id 的 workaround。
func mediaCompose(routes map[string]string) Compose {
	return switchCompose(routes)
}

// systemConfigCompose 收敛 4 类站点配置读写：setting 走按 section 选读写能力，
// 其余按 action 委托对应 cap / 端点。
func systemConfigCompose(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
	action, _ := args["action"].(string)
	sub := withoutAction(args)
	switch action {
	case "setting":
		return systemSettingCompose(ctx, sub, cap)
	case "site_info":
		return capResult(callCap(ctx, cap, "website_info", sub))
	case "site_version":
		return capResult(callCap(ctx, cap, "version", sub))
	case "site_anqi":
		return capResult(callCap(ctx, cap, "anqi_info", sub))
	case "template_info":
		return capResult(callCap(ctx, cap, "template_get_info", sub))
	case "template_reload":
		return capResult(callCap(ctx, cap, "template_reload", sub))
	case "storage_get", "storage_save", "storage_upload":
		method, path := "GET", ""
		switch action {
		case "storage_get":
			method, path = "GET", "/plugin/storage"
		case "storage_save":
			method, path = "POST", "/plugin/storage"
		case "storage_upload":
			method, path = "POST", "/plugin/storage/upload"
		}
		out, err := cap(ctx, "api_invoke", map[string]any{
			"method": method, "path": normalizeEndpointPath(path), "params": sub,
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
	default:
		return nil, fmt.Errorf("不支持的操作: %s（可选: setting/site_info/site_version/site_anqi/storage_get/storage_save/storage_upload/template_info/template_reload）", action)
	}
}

func init() {
	IntentCatalog = append(IntentCatalog, mergedIntentCatalog...)
}

// endpointInvoke 直接以 api_invoke 能力执行一个后台端点（target 形如 "METHOD PATH"）。
// 用于合并意图里"端点类子操作"的分派，与 invokeRoutes 行为一致但不绑定单一 action 字段。
func endpointInvoke(ctx context.Context, cap CapInvoker, target string, params map[string]any) (string, error) {
	parts := strings.Fields(target)
	if len(parts) != 2 {
		return "", fmt.Errorf("端点声明格式错误（应为 \"METHOD PATH\"）: %q", target)
	}
	return cap(ctx, "api_invoke", map[string]any{
		"method": parts[0], "path": normalizeEndpointPath(parts[1]), "params": params,
	})
}

// seoCompose 合并 seo_sitemap + seo_robots + seo_push：
// sitemap 走 cap(sitemap_rebuild+url_push)，robots/push 走端点，均经 api_invoke。
func seoCompose() Compose {
	registerIntentRoutes("seo", map[string]string{
		"robots_get":  "GET /plugin/robots",
		"robots_save": "POST /plugin/robots",
		"push_get":    "GET /plugin/push",
		"push_save":   "POST /plugin/push",
		"push_push":   "POST /plugin/push/push",
		"push_logs":   "GET /plugin/push/logs",
	})
	guestbookRoutes := map[string]string{
		"robots_get":  "GET /plugin/robots",
		"robots_save": "POST /plugin/robots",
		"push_get":    "GET /plugin/push",
		"push_save":   "POST /plugin/push",
		"push_push":   "POST /plugin/push/push",
		"push_logs":   "GET /plugin/push/logs",
	}
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		sub := withoutAction(args)
		switch action {
		case "sitemap":
			out1, err := callCap(ctx, cap, "sitemap_rebuild", map[string]any{})
			if err != nil {
				return nil, err
			}
			out2, err := callCap(ctx, cap, "url_push", map[string]any{"urls": args["urls"]})
			if err != nil {
				return nil, err
			}
			return &Result{Text: out1 + "\n" + out2}, nil
		case "robots_get", "robots_save", "push_get", "push_save", "push_push", "push_logs":
			out, err := endpointInvoke(ctx, cap, guestbookRoutes[action], sub)
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
		default:
			return nil, fmt.Errorf("不支持的操作: %s（可选: robots_get/robots_save/sitemap/push_get/push_save/push_push/push_logs）", action)
		}
	}
}

// interactionCompose 合并 interaction_comment + interaction_guestbook：
// comment_* 与 guestbook_list 走 cap（均有等价端点 → api_invoke），其余 guestbook_* 走端点。
func interactionCompose() Compose {
	registerIntentRoutes("interaction", map[string]string{
		"guestbook_detail":       "GET /plugin/guestbook/detail",
		"guestbook_setting_get":  "GET /plugin/guestbook/setting",
		"guestbook_setting_save": "POST /plugin/guestbook/setting",
		"guestbook_status":       "POST /plugin/guestbook/status",
		"guestbook_delete":       "POST /plugin/guestbook/delete",
		"guestbook_export":       "POST /plugin/guestbook/export",
	})
	guestbookEndpoints := map[string]string{
		"guestbook_detail":       "GET /plugin/guestbook/detail",
		"guestbook_setting_get":  "GET /plugin/guestbook/setting",
		"guestbook_setting_save": "POST /plugin/guestbook/setting",
		"guestbook_status":       "POST /plugin/guestbook/status",
		"guestbook_delete":       "POST /plugin/guestbook/delete",
		"guestbook_export":       "POST /plugin/guestbook/export",
	}
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		sub := withoutAction(args)
		switch action {
		case "comment_list", "comment_approve", "comment_delete", "guestbook_list":
			capName := map[string]string{
				"comment_list":    "comment_list",
				"comment_approve": "comment_approve",
				"comment_delete":  "comment_delete",
				"guestbook_list":  "guestbook_list",
			}[action]
			return capResult(callCap(ctx, cap, capName, sub))
		case "guestbook_detail", "guestbook_setting_get", "guestbook_setting_save", "guestbook_status", "guestbook_delete", "guestbook_export":
			out, err := endpointInvoke(ctx, cap, guestbookEndpoints[action], sub)
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
		default:
			return nil, fmt.Errorf("不支持的操作: %s（可选: comment_list/comment_approve/comment_delete/guestbook_list/guestbook_detail/guestbook_setting_get/guestbook_setting_save/guestbook_status/guestbook_delete/guestbook_export）", action)
		}
	}
}
