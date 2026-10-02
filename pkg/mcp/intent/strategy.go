package intent

import (
	"reflect"
	"strings"
)

// DomainLabel 能力域的人类可读标签（策略层映射的对外呈现）。
func DomainLabel(d Domain) string {
	switch d {
	case DomainContent:
		return "内容管理"
	case DomainMedia:
		return "素材管理"
	case DomainStructure:
		return "站点结构"
	case DomainSeo:
		return "推广与 SEO"
	case DomainTraffic:
		return "数据统计"
	case DomainInteraction:
		return "用户互动"
	case DomainContentOps:
		return "内容生产"
	case DomainCommerce:
		return "交易与会员"
	case DomainChannel:
		return "触达渠道"
	case DomainSystem:
		return "系统设置"
	case DomainSiteOps:
		return "站点运维"
	case DomainAccount:
		return "管理员"
	case DomainDesign:
		return "模板设计"
	case DomainAgent:
		return "智能体"
	case DomainBuiltin:
		return "内置工具"
	case DomainUser:
		return "用户与订单"
	default:
		return string(d)
	}
}

// AllDomains 返回全部能力域（用于声明与校验）。
//
// 含已废弃的 DomainUser（兼容历史配置），但不含 DomainUnknown——
// 后者不是域，而是「映射表缺口」的哨兵值，出现在端点上即为缺陷。
func AllDomains() []Domain {
	return []Domain{
		DomainContent, DomainMedia, DomainStructure, DomainSeo,
		DomainTraffic, DomainInteraction, DomainContentOps,
		DomainCommerce, DomainChannel, DomainSystem,
		DomainSiteOps, DomainAccount, DomainDesign,
		DomainAgent, DomainBuiltin, DomainUser,
	}
}

// riskRank 风险等级权重，用于 ByMaxRisk 裁剪。
func riskRank(r Risk) int {
	switch r {
	case RiskRead:
		return 0
	case RiskWrite:
		return 1
	case RiskDestructive:
		return 2
	case RiskSystem:
		return 3
	default:
		return 0
	}
}

// ByMaxRisk 返回风险等级不超过 max 的意图名（用于"只读令牌"等场景）。
func ByMaxRisk(specs []*IntentSpec, max Risk) []string {
	var out []string
	for _, s := range specs {
		if riskRank(s.Risk) <= riskRank(max) {
			out = append(out, s.Name)
		}
	}
	return out
}

// ByDomain 返回属于指定域（可多选）的意图名。空 domains 表示全部。
func ByDomain(specs []*IntentSpec, domains ...Domain) []string {
	want := map[Domain]bool{}
	for _, d := range domains {
		want[d] = true
	}
	var out []string
	for _, s := range specs {
		if len(want) == 0 || want[s.Domain] {
			out = append(out, s.Name)
		}
	}
	return out
}

// BuildInputSchema 由声明式参数生成 MCP InputSchema（JSON Schema 2020-12）。
func BuildInputSchema(params map[string]ParamSpec, required []string) map[string]any {
	props := map[string]any{}
	order := make([]string, 0, len(params))
	for name := range params {
		order = append(order, name)
	}
	// 稳定顺序：先 required 后可选，按名字排序
	for _, name := range sortedKeys(params, required) {
		p := params[name]
		prop := map[string]any{"description": p.Desc}
		if p.Type == "" {
			prop["type"] = "string"
		} else {
			prop["type"] = p.Type
		}
		if p.Items != "" && p.Type == "array" {
			prop["items"] = map[string]any{"type": p.Items}
		}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		props[name] = prop
	}
	_ = order
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func sortedKeys(params map[string]ParamSpec, required []string) []string {
	reqSet := map[string]bool{}
	for _, r := range required {
		reqSet[r] = true
	}
	names := make([]string, 0, len(params))
	for n := range params {
		names = append(names, n)
	}
	// 必填优先，组内按字典序
	req := make([]string, 0)
	opt := make([]string, 0)
	for _, n := range names {
		if reqSet[n] {
			req = append(req, n)
		} else {
			opt = append(opt, n)
		}
	}
	sortStrings(req)
	sortStrings(opt)
	return append(req, opt...)
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// BuildOutputSchema 由样例结构推导 OutputSchema（最小反射，覆盖 struct/map/slice/基础类型）。
// 失败返回 nil（由调用方忽略）。
func BuildOutputSchema(sample any) any {
	if sample == nil {
		return nil
	}
	v := reflect.ValueOf(sample)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	schema := typeToJSONSchema(v.Type())
	if schema == nil {
		return nil
	}
	// MCP 契约：outputSchema 顶层必须是 object。若样例是切片/标量，
	// typeToJSONSchema 会产出 {"type":"array"} 等，而 mcp.Server.AddTool
	// 对非 object 的 outputSchema 会直接 panic（表现为服务启动即崩）。
	// 这里主动降级为 nil（不声明 outputSchema），规避该启动崩溃隐患。
	if typ, _ := schema["type"].(string); typ != "object" {
		return nil
	}
	return schema
}

func typeToJSONSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		item := typeToJSONSchema(t.Elem())
		if item == nil {
			item = map[string]any{}
		}
		return map[string]any{"type": "array", "items": item}
	case reflect.Map:
		return map[string]any{"type": "object"}
	case reflect.Struct:
		props := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name := jsonName(f)
			if name == "-" || name == "" {
				continue
			}
			fs := typeToJSONSchema(f.Type)
			if fs == nil {
				fs = map[string]any{}
			}
			props[name] = fs
		}
		return map[string]any{"type": "object", "properties": props}
	default:
		return map[string]any{}
	}
}

func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return f.Name
	}
	return parts[0]
}

// Summary 生成意图摘要（两阶段 tools/list 的轻量形态）。
func Summary(s *IntentSpec) map[string]any {
	return map[string]any{
		"name":   s.Name,
		"title":  s.Title,
		"domain": s.Domain,
		"risk":   s.Risk,
		"desc":   s.Desc,
	}
}
