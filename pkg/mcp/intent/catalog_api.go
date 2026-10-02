package intent

// 本文件声明「REST 通用调用」意图，是 G 阶段的核心产物。
//
// 为什么要有它：精选意图体系只覆盖 ALL 端点的约五分之一，剩下尤以
// plugin 命名空间（223 个端点）为主的长尾几乎没有任何 cap。写满 394 个 cap 去
// 追平这套 REST 平行实现既不经济也会持续漂移。
//
// 取而代之的是「按需发现 + 调用」的三段式（现已合并为单个 api 意图的 action）：
//
//	api action=list    先检索有哪些端点可用（可按命名空间/风险/方法过滤）
//	api action=schema  取目标端点的完整参数定义（由源码派生，不手写）
//	api action=invoke  才真正执行这次调用（受 G4 两道闸门 + 显式身份约束）
//
// 与业界 Schema-aware 路线一致（先 search 再 execute），避免把上百个工具一次性
// 塞进上下文。这也正是 Anthropic 明确反对的 1:1 端点镜像反模式的解法。
//
// 安全设计：api 标记 DefaultOff，默认不出现在任何工具清单里；
// 且 api action=invoke 的执行身份只能来自显式配置的专用管理员账号，模型无法通过参数指定。

// apiIntentCatalog 是通用调用意图的声明，由 init 合并进总的 IntentCatalog。
// 单独成文件是为了让「通用调用」这组能力与其安全约束集中可见，不被精选意图淹没。
var apiIntentCatalog = []*IntentSpec{
	{
		// 合并 api_list / api_schema / api_invoke 三个意图为一个 api 工具，
		// 用 action 区分"发现 / 查参 / 执行"三档能力。
		Name:       "api",
		Title:      "后台接口发现与调用",
		Desc:       "按需发现并调用本站后台管理接口。action: list(按命名空间/方法/风险/关键词检索可用端点)/schema(查看目标端点参数定义，由源码自动派生)/invoke(以配置的管理员身份执行端点，写操作产生真实数据变更)。当精选意图无法完成需求时，用它查找底层可用的管理接口。",
		Domain:     DomainSystem,
		Risk:       RiskWrite, // invoke 为写操作，取最高风险
		DefaultOff: true,
		Params: map[string]ParamSpec{
			"action":  {Type: "string", Desc: "操作", Required: true, Enum: []string{"list", "schema", "invoke"}},
			"keyword": {Type: "string", Desc: "路径、处理器名或资源名中的关键词（list）"},
			"ns":      {Type: "string", Desc: "命名空间前缀，如 archive、plugin、plugin/keyword（list）"},
			"risk":    {Type: "string", Desc: "风险等级过滤（list）", Enum: []string{"read", "write", "destructive"}},
			"method":  {Type: "string", Desc: "HTTP 方法过滤（list）/ 调用方法（invoke）", Enum: []string{"GET", "POST", "DELETE"}},
			"source":  {Type: "string", Desc: "参数来源过滤（list）", Enum: []string{"struct", "urlparam", "form", "multipart", "none"}},
			"limit":   {Type: "integer", Desc: "返回条数上限，默认 30（list）"},
			"offset":  {Type: "integer", Desc: "分页偏移，默认 0（list）"},
			"path":    {Type: "string", Desc: "接口路径（schema/invoke），支持 /system/api/archive/detail 或简写 /archive/detail"},
			"params":  {Type: "object", Desc: "接口参数对象（invoke）；上传类接口的 file 字段可传 data URI 或裸 base64 字符串"},
		},
		Required: []string{"action"},
		Caps:     []string{"api_list", "api_schema", "api_invoke"},
		Compose: switchCompose(map[string]string{"list": "api_list", "schema": "api_schema", "invoke": "api_invoke"}),
	},
}

// init 把通用调用意图合并进总清单。
// 走 init 而非手动拼接，是为了保证任何人新增意图都不会遗漏注册路径。
func init() {
	IntentCatalog = append(IntentCatalog, apiIntentCatalog...)
}
