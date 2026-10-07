package intent

import (
	"sort"
	"strings"
)

// coverage.go 把"意图层能触达哪些后台端点"变成可枚举的事实，供覆盖审计消费。
//
// 背景：H 阶段删掉了 aiTools.go 里 84 个与后端平行的 bespoke 能力实现之后，
// provider/mcp_coverage.go 原先的口径（拿 getEinoTools() 当 caps 真相源）彻底失真——
// 那里只剩 11 个没有 REST 端点的能力，覆盖率从 20.8% 掉到 0.8%。
// 真实的 AI 可达性现在**只**由两处声明决定：
//  1. invokeRoutes：意图直接声明 "METHOD PATH"；
//  2. capEndpoints：底层能力声明等价端点，由 callCap 经 api_invoke 执行。
//
// 本文件把这两处汇总成一张表，使覆盖审计不再依赖任何运行时工具清单。

// intentRouteRegistry 登记每个意图通过 invokeRoutes 声明的端点（action → "METHOD PATH"）。
// 由 invokeRoutes 在构造时写入，因此与执行用的 routes 是同一份字面量，不会漂移。
var intentRouteRegistry = map[string]map[string]string{}

// registerIntentRoutes 登记意图声明的端点路由。仅供 invokeRoutes 调用。
func registerIntentRoutes(intentName string, routes map[string]string) {
	if intentName == "" {
		return
	}
	cp := make(map[string]string, len(routes))
	for action, target := range routes {
		cp[action] = target
	}
	intentRouteRegistry[intentName] = cp
}

// normalizeEndpointPath 把端点路径补全成 /system/api 前缀的规范化形式，
// 与 provider 侧 api_catalog.json / route/manage.go 里的路径口径一致。
//
// 前缀判断必须带边界（"/system/api" 或 "/system/api/"），否则 "/system/apiadmin/detail"
// 会被误判成已补全——这类拼接 bug 不会报编译错，只会让调用静默 404。
func normalizeEndpointPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p == "/system/api" || strings.HasPrefix(p, "/system/api/") {
		return strings.TrimRight(p, "/")
	}
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		return strings.TrimRight("/system"+p, "/")
	}
	return strings.TrimRight("/system/api"+p, "/")
}

// DeclaredEndpointTarget 是一个被意图层声明为可达的后台端点。
type DeclaredEndpointTarget struct {
	Method string   `json:"method"`
	Path   string   `json:"path"`  // 规范化后的完整后台路径
	Via    []string `json:"via"`   // 声明来源："intent:<name>" 或 "cap:<name>"
	Key    string   `json:"key"`   // "METHOD PATH"，便于与外部端点表比对
}

// DeclaredEndpointTargets 汇总意图层声明可达的全部后台端点。
//
// key 为 "METHOD /system/api/xxx"，value 为该端点的声明来源（已排序去重）。
// 调用方（provider 的覆盖审计）拿它跟真实的 400 个端点做差集，
// 得到的缺口就是"AI 还没有语义化入口、只能靠 api_invoke 手填路径"的端点。
func DeclaredEndpointTargets() map[string][]string {
	out := map[string][]string{}
	add := func(method, path, via string) {
		method = strings.ToUpper(strings.TrimSpace(method))
		p := normalizeEndpointPath(path)
		if method == "" || p == "" || via == "" {
			return
		}
		key := method + " " + p
		for _, old := range out[key] {
			if old == via {
				return
			}
		}
		out[key] = append(out[key], via)
	}

	// 1) 意图直接声明的端点
	for name, routes := range intentRouteRegistry {
		for _, target := range routes {
			parts := strings.Fields(strings.TrimSpace(target))
			if len(parts) != 2 {
				// 格式错误的声明由 TestDeclaredEndpointTargetsRegistered 拦下
				continue
			}
			add(parts[0], parts[1], "intent:"+name)
		}
	}

	// 2) 底层能力声明的等价端点；顺带标出引用该能力的意图，便于人工复核
	capOwners := map[string][]string{}
	for _, spec := range IntentCatalog {
		if spec == nil {
			continue
		}
		for _, c := range spec.Caps {
			capOwners[c] = append(capOwners[c], spec.Name)
		}
	}
	for capName, ce := range capEndpoints {
		add(ce.Method, ce.Path, "cap:"+capName)
		for _, owner := range capOwners[capName] {
			add(ce.Method, ce.Path, "intent:"+owner)
		}
	}

	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// DeclaredTargetsList 返回排序后的声明列表，供测试与报告使用。
func DeclaredTargetsList() []DeclaredEndpointTarget {
	m := DeclaredEndpointTargets()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]DeclaredEndpointTarget, 0, len(keys))
	for _, k := range keys {
		parts := strings.Fields(k)
		out = append(out, DeclaredEndpointTarget{
			Method: parts[0],
			Path:   parts[1],
			Via:    m[k],
			Key:    k,
		})
	}
	return out
}

// DeclaredTargetKeys 返回声明端点的 "METHOD PATH" 键集合（排序）。
func DeclaredTargetKeys() []string {
	m := DeclaredEndpointTargets()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
