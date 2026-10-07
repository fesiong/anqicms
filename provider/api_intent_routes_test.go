package provider

import (
	"context"
	"sort"
	"strings"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// 本文件校验「补齐域意图」（catalog_domains.go）声明的路由真实存在。
//
// 为什么必须穷举校验：这些意图不再委托专属 cap，而是直接指向后台端点。
// 路径写错（拼写、路径变更、插件下线）不会在编译期暴露，只会在调用时
// 报"未找到端点"——而那时模型已经把它当成了可用能力。
// 用行为验证（真的跑一次 Compose 并捕获它交给 api_invoke 的参数），
// 比读取声明表更可靠：它同时覆盖 action→路由的映射与参数组装两段逻辑。

// TestDomainIntentRoutesExistInCatalog 穷举每个 action，断言端点在真实目录里存在。
//
// 参数按声明现造（见 sampleArgs）：只传 {"action":...} 会让 system_config/setting
// 这类"必填字段在 action 之外"的 action 直接报错，那是测试喂参不全，不是声明错配。
func TestDomainIntentRoutesExistInCatalog(t *testing.T) {
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("目录构建失败: %v", err)
	}

	checked, skipped := 0, 0
	for _, s := range intent.IntentCatalog {
		actSpec, ok := s.Params["action"]
		if !ok || len(actSpec.Enum) == 0 {
			continue
		}
		// api 意图是通用调用入口：method/path 是它的参数而非固定路由，
		// 没有"声明的端点"可校验（端点存在性由 api_catalog 目录与硬规则保证）。
		if declaresCap(s, "api_list") {
			continue
		}
		// invokeRoutes 类意图（补齐域）是全量透传，可以逐字段断言；
		// cap 类意图会按 action 重塑参数（如 archive_publish 把 id 归一成 ids），
		// 只断言路由与端点，否则就是拿一种 Compose 的约定去要求另一种。
		passthrough := len(s.Caps) == 1 && s.Caps[0] == "api_invoke"
		_, declaresID := s.Params["id"]
		for _, action := range actSpec.Enum {
			var (
				gotCap  string
				gotArgs map[string]any
			)
			inv := func(ctx context.Context, name string, args map[string]any) (string, error) {
				gotCap, gotArgs = name, args
				return "{}", nil
			}
			if _, err := s.Compose(context.Background(), sampleIntentArgs(s, action), inv); err != nil {
				t.Fatalf("意图 %s 的 action=%s 无法执行（声明与实现不一致）: %v", s.Name, action, err)
			}
			method, _ := gotArgs["method"].(string)
			path, _ := gotArgs["path"].(string)
			if method == "" || path == "" {
				// switchCompose 类意图路由到 cap 名而非端点，不在本测试范围内
				skipped++
				continue
			}
			if gotCap != "api_invoke" {
				t.Errorf("意图 %s 的 action=%s 应经 api_invoke 执行，实际=%s", s.Name, action, gotCap)
			}
			if _, found := cat.FindEndpoint(method, path); !found {
				t.Errorf("意图 %s 的 action=%s 指向不存在的端点: %s %s", s.Name, action, method, path)
			}
			// action 是分派用字段，不能混进端点参数
			params, _ := gotArgs["params"].(map[string]any)
			if _, leaked := params["action"]; leaked {
				t.Errorf("意图 %s 把 action 透传进了端点参数", s.Name)
			}
			// 声明的业务 id 必须在参数组装后仍存活：invokeRoutes 允许改名
			// （commerce_order 把 id 归一成 order_id），所以断言"某个 id 类键
			// 仍带着采样值 1"，而不是"键名必须叫 id"。这里只查组装层是否把它
			// 悄悄丢弃；至于每个 action 的端点是否真认得这个字段名，属穷举门禁
			// 的职责（见 TestIntentParamsRecognizedByEndpoints）。
			if passthrough && declaresID && !businessIDSurvives(params) {
				t.Errorf("意图 %s 的 action=%s 声明的业务 id 在组装后被丢弃: params=%#v",
					s.Name, action, params)
			}
			checked++
		}
	}
	// 防止判定条件失效导致"一个都没校验却通过"
	if checked < 100 {
		t.Fatalf("校验到的端点路由仅 %d 条，判定条件可能已失效（switchCompose 类 %d 条）", checked, skipped)
	}
	t.Logf("已校验 %d 条端点路由（另有 %d 条走专属 cap，不在此范围）", checked, skipped)
}

// sampleIntentArgs 为一个 action 造出可通过 Compose 的示例参数：
// 声明过的字段全部按类型给值，枚举取首个合法值（section=setting 类必填字段才不会空）。
func sampleIntentArgs(s *intent.IntentSpec, action string) map[string]any {
	out := map[string]any{"action": action}
	for name, p := range s.Params {
		if name == "action" {
			continue
		}
		switch {
		case len(p.Enum) > 0:
			out[name] = p.Enum[0]
		case p.Type == "integer":
			out[name] = 1
		case p.Type == "boolean":
			out[name] = false
		case p.Type == "array":
			out[name] = []any{"https://example.com"}
		case p.Type == "object":
			out[name] = map[string]any{}
		default:
			out[name] = "sample"
		}
	}
	return out
}

// businessIDSurvives 判断采样出的业务 id 是否在组装后仍以某个 id 类键名存活。
// 只认 id 或 *_id 形式的键：invokeRoutes 可把 id 改名（如 order_id），
// 但不会把它变成非 id 类字段，所以这既能容忍改名、又能抓住"被悄悄丢弃"。
//
// 值要比对**采样时用的那个值**，不能写死 == 1：id 不一定是整数。
// commerce_order 的 id 声明为 string（业务单号 wc2021101838889109642，
// 不是数据库自增 id），sampleIntentArgs 给它采样的是 "sample"，
// 写死 v == 1 会把它 11 个 action 全部误报成「id 被丢弃」。
func businessIDSurvives(params map[string]any) bool {
	for k, v := range params {
		if k != "id" && !strings.HasSuffix(k, "_id") {
			continue
		}
		// 整数 id 采样为 1，字符串 id 采样为 "sample"（见 sampleIntentArgs）。
		// 两种都算存活；bool/数组/对象不是 id，不认。
		if v == 1 || v == "sample" {
			return true
		}
	}
	return false
}

// declaresCap 判断意图是否声明了某个 cap（用于识别通用调用意图 api：
// 它声明 api_list/api_schema/api_invoke，端点由参数给出而非声明表）。
func declaresCap(s *intent.IntentSpec, capName string) bool {
	for _, c := range s.Caps {
		if c == capName {
			return true
		}
	}
	return false
}

// invocation 记录 Compose 一次调用底层能力的现场：cap 名与它收到的参数。
// 端点路由表现为 name=="api_invoke" 且 args 带 method/path；其余是 cap 路由
// （switchCompose/无 REST 等价物的能力，参数交给真实 handler，不经端点字段表）。
type invocation struct {
	name string
	args map[string]any
}

func (iv invocation) isEndpoint() bool {
	if iv.name != "api_invoke" {
		return false
	}
	m, _ := iv.args["method"].(string)
	p, _ := iv.args["path"].(string)
	return m != "" && p != ""
}

func (iv invocation) endpoint() (method, path string, params map[string]any) {
	method, _ = iv.args["method"].(string)
	path, _ = iv.args["path"].(string)
	params, _ = iv.args["params"].(map[string]any)
	return
}

// captureInvocations 跑遍一个意图的所有 action，收集每次 Compose 交给底层的调用。
// 参数用 buildArgs 造，便于按需隔离单个参数（见 isolatedStringArgs）。
func captureInvocations(t *testing.T, s *intent.IntentSpec, buildArgs func(action string) map[string]any) []invocation {
	t.Helper()
	actSpec, ok := s.Params["action"]
	if !ok || len(actSpec.Enum) == 0 {
		return nil
	}
	var out []invocation
	for _, action := range actSpec.Enum {
		inv := func(ctx context.Context, name string, args map[string]any) (string, error) {
			out = append(out, invocation{name: name, args: args})
			return "{}", nil
		}
		if _, err := s.Compose(context.Background(), buildArgs(action), inv); err != nil {
			t.Fatalf("意图 %s 的 action=%s 无法执行: %v", s.Name, action, err)
		}
	}
	return out
}

// isolatedStringArgs 只把被测的 string 参数（focus）填成哨兵，省略其余非枚举
// string 参数。这样能避免"两个声明参数改名后撞到同一端点字段"（keyword 与 title
// 都落到 archive/list 的 title）时，谁的哨兵存活取决于 map 迭代顺序的不确定性。
// 枚举/数值/布尔/数组/对象参数照给，保证 compose 的分派逻辑与真实调用一致。
func isolatedStringArgs(s *intent.IntentSpec, action, focus string) map[string]any {
	out := map[string]any{"action": action}
	for name, p := range s.Params {
		switch {
		case name == "action":
		case name == focus:
			out[name] = marker(focus)
		case len(p.Enum) > 0:
			out[name] = p.Enum[0]
		case p.Type == "string":
			// 省略其它非枚举 string，隔离 focus 的去向
		case p.Type == "integer":
			out[name] = 1
		case p.Type == "boolean":
			out[name] = false
		case p.Type == "array":
			out[name] = []any{"https://example.com"}
		case p.Type == "object":
			out[name] = map[string]any{}
		}
	}
	return out
}

// marker 为一个声明参数生成唯一哨兵值；containsMarker 在参数树里递归查它。
// 改名只换键不换值，所以哨兵能把"声明名"与"端点实际读取的字段名"关联起来。
func marker(name string) string { return "\u00ab" + name + "\u00bb" }

func containsMarker(v any, m string) bool {
	switch t := v.(type) {
	case string:
		return t == m
	case map[string]any:
		for _, vv := range t {
			if containsMarker(vv, m) {
				return true
			}
		}
	case []any:
		for _, vv := range t {
			if containsMarker(vv, m) {
				return true
			}
		}
	}
	return false
}

// sentinelArgs 与 sampleIntentArgs 同构，但给每个"非枚举 string"参数填唯一标记。
// 枚举参数保留合法枚举值（compose 可能据其分派），数值/布尔/数组/对象按类型给值。
func sentinelArgs(s *intent.IntentSpec, action string) map[string]any {
	out := map[string]any{"action": action}
	for name, p := range s.Params {
		if name == "action" {
			continue
		}
		switch {
		case len(p.Enum) > 0:
			out[name] = p.Enum[0]
		case p.Type == "string":
			out[name] = marker(name)
		case p.Type == "integer":
			out[name] = 1
		case p.Type == "boolean":
			out[name] = false
		case p.Type == "array":
			out[name] = []any{"https://example.com"}
		case p.Type == "object":
			out[name] = map[string]any{}
		default:
			out[name] = "sample"
		}
	}
	return out
}

// TestIntentParamsRecognizedByEndpoints 是「声明的参数必须被落点端点认识」的穷举门禁。
//
// 背景：裸路由与 cap 路由都允许改名（keyword→title、logo→images、from→from_url、
// id→order_id…）。若某声明参数在它真正服务的那个 action 上没有被改成端点认得的
// 字段名，端点 ReadJSON 会静默忽略它——工具"看得见却调不动"。
//
// 判定必须逐参数、且穿过改名，不能用"意图级字段名并集"：同一个声明参数在不同
// action 上会被改成不同字段名，或在无关 action 上原样透传被端点忽略（无害）。
// 用唯一哨兵值追踪每个 string 参数的去向，判它合法的两条出路：
//  1. 某个端点路由里，哨兵落在该端点认得的字段名下（改名成功、端点读得到）；
//  2. 某个 cap 路由消费了它（无 REST 等价物，由真实 handler 自行校验参数）。
//
// 数值/布尔/数组/对象参数不用哨兵（值无法唯一化且 compose 可能据其分派），改用
// 字段名并集判定——这些参数名在各端点间通常一致，改名后也落在认得的字段上。
//
// 放行项：action/section 是分派字段、values 是运行时展开的对象通配、multipart
// 落点的文件字段（file/file_name/name/base64/url）由 api_invoke 单独装配。
func TestIntentParamsRecognizedByEndpoints(t *testing.T) {
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("目录构建失败: %v", err)
	}

	exempt := map[string]bool{"action": true, "section": true, "values": true}
	// partial：由 capEndpoints 的 Fixed 常量注入（见 pkg/mcp/intent 的 partialUpdate），
	// 用于让表单端点走 PATCH 语义——只覆盖显式传入的字段，未传的保持原值。
	//
	// 它不出现在 api_catalog 里是**正确的**：端点侧的 request 结构体给它标了
	// ast:"-"（生成器刻意排除这类内部控制字段）。它在调用时由 cap 层注入，
	// 属于「被 cap 消费」，与 action/section 同性质，故豁免。
	exempt["partial"] = true
	multipartAllowed := map[string]bool{"file": true, "file_name": true, "name": true, "base64": true, "url": true}

	judgedIntents, judgedEndpoints, capRoutes := 0, 0, 0
	for _, s := range intent.IntentCatalog {
		// 通用调用意图 api：method/path/params 是它的参数而非固定端点字段，无从比对。
		if declaresCap(s, "api_list") {
			continue
		}
		invocations := captureInvocations(t, s, func(a string) map[string]any { return sentinelArgs(s, a) })
		if len(invocations) == 0 {
			continue
		}

		// 预扫描：该意图是否触达 multipart 端点、是否有 cap 路由。
		hasMultipart, hasEndpoint := false, false
		recognizedUnion := map[string]bool{}
		for _, iv := range invocations {
			if !iv.isEndpoint() {
				capRoutes++
				continue
			}
			method, path, _ := iv.endpoint()
			ep, found := cat.FindEndpoint(method, path)
			if !found {
				t.Errorf("意图 %s 指向不存在的端点: %s %s", s.Name, method, path)
				continue
			}
			hasEndpoint = true
			if ep.ParamSource == "multipart" {
				hasMultipart = true
			}
			for _, p := range ep.Params {
				recognizedUnion[p.Name] = true
			}
		}
		if !hasEndpoint {
			continue // 纯 cap 意图，参数由 handler 校验，不在端点字段表范畴
		}
		judgedIntents++

		// ── string 参数：逐个隔离，哨兵追踪 ──
		var ghosts []string
		for name, p := range s.Params {
			if exempt[name] || len(p.Enum) > 0 || p.Type != "string" {
				continue
			}
			m := marker(name)
			// 用只含 focus 哨兵的参数跑一遍，避免与其它 string 参数改名撞车。
			iso := captureInvocations(t, s, func(a string) map[string]any { return isolatedStringArgs(s, a, name) })
			legit := false
			for _, iv := range iso {
				if iv.isEndpoint() {
					method, path, params := iv.endpoint()
					ep, found := cat.FindEndpoint(method, path)
					if !found {
						continue
					}
					for key, val := range params {
						if containsMarker(val, m) && endpointRecognizes(ep, key) {
							legit = true
							judgedEndpoints++
							break
						}
					}
				} else if containsMarker(iv.args, m) {
					legit = true // 被 cap 消费
				}
				if legit {
					break
				}
			}
			if !legit && hasMultipart && multipartAllowed[name] {
				legit = true
			}
			if !legit {
				ghosts = append(ghosts, name)
			}
		}

		// ── 非 string 参数：字段名并集 ──
		sentNonString := map[string]bool{}
		for _, iv := range invocations {
			if !iv.isEndpoint() {
				continue
			}
			_, _, params := iv.endpoint()
			for key, val := range params {
				if isMarkerValue(val) {
					continue // string 参数已由哨兵判定
				}
				sentNonString[key] = true
			}
		}
		for key := range sentNonString {
			if recognizedUnion[key] || exempt[key] {
				continue
			}
			if hasMultipart && multipartAllowed[key] {
				continue
			}
			ghosts = append(ghosts, key)
		}

		if len(ghosts) > 0 {
			sort.Strings(ghosts)
			t.Errorf("意图 %s 声明的参数未被任何落点端点认识、也未被 cap 消费（会被静默忽略）: %v", s.Name, dedup(ghosts))
		}
	}

	if judgedIntents < 25 {
		t.Fatalf("参与判定的意图仅 %d 个，门禁可能已失效", judgedIntents)
	}
	t.Logf("参数认识门禁：%d 个意图参与判定，命中 %d 条端点路由、%d 条 cap 路由",
		judgedIntents, judgedEndpoints, capRoutes)
}

func endpointRecognizes(ep EndpointMeta, key string) bool {
	for _, p := range ep.Params {
		if p.Name == key {
			return true
		}
	}
	return false
}

func isMarkerValue(v any) bool {
	s, ok := v.(string)
	return ok && strings.HasPrefix(s, "\u00ab") && strings.HasSuffix(s, "\u00bb")
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// TestDomainIntentDefaultExposure 校验补齐域意图的默认可见面：
// 常用意图开箱可见，高危意图必须仍然 DefaultOff。
//
// 这里刻意用一份独立名单（与 intent 包内的 gatedDomainIntents 并行维护）：
// provider 看不到包内私有表，从外部再钉一遍才算真正的复核。两张表不一致时，
// intent 包的 TestDomainIntentExposureIsDeliberate 与本用例至少有一边会失败，
// 调整默认策略时必须同时改这两处。
func TestDomainIntentDefaultExposure(t *testing.T) {
	mustStayGated := map[string]string{
		"contentops_collector": "批量采集写库",
		"contentops_import":    "批量导入与导入 Token 配置",
		"contentops_transfer":  "跨站点迁移不可回滚",
		"contentops_imagedeco": "上传文件并批量改写附件",
		"commerce_pay":         "资金凭证",
		"commerce_finance":     "提现审批",
		"channel_wechat":       "对外同步到微信侧",
		"channel_thirdparty":   "授权 AppID/Secret",
		"channel_sendmail":     "真实投递邮件",
		"channel_subscriber":   "订阅用户群发",
		"system_security":      "风控配置可锁站",
		"system_multilang":     "跨站点同步与删除",
		"system_rewrite":       "全站 URL 结构",
		"siteops_backup":       "数据库破坏性操作",
		"siteops_upgrade":      "版本升级",
		"siteops_website":      "多站点增删",
		"api":                  "通用调用等价 shell",
		// 2026-10-07：system_plugin 转入（它只剩主机级/不可逆动作，robots 已迁往siteops_maintain）。
		"system_plugin": "主机级不可逆动作：建缓存索引/重建全文索引/导出整站备份/迁移数据库",
		// 2026-10-07：以下四个转入默认关闭——站点未发行交易域，且素材池/译文的
		// 误操作要到「取稿时」「切语言后」才暴露，事后难追溯。
		"contentops_material":  "素材池写入与删除，出错要等取稿时才发现",
		"contentops_translate": "批量改写多语言副本，译文未经审校",
		"commerce":             "会员/分组/分销商写操作影响真实用户",
		"commerce_order":       "订单状态流转直接动资金，退款/取消不可回滚",
		// 注：skill 曾以「涉及文件系统」列在此处，2026-10-07 移出并改为默认开放——
		// 该理由不成立，它的 delete 已被审批门 + 端点字符白名单 + 只删 skills/<name> 覆盖。
		// 改默认策略时必须同时改intent 包的 openNonDomainIntents 与这里。
	}
	byName := map[string]*intent.IntentSpec{}
	for _, s := range intent.IntentCatalog {
		byName[s.Name] = s
	}
	for name, why := range mustStayGated {
		s, ok := byName[name]
		if !ok {
			t.Errorf("意图 %s 已不存在，名单需同步（原理由：%s）", name, why)
			continue
		}
		if !s.DefaultOff {
			t.Errorf("高危意图 %s 未标记 DefaultOff（%s）", name, why)
		}
	}

	// 日常运营意图不得被整体关回去：留空白名单时它们必须出现在模型面。
	// 2026-10-07：contentops_material / contentops_translate / commerce / commerce_order
	// 移入 mustStayGated；skill 原本就在 mustStayGated（理由不成立），现改列入本名单。
	dailyOpen := []string{
		"content_article", "content_manage", "content_place", "media", "structure",
		"seo", "seo_keyword", "seo_anchor", "seo_jsonld", "seo_llms", "traffic_statistics",
		"interaction", "system_config", "siteops_maintain",
		"account", "design_manage", "agent", "web", "skill",
	}
	for _, name := range dailyOpen {
		s, ok := byName[name]
		if !ok {
			t.Errorf("常用意图 %s 已不存在，名单需同步", name)
			continue
		}
		if s.DefaultOff {
			t.Errorf("常用意图 %s 被标记为 DefaultOff，站点未配置白名单时它不再默认可见", name)
		}
	}

	// 推荐白名单引用的意图必须真实存在，且都不是高危默认关闭项：
	// 这份清单对外宣传为"保守起步"，混进备份/资金类意图就成了误导。
	for _, name := range intent.RecommendedExposed() {
		s, ok := byName[name]
		if !ok {
			t.Errorf("推荐白名单引用了不存在的意图: %s", name)
		} else if s.DefaultOff {
			t.Errorf("推荐白名单包含默认关闭的高危意图 %s，与「保守起步」的定位不符", name)
		}
	}
}
