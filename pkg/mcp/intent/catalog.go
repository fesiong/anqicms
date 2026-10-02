package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "create", "delete"}},
			"id":     {Type: "integer", Desc: "关键词 ID"},
			"title":  {Type: "string", Desc: "关键词"},
		},
		Required: []string{"action"},
		Caps:     []string{"keyword_list", "keyword_create", "keyword_delete"},
		Compose:  switchCompose(map[string]string{"list": "keyword_list", "create": "keyword_create", "delete": "keyword_delete"}),
	},
	{
		Name: "seo_anchor", Title: "管理锚文本", Domain: DomainSeo, Risk: RiskWrite,
		Desc: "SEO 锚文本的增删查。action: list/create/delete。",
		Params: map[string]ParamSpec{
			"action": {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "create", "delete"}},
			"id":     {Type: "integer", Desc: "锚文本 ID"},
			"title":  {Type: "string", Desc: "锚文本"},
			"link":   {Type: "string", Desc: "链接"},
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
		Desc: "插件相关操作。action: robots_get/robots_set/htmlcache_build/fulltext_rebuild/backup_dump/migrate_db（其余缓存/全文/重定向等已并入 siteops_maintain / system_config）。",
		Params: map[string]ParamSpec{
			"action":  {Type: "string", Desc: "操作", Required: true, Enum: []string{"robots_get", "robots_set", "htmlcache_build", "fulltext_rebuild", "backup_dump", "migrate_db"}},
			"content": {Type: "string", Desc: "robots.txt 内容（robots_set 用）"},
		},
		Required: []string{"action"},
		Caps:     []string{"plugin_robots_get", "plugin_robots_set", "plugin_htmlcache_build", "plugin_fulltext_rebuild", "plugin_backup_dump", "setting_migrate_db"},
		Compose: switchCompose(map[string]string{
			"robots_get": "plugin_robots_get", "robots_set": "plugin_robots_set",
			"htmlcache_build": "plugin_htmlcache_build", "fulltext_rebuild": "plugin_fulltext_rebuild",
			"backup_dump": "plugin_backup_dump", "migrate_db": "setting_migrate_db",
		}),
	},

	// ───────────────────────── 内置域（按 C 决策维持暴露）─────────────────────────
	{
		Name: "shell_exec", Title: "执行 Shell 命令", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "在服务器执行 shell 命令（主机级能力，谨慎使用）。",
		Params: map[string]ParamSpec{
			"command": {Type: "string", Desc: "要执行的命令", Required: true},
			"timeout": {Type: "integer", Desc: "超时秒数，默认 60"},
		},
		Required: []string{"command"},
		Caps:     []string{"bash"},
		Compose:  Delegate("bash"),
	},
	{
		Name: "fs_read", Title: "读取文件", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "读取服务器上的文本文件。",
		Params: map[string]ParamSpec{
			"path": {Type: "string", Desc: "文件路径", Required: true},
		},
		Required: []string{"path"},
		Caps:     []string{"read_file"},
		Compose:  Delegate("read_file"),
	},
	{
		Name: "fs_write", Title: "写入文件", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "把内容写入服务器文件（覆盖）。",
		Params: map[string]ParamSpec{
			"path":    {Type: "string", Desc: "文件路径", Required: true},
			"content": {Type: "string", Desc: "文件内容", Required: true},
		},
		Required: []string{"path", "content"},
		Caps:     []string{"write_file"},
		Compose:  Delegate("write_file"),
	},
	{
		Name: "fs_edit", Title: "编辑文件", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "以旧文本替换为新文本的方式编辑文件。",
		Params: map[string]ParamSpec{
			"path":       {Type: "string", Desc: "文件路径", Required: true},
			"old_string": {Type: "string", Desc: "待替换文本", Required: true},
			"new_string": {Type: "string", Desc: "新文本", Required: true},
		},
		Required: []string{"path", "old_string", "new_string"},
		Caps:     []string{"edit_file"},
		Compose:  Delegate("edit_file"),
	},
	{
		Name: "fs_search", Title: "搜索文件内容", Domain: DomainBuiltin, Risk: RiskRead,
		Desc: "在文件中按正则/关键字搜索。",
		Params: map[string]ParamSpec{
			"pattern": {Type: "string", Desc: "搜索模式", Required: true},
			"path":    {Type: "string", Desc: "搜索路径"},
		},
		Required: []string{"pattern"},
		Caps:     []string{"grep"},
		Compose:  Delegate("grep"),
	},
	{
		Name: "fs_replace", Title: "批量替换", Domain: DomainBuiltin, Risk: RiskSystem,
		Desc: "按模式批量替换文件内容。",
		Params: map[string]ParamSpec{
			"pattern":     {Type: "string", Desc: "匹配模式", Required: true},
			"replacement": {Type: "string", Desc: "替换文本", Required: true},
			"path":        {Type: "string", Desc: "目标路径"},
		},
		Required: []string{"pattern", "replacement"},
		Caps:     []string{"search_replace"},
		Compose:  Delegate("search_replace"),
	},
	{
		Name: "fs_glob", Title: "匹配文件路径", Domain: DomainBuiltin, Risk: RiskRead,
		Params: map[string]ParamSpec{
			"pattern": {Type: "string", Desc: "glob 模式", Required: true},
		},
		Required: []string{"pattern"},
		Caps:     []string{"glob"},
		Compose:  Delegate("glob"),
	},
	{
		Name: "fs_list_dir", Title: "列出目录", Domain: DomainBuiltin, Risk: RiskRead,
		Params: map[string]ParamSpec{
			"path": {Type: "string", Desc: "目录路径", Required: true},
		},
		Required: []string{"path"},
		Caps:     []string{"list_directory"},
		Compose:  Delegate("list_directory"),
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
				res.Data = data
			}
		}
		return res, nil
	}
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

// parseArticle 从 api_invoke 的 JSON 包络里取出实体对象（标准双层信封下在 data.data）。
// 仅用于在 content_save_article 回执里回填标题/链接/状态等结构化字段，与 ArticleOut 对齐。
func parseArticle(text string) map[string]any {
	var j map[string]any
	if err := json.Unmarshal([]byte(text), &j); err != nil {
		return nil
	}
	if d, ok := j["data"].(map[string]any); ok {
		if dd, ok := d["data"].(map[string]any); ok {
			return dd
		}
		return d
	}
	return nil
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}
