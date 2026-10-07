package intent

import (
	"context"
	"sort"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/server"
)

// 捕获型 CapInvoker：记录被调用的能力名与参数，供断言使用。
type captureInvoker struct {
	name string
	args map[string]any
}

func (c *captureInvoker) invoke(ctx context.Context, name string, args map[string]any) (string, error) {
	c.name = name
	c.args = args
	return "{}", nil
}

// TestInvokeRoutesDispatchesToAPIInvoke 验证 action → method/path 的分派与参数组装。
//
// 关键断言是 action **不能**混进端点参数：它是分派用字段，
// 若透传下去，端点会收到一个自己不认识的字段（多数接口直接忽略，但会污染 multipart/struct 绑定）。
func TestInvokeRoutesDispatchesToAPIInvoke(t *testing.T) {
	c := &captureInvoker{}
	compose := invokeRoutes("", map[string]string{
		"list": "GET /system/api/plugin/push",
		"push": "POST /system/api/plugin/push",
	})
	if _, err := compose(context.Background(), map[string]any{"action": "push", "urls": []string{"https://a"}}, c.invoke); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if c.name != "api_invoke" {
		t.Fatalf("应经 api_invoke 执行，实得 %s", c.name)
	}
	if c.args["method"] != "POST" || c.args["path"] != "/system/api/plugin/push" {
		t.Fatalf("分派错误: %#v", c.args)
	}
	params, _ := c.args["params"].(map[string]any)
	if _, leaked := params["action"]; leaked {
		t.Error("action 被透传进了端点参数")
	}
	if _, ok := params["urls"]; !ok {
		t.Errorf("业务参数应原样透传: %#v", params)
	}
}

// TestInvokeRoutesRejectsUnknownAction 未知 action 必须报错，不能静默落到某个默认路由。
func TestInvokeRoutesRejectsUnknownAction(t *testing.T) {
	c := &captureInvoker{}
	compose := invokeRoutes("", map[string]string{"list": "GET /system/api/plugin/push"})
	if _, err := compose(context.Background(), map[string]any{"action": "nope"}, c.invoke); err == nil {
		t.Fatal("未知 action 应报错")
	}
}

// TestInvokeRoutesRejectsMalformedTarget 路由声明格式错误应显式报错。
// 静默分派会让调用落到错误方法上（例如把写操作当读操作执行），代价很高。
func TestInvokeRoutesRejectsMalformedTarget(t *testing.T) {
	c := &captureInvoker{}
	for _, bad := range []string{"", "/system/api/plugin/push", "GET"} {
		compose := invokeRoutes("", map[string]string{"x": bad})
		if _, err := compose(context.Background(), map[string]any{"action": "x"}, c.invoke); err == nil {
			t.Errorf("格式错误的目标 %q 应报错", bad)
		}
	}
}

// TestDomainIntentsDeclareEveryAction 声明的 action 枚举必须都能分派。
//
// 与 provider 侧的端点存在性校验互补：这里在 intent 包内单跑即可守住
// 「Enum 与 routes 不一致」这类声明错配，不依赖 provider 的目录构建。
func TestDomainIntentsDeclareEveryAction(t *testing.T) {
	checked := 0
	for _, s := range domainIntentCatalog {
		for _, action := range s.Params["action"].Enum {
			c := &captureInvoker{}
			if _, err := s.Compose(context.Background(), map[string]any{"action": action}, c.invoke); err != nil {
				t.Errorf("意图 %s 声明了 action=%s 却无法分派: %v", s.Name, action, err)
				continue
			}
			if c.name != "api_invoke" {
				t.Errorf("意图 %s 应经 api_invoke 执行，实得 %s", s.Name, c.name)
			}
			checked++
		}
	}
	if checked < 150 {
		t.Fatalf("仅校验 %d 条，判定条件可能已失效", checked)
	}
}

// TestDomainIntentsUseDeclaredDomains 新意图的域必须是已声明域（防拼写造域）。
func TestDomainIntentsUseDeclaredDomains(t *testing.T) {
	known := map[Domain]bool{}
	for _, d := range AllDomains() {
		known[d] = true
	}
	for _, s := range domainIntentCatalog {
		if !known[s.Domain] {
			t.Errorf("意图 %s 的域 %q 未声明", s.Name, s.Domain)
		}
		if s.Risk == "" {
			t.Errorf("意图 %s 未声明风险等级", s.Name)
		}
		if len(s.Caps) != 1 || s.Caps[0] != "api_invoke" {
			t.Errorf("意图 %s 的 Caps 应恰为 [api_invoke]，实得 %v", s.Name, s.Caps)
		}
	}
}

// TestExecuteRejectsUnopenedDefaultOff 未开放的 DefaultOff 意图必须被 Execute 拒绝。
//
// 这条断言曾经失败过：准入原本只在"注册到模型面"时判定，Execute 不检查，
// 于是任何绕过注册直接调 Execute 的内部路径都能执行备份/升级/迁移这类高危能力。
// 现在 Execute 自己再拦一次（纵深防御），这里把它钉住。
func TestExecuteRejectsUnopenedDefaultOff(t *testing.T) {
	inv := func(ctx context.Context, name string, args map[string]any) (string, error) { return "{}", nil }

	// 只开了 design_manage
	k := NewKernel(Config{ExposedIntents: []string{"design_manage"}}, inv, nil)
	if _, err := k.Execute(context.Background(), "design_manage", `{"action":"list"}`); err != nil {
		t.Fatalf("已开放意图应可执行: %v", err)
	}
	if _, err := k.Execute(context.Background(), "siteops_backup", `{"action":"list"}`); err == nil {
		t.Fatal("未开放的 siteops_backup 竟能执行，DefaultOff 在 Execute 层失效")
	}

	// 白名单写域名可整域放行（与 matchIntentPattern 的约定一致）
	k2 := NewKernel(Config{ExposedIntents: []string{"siteops"}}, inv, nil)
	if _, err := k2.Execute(context.Background(), "siteops_backup", `{"action":"list"}`); err != nil {
		t.Errorf("域级白名单应放行整个 siteops 域: %v", err)
	}

	// 白名单为空时 DefaultOff 仍不开放（不能因"没配就全开"而暴露）
	k3 := NewKernel(Config{}, inv, nil)
	if _, err := k3.Execute(context.Background(), "siteops_backup", `{"action":"list"}`); err == nil {
		t.Fatal("白名单为空时 DefaultOff 意图仍不应开放")
	}
}

// TestRecommendedExposedIsConservative 推荐白名单不得包含高风险运维类意图。
// 这份清单会被一键填入配置，混进备份/升级/迁移等于绕过了人工确认环节。
func TestRecommendedExposedIsConservative(t *testing.T) {
	forbidden := map[string]bool{
		"siteops_backup": true, "siteops_upgrade": true, "siteops_website": true,
		"contentops_transfer": true, "api_invoke": true, "api": true,
	}
	for _, n := range RecommendedExposed() {
		if forbidden[n] {
			t.Errorf("推荐白名单不应包含高风险意图 %s", n)
		}
	}
}

// TestDomainIntentExposureIsDeliberate 每个补齐域意图都必须显式表态：
// 要么默认开放，要么登记进 gatedDomainIntents 并写明理由。
//
// 双向校验的意义在于把"高危能力默认关闭"从人的记忆变成机器约束：
// 漏标一行 DefaultOff 会让备份/升级直接出现在模型面上，
// 多标一行则让日常功能凭空消失，两种错都没有编译告警。
func TestDomainIntentExposureIsDeliberate(t *testing.T) {
	open, gated := 0, 0
	for _, s := range domainIntentCatalog {
		reason, listed := gatedDomainIntents[s.Name]
		switch {
		case s.DefaultOff && !listed:
			t.Errorf("意图 %s 标记了 DefaultOff 却未在 gatedDomainIntents 登记理由", s.Name)
		case listed && !s.DefaultOff:
			t.Errorf("意图 %s 已登记为高危却漏标 DefaultOff", s.Name)
		case listed && reason == "":
			t.Errorf("意图 %s 的高危理由为空", s.Name)
		}
		if s.DefaultOff {
			gated++
		} else {
			open++
		}
	}
	if len(gatedDomainIntents) != gated {
		t.Fatalf("gatedDomainIntents 登记 %d 条，实际 DefaultOff 意图 %d 个", len(gatedDomainIntents), gated)
	}
	if open < minOpenDomainIntents {
		t.Fatalf("补齐域默认开放仅 %d 个（下限 %d），默认可见面可能又被整体关回去了",
			open, minOpenDomainIntents)
	}
	t.Logf("补齐域意图 %d 个：默认开放 %d，需显式开启 %d", len(domainIntentCatalog), open, gated)
}

// minOpenDomainIntents 是补齐域默认开放意图数的下限，低于它就判为「整体关回去」。
//
// 下限从 10 下调到 8（2026-10-07）：站点未发行交易域，commerce / commerce_order
// 与 contentops_material / contentops_translate 一并转入默认关闭，
// 补齐域开放数由 12 降到 8。**这不是放宽护栏**，恰恰相反——
// 单纯下调下限会让「继续往下降」变得无声，所以下面配了
// TestDefaultOpenSurfaceStaysSmall 按 token 量级设了一道更紧的护栏，
// 把「默认可见面过大」和「过小」两个方向同时看住。
const minOpenDomainIntents = 8

// TestDefaultOpenSurfaceStaysSmall 给默认可见面加一道**规模**护栏。
//
// 为什么需要它：`minOpenDomainIntents` 只能挡住「关得太多」，
// 挡不住「开得太多」——把某个高危意图从 gatedDomainIntents 挪回默认开放，
// 数量反而上升，minOpenDomainIntents 不会报警。
//
// 而默认可见面的大小直接等于每轮请求的常驻工具定义体积
// （2026-10-05 实测：49 个意图的工具面约 15k token，纯结构开销占 35%、
// desc 占 58%，且工具定义是每轮都要重发的）。所以这里按 token 量级设上限，
// 任何「顺手多开几个」都会立刻触发。
//
// 上限取 3k token 对应约 10 个默认开放意图（按实测每意图 200~400 token
// 估），留了一点余量给后续正常新增常用意图。
func TestDefaultOpenSurfaceStaysSmall(t *testing.T) {
	const maxOpenTokens = 3000

	total := 0
	var names []string
	for _, s := range domainIntentCatalog {
		if s.DefaultOff {
			continue
		}
		names = append(names, s.Name)
		total += approxTokenCount(s.Desc)
		for _, p := range s.Params {
			total += approxTokenCount(p.Desc)
			for _, e := range p.Enum {
				total += approxTokenCount(e)
			}
		}
	}
	// 结构性开销（name/type/properties 等）按实测约占 desc 文本的 60%，
	// 这里按同样比例补上，避免只看 desc 文本而低估真实体积。
	estimated := total * 8 / 5
	if estimated > maxOpenTokens {
		sort.Strings(names)
		t.Fatalf("补齐域默认可见面估算 ~%d token，超过上限 %d。默认开放 %d 个：%v\n"+
			"新增默认开放意图前请先算token 成本——每轮请求都要重发这份工具定义。",
			estimated, maxOpenTokens, len(names), names)
	}
	t.Logf("默认可见面估算 ~%d token（上限 %d），%d 个意图：%v",
		estimated, maxOpenTokens, len(names), names)
}

// approxTokenCount 是 token 的粗估：中文按 1.1/字、ASCII 按 1/3.5 字符。
// 与 cmd 下的实测探针同一口径，够用于规模护栏的量级判断。
func approxTokenCount(s string) int {
	var cn, other int
	for _, r := range s {
		if r > 0x4E00 && r <= 0x9FFF {
			cn++
		} else {
			other++
		}
	}
	return cn*11/10 + other*10/35
}

// TestCommonIntentsVisibleWithoutWhitelist 站点未配置白名单时，常用意图必须出现在工具面，
// 高危意图必须仍然缺席 —— 这条是"默认开放常用能力"改造的正反两面。
func TestCommonIntentsVisibleWithoutWhitelist(t *testing.T) {
	f := &fakeCap{}
	k := NewKernel(Config{}, f.invoker(), nil)
	srv, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	k.RegisterAll(srv.GetServer())

	for _, name := range defaultOpenDomainIntents {
		if !k.IsRegistered(name) {
			t.Errorf("常用意图 %s 未默认可见", name)
		}
	}
	for name := range gatedDomainIntents {
		if k.IsRegistered(name) {
			t.Errorf("高危意图 %s 在未配置白名单时被暴露了", name)
		}
	}
	// 两个清单必须刚好覆盖全部补齐域意图：新增意图不表态就会在这里失败。
	declared := len(defaultOpenDomainIntents) + len(gatedDomainIntents)
	if declared != len(domainIntentCatalog) {
		t.Fatalf("默认开放 %d + 需显式开启 %d = %d，与补齐域意图总数 %d 不符（新增意图未表态）",
			len(defaultOpenDomainIntents), len(gatedDomainIntents), declared, len(domainIntentCatalog))
	}
}

// TestRecommendedExposedDoesNotNarrowDefaults 推荐清单不得要求显式开启高危意图。
// 它的定位是"日常运营起步面"，若混进一个 DefaultOff 意图，填入它反而把用户领到
// 备份/资金/对外发信这类高风险能力上，与"保守起步"的说明正好相反。
func TestRecommendedExposedDoesNotNarrowDefaults(t *testing.T) {
	for _, n := range RecommendedExposed() {
		spec, ok := specByName(n)
		if !ok {
			continue // 名字是否真实存在由 provider 侧 TestDomainIntentDefaultExposure 负责
		}
		if spec.DefaultOff {
			t.Errorf("推荐白名单包含默认关闭的意图 %s", n)
		}
	}
}

// defaultOpenDomainIntents 是补齐域中必须默认可见的意图（人工维护的策略表述）。
// 与 gatedDomainIntents 互补：两者之和必须等于补齐域意图总数，新增意图不表态即失败。
//
// 2026-10-07：contentops_material / contentops_translate / commerce / commerce_order
// 移出本名单（转入 gatedDomainIntents，默认关闭）。这条名单的语义是
// 「站点不配任何白名单时也必须能用的能力」，而这四个都属误操作后果超出
// 「改回来」范围的类别——素材池/译文批量改写、会员与订单写操作。
var defaultOpenDomainIntents = []string{
	"content_place", "seo", "seo_jsonld", "seo_llms", "interaction",
	"siteops_maintain", "account", "design_manage",
}

// ── 非补齐域意图的暴露策略（2026-10-07 新增）──
//
// 补齐域（domainIntentCatalog）有 gatedDomainIntents + defaultOpenDomainIntents
// 两张互斥表把守，但**非补齐域**的意图（catalog.go / catalog_merged.go 里的
// content_article、skill、fs_*、system_plugin 等19 个）此前完全在门禁视野之外——
// 反向对照实测：把 skill 改回 DefaultOff:true，不会有任何测试报警。
//
// 这类意图的风险等级差异极大（content_article 是日常核心，system_plugin 是
// 主机级迁移），却没人要求它们表态。所以这里补一张表。
var openNonDomainIntents = map[string]string{
	"content_article":    "文档写作链路核心，站点日常最高频的意图",
	"content_manage":     "分类/标签/单页/模型的结构维护，日常写作必需",
	"media":              "附件上传与查询，AI 配图链路必需",
	"structure":          "导航/友链/重定向，站点结构的基础设置",
	"system_config":      "站点配置读写（内容/联系方式等日常项）",
	"skill":              "本地技能：list/get/reload/save 是 AI 按站点约定写作的前提；" +
		"唯一的不可逆动作 delete 已由审批门 + 端点字符白名单 + 只删 skills/<name> 三重覆盖",
	"agent":              "Agent 与任务的查询与调度，只读为主",
	"web":                "联网搜索与抓取，只读低危",
	"seo_keyword":        "SEO 关键词维护，站点日常运营",
	"seo_anchor":         "SEO 锚文本维护，站点日常运营",
	"traffic_statistics": "统计只读",
}

// gatedNonDomainIntents 是非补齐域中默认关闭的意图及理由。
//
// 与 openNonDomainIntents 互为 complements：非补齐域意图必须二选一。
// 2026-10-07 新增——此前这一类意图完全在门禁视野之外，
// 把system_plugin 从 DefaultOff 改回开放，不会有任何测试报警。
var gatedNonDomainIntents = map[string]string{
	"system_plugin": "只剩主机级/不可逆动作：建缓存索引、重建全文索引、导出整站备份、迁移数据库",
	"api":           "通用 REST 调用：一次开放 402 个端点（含写与破坏性操作），等价于 shell",
}

// TestNonDomainIntentsExposureIsDeliberate 为非补齐域意图补上暴露门禁。
//
// 双向校验，与补齐域同构：标记 DefaultOff 却不在本表、或在本表却没标
// DefaultOff，都会失败。新增非补齐域意图必须显式二选一。
func TestNonDomainIntentsExposureIsDeliberate(t *testing.T) {
	inDomain := map[string]bool{}
	for _, s := range domainIntentCatalog {
		inDomain[s.Name] = true
	}
	seen := map[string]bool{}
	for _, s := range IntentCatalog {
		if inDomain[s.Name] {
			continue
		}
		reason, listed := openNonDomainIntents[s.Name]
		gatedReason, gated := gatedNonDomainIntents[s.Name]
		switch {
		case s.DefaultOff && !gated:
			t.Errorf("非补齐域意图 %s 标记了 DefaultOff 却未在 gatedNonDomainIntents 登记理由", s.Name)
		case gated && !s.DefaultOff:
			t.Errorf("非补齐域意图 %s 已登记为默认关闭却漏标 DefaultOff", s.Name)
		case gated && gatedReason == "":
			t.Errorf("非补齐域意图 %s 的关闭理由为空", s.Name)
		case listed && s.DefaultOff:
			t.Errorf("非补齐域意图 %s 已登记为默认开放却标了 DefaultOff", s.Name)
		case listed && reason == "":
			t.Errorf("非补齐域意图 %s 的开放理由为空", s.Name)
		}
		seen[s.Name] = true
	}
	for name := range openNonDomainIntents {
		if !seen[name] {
			t.Errorf("openNonDomainIntents 登记了不存在的非补齐域意图 %s", name)
		}
	}
	for name := range gatedNonDomainIntents {
		if !seen[name] {
			t.Errorf("gatedNonDomainIntents 登记了不存在的非补齐域意图 %s", name)
		}
	}
}

// TestSkillStaysOpenByDefault 单独钉住 skill 的默认开放。
//
// 2026-10-07 把它从 DefaultOff 改为默认开放。理由见 openNonDomainIntents。
// 单列一条是因为它与其它非补齐域意图不同：这是一次**有意放宽**，
// 将来若有人以「skill 涉及文件系统」为由改回 DefaultOff，
// 应当被这条测试拦住并要求先讨论，而不是静默生效。
func TestSkillStaysOpenByDefault(t *testing.T) {
	spec, ok := specByName("skill")
	if !ok {
		t.Fatal("skill 意图不存在")
	}
	if spec.DefaultOff {
		t.Error("skill 应默认开放：AI 看不见技能就退回裸写作，无法按站点约定产出内容。" +
			"如需收紧请先讨论 skill 域的整体暴露策略")
	}
}

// TestSystemPluginStaysGated 钉住 system_plugin 的默认关闭。
//
// 2026-10-07 起它只剩四个主机级动作（htmlcache_build / fulltext_rebuild /
// backup_dump / migrate_db），每个误操作后果都超出「改回来」范围。
// 它的 DefaultOff 不在 gatedDomainIntents 里（那表只管补齐域），
// 所以在这里单独锁住，防止有人顺手把整条意图挪回默认开放。
func TestSystemPluginStaysGated(t *testing.T) {
	spec, ok := specByName("system_plugin")
	if !ok {
		t.Fatal("system_plugin 意图不存在")
	}
	if !spec.DefaultOff {
		t.Error("system_plugin 应默认关闭：它现在只剩主机级/不可逆动作" +
			"（建缓存索引/重建全文索引/导出整站备份/迁移数据库）")
	}
	// robots 已迁走，这里顺带守住迁移的完整性：两个动作不该再出现。
	for _, a := range spec.Params["action"].Enum {
		if a == "robots_get" || a == "robots_set" {
			t.Errorf("system_plugin 不应再暴露 %s：robots 读写已迁到 siteops_maintain", a)
		}
	}
}

// TestRobotsMovedToSiteopsMaintain 钉住 robots 的迁移结果。
//
// 迁移涉及三层：action 枚举、端点路由、参数改名。任一层漏掉都会让
// 「看得见却调不动」或直接报错，所以三层一起断言。
func TestRobotsMovedToSiteopsMaintain(t *testing.T) {
	maintain, ok := specByName("siteops_maintain")
	if !ok {
		t.Fatal("siteops_maintain 意图不存在")
	}
	// 1. action 枚举含两个 robots 动作
	acts := map[string]bool{}
	for _, a := range maintain.Params["action"].Enum {
		acts[a] = true
	}
	for _, a := range []string{"robots_get", "robots_set"} {
		if !acts[a] {
			t.Errorf("siteops_maintain 的 action 枚举缺少 %s（robots 迁移未完成）", a)
		}
	}
	// 2. content 参数仍在（robots_set 的正文）
	if _, has := maintain.Params["content"]; !has {
		t.Error("siteops_maintain 缺少 content 参数：robots_set 无处传正文")
	}
	// 3. 路由真的声明到 plugin/robots 端点
	//
	// 注意比的是**声明原文**：invokeRoutes 在执行时才调 normalizeEndpointPath
	// 补 /system/api 前缀，路由表里存的是短路径。写成带前缀的形式会永远失败。
	//
	// 端点是否真实存在由 provider 侧 TestActionRisk_AuditAllIntents 穷举校验
	// （api_catalog.json 在 provider 包，intent 包访问不到）。
	routes := intentRouteRegistry["siteops_maintain"]
	for action, want := range map[string]string{
		"robots_get": "GET /plugin/robots",
		"robots_set": "POST /plugin/robots",
	} {
		got, declared := routes[action]
		if !declared {
			t.Errorf("siteops_maintain 缺少 %s 的端点路由声明", action)
			continue
		}
		if got != want {
			t.Errorf("%s 路由应声明为 %q，实际 %q", action, want, got)
		}
	}
	// 4. **实跑一次** robots_set，验证 content 真的落到了端点字段 robots。
	//
	//    这是迁移最容易断的一环：端点 request.PluginRobotsConfig 的字段名是 robots，
	//    意图层参数叫 content，invokeRoutes 靠第三个参数的 rename 表换名。
	//    漏了 rename 不会编译失败、不会 panic，只会让 POST 收到空正文——
	//    而 robots.txt 被清空是要等搜索引擎重新抓取后才发现的事故。
	//    所以这里断言实际下发的参数，而不是断言内部表里有改名记录。
	c := &captureInvoker{}
	if _, err := maintain.Compose(context.Background(),
		map[string]any{"action": "robots_set", "content": "User-agent: *\nDisallow:"}, c.invoke); err != nil {
		t.Fatalf("robots_set 执行失败: %v", err)
	}
	if c.name != "api_invoke" {
		t.Fatalf("robots_set 应经 api_invoke 落到真实端点，实际 %s", c.name)
	}
	if c.args["method"] != "POST" || c.args["path"] != "/system/api/plugin/robots" {
		t.Fatalf("robots_set 端点分派错误: %#v", c.args)
	}
	params, _ := c.args["params"].(map[string]any)
	if got := params["robots"]; got != "User-agent: *\nDisallow:" {
		t.Errorf("正文未改名为 robots 落到端点：params=%#v（端点字段名是 robots，"+
			"意图参数是 content，不改名则 robots.txt 被清空）", params)
	}
	if _, leaked := params["content"]; leaked {
		t.Error("content 不应原样出现在端点参数里（端点不认该字段名）")
	}
}
