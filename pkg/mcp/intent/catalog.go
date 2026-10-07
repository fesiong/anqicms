package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// IntentCatalog 是"能力表达清单"的声明式载体：每个意图对外是一个工作流工具，
// 内部通过 Caps 指向底层能力（原 endpoint 工具），由 Compose 编排。
// 新增/裁剪工具只改这份数据 + 对应 Compose，不再散落在 105 个匿名闭包里。
//
// 注：内容/素材/结构/技能/Agent/系统配置等组合工具已合并到 catalog_merged.go，
// 端点类补齐域意图在 catalog_domains.go，通用调用意图在 catalog_api.go。本文件仅保留
// 未参与合并的精选意图与所有共享 helper。
var IntentCatalog = []*IntentSpec{
	// ───────────────────────── SEO 域 ─────────────────────────
	{
		Name: "seo_keyword", Title: "管理关键词", Domain: DomainSeo, Risk: RiskWrite,
		Desc: "SEO 关键词的增删查。action: list/create/delete。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "create", "delete"}},
			"id":        {Type: "integer", Desc: "关键词 ID"},
			"title":     {Type: "string", Desc: "关键词"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（list）", Default: 20},
		},
		Required: []string{"action"},
		Caps:     []string{"keyword_list", "keyword_create", "keyword_delete"},
		Compose:  switchCompose(map[string]string{"list": "keyword_list", "create": "keyword_create", "delete": "keyword_delete"}),
	},
	{
		Name: "seo_anchor", Title: "管理锚文本", Domain: DomainSeo, Risk: RiskWrite,
		Desc: "SEO 锚文本的增删查。action: list/create/delete。",
		Params: map[string]ParamSpec{
			"action":    {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "create", "delete"}},
			"id":        {Type: "integer", Desc: "锚文本 ID"},
			"title":     {Type: "string", Desc: "锚文本"},
			"link":      {Type: "string", Desc: "链接"},
			"page":      {Type: "integer", Desc: "页码，从 1 开始（list）", Default: 1},
			"page_size": {Type: "integer", Desc: "每页条数（list）", Default: 20},
		},
		Required: []string{"action"},
		Caps:     []string{"anchor_list", "anchor_create", "anchor_delete"},
		Compose:  switchCompose(map[string]string{"list": "anchor_list", "create": "anchor_create", "delete": "anchor_delete"}),
	},
	{
		// 统计属数据域而非推广域：它只读取观测数据，不产生任何对外可见的 SEO 动作。
		Name: "traffic_statistics", Title: "查看统计数据", Domain: DomainTraffic, Risk: RiskRead,
		Desc: "站点统计。action: dashboard(概览)/spider(蜘蛛)/traffic(流量)。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"dashboard", "spider", "traffic"}},
		},
		Required: []string{"action"},
		Caps:     []string{"statistic_dashboard", "statistic_spider", "statistic_traffic"},
		Compose:  switchCompose(map[string]string{"dashboard": "statistic_dashboard", "spider": "statistic_spider", "traffic": "statistic_traffic"}),
	},

	// ───────────────────────── 系统域（插件/维护，未合并）─────────────────────────
	{
		Name: "system_plugin", Title: "管理插件与维护", Domain: DomainSystem, Risk: RiskSystem,
		// 2026-10-07：robots_get/robots_set 已迁到 siteops_maintain。迁移动机是**域归属**：
		//   robots 端点在 domain.go 里 ns=plugin/robots 归 DomainSeo，
		//   放在 system 域与域映射自相矛盾（工具 desc 声明的域与端点实际所属域不一致）。
		//   迁到 siteops_maintain 后统一走 invokeRoutes 直落端点，
		//   不再依赖 capEndpoints 里的 plugin_robots_* 回落（那两条记录保留，
		//   cap 表是端点真相源，其他调用路径仍可能命中）。
		// 剩下的四个动作全是主机级/全局动作，误操作后果都超出「改回来」范围：
		//   - htmlcache_build / fulltext_rebuild：触发全站重活；
		//   - backup_dump：导出整站数据；
		//   - migrate_db：改表结构，不可回滚。
		// 故整条意图默认关闭，需 ExposedIntents 显式开启。
		Desc:       "插件级主机维护。（已放开），需 ExposedIntents 显式开启。action: htmlcache_build/fulltext_rebuild/backup_dump/migrate_db。",
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"htmlcache_build", "fulltext_rebuild", "backup_dump", "migrate_db"}},
		},
		Required: []string{"action"},
		Caps:     []string{"plugin_htmlcache_build", "plugin_fulltext_rebuild", "plugin_backup_dump", "setting_migrate_db"},
		Compose: switchCompose(map[string]string{
			"htmlcache_build": "plugin_htmlcache_build", "fulltext_rebuild": "plugin_fulltext_rebuild",
			"backup_dump": "plugin_backup_dump", "migrate_db": "setting_migrate_db",
		}),
	},

	// ───────────────────────── 内置域（主机级能力）─────────────────────────
	//
	// 这 9 个意图是 read_file / write_file / edit_file / search_replace / grep / glob /
	// list_directory / bash / web_fetch / web_search 这些内置 cap 的**唯一模型面出口**：
	// aiChat 与 MCP 都只绑定意图，cap 名不再作为工具出现在模型面。
	// 因此这里必须把 cap 的入参契约完整搬过来（含分段读取、批量续扫这类操作规则），
	// 否则"收掉重复工具名"就变成了"能力缩水"。
	// provider 侧 TestBuiltinIntentCarriesCapContract 用反射逐字段核对参数名，
	// 并校验每个内置 cap 都有对应意图。
	{
		Name: "shell_exec", Title: "执行 Shell 命令", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "在项目根目录执行 shell 命令，用于运行构建、测试、代码生成等开发命令。" +
			"不能使用交互式命令；临时生成的脚本与输出文件写到 cache/ 目录。",
		Params: map[string]ParamSpec{
			"command": {Type: "string", Desc: "要执行的 shell 命令", Required: true},
			"timeout": {Type: "integer", Desc: "超时时间（秒），默认 30，最大 120"},
		},
		Required: []string{"command"},
		Caps:     []string{"bash"},
		Compose:  Delegate("bash"),
	},
	{
		Name: "fs_read", Title: "读取文件", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "读取项目内文本文件的内容。offset（起始行号，从 1 开始）与 limit（最大行数）可分段读取大文件；" +
			"超过 300 行的 Go 文件会先返回骨架结构。结果末尾会写明本次实际返回的行区间，" +
			"若仍有剩余就按提示传 offset 续读，不要重复同一次调用。只能读项目目录内的文件。",
		Params: map[string]ParamSpec{
			"path":   {Type: "string", Desc: "文件路径，相对项目根目录或绝对路径", Required: true},
			"offset": {Type: "integer", Desc: "起始行号（从 1 开始），可选"},
			"limit":  {Type: "integer", Desc: "最大读取行数，可选；实际返回还受单次结果大小限制，以尾部说明为准"},
		},
		Required: []string{"path"},
		Caps:     []string{"read_file"},
		Compose:  Delegate("read_file"),
	},
	{
		Name: "fs_write", Title: "写入文件", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "写入或创建文件，已存在则整体覆盖，自动创建父目录。只能操作项目目录内的文件；" +
			"临时脚本（py/sh 等）请写入 cache/ 目录。修改模板后需再用 system_config 的 template_reload " +
			"重载才会生效。",
		Params: map[string]ParamSpec{
			"path":    {Type: "string", Desc: "文件路径，相对项目根目录或绝对路径", Required: true},
			"content": {Type: "string", Desc: "文件内容", Required: true},
			"confirm": {Type: "boolean", Desc: "目标带警告（如覆盖非空文件）时，须传 true 确认写入"},
		},
		Required: []string{"path", "content"},
		Caps:     []string{"write_file"},
		Compose:  Delegate("write_file"),
	},
	{
		Name: "fs_edit", Title: "编辑文件", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "编辑文件内容，两种模式：文本模式传 old_string/new_string 做精确替换；" +
			"行模式只给 start_line/end_line 和 new_string，整段替换这些行。" +
			"old_string 留空即按行模式处理。修改模板后需再用 system_config 的 template_reload 重载才会生效。",
		Params: map[string]ParamSpec{
			"path":       {Type: "string", Desc: "文件路径", Required: true},
			"old_string": {Type: "string", Desc: "（文本模式）要替换的旧文本"},
			"new_string": {Type: "string", Desc: "替换后的新文本"},
			"start_line": {Type: "integer", Desc: "（行模式）起始行号（从 1 开始）"},
			"end_line":   {Type: "integer", Desc: "（行模式）结束行号（从 1 开始），默认等于 start_line"},
		},
		Required: []string{"path", "new_string"},
		Caps:     []string{"edit_file"},
		Compose:  Delegate("edit_file"),
	},
	{
		Name: "fs_search", Title: "搜索文件内容", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "在项目文件中搜索文本或正则表达式。可用 path 限定单个文件或子目录——" +
			"读回 web 抓取超长页面时留下的存档就传 path，指定单个文件时不受大文件跳过限制。" +
			"匹配过多时用 offset 续取，不要重复同一次调用。",
		Params: map[string]ParamSpec{
			"pattern": {Type: "string", Desc: "搜索模式文本", Required: true},
			"path":    {Type: "string", Desc: "只搜索该路径（相对项目根的文件或目录），可选"},
			"glob":    {Type: "string", Desc: "文件匹配模式，如 '*.go'、'*.html'，默认所有文件"},
			"context": {Type: "integer", Desc: "上下文行数（匹配行前后各 N 行），默认 0"},
			"offset":  {Type: "integer", Desc: "从第几处匹配开始返回（从 1 开始），可选"},
		},
		Required: []string{"pattern"},
		Caps:     []string{"grep"},
		Compose:  Delegate("grep"),
	},
	{
		Name: "fs_replace", Title: "批量替换", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "在多个文件中搜索并替换文本。glob 限定文件范围（默认 '**/*'）；" +
			"regex 为 true 时 search 按正则解释。单次只处理一批文件，结果会说明未扫描的尾部，" +
			"须带 offset 续跑直到扫完，不要凭印象认为已全部生效。",
		Params: map[string]ParamSpec{
			"search":  {Type: "string", Desc: "要搜索的文本（或正则表达式）", Required: true},
			"replace": {Type: "string", Desc: "替换后的文本", Required: true},
			"glob":    {Type: "string", Desc: "文件匹配模式，如 '**/*.go'、'*.html'，默认 '**/*'"},
			"regex":   {Type: "boolean", Desc: "是否把 search 视为正则表达式，默认 false"},
			"offset":  {Type: "integer", Desc: "从第几个匹配文件开始扫描（从 1 开始），用于续接上一批"},
		},
		Required: []string{"search", "replace"},
		Caps:     []string{"search_replace"},
		Compose:  Delegate("search_replace"),
	},
	{
		Name: "fs_glob", Title: "匹配文件路径", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "按模式查找文件与目录，递归遍历整个项目。两种 pattern 写法：" +
			"① 不含 '/'（'*.go'、'catalog_*.go'）= 文件名模式，匹配任意层级的文件名；" +
			"② 含 '/'（'pkg/mcp/intent/*.go'、'**/*.go'、'template/**'、'a/**/b/**/c'）= 路径模式，" +
			"按 / 分段匹配，'**' 可跨任意层级（含零层），段内支持 * ? [...]。" +
			"匹配项过多时用 offset 续取。",
		Params: map[string]ParamSpec{
			"pattern": {Type: "string", Desc: "匹配模式。不含 / 时按文件名匹配任意层级（'*.go'）；含 / 时按路径分段匹配（'pkg/**/*.go'、'template/**'）", Required: true},
			"offset":  {Type: "integer", Desc: "从第几个匹配项开始返回（从 1 开始），可选"},
		},
		Required: []string{"pattern"},
		Caps:     []string{"glob"},
		Compose:  Delegate("glob"),
	},
	{
		Name: "fs_list_dir", Title: "列出目录", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "列出目录结构和文件，隐藏目录自动跳过。",
		Params: map[string]ParamSpec{
			"path":  {Type: "string", Desc: "目录路径，相对项目根目录或绝对路径，默认项目根目录"},
			"depth": {Type: "integer", Desc: "递归深度，默认 2，最大 5"},
		},
		Caps:    []string{"list_directory"},
		Compose: Delegate("list_directory"),
	},
}

// ArticleOut 是内容类工作流的结构化输出样例（用于推导 OutputSchema）。
type ArticleOut struct {
	Id     int64  `json:"id"`
	Title  string `json:"title,omitempty"`
	Url    string `json:"url,omitempty"`
	Status string `json:"status,omitempty"`
}

// switchCompose 构造一个"按 action 字段路由到不同底层能力"的组合工具。
func switchCompose(routes map[string]string) Compose {
	return func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
		action, _ := args["action"].(string)
		capName, ok := routes[action]
		if !ok {
			return nil, fmt.Errorf("不支持的操作: %s（可选: %v）", action, keysOf(routes))
		}
		// 去掉 action 字段，避免底层能力收到未知参数
		sub := map[string]any{}
		for k, v := range args {
			if k == "action" {
				continue
			}
			sub[k] = v
		}
		// 更新类动作无需在此补齐字段：capEndpoints 给所有 update 类 cap 注入了
		// partial=true（见 partialUpdate），端点据此走 PATCH 语义——
		// 只覆盖显式传入的字段，未传的一律保持库中原值。
		//
		// 这里曾有一套「回查旧值再补齐」的机制（preserveOnUpdate），已随根因修复删除。
		// 它不仅多余，还有害：读端点会覆写字段（GetNavList 用 GetUrl 覆盖 link），
		// 拿派生值写回去等于用假数据覆盖真值。
		//
		// 优先走真实后台端点（见 cap_routes.go）；无端点映射的能力才回落 handler。
		out, err := callCap(ctx, cap, capName, sub)
		if err != nil {
			return nil, err
		}
		// 解析 JSON 响应，提取 data 字段填充 Result.Data
		res := &Result{Text: out}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err == nil {
			if data, ok := envelope["data"]; ok && data != nil {
				// 列表类动作要连分页信息一起保留。
				//
				// 背景（2026-10-03）：后台 34 个分页端点把 total 放在**信封顶层**
				// （如 ArchiveList / TagList / AttachmentList），
				// 只取 data 会让 total 被静默丢弃——调用方拿到一个裸数组，
				// 既不知道命中总数，也无法判断还有没有下一页。
				// 实测 content_manage 的 category_list/tag_list/module_list/page_list、
				// media list、structure nav_list 全部中招。
				//
				// 端点没给 total 时 listWithTotal 原样返回，不硬造假数字。
				if isListAction(action) {
					res.Data = listWithTotal(data, envelope, sub)
				} else {
					res.Data = data
				}
			}
		}
		return res, nil
	}
}

// preserveOnUpdate 在「更新已有对象」前，把调用方没传的受保护字段用库里的现值补齐。
//
// 背景（2026-10-03 实测）：后台多个 form handler 无条件 req.UpdateAll = true
// （category.go:201 / pluginTag.go:130 / module.go:96 …），provider 的 Save*
// 据此把请求里的零值直接写回。于是「只传 id+title 改个名」会把 description、
// keywords、module_id、status 一并清零，而端点回 ok=true —— 调用方无从察觉。
//
// 实测受害数据：分类 id=16「Technology」只改标题后 description 变空、
// module_id 与 status 归 0。分类是全站结构，module_id 归 0 会影响前台路由与模板渲染。
//
// 与 archive 的 fillMissingOnUpdate 同一思路，抽成表是为了覆盖更多 cap。
// 查旧值失败时原样返回：让端点照旧报错，不在这里猜值——凭空填一个
// module_id 反而可能把数据写坏到别处。

// isListAction 判断 action 是否为列表类动作（需要连分页信息一起回填）。
//
// 命名以 List 结尾，或属于 attachment/media 这类固定叫 list 的动作。
// 刻意保守：判错方向是"漏掉分页信息"（退回裸数组）而非"给非列表套上分页对象"，
// 后者会让 detail/save 的返回形状变得莫名其妙。
//
// 判定只认两种形态：精确的 "list"，或 "<资源>_list" 下划线分词形式。
// 不用裸 HasSuffix("list")——blacklist/listing 这类单词也以 list 结尾，
// 会被误判成列表动作，把单个对象包成 {list:[...], total:N}。
func isListAction(action string) bool {
	if action == "" {
		return false
	}
	lower := strings.ToLower(action)
	return lower == "list" || strings.HasSuffix(lower, "_list")
}

// withoutAction 返回去掉 action 字段的副本，供自定义 Compose 调用底层能力时避免透传分派键。
func withoutAction(args map[string]any) map[string]any {
	sub := make(map[string]any, len(args))
	for k, v := range args {
		if k == "action" {
			continue
		}
		sub[k] = v
	}
	return sub
}

// capResult 把 callCap 的 (string, error) 包成 Compose 的 (*Result, error)。
func capResult(out string, err error) (*Result, error) {
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

// systemSettingCompose 收敛 setting_*_form 系列：按 section 选择读写能力。
func systemSettingCompose(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error) {
	section, _ := args["section"].(string)
	values, _ := args["values"].(map[string]any)

	readCap, writeCap := settingCaps(section)
	if readCap == "" {
		return nil, fmt.Errorf("未知设置分区: %s", section)
	}
	if len(values) > 0 {
		out, err := callCap(ctx, cap, writeCap, values)
		if err != nil {
			return nil, err
		}
		return &Result{Text: out, Data: map[string]any{"section": section, "action": "update"}}, nil
	}
	out, err := callCap(ctx, cap, readCap, map[string]any{})
	if err != nil {
		return nil, err
	}
	return &Result{Text: out, Data: map[string]any{"section": section, "action": "read"}}, nil
}

func settingCaps(section string) (read, write string) {
	switch section {
	case "system", "content", "contact", "diy_field", "index", "safe":
		return "setting_" + section, "setting_" + section + "_form"
	case "rewrite":
		return "rewrite_get", "rewrite_form"
	default:
		return "", ""
	}
}

// ───────────────────────── 工具函数 ─────────────────────────

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

var idRe = regexp.MustCompile(`ID[:：]\s*(\d+)`)

// extractID 从工具返回文本里取出实体 ID。
//
// 返回文本有两种形态：
//  1. JSON 包络（api_invoke 经真实 REST 端点返回）：{"ok":..,"data":<控制器响应>}，
//     控制器再包一层 {"code","msg","data":<实体>}，实体 id 落在 data.data.id。
//  2. 纯文本（无 REST 等价端点的 handler，如 attachment_upload 的 "附件上传成功！ID: 123"）。
//
// 因此优先按 JSON 解析取下层 data.data.id / data.id / 顶层 id；解析失败再回退正则
// 匹配 "ID:" / "ID："。此前只认大写 "ID:"，对 JSON 包络（小写 id）永远返回 0，
// 导致 content_save_article 新建文档的回执 id 恒为 0。
func extractID(text string) int64 {
	if text == "" {
		return 0
	}
	var j map[string]any
	if err := json.Unmarshal([]byte(text), &j); err == nil {
		if id := jsonID(j); id != 0 {
			return id
		}
	}
	m := idRe.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	i, _ := strconv.ParseInt(m[1], 10, 64)
	return i
}

// jsonID 在 api_invoke 的 JSON 包络里定位实体 id：
// 优先 data.data.id（标准双层信封），其次 data.id，再次顶层 id。
func jsonID(j map[string]any) int64 {
	if d, ok := j["data"].(map[string]any); ok {
		if dd, ok := d["data"].(map[string]any); ok {
			if id, ok := dd["id"].(float64); ok {
				return int64(id)
			}
		}
		if id, ok := d["id"].(float64); ok {
			return int64(id)
		}
	}
	if id, ok := j["id"].(float64); ok {
		return int64(id)
	}
	return 0
}

func parseArticle(text string) map[string]any {
	var j map[string]any
	if err := json.Unmarshal([]byte(text), &j); err != nil {
		return nil
	}
	d, ok := j["data"].(map[string]any)
	if !ok {
		return nil
	}
	// 外层已像文档实体（有 link 或 title），直接用。
	if _, hasLink := d["link"]; hasLink {
		return d
	}
	if t, hasTitle := d["title"].(string); hasTitle && t != "" {
		return d
	}
	// 否则才尝试内层。
	if dd, ok := d["data"].(map[string]any); ok {
		return dd
	}
	return d
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// endpointFailure 从一次工具调用的输出里取出失败原因，两条通道都认：
// JSON 包络（api_invoke 系端点）与纯文本（fs_* 文件工具）。
//
// 为什么需要它：capEndpoints 里的写端点（archive_update、archive_create 等）在
// 业务失败时返回的是 {"ok":false,"msg":"未定义模型","data":null}——**不是** Go error，
// 所以 callCap 不会报错，Compose 若无条件构造回执就会把失败报成成功。
//
// 实测踩过：content_article action=save 更新一篇 module_id 为空的文章，端点返回
// 「未定义模型」且什么都没改，但 StructuredContent 给的是 {"ok":true,"data":{"id":1847}}，
// AI 会据此认为保存成功。
//
// 纯文本通道见 textFailureMarkers 的说明（2026-10-04 补，fs_* 系列漏判 isError）。
func endpointFailure(text string) string {
	if text == "" {
		return ""
	}
	// 先试纯文本通道：文件工具等根本不返回 JSON，用 Unmarshal 判会直接漏掉。
	if msg := textFailure(text); msg != "" {
		return msg
	}
	var j map[string]any
	if err := json.Unmarshal([]byte(text), &j); err != nil {
		return ""
	}
	// HTTP 层错误：api_invoke 会把 5xx 表达成 ok=true + status>=500 + 空 msg
	// （因为控制器压根没写响应体，iris 返回默认 500）。
	// 实测：seo action=sitemap 不传参数时端点 500，却回 {"ok":true,"status":500,"msg":""}。
	if st, present := j["status"]; present {
		if f, isNum := st.(float64); isNum && f >= 500 {
			// msg 为空时不要用通用文案盖掉状态码 —— 调用方更需要知道是 5xx。
			if m, ok := j["msg"].(string); ok && strings.TrimSpace(m) != "" {
				return m
			}
			return "端点返回 HTTP " + strconv.FormatInt(int64(f), 10)
		}
	}
	// ok 缺省不代表失败（不少成功响应不带 ok），只有显式 false 才算失败。
	if ok, present := j["ok"].(bool); present && !ok {
		return responseMsg(j)
	}
	// 双层信封：控制器层的 code 非 0 也是失败。
	if d, ok := j["data"].(map[string]any); ok {
		if code, present := d["code"]; present {
			if f, isNum := code.(float64); isNum && f != 0 {
				return responseMsg(d)
			}
		}
	}
	return ""
}

// textFailureMarkers 是「纯文本通道」里的失败标记。
//
// 为什么需要它：endpointFailure 只能解析 JSON 信封，但有一整类 cap 根本不返回
// JSON —— fs_read / fs_write / fs_edit / fs_replace / fs_glob 等文件工具失败时
// 直接返回一段人话文本（provider/aiBuiltinTools.go 里 `return "错误：…", nil`）。
// 这些不是 Go error，于是 cerr 为 nil、IsError 保持 false，
// 而 MCP 规范要求工具执行失败必须置 isError=true —— 严格依赖该字段的客户端
// 会把「禁止访问系统敏感路径」「文件不存在」当成**成功**。
//
// 2026-10-04 实测：`fs_read` 传 `../../../../etc/passwd` 返回
// `{"result":{"content":[{"type":"text","text":"错误：禁止访问系统敏感路径"}]}}`，
// isError 字段整个缺失。
//
// 判定必须用**前缀**而非包含：这些工具的成功文案（如「文件 xxx 已更新，共替换
// 1 处」「读取成功」）不含下列任何前缀；已核对 provider/aiBuiltinTools.go 的
// 全部 return 文案，前缀无冲突。新增文件工具时沿用同一批前缀。
//
// ⚠️ 「未找到匹配」不能整段作为失败标记：fs_search（grep）无命中返回
// 「未找到匹配的内容」+ 跳过说明，那是**搜索成功但结果为空**的正常回执，
// 标成失败会让 AI 反复重搜同一个不存在的关键词。只有 fs_replace 的
// 「未找到匹配的文件」才是真失败（要改的东西一个都没改）。故精确到「的文件」。
var textFailureMarkers = []string{
	"错误：",      // 参数非法、路径穿越、文件不存在、超过大小限制
	"未找到匹配的文件", // fs_replace 无命中：一个文件都没改
	"精确匹配失败",   // fs_edit 文本模式没匹配上
	"⚠ 警告：",    // fs_write 覆盖风险，等待 confirm 二次确认
}

// textFailure 判定纯文本通道的失败，返回可读原因；不是失败则返回 ""。
func textFailure(text string) string {
	trimmed := strings.TrimSpace(text)
	for _, m := range textFailureMarkers {
		if strings.HasPrefix(trimmed, m) {
			return trimmed
		}
	}
	return ""
}

// responseMsg 从一层响应里取可读的错误文案，逐级回退。
func responseMsg(m map[string]any) string {
	for _, k := range []string{"msg", "message", "errmsg", "err"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return "端点返回失败但未给出原因"
}

// articleStatusCode 把文档状态的各种写法归一成端点要的 uint。
//
// 端点是 ArchiveStatusRequest.Status uint（0=草稿，1=正式文档），而意图层对外
// 声明 status 是枚举字符串（ok/draft/plan）—— 直接透传会让 ReadJSON 报
// 「cannot unmarshal string into ... of type uint」，publish 动作完全不可用。
// 列表过滤用的 status 字符串不受影响（那只在 archive_list 里用，不经这里）。
func articleStatusCode(v any) any {
	switch s := v.(type) {
	case nil:
		return v
	case bool:
		// 顺手兼容 draft=true/false 的写法。统一返回 int64：
		// 断言与下游 JSON 序列化都按 int64 处理，混用 int 会让测试看不出类型漂移。
		if s {
			return int64(0)
		}
		return int64(1)
	case float64:
		return int64(s)
	case int:
		return int64(s)
	case int64:
		return s
	case string:
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "ok", "1", "publish", "published", "release":
			return int64(1)
		default: // draft / plan / 未知值都落到草稿
			return int64(0)
		}
	}
	return v
}
