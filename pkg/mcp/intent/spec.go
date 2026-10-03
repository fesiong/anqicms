// Package intent 实现 AnQiCMS MCP 的"意图层"内核。
//
// 设计定位（2026-09-18 重构 ABD）：
//   - 策略层与能力域映射：Domain 把全部能力划分为 15 个域，意图按域归类。
//   - 意图导向工作流工具（IntentSpec）是对外接口，替代原先 105 个"endpoint 镜像"工具。
//   - 组合工具（Compose）是执行机制：单步意图 delegate 到某个底层能力，工作流意图编排多个能力。
//   - 声明式配置（catalog.go 的 IntentCatalog 切片）是"能力表达清单"的载体。
//   - 全部下沉到 MCP Server 内核：本包负责工具的注册、调用路由、两阶段 tools/list、结构化输出。
//
// 本包不依赖 provider，底层能力通过 CapInvoker 回调注入，避免循环依赖。
package intent

import (
	"context"
	"encoding/json"
)

// Domain 能力域。策略层据此对意图分组、裁剪与鉴权。
//
// 2026-09-26 重划（见 doc/intent-domain-split-proposal.md）：由 9 个域扩到 15 个。
// 划分骨架取自后台功能菜单（config/menu.go 的 8 个分组），但按语义重切——
// 尤其是把原先挤在一个菜单组里的 /plugin/*（229 端点、占后台 58%）拆到 9 个域，
// 否则 ApiExposure 的 allow_ns / deny_ns 白名单无从下手。
type Domain string

const (
	DomainContent     Domain = "content"     // 内容：文档/分类/标签/页面/模型/城市站
	DomainMedia       Domain = "media"       // 素材：附件/图片
	DomainStructure   Domain = "structure"   // 结构：导航/友链/301
	DomainSeo         Domain = "seo"         // 推广：关键词/锚文本/推送/sitemap/robots/结构化数据
	DomainTraffic     Domain = "traffic"     // 数据：蜘蛛/流量/收录/概览统计
	DomainInteraction Domain = "interaction" // 互动：评论/留言
	DomainContentOps  Domain = "contentops"  // 内容生产：素材/采集/导入/迁移/AI写作/翻译/配图/水印
	DomainCommerce    Domain = "commerce"    // 交易：会员/订单/支付/财务/分销
	DomainChannel     Domain = "channel"     // 渠道：公众号/小程序/邮件/订阅
	DomainSystem      Domain = "system"      // 系统：设置/存储/安全/伪静态/多语言/验证文件
	DomainSiteOps     Domain = "siteops"     // 站点运维（高危）：备份/缓存/全站替换/升级/全文索引/多站点
	DomainAccount     Domain = "account"     // 管理员：账号/分组/日志（默认仅只读）
	DomainDesign      Domain = "design"      // 模板设计（默认仅只读）
	DomainAgent       Domain = "agent"       // 智能体：Agent/Skill/任务
	DomainBuiltin     Domain = "builtin"     // 内置：shell/文件/网络（按决策维持暴露）

	// DomainUser 已并入 DomainCommerce（会员与订单同属交易域）。
	// Deprecated: 保留常量仅为兼容已持久化的配置，新意图请一律使用 DomainCommerce。
	DomainUser Domain = "user"

	// DomainUnknown 表示命名空间未登记到映射表。它不应出现在任何真实端点上——
	// provider 侧 TestAllEndpointsHaveDomain 会穷举 394 个端点校验这一点，
	// 一旦出现即说明有新的 ns 忘记登记，属需要修的错误而非可容忍的降级。
	DomainUnknown Domain = "unknown"
)

// Risk 风险等级，驱动 ToolAnnotations 与审计标注。
type Risk string

const (
	RiskRead        Risk = "read"        // 只读查询
	RiskWrite       Risk = "write"       // 站点内写入（建/改）
	RiskDestructive Risk = "destructive" // 删除/覆盖，不可回滚
	RiskSystem      Risk = "system"      // 主机级（shell/文件写）或全局配置
)

// ParamSpec 声明式参数（用于生成 InputSchema，亦作为"能力表达清单"的一部分）。
type ParamSpec struct {
	Type     string   // string | integer | boolean | array | object
	Desc     string   // 参数说明
	Required bool     // 是否必填
	Enum     []string // 枚举值（可选）
	Default  any      // 默认值（可选）
	Items    string   // array 元素类型（可选）
}

// Result 组合工具的执行结果。
// Text 给模型的叙事文本（兼容现状）；Data 为结构化数据，映射到 MCP StructuredContent。
type Result struct {
	Text string
	Data any
}

// CapInvoker 调用底层能力（原 endpoint 工具）。由 provider 注入：
// 把 args 序列化为 JSON 后调用对应能力 handler。
type CapInvoker func(ctx context.Context, name string, args map[string]any) (string, error)

// Compose 组合工具的执行逻辑：把意图参数映射、编排底层能力，产出结果。
type Compose func(ctx context.Context, args map[string]any, cap CapInvoker) (*Result, error)

// IntentSpec 一个意图导向工作流工具。字段全部为声明式数据 + 一个 Compose 执行函数。
type IntentSpec struct {
	Name       string               // 对外工具名（意图）
	Title      string               // 展示名
	Desc       string               // 说明
	Domain     Domain               // 所属能力域
	Risk       Risk                 // 风险等级（必填）
	Idempotent bool                 // 幂等提示
	Params     map[string]ParamSpec // 参数声明
	Required   []string             // 必填参数名
	Caps       []string             // 底层能力名（用于 ExposedTools 白名单兼容过滤与发现）
	Compose    Compose              // 执行函数（nil 表示 delegate 到 Caps[0]）
	// Output 输出样例，仅用于推导工具的 OutputSchema（告诉模型返回结构长什么样）。
	// 重要：声明 Output ≠ 工具真的返回结构化结果。只有 Compose 显式 return 非 nil 的
	// Data 字段，该 Data 才会作为 MCP StructuredContent 交付给模型；Delegate / switchCompose
	// 等透传类 Compose 不 return Data，OutputSchema 因此只是"装饰"，不会被填进 StructuredContent。
	// 需要真实结构化结果的意图（如 content_save_article）必须在 Compose 里 return &Result{Data: ...}。
	Output any

	// DefaultOff 表示该意图默认不出现在任何工具清单里，只有被 ExposedIntents 显式命中才开放。
	//
	// 用于误操作后果超出"改回来"范围的能力：通用调用（一次开放 394 个端点）、
	// 备份/升级/迁移等主机级运维、凭证与资金、对外发信、全站性配置。
	// 日常运营类意图不打这个标记，站点未配置白名单时也应开箱可见。
	// 注意它**不接受** ExposedTools 解锁，因为 ExposedTools 是能力名粒度，
	// 无法表达"这条通用调用要不要开"的意图语义。
	DefaultOff bool
}

// SpecByName 按意图名查找声明。
//
// 供意图层之外的调用方（如 AI 对话的写操作审批门）读取意图元数据，
// 避免它们各自硬编码一份"写工具名单"而与 IntentCatalog 漂移。
// 未找到返回 ok=false。
func SpecByName(name string) (*IntentSpec, bool) {
	for _, spec := range IntentCatalog {
		if spec != nil && spec.Name == name {
			return spec, true
		}
	}
	return nil, false
}

// Delegate 单步意图：把意图参数原样转发给底层能力，是其结果的透传。
// 用于"意图 == 单一能力"的场景，组合机制退化为一次调用。
func Delegate(capName string) Compose {
	return func(ctx context.Context, args map[string]any, inv CapInvoker) (*Result, error) {
		// 与 switchCompose 一致：命中 capEndpoints 的能力改走真实端点。
		out, err := callCap(ctx, inv, capName, args)
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

// boolPtr 便捷构造 *bool（ToolAnnotations 字段为指针）。
func boolPtr(b bool) *bool { return &b }
