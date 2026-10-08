package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
		Desc: "文档的列出/获取/保存(创建或更新)/发布下架/删除。action: list/get/save/publish/delete。save 时 id 为空则新建，否则按 id 更新（只覆盖传入字段，未传的正文/封面/模型/分类/标签/标记/相关文档自动沿用原值，不会被清空）。save 传 draft=false 即直接发布（无需再调 publish），返回 id、link 与 status，草稿的 link 末尾带 ?preview=true。publish/delete 对不存在的 id 会直接报错，不会假成功。delete 正式文档是移入回收站（回执 moved_to_trash=true，可恢复）而非物理删除，只有删草稿才是真删。list 的 data 为 {list,total,page,page_size,count}，total 是命中总数，可据此翻页。",
		Params: map[string]ParamSpec{
			"action":       {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "get", "save", "publish", "delete"}},
			"page":         {Type: "integer", Desc: "页码，从 1 开始", Default: 1},
			"page_size":    {Type: "integer", Desc: "每页条数", Default: 20},
			"category_id":  {Type: "integer", Desc: "分类 ID 过滤"},
			"module_id":    {Type: "integer", Desc: "模型 ID 过滤"},
			"parent_id":    {Type: "integer", Desc: "父级 ID 过滤"},
			"flag":         {Type: "string", Desc: "文档标记，逗号分隔（h=头条，c=推荐，f=幻灯，a=特荐，s=滚动，b=加粗，p=图片，j=跳转）。list 时作为过滤条件；save 时写入该标记"},
			"relation_ids": {Type: "array", Items: "integer", Desc: "相关文档 ID 列表（save 写入）"},
			"keyword":      {Type: "string", Desc: "标题/关键词模糊搜索"},
			"status":       {Type: "string", Desc: "状态。list 时为过滤条件：ok=正式文档、draft=草稿、plan=待发布、delete=回收站（已删除可恢复）；publish 时为变更动作：ok=上架、draft=下架（意图层自动转成端点的 1/0）。传其它值会直接报错，不会静默返回错数据", Enum: []string{"ok", "draft", "plan", "delete"}},
			"order_by":     {Type: "string", Desc: "排序字段，推荐用 id/created_time/updated_time/views/sort。也可填 archives 表的其他真实列名（如 title、comment_count）；填不存在的列会直接报错，不会静默返回空列表", Enum: archiveOrderColumnList()},
			"order_dir":    {Type: "string", Desc: "排序方向 asc/desc。非法值会报错", Enum: []string{"asc", "desc"}},
			"id":           {Type: "integer", Desc: "文档 ID（get/save 更新/publish/delete 必填）"},
			"title":        {Type: "string", Desc: "标题（save 必填）"},
			"content":      {Type: "string", Desc: "正文（Markdown）。新建时必填"},
			"logo":         {Type: "string", Desc: "封面图 URL"},
			"draft":        {Type: "boolean", Desc: "true=草稿，false=发布。草稿的 link 带 ?preview=true"},
			"tags":         {Type: "array", Items: "string", Desc: "标签列表。不传则沿用原标签，传空数组则清空"},
			"created_time": {Type: "integer", Desc: "创建时间 Unix 时间戳"},
		},
		Required: []string{"action"},
		Caps:     []string{"archive_list", "archive_get", "archive_create", "archive_update", "archive_publish", "archive_delete"},
		Compose:  contentArticleCompose,
	},
	// ───────────────────────── 内容域：分类/标签/单页/模型 ─────────────────────────
	{
		Name: "content_manage", Title: "管理分类/标签/单页/模型", Domain: DomainContent, Risk: RiskWrite,
		Desc: "内容结构对象的增删改查。action: category_list/category_get/category_create/category_update/category_delete/tag_list/tag_get/tag_create/tag_update/tag_delete/page_list/page_get/page_create/page_update/page_delete/module_list/module_get/module_create/module_update/module_delete。*_update 传 id 即按 id 更新，只覆盖传入字段（description/keywords/module_id/status 等未传的会自动沿用原值，不会被清零）。*_list 的 data 为 {list,total,page,page_size,count}，total 是命中总数；分类/模型/导航这类全量返回的 list 没有分页，data 直接是数组。",
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
			"page":        {Type: "integer", Desc: "页码，从 1 开始（tag_list）", Default: 1},
			"page_size":   {Type: "integer", Desc: "每页条数（tag_list）", Default: 20},
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
		Desc: "站点结构的增删查。action: nav_list/nav_create/nav_update/nav_delete/friendlink_list/friendlink_create/friendlink_delete/redirect_list/redirect_create/redirect_update/redirect_delete。nav_update 只改标题等安全字段时，未传的字段会用库里的现值补齐；**但 link 例外，必须显式传入**——导航列表端点返回的 link 是 GetUrl 派生的（系统导航恒为 /），拿它补齐会把真实链接改坏。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"nav_list", "nav_create", "nav_update", "nav_delete", "friendlink_list", "friendlink_create", "friendlink_delete", "redirect_list", "redirect_create", "redirect_update", "redirect_delete"}},
			"id":        {Type: "integer", Desc: "对象 ID（nav_update/nav_delete/friendlink_delete/redirect_update/redirect_delete 必填）"},
			"title":     {Type: "string", Desc: "标题/站点名"},
			"link":      {Type: "string", Desc: "链接。nav_update 时**必须显式传入**（外链型导航全靠它），不传会被清空——系统无法从库安全地补出这个值"},
			"logo":      {Type: "string", Desc: "Logo URL"},
			"parent_id": {Type: "integer", Desc: "父级 ID"},
			"from":      {Type: "string", Desc: "重定向源路径（访问地址 URI）。redirect_create/redirect_update 用"},
			"to":        {Type: "string", Desc: "重定向目标路径。redirect_create/redirect_update 用"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（redirect_list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（redirect_list）", Default: 20},
		},
		Required: []string{"action"},
		Caps:     []string{"nav_list", "nav_create", "nav_update", "nav_delete", "friendlink_list", "friendlink_create", "friendlink_delete", "redirect_list", "redirect_create", "redirect_update", "redirect_delete"},
		Compose: switchCompose(map[string]string{
			"nav_list": "nav_list", "nav_create": "nav_create", "nav_update": "nav_update", "nav_delete": "nav_delete",
			"friendlink_list": "friendlink_list", "friendlink_create": "friendlink_create", "friendlink_delete": "friendlink_delete",
			"redirect_list": "redirect_list", "redirect_create": "redirect_create",
			"redirect_update": "redirect_update", "redirect_delete": "redirect_delete",
		}),
	},
	// ───────────────────────── 智能体域：技能（本地）─────────────────────────
	//
	// 2026-10-07：去掉 DefaultOff，默认开放。
	// 判断依据是**误操作后果**而非「是否碰文件系统」——skill 的五个动作里
	// 四个（list/get/reload/save）是日常写作链路的一部分，AI 看不见技能就
	// 没法按站点约定的方式产出内容；真正不可逆的只有 delete，而它：
	//   - 走审批门（Risk=write → NeedsApproval）；
	//   - 端点侧有字符白名单挡路径穿越（controller/manageController/skill.go），
	//     name 只接受不含路径分隔符的技能名；
	//   - 只删 skills/<name> 目录，不碰站点其它路径。
	// 也就是说 delete 的风险已被三道闸门覆盖，不构成「默认不该可见」的理由。
	{
		Name: "skill", Title: "管理本地技能", Domain: DomainAgent, Risk: RiskWrite,
		Desc: "本地技能（SKILL）的列出/加载/重载/保存/删除。action: list/get/reload/save/delete。delete 不可恢复且需确认，只接受不含路径分隔符的技能名（不会波及技能目录以外的文件）。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "get", "reload", "save", "delete"}},
			// name 不写进 Required：list/reload 不需要它，而两条通道对
			// ParamSpec.Required 的处理不同（Eino 会标必填、MCP 只看 IntentSpec.Required），
			// 写在这里会让同一个工具在后台对话与 MCP 里契约不一致。缺 name 由 cap 报错。
			"name":        {Type: "string", Desc: "技能名称（get/save/delete 必填，英文小写连字符，如 seo-analyzer）"},
			"content":     {Type: "string", Desc: "技能正文 Markdown（save 必填）"},
			"description": {Type: "string", Desc: "一句话描述"},
			"category":    {Type: "string", Desc: "分类，如 SEO、写作、运维"},
			// tags 必须是数组：端点 SkillEditRequest.Tags 是 []string，
			// 传逗号分隔字符串会让 ReadJSON 直接失败（invalid request）。
			"tags":   {Type: "array", Items: "string", Desc: "标签数组，如 [\"seo\",\"analysis\"]（save 可选）"},
			"author": {Type: "string", Desc: "作者（save 可选）"},
		},
		Required: []string{"action"},
		Caps:     []string{"skill_list", "skill_get", "skill_reload", "skill_save", "skill_delete"},
		Compose:  switchCompose(map[string]string{"list": "skill_list", "get": "skill_get", "reload": "skill_reload", "save": "skill_save", "delete": "skill_delete"}),
	},
	// ───────────────────────── 智能体域：Agent 调度 ─────────────────────────
	{
		Name: "agent", Title: "管理 AI Agent", Domain: DomainAgent, Risk: RiskWrite,
		Desc: "Agent 与子任务的增删查改/运行，及远程技能检索安装。action: manage_create/manage_list/manage_edit/manage_delete/manage_toggle/manage_run/manage_chat/skill_search/skill_install/task。manage_create 需配 cron 才会定时执行。",
		Params: map[string]ParamSpec{
			"action":     {Type: "string", Desc: "操作", Required: true, Enum: []string{"manage_create", "manage_list", "manage_edit", "manage_delete", "manage_toggle", "manage_run", "manage_chat", "skill_search", "skill_install", "task"}},
			"id":         {Type: "integer", Desc: "Agent ID（除 create 外均必填）"},
			"name":       {Type: "string", Desc: "Agent 名称，如 'GEO 热点关键词日报'（create 必填；edit 传了才改）"},
			"title":      {Type: "string", Desc: "名称，manage_create 时等价于 name"},
			"prompt":     {Type: "string", Desc: "提示词/对话内容"},
			"strategy":   {Type: "string", Desc: "执行策略（create 必填；edit 传了才改）。描述每次执行要做什么：步骤、标准、输出要求。传了 strategy 则优先于 prompt"},
			"cron":       {Type: "string", Desc: "Cron 表达式，如 '0 8 * * *' 每天8点。留空表示仅手动触发（create）；edit 传空字符串可清空"},
			"max_runs":   {Type: "integer", Desc: "最大执行次数，0=不限（create/edit）"},
			"max_rounds": {Type: "integer", Desc: "单次执行最大轮数，0=用默认20（create/edit）"},
			"message":    {Type: "string", Desc: "发送给 Agent 的消息（manage_chat），等价于 prompt"},
			"enabled":    {Type: "integer", Desc: "1=启用(定时执行) 0=暂停（manage_toggle），等价于 status"},
			"status":     {Type: "integer", Desc: "状态 1/0（toggle），等价于 enabled"},
			"query":      {Type: "string", Desc: "技能搜索关键词（skill_search）"},
			"limit":      {Type: "integer", Desc: "搜索结果上限 1-50，默认 10（skill_search）"},
			"slug":       {Type: "string", Desc: "SkillHub 技能 slug，如 find-skills（skill_install）"},
			"force":      {Type: "boolean", Desc: "是否覆盖已安装同名技能（skill_install）"},
			"tasks":      {Type: "array", Items: "object", Desc: "子任务列表（task）。每项字段：description(必填,3-5词标签)、prompt(必填,完整指令)、type(必填,explore只读/worker可写)、scope(可选,worker允许写的文件glob数组)"},
		},
		Required: []string{"action"},
		Caps:     []string{"agent_create", "agent_list", "agent_edit", "agent_delete", "agent_toggle", "agent_run", "agent_chat", "skill_search", "skill_install", "task"},
		Compose:  agentCompose(),
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
		Desc: "通过 HTTP 获取网页内容(fetch) 或在线搜索(search)。action: fetch/search；fetch 需 url，search 需 query。" +
			"仅支持 http/https，禁止访问内网地址。内容超长时只返回前一段，全文会存档为项目文件——" +
			"按结果末尾标记里的路径用 fs_read 的 offset 或 fs_search 续读，不要重复抓取同一个 URL。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"fetch", "search"}},
			"url":    {Type: "string", Desc: "要获取的网页 URL（fetch 必填）"},
			"query":  {Type: "string", Desc: "搜索关键词（search 必填）"},
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
	case "list", "get", "delete":
		capName := map[string]string{
			"list": "archive_list", "get": "archive_get", "delete": "archive_delete",
		}[action]
		sub := withoutAction(args)
		if action == "list" {
			// status 非法值必须在这里挡住，不能透传给端点。
			//
			// 端点 ArchiveList 的 status 语义是「查哪张表」：只有精确等于 "ok"
			// 才查 archives 正式表，**其它任何值**都落到 archive_drafts，且
			// controller/manageController/archive.go:123-131 的 if/else if 链
			// 一个都不匹配 → 不加任何 status 过滤。
			//
			// 实测（2026-10-03）：传 status="1"（AI 很容易把 publish 的数字语义
			// 顺手用到这里）返回的是**全部草稿**（status 0 和 99 混在一起），
			// 而 AI 以为自己在查正式文档。这类「静默返回错数据」比报错危险得多，
			// 因为 AI 无从察觉。
			if err := normalizeListStatus(sub); err != nil {
				return nil, err
			}
			if err := normalizeListOrder(sub); err != nil {
				return nil, err
			}
		} else {
			// get/delete 靠 id 定位，缺 id 时端点会回一句含义不明的
			// 「record not found」（实测），调用方会误以为是文档不存在。
			// 在意图层就说清是参数缺失。
			if toInt64(sub["id"]) <= 0 {
				return nil, fmt.Errorf("action=%s 需要提供 id（文档 ID），当前未传", action)
			}
			if action == "delete" {
				// delete 端点对不存在的 id 也回「文章已删除」，先确认存在。
				if err := requireArchivesExist(ctx, cap, sub["id"]); err != nil {
					return nil, err
				}
			}
			// 列表筛选参数只对 list 有意义。get/save/publish/delete 的端点
			// （/archive/detail、/archive/status、/archive/delete）**不认**
			// page/page_size/category_id/order_by/order_dir/keyword 等，
			// 透传过去会被 ReadJSON 静默忽略——工具「看得见却调不动」，
			// 调用方却以为筛选生效了。这里显式剔除，不依赖端点宽容。
			stripListOnlyParams(sub)
		}
		out, err := callCap(ctx, cap, capName, sub)
		if err != nil {
			return nil, err
		}
		// 解析 JSON 响应，提取 data 字段填充 Result.Data
		res := &Result{Text: out}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err == nil {
			data, hasData := envelope["data"]
			if action == "list" {
				// 命中 0 条时端点回的是 data:null（实测 status=plan 如此）。
				// 直接透传 null 会让调用方分不清「没有数据」和「调用出错」，
				// 形状也不稳定（有时数组、有时 null、有时对象），故统一成
				// {list,total,page,page_size,count}，list 至少是空数组。
				if !hasData || data == nil {
					data = []any{}
				}
				res.Data = listWithTotal(data, envelope, sub)
			} else if action == "delete" {
				// 删除端点成功时内层只回 {"code":0,"msg":"文章已删除","data":null}，
				// 没有任何载荷。外层 api_invoke 的 data 是整个控制器信封，
				// 直接透传出去对调用方毫无信息量（只有 code/msg 两个占位），
				// 还得去解析中文文案才知道删的是哪篇。补一个最小回执。
				//
				// moved_to_trash=true 是必须交代的事实：正式文档删除是**移入回收站**
				//（archive_drafts，status=99），不是物理删除；草稿删除才是真删。
				// 不说清楚，调用方会以为数据已经彻底消失而放弃恢复，
				// 也无法解释「删除后 get 仍能查到」。
				data := map[string]any{
					"action": "delete",
					"id":     toInt64(sub["id"]),
				}
				if !isDraftRequest(args) && !isTrashStatus(sub["status"]) {
					data["moved_to_trash"] = true
				}
				res.Data = data
			} else if hasData && data != nil {
				res.Data = data
			}
		}
		return res, nil
	case "publish":
		// 端点是 ArchiveStatusRequest{Ids []int64, Status uint}：
		// status 要 uint，传 "ok"/"draft" 这类字符串会 ReadJSON 失败。
		// 实测踩过：直接透传 status="draft" 报
		// 「json: cannot unmarshal string into Go struct field ArchiveStatusRequest.status of type uint」。
		// id→ids 的换算与数组包装由 capEndpoints 的 Rename/Arrays 统一处理，这里不重复。
		sub := withoutAction(args)
		if toInt64(sub["id"]) <= 0 {
			return nil, fmt.Errorf("action=publish 需要提供 id（文档 ID），当前未传")
		}
		// /archive/status 只认 {ids, status}，其余一律剔除（理由同 stripListOnlyParams）。
		stripListOnlyParams(sub)
		if v, ok := sub["status"]; ok {
			sub["status"] = articleStatusCode(v)
		}
		// 端点对不存在的 id 也会回成功，必须先确认目标存在。
		// 见 requireArchivesExist 的注释。
		if err := requireArchivesExist(ctx, cap, sub["id"]); err != nil {
			return nil, err
		}
		out, err := callCap(ctx, cap, "archive_publish", sub)
		if err != nil {
			return nil, err
		}
		if msg := endpointFailure(out); msg != "" {
			return nil, fmt.Errorf("发布状态变更失败：%s", msg)
		}
		data := map[string]any{
			"action": "publish",
			"id":     toInt64(sub["id"]),
		}
		if v, ok := sub["status"]; ok {
			// sub["status"] 已被 articleStatusCode 换成端点要的 uint（1=上架 0=下架），
			// 直接回传会让 AI 看到裸数字 1/0 —— 这里的契约是对外语义（ok/draft），
			// 与 save 的 data["status"] 保持一致。数值另存 status_code 便于排查。
			code := toInt64(v)
			data["status_code"] = code
			if code == 1 {
				data["status"] = "ok"
			} else {
				data["status"] = "draft"
			}
		} else {
			// 不传 status 时端点保持原状态，回执不能编一个。
			data["status"] = "unchanged"
		}
		return &Result{Text: out, Data: data}, nil
	case "save":
		// 两个能力都映射到 POST /archive/detail（端点靠 id 是否为空区分新建与更新）。
		id := toInt64(args["id"])
		capName := "archive_update"
		if id == 0 {
			capName = "archive_create"
		}
		sub := withoutAction(args)
		// /archive/detail 认 category_id/module_id/parent_id/keyword/status 等，
		// 但**不认**分页与排序（page/page_size/order_by/order_dir）——
		// 透传过去会被静默忽略，调用方却以为分页或排序生效了。
		// 只剔真正无交集的那几个，别把 category_id 这类共用的误删。
		for _, k := range []string{"page", "page_size", "order_by", "order_dir"} {
			delete(sub, k)
		}
		if id != 0 {
			// 无需补齐未传字段：archive_update 已注入 partial=true，
			// 端点走 PATCH 语义，只覆盖显式传入的字段。
		} else {
			// 新建时 title 与正文都必填，缺了端点要么报「未定义模型」要么建出空文。
			if t, _ := sub["title"].(string); strings.TrimSpace(t) == "" {
				return nil, fmt.Errorf("action=save 新建文档需要提供 title（标题）")
			}
			if c, _ := sub["content"].(string); strings.TrimSpace(c) == "" {
				return nil, fmt.Errorf("action=save 新建文档需要提供 content（正文）")
			}
		}
		out, err := callCap(ctx, cap, capName, sub)
		if err != nil {
			return nil, err
		}
		// 端点业务失败时返回的是 {"ok":false,...} 而不是 Go error，callCap 不会报错。
		// 不拦住的话回执会写成 {"ok":true,"data":{"id":N}}，AI 会认为保存成功 ——
		// 实测踩过：更新 module_id 为空的文章，端点回「未定义模型」且什么都没改。
		if msg := endpointFailure(out); msg != "" {
			return nil, fmt.Errorf("保存失败：%s（未做任何修改）", msg)
		}
		// 解析端点返回的文档对象，回填结构化回执（id/title/link/url/status）。
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
				data["link"] = draftAwareLink(v, art, args)
				data["url"] = data["link"]
			}
			if v, ok := art["status"]; ok {
				data["status"] = v
			}
		}
		// 端点返回的文档对象里不一定带 status（更新正式文档时 archives 表用
		// `1 as status` 伪造列、草稿分支则原样返回 drafts.status，两条路径都可能缺）。
		// 回执缺 status 会让调用方无法判断这篇到底是草稿还是已发布——
		// 实测踩过：save 更新后 sc.data 只有 action/id/link/title。
		// 这里按请求语义兜底：draft=true 一定是草稿，否则是正式文档。
		if _, ok := data["status"]; !ok {
			if isDraftRequest(args) {
				data["status"] = "draft"
			} else {
				data["status"] = "ok"
			}
		}
		// 曾这里有一段「草稿新建时端点只回 data:null，故按标题回查补 id/link」的兜底，
		// 已在 2026-10-03 删除——那段注释与实际行为不符：
		//   - controller/manageController/archive.go:666 的 ArchiveDetailForm 只有**一条**响应分支，
		//     草稿与正式完全相同，都是 {"code":0,"msg":"文档已更新","data":archive}；
		//   - provider.SaveArchive 末尾（archive.go:713）无条件 `draft.Link = w.GetUrl(...)`，
		//     返回的 draft 自带 link，id 也一并带上；
		//   - 上面的通用解析因此对草稿/正式走同一条路径，实测草稿同样能拿到
		//     id 与 link（只是被 draftAwareLink 加上 ?preview=true）。
		// 兜底不仅多余，还会在端点确实没回 link 时用「按标题模糊回查」引入误报风险
		// （同前缀的不同文档可能被认成刚写的那篇）。
		return &Result{Text: out, Data: data}, nil
	default:
		return nil, fmt.Errorf("不支持的操作: %s（可选: list/get/save/publish/delete）", action)
	}
}

// listWithTotal 把列表载荷与分页信息合成一个对象返回。
//
// 背景：ArchiveList 的响应是 {"code","msg","total","exact","data":[...]}，
// total 在信封顶层而不在 data 里。结构化回执过去只取 data，于是调用方
// 拿到一个裸数组——既不知道命中总数，也无法判断还有没有下一页。
//
// page/page_size 只能从请求参数取：端点响应里没有回显（ArchiveList 只回
// code/msg/total/exact/data），而 capEndpoints 又把 page→current、page_size→pageSize
// 重命名过，sub 里保留的是**意图层原始参数名**。
//
// 合成 {list,total,page,page_size} 而不是裸数组，是因为裸数组在
// structuredObject 里会被包成 data 数组，分页信息没有落点；
// 而 MCP 契约要求 StructuredContent 顶层是对象，列表项放 data.list 正好。
func listWithTotal(data any, envelope map[string]any, req map[string]any) any {
	total, hasTotal := envelope["total"]
	if !hasTotal {
		// 端点没给 total（如某些全量返回），保持裸数组不硬造一个假数字。
		return data
	}
	page := toInt64(req["page"])
	if page <= 0 {
		page = 1
	}
	pageSize := toInt64(req["page_size"])
	if pageSize <= 0 {
		pageSize = 20
	}
	out := map[string]any{
		"list":      data,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}
	// 命中数与是否精确统计，来自端点的 exact 字段（total 可能是估算值）。
	if lst, ok := data.([]any); ok {
		out["count"] = len(lst)
	}
	if e, ok := envelope["exact"]; ok {
		out["exact"] = e
	}
	return out
}

// normalizeListStatus 校验并归一 list 的 status 参数。
//
// 端点 ArchiveList 的 status 语义（controller/manageController/archive.go:59-62、123-131）：
//   - "ok"     → 查 archives 正式表（表无 status 列，恒为已发布）
//   - "draft"  → 查 archive_drafts 且 status=0（草稿）
//   - "plan"   → 查 archive_drafts 且 status=99（待发布）
//   - "delete" → 查 archive_drafts 且 status=ContentStatusDelete（**回收站**）
//   - 其它值    → 落到 archive_drafts 且不加任何 status 过滤，返回混合数据
//
// 最后这条是必须挡住的：静默返回一堆无关数据比报错危险得多，AI 无从察觉。
// delete 是端点真实支持的值（DeleteArchive 把正式文档移到 archive_drafts
// status=99），不加进来的话 AI 删完就再也找不到这篇文档，无法确认也无法恢复。
func normalizeListStatus(sub map[string]any) error {
	raw, ok := sub["status"]
	if !ok || raw == nil {
		return nil
	}
	var key string
	switch v := raw.(type) {
	case string:
		key = strings.ToLower(strings.TrimSpace(v))
	case bool:
		// draft=true/false 是 save/publish 的写法，list 这里也接受一下。
		if v {
			key = "draft"
		} else {
			key = "ok"
		}
	default:
		key = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	}
	switch key {
	case "":
		// 显式传空串等于不过滤，删掉让端点用默认 ok。
		delete(sub, "status")
		return nil
	case "ok", "1", "publish", "published", "release", "online":
		sub["status"] = "ok"
	case "draft", "0":
		sub["status"] = "draft"
	case "plan":
		sub["status"] = "plan"
	case "delete", "trash", "recycle", "recycled", "deleted":
		// 回收站：与端点的 recycle=true 等价，端点内部就是置 status="delete"。
		sub["status"] = "delete"
	default:
		return fmt.Errorf(
			"status 取值非法：%q。list 只支持 ok（正式文档）/draft（草稿）/plan（待发布）/"+
				"delete（回收站）；要上架或下架请用 action=publish", key)
	}
	return nil
}

// listOnlyParams 是只在 action=list 时有意义的参数。
//
// 它们的落点端点（/archive/detail、/archive/status、/archive/delete）
// 根本不认这些字段：ReadJSON 会静默丢弃，调用方却以为筛选/分页生效了。
// 意图层显式剔除，不依赖端点宽容——「看得见却调不动」比报错更难排查。
//
// 注意 status 不在其中：list 用它过滤、publish 用它做状态变更、save 也会读它
// （草稿标记），三个 action 都有意义。
var listOnlyParams = []string{
	"page", "page_size",
	"category_id", "module_id", "parent_id",
	"keyword", "flag",
	"order_by", "order_dir",
}

// stripListOnlyParams 就地删除列表专用参数。
func stripListOnlyParams(sub map[string]any) {
	for _, k := range listOnlyParams {
		delete(sub, k)
	}
}

// requireArchivesExist 在改状态/删文档前确认目标真的存在。
//
// 为什么必须查：这两个端点都用 GORM 的 Find 批量查（provider/archive.go:1425
// 与 ArchiveDelete 内部同构），**查不到不是错误**——Find 得到空切片、error 为 nil，
// for 循环零次执行，控制器照样回 {"code":0,"msg":"文章已更新/已删除"}。
//
// 实测（2026-10-03）：publish id=999999 与 delete id=999999 都回
// ok:true「文章已更新」/「文章已删除」，但库里根本没有这行。
// AI 会据此认为操作已生效并继续往下走（比如接着改同一篇的其它字段），
// 实际什么都没发生——这类假成功比报错难发现得多。
//
// 用 archive_get 判定：它对不存在的 id 回 ok:false「record not found」，
// 正好是权威的存在性判断，且与写操作读同一套模型（草稿表优先）。
//
// 只把「明确的 not found」当不存在：5xx / 网络异常一律放行。
// 把端点临时故障误报成「文档不存在」比不检查更糟——它会让一次本可成功的
// 操作凭空失败，而真实原因（网络抖动）被完全掩盖。
func requireArchivesExist(ctx context.Context, cap CapInvoker, idAny any) error {
	id := toInt64(idAny)
	if id <= 0 {
		return nil
	}
	out, err := callCap(ctx, cap, "archive_get", map[string]any{"id": id})
	if err != nil {
		return nil // 读不通就放行，让写操作自己说话
	}
	if parseArticle(out) == nil {
		// 只有端点明确说「找不到」才认定不存在。
		if !isNotFoundResponse(out) {
			return nil
		}
		return fmt.Errorf("文档 id=%d 不存在，未做任何修改", id)
	}
	return nil
}

// isNotFoundResponse 判断端点响应是否明确表示「记录不存在」。
//
// GORM 的 ErrRecordDisabled / gorm.ErrRecordNotFound 都会被控制器原样放进 msg，
// 文本是 "record not found"。只认这一种，不做模糊匹配——把「暂时查不到」
// 误判成「已删除」会产生比不检查更坏的副作用。
func isNotFoundResponse(out string) bool {
	if out == "" {
		return false
	}
	var j map[string]any
	if err := json.Unmarshal([]byte(out), &j); err != nil {
		return false
	}
	for _, key := range []string{"msg", "hint"} {
		if m, ok := j[key].(string); ok &&
			strings.Contains(strings.ToLower(m), "record not found") {
			return true
		}
	}
	if d, ok := j["data"].(map[string]any); ok {
		if m, ok := d["msg"].(string); ok &&
			strings.Contains(strings.ToLower(m), "record not found") {
			return true
		}
	}
	return false
}

// archiveOrderColumns 是 archive_list 允许的排序字段白名单。
//
// 端点把 order_by 重命名为 sort（capEndpoints["archive_list"].Rename），
// controller/manageController/archive.go:46-57 读 sort 后拼成
// `archives.<sort> <order>` 交给 provider.ParseOrderBy。
// ParseOrderBy 只做**词法**安全校验（orderColumnRegex 匹配即放行），
// 不校验列是否真实存在——列不存在时它照样返回 "archives.xxx desc"，
// MySQL 报 Unknown column，而控制器 Find 的 error 被丢弃，
// 于是 list 回 ok=true + 空列表 + total=1856。
//
// 实测（2026-10-03）：order_by="nonexistent_col" 返回
// {"ok":true,"data":{"list":[],"total":1856}}。AI 看到 ok=true 会认为
// 「筛选条件太窄所以没数据」，从而得出完全错误的结论——
// 这类静默错数据比报错危险得多，所以必须在意图层挡住。
var archiveOrderColumns = map[string]bool{
	"id": true, "created_time": true, "updated_time": true,
	"title": true, "sort": true, "views": true,
	"comment_count": true, "favorite_count": true, "stock": true,
	"user_id": true, "module_id": true, "category_id": true,
	"price": true, "read_level": true,
}

// archiveOrderColumnList 返回 order_by 在 schema 里**推荐**的常用排序列，供 ParamSpec.Enum 使用。
//
// 与 archiveOrderColumns 白名单的关系是「推荐 ⊂ 允许」，不是相等：
// enum 只给 AI 看，列出日常排序真正用得上的几列（按时间/热度/权重/主键）；
// normalizeListOrder 仍按全量白名单校验，因此 enum 之外的真实列（如 title、
// comment_count）照常可用，只是 schema 不再逐条铺开。
//
// 为什么 enum 只列推荐集而不是白名单全量（2026-10-05）：
//
// ⚠️ **省 token 是次要理由，不要拿它当主要论据**。实测 enum 从 14 列缩到
// 5 列省约 28 token，但为了让 desc 说清「这是推荐而非仅限」（措辞由
// 「只能是」改为「推荐用…也可填…」多 71 字符），净收益仅 **3 token**。
// enum 值的 token 成本本就远低于直觉——它是纯 ASCII 列名，1 token 能装
// 3~4 个字符，14 个列名加起来才百来字符。
//
// 真正的收益是**选择质量**：enum 是 AI 排列表时的候选集，铺开 14 列会让
// 它在 user_id/stock/price/read_level 这些 0 值常量列之间反复权衡——
// 按这些列排序出来的顺序对调用方是无意义噪声，而 module_id/category_id
// 的正确用法是当过滤条件而非排序键。收窄后它面对的是 5 个真正可用的键。
//
// 注意「不推荐」不等于「禁用」：白名单里那 9 列（title、comment_count、
// favorite_count、stock、user_id、module_id、category_id、price、
// read_level）在 normalizeListOrder 里全部合法，填了照常工作。
// TestListAcceptsRealOrderColumns 遍历白名单全量来钉住这一点——
// 收窄 enum 最容易犯的错是顺手把校验也收窄成推荐集，那会让 9 个真实
// 列凭空不可用且无任何报错。
//
// 必须**排序**：map 迭代顺序随机，直接展开会让每次 tools/list 响应里
// enum 顺序都不一样，依赖它的客户端（含 provider 的穷举门禁
// TestIntentParamsRecognizedByEndpoints）会拿到不稳定的值。
func archiveOrderColumnList() []string {
	out := make([]string, 0, len(archiveOrderRecommended))
	for c := range archiveOrderRecommended {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// archiveOrderRecommended 是 order_by 愿意在 schema 里推荐给 AI 的排序列。
//
// 独立于 archiveOrderColumns 声明，是为了让「推荐什么」与「允许什么」
// 分属两处、可以各自独立调整：收紧推荐不该顺手放松安全校验，
// 放宽白名单也不该逼着 schema 铺开一堆用不上的列。
var archiveOrderRecommended = map[string]bool{
	"id": true, "created_time": true, "updated_time": true,
	"views": true, "sort": true,
}

// archiveOrderColumnNames 返回白名单**全量**列名（字典序），用于报错文案。
//
// 报错必须列全量而非只列推荐值：AI 拿到"非法值"提示时需要知道
// 到底哪些值能用，否则会误以为推荐值就是全部，撞了白名单里
// 真实存在却不推荐的列（如 title）时无从修正。
func archiveOrderColumnNames() []string {
	out := make([]string, 0, len(archiveOrderColumns))
	for c := range archiveOrderColumns {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// normalizeListOrder 校验 order_by / order_dir，非法值直接报错。
//
// 只校验不纠正：sort 是有业务含义的字段名，猜不出调用方想按什么排。
// order_dir 端点自己会归一（order != "asc" → desc），这里提前拦是为了
// 给出可读的错误，而不是让它静默变成 desc。
func normalizeListOrder(sub map[string]any) error {
	if raw, ok := sub["order_by"]; ok && raw != nil {
		key := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
		if key != "" && !archiveOrderColumns[key] {
			// 这里列**全量白名单**而不是 archiveOrderColumnList() 的推荐子集：
			// 调用方撞墙时要看到全部合法值，否则会误以为推荐值即全部，
			// 碰到真实存在却不推荐的列（如 title）时不知道该怎么改。
			return fmt.Errorf(
				"order_by 取值非法：%q。可用字段：%s（端点按 archives 表的列名排序，不存在的列会让查询报错并返回空列表）",
				key, strings.Join(archiveOrderColumnNames(), "、"))
		}
	}
	if raw, ok := sub["order_dir"]; ok && raw != nil {
		key := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
		switch key {
		case "", "asc", "desc":
		default:
			return fmt.Errorf("order_dir 取值非法：%q。只支持 asc 或 desc", key)
		}
	}
	return nil
}

// isTrashStatus 判断调用方是否在查/操作回收站态。
// archive_drafts 的 status=99 表示已删除进回收站（provider.DeleteArchive）。
func isTrashStatus(v any) bool {
	switch s := v.(type) {
	case float64:
		return s == 99
	case int:
		return s == 99
	case int64:
		return s == 99
	case string:
		return strings.EqualFold(strings.TrimSpace(s), "delete")
	}
	return false
}

// digList 从信封里取出文档数组，兼容 data/list/items 三种常见位置。
func digList(v any) []any {
	var cur any = v
	for i := 0; i < 4; i++ {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		for _, key := range []string{"data", "list", "items"} {
			if sub, ok := m[key]; ok {
				if arr, ok := sub.([]any); ok {
					return arr
				}
				cur = sub
				break
			}
		}
	}
	return nil
}

// draftAwareLink 给正式文档 link 补上可访问地址：草稿需追加 ?preview=true。
//
// 背景：content_article save 过去只回 id，调用方拿不到链接，无法直接查看刚写入的内容。
// 端点返回的 art.link 是正式文档地址，但草稿在 archives 表中不存在（archives 无 status 列，
// 草稿状态存在 archive_drafts 里，取值 0/99），前台路由需要 ?preview=true 才能读到草稿
// （见 controller/archive.go:24 的 preview 分支）。
//
// 判据两条，任一成立即视为草稿：
//  1. 调用方显式传了 draft=true；
//  2. 端点回传的 status 为草稿态（0）。
func draftAwareLink(link string, art map[string]any, args map[string]any) string {
	if link == "" {
		return link
	}
	if isDraftRequest(args) || isDraftStatus(art["status"]) {
		if strings.Contains(link, "preview=") {
			return link
		}
		sep := "?"
		if strings.Contains(link, "?") {
			sep = "&"
		}
		return link + sep + "preview=true"
	}
	return link
}

// isDraftRequest 判断调用方是否显式要求存草稿。
func isDraftRequest(args map[string]any) bool {
	switch v := args["draft"].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	}
	return false
}

// isDraftStatus 判断端点回传的 status 是否为「不可直接访问」的非正式态。
//
// archives 表本身没有 status 列，已发布文档不存在 status 字段；草稿态的 0 与
// 待发布态的 99 都来自 archive_drafts，前台路由都需要 ?preview=true 才能读到
// （见 controller/archive.go:24 的 preview 分支）。
// status 缺省（nil）视为已发布，不加后缀。
func isDraftStatus(v any) bool {
	switch s := v.(type) {
	case float64:
		return s == 0 || s == 99
	case int:
		return s == 0 || s == 99
	case int64:
		return s == 0 || s == 99
	}
	return false
}

// mediaCompose 直接委托 switchCompose。
// attachment_upload 现已返回标准 JSON 信封（含 data 字段），switchCompose 会自动提取，
// 不再需要从纯文本中抠 id 的 workaround。
func mediaCompose(routes map[string]string) Compose {
	return switchCompose(routes)
}

// agentCompose 把意图参数换算成底层 agent_* handler 认识的字段名。
//
// 背景：底层 agent_create/agent_chat 是 provider/aiTools.go 里的内存能力，
// 参数名是 name/strategy/message/enabled；意图层为了对外统一用了 title/prompt/status。
// 两者语义一致但字段名不同，且 switchCompose 只做 action 分派、不做字段改名，
// 于是 agent_create 收到 args 里没有 name/strategy，直接报「名称和策略不能为空」。
//
// 这里同时补上 cron/max_runs/max_rounds —— 意图层原先根本没暴露这三个参数，
// 导致「每天定时执行」这类需求在 MCP 侧无法表达（Agent 只能手动触发）。
func agentCompose() Compose {
	routes := map[string]string{
		"manage_create": "agent_create", "manage_list": "agent_list", "manage_edit": "agent_edit",
		"manage_delete": "agent_delete", "manage_toggle": "agent_toggle", "manage_run": "agent_run",
		"manage_chat":  "agent_chat",
		"skill_search": "skill_search", "skill_install": "skill_install",
		"task": "task",
	}
	// 别名 → 底层字段名，按 action 分别声明。
	//
	// 必须分 action 而不能用一张全局表：prompt 同时是 agent_create 的 strategy 别名
	// 和 agent_chat 的 message 别名，全局映射会让 strategy 先抢走 prompt 并把它删掉，
	// 导致 manage_chat 收不到消息（TestAgentComposeChatUsesMessage 抓到过）。
	// 同一目标出现多个别名时按顺序取第一个非空值。
	aliasByAction := map[string]map[string][]string{
		"manage_create": {"name": {"name", "title"}, "strategy": {"strategy", "prompt"}},
		"manage_edit":   {"name": {"name", "title"}, "strategy": {"strategy", "prompt"}},
		"manage_chat":   {"message": {"message", "prompt"}},
		"manage_toggle": {"enabled": {"enabled", "status"}},
	}
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		capName, ok := routes[action]
		if !ok {
			return nil, fmt.Errorf("不支持的操作: %s（可选: %v）", action, keysOf(routes))
		}
		sub := withoutAction(args)
		// 命中后把来源字段删掉，避免底层 handler 收到它不认识的键。
		for dst, srcs := range aliasByAction[action] {
			if _, exists := sub[dst]; exists {
				continue
			}
			for _, src := range srcs {
				if v, ok := sub[src]; ok {
					sub[dst] = v
					if src != dst {
						delete(sub, src)
					}
					break
				}
			}
		}
		return capResult(callCap(ctx, cap, capName, sub))
	}
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
		target := map[string]string{
			"storage_get":    "GET /plugin/storage",
			"storage_save":   "POST /plugin/storage",
			"storage_upload": "POST /plugin/storage/upload",
		}[action]
		return capResult(endpointInvoke(ctx, cap, target, args, nil))
	default:
		return nil, fmt.Errorf("不支持的操作: %s（可选: setting/site_info/site_version/site_anqi/storage_get/storage_save/storage_upload/template_info/template_reload）", action)
	}
}

func init() {
	IntentCatalog = append(IntentCatalog, mergedIntentCatalog...)
}

// endpointInvoke 直接以 api_invoke 能力执行一个后台端点（target 形如 "METHOD PATH"）。
// 用于合并意图里"端点类子操作"的分派，与 invokeRoutes 行为一致但不绑定单一 action 字段。
// 它是裸路由的统一出口：参数一律经 endpointParams 整形（去 action、展开 values、
// 换算分页与各意图自己的改名），否则各 Compose 自己拼参数就会各自漏掉其中一步。
func endpointInvoke(ctx context.Context, cap CapInvoker, target string, args map[string]any, renames map[string]string) (string, error) {
	parts := strings.Fields(target)
	if len(parts) != 2 {
		return "", fmt.Errorf("端点声明格式错误（应为 \"METHOD PATH\"）: %q", target)
	}
	return cap(ctx, "api_invoke", map[string]any{
		"method": parts[0], "path": normalizeEndpointPath(parts[1]), "params": endpointParams(args, renames),
	})
}

// seoCompose 合并 seo_sitemap + seo_robots + seo_push：
// sitemap 走 cap(sitemap_rebuild+url_push)，robots/push 走端点，均经 api_invoke。
func seoCompose() Compose {
	routes := map[string]string{
		"robots_get":  "GET /plugin/robots",
		"robots_save": "POST /plugin/robots",
		"push_get":    "GET /plugin/push",
		"push_save":   "POST /plugin/push",
		"push_push":   "POST /plugin/push/push",
		"push_logs":   "GET /plugin/push/logs",
	}
	registerIntentRoutes("seo", routes)
	// robots.txt 的正文在端点侧叫 robots（request.PluginRobotsConfig.Robots），
	// 意图对外沿用更直白的 content；不换算就会被 ReadJSON 忽略，
	// 表现为"保存成功"却把 robots.txt 写空。
	renames := map[string]string{"content": "robots"}
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		switch action {
		case "sitemap":
			// 端点是 PluginSitemapConfig，type 为空会拼出畸形路径 /sitemap.
			// 并在生成阶段报 500（实测：不传任何参数时 status=500、msg 为空）。
			// 这里给个合法默认值，调用方仍可通过 values.type 覆盖。
			rebuildArgs := map[string]any{"type": "xml"}
			if v, ok := args["values"]; ok {
				if vm, ok := v.(map[string]any); ok {
					if t, has := vm["type"]; has {
						rebuildArgs["type"] = t
					}
					if ab, has := vm["auto_build"]; has {
						rebuildArgs["auto_build"] = ab
					}
				}
			}
			out1, err := callCap(ctx, cap, "sitemap_rebuild", rebuildArgs)
			if err != nil {
				return nil, err
			}
			if msg := endpointFailure(out1); msg != "" {
				return nil, fmt.Errorf("sitemap 重建失败：%s", msg)
			}
			out2, err := callCap(ctx, cap, "url_push", map[string]any{"urls": args["urls"]})
			if err != nil {
				return nil, err
			}
			return &Result{Text: out1 + "\n" + out2}, nil
		case "robots_get", "robots_save", "push_get", "push_save", "push_push", "push_logs":
			return capResult(endpointInvoke(ctx, cap, routes[action], args, renames))
		default:
			return nil, fmt.Errorf("不支持的操作: %s（可选: robots_get/robots_save/sitemap/push_get/push_save/push_push/push_logs）", action)
		}
	}
}

// interactionCompose 合并 interaction_comment + interaction_guestbook：
// comment_* 与 guestbook_list 走 cap（均有等价端点 → api_invoke），其余 guestbook_* 走端点。
func interactionCompose() Compose {
	guestbookEndpoints := map[string]string{
		"guestbook_detail":       "GET /plugin/guestbook/detail",
		"guestbook_setting_get":  "GET /plugin/guestbook/setting",
		"guestbook_setting_save": "POST /plugin/guestbook/setting",
		"guestbook_status":       "POST /plugin/guestbook/status",
		"guestbook_delete":       "POST /plugin/guestbook/delete",
		"guestbook_export":       "POST /plugin/guestbook/export",
	}
	registerIntentRoutes("interaction", guestbookEndpoints)
	commentCaps := map[string]string{
		"comment_list":    "comment_list",
		"comment_approve": "comment_approve",
		"comment_delete":  "comment_delete",
		"guestbook_list":  "guestbook_list",
	}
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		// 两条分支都先经 endpointParams：cap 分支要展开 values，
		// 端点分支还要它把分页名换算成 current/pageSize。
		sub := endpointParams(args, nil)
		if capName, ok := commentCaps[action]; ok {
			return capResult(callCap(ctx, cap, capName, sub))
		}
		if target, ok := guestbookEndpoints[action]; ok {
			return capResult(endpointInvoke(ctx, cap, target, args, nil))
		}
		return nil, fmt.Errorf("不支持的操作: %s（可选: comment_list/comment_approve/comment_delete/guestbook_list/guestbook_detail/guestbook_setting_get/guestbook_setting_save/guestbook_status/guestbook_delete/guestbook_export）", action)
	}
}
