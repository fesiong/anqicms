package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// 这组测试锁住 2026-10-03 修复的三个线上实测问题，全部属于「静默给错数据」类别：
//
//  1. save 更新时未传的字段被端点清空（正文变空串 / module_id 变 0 报「未定义模型」）；
//  2. list 的 status 传非法值时端点不加过滤，静默返回全部草稿；
//  3. list 响应的 total 被 InvokeResult 丢弃，调用方无法翻页。
//
// 它们共同的特点是**不报错但结果错**，AI 侧完全无从察觉，因此必须有断言兜住。




// TestNormalizeListStatus 白名单外的 status 必须报错，不能透传给端点。
//
// 端点 ArchiveList 只对精确的 "ok"/"draft"/"plan" 做处理，其它值一律
// 「查 archive_drafts 且不加 status 过滤」。实测传 "1" 返回的是全部草稿
// （status 0 与 99 混杂），而调用方以为自己在查正式文档。
func TestNormalizeListStatus(t *testing.T) {
	cases := []struct {
		name    string
		in      any
		want    any // nil 表示应删除该键
		wantErr bool
	}{
		{"ok 原样", "ok", "ok", false},
		{"数字 1 归一为 ok", "1", "ok", false},
		{"draft", "draft", "draft", false},
		{"数字 0 归一为 draft", "0", "draft", false},
		{"plan", "plan", "plan", false},
		{"大小写不敏感", "OK", "ok", false},
		{"publish 归一为 ok", "publish", "ok", false},
		{"布尔 true 归一为 draft", true, "draft", false},
		{"布尔 false 归一为 ok", false, "ok", false},
		{"空串删除该键", "", nil, false},
		{"非法值报错", "99", nil, true},
		{"中文报错", "已发布", nil, true},
	}
	for _, c := range cases {
		sub := map[string]any{"status": c.in}
		err := normalizeListStatus(sub)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: 应报错，实际通过（会被端点当成「查全部草稿」）", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 不应报错：%v", c.name, err)
			continue
		}
		if c.want == nil {
			if _, ok := sub["status"]; ok {
				t.Errorf("%s: 该键应被删除，实际=%#v", c.name, sub["status"])
			}
			continue
		}
		if sub["status"] != c.want {
			t.Errorf("%s: status = %#v，期望 %#v", c.name, sub["status"], c.want)
		}
	}
}

// TestNormalizeListStatusNoKey 不传 status 时不能凭空加键
// （端点默认就是 ok，多加反而可能改变行为）。
func TestNormalizeListStatusNoKey(t *testing.T) {
	sub := map[string]any{"page": 1}
	if err := normalizeListStatus(sub); err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if _, ok := sub["status"]; ok {
		t.Errorf("不该凭空加 status，实际=%#v", sub)
	}
}

// TestContentArticleListRejectsBadStatus 非法 status 必须在 Compose 层就拦住，
// 不能走到端点。
func TestContentArticleListRejectsBadStatus(t *testing.T) {
	called := false
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		called = true
		return `{"ok":true,"data":{"code":0,"msg":"","total":0,"data":[]}}`, nil
	}
	spec := mustSpec(t, "content_article")
	if _, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "status": "99"}, inv); err == nil {
		t.Error("非法 status 应返回 error")
	}
	if called {
		t.Error("非法 status 不应打到端点")
	}
}

// TestContentArticleListKeepsTotal list 必须回传分页信息，否则调用方
// 拿到一个裸数组，既不知道总数也无法翻页。
//
// 实测：ArchiveList 返回 {"code","msg","total","exact","data":[...]}，
// total 在信封顶层；InvokeResult 过去只解 code/msg/data，total 直接消失。
func TestContentArticleListKeepsTotal(t *testing.T) {
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true,"status":200,"code":0,"msg":"","total":268,"exact":true,
			"data":[{"id":1,"title":"A"},{"id":2,"title":"B"}]}`, nil
	}
	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "page": 1, "page_size": 2}, inv)
	if err != nil {
		t.Fatalf("list 不应报错：%v", err)
	}
	d, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data 应为含分页信息的对象，实际=%T（裸数组说明 total 被丢了）", res.Data)
	}
	if fmt.Sprint(d["total"]) != "268" {
		t.Errorf("total = %#v，期望 268", d["total"])
	}
	if lst, ok := d["list"].([]any); !ok || len(lst) != 2 {
		t.Errorf("list 应为 2 条，实际=%#v", d["list"])
	}
	if fmt.Sprint(d["page"]) != "1" || fmt.Sprint(d["page_size"]) != "2" {
		t.Errorf("page/page_size 回显不对：%#v / %#v", d["page"], d["page_size"])
	}
	// count 供调用方判断「本页是否还有更多」
	if fmt.Sprint(d["count"]) != "2" {
		t.Errorf("count = %#v，期望 2", d["count"])
	}
}

// TestListWithTotalNoTotal 端点没给 total 时保持裸数组，不硬造假数字。
func TestListWithTotalNoTotal(t *testing.T) {
	arr := []any{map[string]any{"id": 1}}
	got := listWithTotal(arr, map[string]any{}, map[string]any{"page": 1})
	if _, isMap := got.(map[string]any); isMap {
		t.Errorf("无 total 时应保持原形状，实际=%T", got)
	}
}

// TestContentArticleSaveAlwaysReportsStatus 回执必须带 status，
// 否则调用方无法判断这篇是草稿还是已发布。
//
// 实测：更新正式文档时端点返回的文档对象里没有 status 键，
// 回执只有 action/id/link/title。
func TestContentArticleSaveAlwaysReportsStatus(t *testing.T) {
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if fmt.Sprint(m["method"]) == "GET" {
			return `{"ok":true,"data":{"code":0,"msg":"","data":{"id":7,"module_id":1,"category_id":13}}}`, nil
		}
		// 端点返回的文档对象里没有 status 键
		return `{"ok":true,"data":{"code":0,"msg":"","data":{"id":7,"title":"T","link":"http://h/zixun/7.html"}}}`, nil
	}
	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "save", "id": 7, "title": "T"}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	d, _ := res.Data.(map[string]any)
	if d == nil {
		t.Fatalf("Data 应为 map，实际=%T", res.Data)
	}
	if d["status"] != "ok" {
		t.Errorf("非草稿保存应回填 status=ok，实际=%#v", d["status"])
	}
}

// TestContentArticleSaveDraftReportsStatus draft=true 的回执必须是 draft。
func TestContentArticleSaveDraftReportsStatus(t *testing.T) {
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true,"data":{"code":0,"msg":"","data":null}}`, nil
	}
	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(), map[string]any{
		"action": "save", "title": "草稿", "content": "c", "draft": true,
	}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	d, _ := res.Data.(map[string]any)
	if d == nil {
		t.Fatalf("Data 应为 map，实际=%T", res.Data)
	}
	if d["status"] != "draft" {
		t.Errorf("草稿保存应回填 status=draft，实际=%#v", d["status"])
	}
}

// TestListWithTotalShapeIsJSONObject 回执形状要能被 JSON 序列化，
// 且列表项落在 data.list（与 desc 承诺一致）。
func TestListWithTotalShapeIsJSONObject(t *testing.T) {
	got := listWithTotal(
		[]any{map[string]any{"id": float64(1)}},
		map[string]any{"total": float64(1), "exact": true},
		map[string]any{"page": float64(1), "page_size": float64(20)},
	)
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("回执应可序列化：%v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("不应反序列化失败：%v", err)
	}
	if _, ok := back["list"]; !ok {
		t.Errorf("应含 list 键，实际=%s", b)
	}
}

// TestContentArticleRequiredParams 缺必填参数时必须在 Compose 层报明确的错，
// 而不是让端点回一句含义不明的「record not found」让调用方误判成文档不存在。
func TestContentArticleRequiredParams(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"get 缺 id", map[string]any{"action": "get"}},
		{"get id=0", map[string]any{"action": "get", "id": 0}},
		{"delete 缺 id", map[string]any{"action": "delete"}},
		{"publish 缺 id", map[string]any{"action": "publish", "status": "ok"}},
		{"save 新建缺 title", map[string]any{"action": "save", "content": "c"}},
		{"save 新建缺 content", map[string]any{"action": "save", "title": "t"}},
		{"save 新建 title 为空白", map[string]any{"action": "save", "title": "  ", "content": "c"}},
	}
	called := false
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		called = true
		return `{"ok":true,"data":{"code":0,"msg":"","data":null}}`, nil
	}
	spec := mustSpec(t, "content_article")
	for _, c := range cases {
		called = false
		_, err := spec.Compose(context.Background(), c.args, inv)
		if err == nil {
			t.Errorf("%s: 应返回 error（缺必填参数）", c.name)
			continue
		}
		if called {
			t.Errorf("%s: 缺参数时不应打到端点", c.name)
		}
	}
}

// TestListEmptyResultKeepsShape 命中 0 条时也必须给出完整的分页对象，
// 而不是 data:null——调用方需要靠形状稳定来区分「没有数据」和「调用出错」。
//
// 实测：status=plan（库中无待发布文档）时端点回 data:null。
func TestListEmptyResultKeepsShape(t *testing.T) {
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true,"data":null,"exact":true,"total":0}`, nil
	}
	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "status": "plan"}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	d, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("空结果也应返回分页对象，实际=%#v", res.Data)
	}
	lst, ok := d["list"].([]any)
	if !ok || len(lst) != 0 {
		t.Errorf("list 应为空数组而非 null，实际=%#v", d["list"])
	}
	if fmt.Sprint(d["total"]) != "0" {
		t.Errorf("total = %#v，期望 0", d["total"])
	}
}

// TestSwitchComposeKeepsListTotal switchCompose 覆盖 30+ 个工具的列表类动作，
// 过去只取 envelope["data"]，把信封顶层的 total 静默丢弃。
//
// 实测（2026-10-03）：content_manage 的 category_list/tag_list/module_list/page_list、
// media list、structure nav_list 全部返回裸数组，调用方无法判断是否还有下一页。
// 后台有 34 个分页端点一律把 total 放信封顶层，所以这是系统性问题而非个例。
func TestSwitchComposeKeepsListTotal(t *testing.T) {
	cases := []struct {
		name   string
		tool   string
		action string
	}{
		{"标签列表", "content_manage", "tag_list"},
		{"分类列表", "content_manage", "category_list"},
		{"模型列表", "content_manage", "module_list"},
		{"单页列表", "content_manage", "page_list"},
		{"附件列表", "media", "list"},
		{"导航列表", "structure", "nav_list"},
	}
	for _, c := range cases {
		spec := mustSpec(t, c.tool)
		inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return `{"ok":true,"code":0,"msg":"","total":42,"exact":true,
				"data":[{"id":1},{"id":2}]}`, nil
		}
		res, err := spec.Compose(context.Background(),
			map[string]any{"action": c.action, "page": 1, "page_size": 2}, inv)
		if err != nil {
			t.Errorf("%s(%s): 不应报错：%v", c.name, c.action, err)
			continue
		}
		d, ok := res.Data.(map[string]any)
		if !ok {
			t.Errorf("%s(%s): 应返回含分页信息的对象，实际=%T（total 被丢弃）",
				c.name, c.action, res.Data)
			continue
		}
		if fmt.Sprint(d["total"]) != "42" {
			t.Errorf("%s(%s): total 应为 42，实际=%#v", c.name, c.action, d["total"])
		}
		if lst, ok := d["list"].([]any); !ok || len(lst) != 2 {
			t.Errorf("%s(%s): list 应含 2 条，实际=%#v", c.name, c.action, d["list"])
		}
	}
}

// TestSwitchComposeNonListKeepsShape 非列表动作不能被套上分页对象。
//
// detail/save 这类动作即使端点碰巧返回了 total，形状也不该变——
// 否则调用方要针对不同动作猜两套形状。
func TestSwitchComposeNonListKeepsShape(t *testing.T) {
	spec := mustSpec(t, "content_manage")
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		// 故意带上 total，验证非列表动作不会被 listWithTotal 处理
		return `{"ok":true,"code":0,"msg":"","total":42,"data":{"id":7,"title":"分类"}}`, nil
	}
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "category_get", "id": 7}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if _, wrapped := res.Data.(map[string]any); !wrapped {
		t.Fatalf("Data 应为端点返回的对象，实际=%T", res.Data)
	}
	// 断言它没被包成 {list:..., total:...}
	d := res.Data.(map[string]any)
	if _, hasList := d["list"]; hasList {
		t.Errorf("非列表动作不应被包成 {list:...}，实际=%#v", d)
	}
	if d["id"] == nil {
		t.Errorf("非列表动作应保留端点原字段，实际=%#v", d)
	}
}

// TestIsListAction 判定表：只放宽到"看起来是列表"，宁可漏判不可误判。
func TestIsListAction(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"list", true}, {"tag_list", true}, {"category_list", true},
		{"admin_list", true}, {"LIST", true}, {"Tag_List", true},
		{"get", false}, {"detail", false}, {"save", false}, {"delete", false},
		{"", false}, {"publish", false},
		// 只认精确的 list 与 _list 后缀：listing/blacklist 这类不该被误判成列表动作
		{"listing", false}, {"blacklist", false},
	}
	for _, c := range cases {
		if got := isListAction(c.in); got != c.want {
			t.Errorf("isListAction(%q) = %v，期望 %v", c.in, got, c.want)
		}
	}
}

// TestSwitchComposeListWithoutTotal 端点没给 total 时保持裸数组，
// 不能硬造一个 total=0 之类的假数字。
func TestSwitchComposeListWithoutTotal(t *testing.T) {
	spec := mustSpec(t, "content_manage")
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true,"code":0,"msg":"","data":[{"id":1}]}`, nil
	}
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "tag_list"}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if _, isMap := res.Data.(map[string]any); isMap {
		t.Errorf("端点未提供 total 时应保持裸数组，实际=%#v（不该硬造假数字）", res.Data)
	}
}

// TestInvokeRoutesKeepsListTotal invokeRoutes 是第三条数据提取路径
// （前两条是 switchCompose 与 contentArticleCompose），过去同样只取 data。
//
// 实测：contentops_translate action=logs 打的是 GET /plugin/translate/logs，
// 该端点明确回信封顶层 total（controller/manageController/pluginTranslate.go:79），
// 但 MCP 侧拿到的响应里没有 total。
//
// 这条路径的 action 命名五花八门（logs/texts/caches/statistic…），
// 所以判定不能靠命名——listWithTotal 以"端点有没有回 total"为准。
func TestInvokeRoutesKeepsListTotal(t *testing.T) {
	spec := mustSpec(t, "contentops_translate")
	inv := func(_ context.Context, name string, m map[string]any) (string, error) {
		if name != "api_invoke" {
			t.Errorf("应经 api_invoke 调用，实际 %q", name)
		}
		if m["path"] != "/system/api/plugin/translate/logs" {
			t.Errorf("path = %v", m["path"])
		}
		return `{"ok":true,"code":0,"msg":"","total":88,"exact":true,
			"data":[{"id":1},{"id":2},{"id":3}]}`, nil
	}
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "logs", "page": 1, "page_size": 3}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	d, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("应返回含分页信息的对象，实际=%T", res.Data)
	}
	if fmt.Sprint(d["total"]) != "88" {
		t.Errorf("total 应为 88，实际=%#v", d["total"])
	}
	if lst, ok := d["list"].([]any); !ok || len(lst) != 3 {
		t.Errorf("list 应含 3 条，实际=%#v", d["list"])
	}
}

// TestInvokeRoutesNonListUnaffected 非分页动作（get/save）形状不变。
func TestInvokeRoutesNonListUnaffected(t *testing.T) {
	spec := mustSpec(t, "contentops_translate")
	inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true,"code":0,"msg":"","data":{"baidu":{"app_id":"x"}}}`, nil
	}
	res, err := spec.Compose(context.Background(), map[string]any{"action": "get"}, inv)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	d, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data 应为对象，实际=%T", res.Data)
	}
	if _, wrapped := d["list"]; wrapped {
		t.Errorf("非分页动作不应被包成 {list:...}，实际=%#v", d)
	}
	if d["baidu"] == nil {
		t.Errorf("应保留端点原字段，实际=%#v", d)
	}
}











// TestInvokeRoutesPartialForUser commerce 的 user_update 走 invokeRoutes
// （不是 switchCompose），且打到 PluginUserDetailForm —— pluginUser.go:242
// 原本无条件 req.UpdateAll = true，SaveUserInfo 里 15 处都是
// `if req.UpdateAll || req.X != ""`，所以不传就清零。
//
// 实测（2026-10-03）会员 id=1564「akun666」只改 user_name 后：
// email/phone/invite_code 被清空、status 从 1 变 0（**账号被禁用**），
// 而端点回 ok=true。危害比分类那次更大。
//
// 修法演变（三步）：
//  1. desc 里警告「这是全量覆盖，自己先 get 再整体写回」——把风险推给调用方，
//     实测 AI 不会可靠照做；
//  2. invokeRouteOverwriteFields 登记该端点做补齐兜底；
//  3. 认清 UpdateAll 是有开关的 → 改用 partial。
//
// 本测试只锁**意图层该做的判断**：走 partial 的端点**不补齐**。
// partial 的实际注入在 provider 的 injectPartialUpdate（那里是所有通道的
// 必经之处，含通用 api 意图），由 TestInjectPartialUpdateInProvider 覆盖。
func TestInvokeRoutesPartialForUser(t *testing.T) {
	spec := mustSpec(t, "commerce")
	var sent map[string]any
	getCalls := 0
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		p, _ := m["params"].(map[string]any)
		if p == nil {
			p = m
		}
		if fmt.Sprint(m["method"]) == "GET" {
			getCalls++
			return `{"ok":true,"code":0,"data":{"code":0,"data":{"id":1564}}}`, nil
		}
		sent = p
		return `{"ok":true,"code":0,"data":{"code":0,"data":{"id":1564}}}`, nil
	}
	if _, err := spec.Compose(context.Background(), map[string]any{
		"action": "user_update", "id": 1564, "user_name": "新名字",
	}, inv); err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if sent == nil {
		t.Fatal("未捕获到写请求")
	}
	// 关键断言：走 partial 的端点**不该回查旧值补齐**。
	// 补齐多余（端点自己能只覆盖传入字段）且有风险（读端点可能返回派生值）。
	if getCalls != 0 {
		t.Errorf("走 partial 后不该回查旧值补齐，实际查了 %d 次", getCalls)
	}
	// PATCH 的要点：未传字段不该出现在请求里，端点会保持原值。
	for _, f := range []string{"email", "phone", "invite_code", "group_id", "status"} {
		if _, ok := sent[f]; ok {
			t.Errorf("未传的 %s 不该出现在请求里（PATCH 语义下端点保持原值）", f)
		}
	}
	if sent["user_name"] != "新名字" {
		t.Errorf("显式传的字段应原样送达，实际=%#v", sent["user_name"])
	}
}


// TestInvokeRouteOverwriteTableConsistency 覆盖表里的端点必须真有 GET 兄弟，
// 否则 preserveOnInvokeRoute 找不到旧值、登记等于空转。
func TestInvokeRouteOverwriteTableConsistency(t *testing.T) {
	for raw := range invokeRouteOverwriteFields {
		// 表 key 是纯路径：不能带空格分隔的 METHOD 前缀
		if strings.ContainsAny(raw, " \t") || strings.HasSuffix(raw, "/") {
			t.Errorf("表 key 应是不含 METHOD 的纯路径，实际=%q", raw)
		}
		// 登记表里是规范化后的路径，补齐时用的也是规范化结果
		wantKey := "GET " + normalizeEndpointPath(raw)
		if _, found := DeclaredEndpointTargets()[wantKey]; !found {
			t.Errorf("%s 没有对应的 GET 兄弟路由（找 %q），补齐会空转", raw, wantKey)
		}
	}
}

// TestDraftAndPublishedShareSameResponsePath 草稿新建与正式新建走**完全相同**的
// 解析路径，都能从端点响应里直接拿到 id 与 link。
//
// 这条测试的来历：代码里曾有一段注释说「草稿新建时控制器只回
// {"ok":false,"data":null}（实测），文档落在 archive_drafts，通用解析拿不到 id/link」，
// 并据此加了 lookupByTitle 按标题回查的兜底。**该注释与实际行为不符**：
//   - controller/manageController/archive.go:666 的 ArchiveDetailForm 只有一条响应分支，
//     草稿与正式返回结构完全相同；
//   - provider.SaveArchive 末尾（archive.go:713）无条件 `draft.Link = w.GetUrl(...)`，
//     返回的 draft 自带 link；
//   - {"ok":false,…} 其实是 api_invoke 的**外层信封**（res.OK 为 false 时），
//     与草稿无关 —— 混淆了两个层次。
//
// 兜底已删除（它不仅多余，还会在端点真没回 link 时用「标题模糊回查」引入误报风险）。
// 本测试锁住真实行为，防止有人依据旧注释把兜底加回来。
func TestDraftAndPublishedShareSameResponsePath(t *testing.T) {
	// 端点对草稿与正式返回同样形状：都带 id 与 link
	endpointResp := `{"ok":true,"data":{"code":0,"msg":"文档已更新","data":{
		"id":1879,"title":"T","link":"http://h/zixun/1879.html"}}}`
	spec := mustSpec(t, "content_article")
	for _, c := range []struct {
		name       string
		draft      bool
		wantStatus string
		wantSuffix string
	}{
		{"正式", false, "ok", ""},
		{"草稿", true, "draft", "?preview=true"},
	} {
		inv := func(_ context.Context, _ string, _ map[string]any) (string, error) {
			return endpointResp, nil
		}
		res, err := spec.Compose(context.Background(), map[string]any{
			"action": "save", "title": "T", "content": "c", "draft": c.draft,
		}, inv)
		if err != nil {
			t.Errorf("%s: 不应报错：%v", c.name, err)
			continue
		}
		d, _ := res.Data.(map[string]any)
		if d == nil {
			t.Errorf("%s: Data 应为 map", c.name)
			continue
		}
		// 两者都必须直接拿到 id 与 link，不需要任何回查兜底
		if fmt.Sprint(d["id"]) != "1879" {
			t.Errorf("%s: 应直接从端点响应拿到 id=1879，实际=%#v（说明又依赖了兜底）", c.name, d["id"])
		}
		link, _ := d["link"].(string)
		if link == "" {
			t.Errorf("%s: 应直接从端点响应拿到 link，实际为空（说明又依赖了兜底）", c.name)
		}
		if d["status"] != c.wantStatus {
			t.Errorf("%s: status = %#v，期望 %q", c.name, d["status"], c.wantStatus)
		}
		if !strings.HasSuffix(fmt.Sprint(d["link"]), c.wantSuffix) {
			t.Errorf("%s: link 应以 %q 结尾，实际=%#v", c.name, c.wantSuffix, d["link"])
		}
	}
}

// TestInvokeRoutesPreserveMaterial contentops_material 的 save 走 invokeRoutes，
// 打到 PluginMaterialDetailForm → provider.SaveMaterial。
//
// 关键：**没有 UpdateAll**，但 provider/material.go:67-70 是逐字段无条件赋值
// （material.Title = req.Title / material.Content = req.Content…），效果等价。
// 所以「搜 UpdateAll 找全量覆盖点」会漏掉这一类。
//
// 实测（2026-10-03）素材 id=2「请输入验证码以便正常访问」只传 id+title 改名后：
// content 被清空（正文丢失）。
//
// ⚠️ status **不在**保护列表里：provider/material.go:68 是硬编码的
// `material.Status = 1`，根本不读 req.Status —— 端点语义就是「保存即启用」，
// 补齐传 0 也会被覆盖成 1，登记了也白登记（实测确认过）。
func TestInvokeRoutesPreserveMaterial(t *testing.T) {
	spec := mustSpec(t, "contentops_material")
	var sent map[string]any
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if fmt.Sprint(m["method"]) == "GET" {
			return `{"ok":true,"code":0,"data":{"code":0,"data":{
				"id":2,"title":"请输入验证码以便正常访问",
				"content":"您的IP是：163.125.126.30…","category_id":0,
				"status":0,"auto_update":0}}}`, nil
		}
		p, _ := m["params"].(map[string]any)
		if p == nil {
			p = m
		}
		sent = p
		return `{"ok":true,"code":0,"data":{"code":0,"data":{"id":2}}}`, nil
	}
	if _, err := spec.Compose(context.Background(), map[string]any{
		"action": "save", "id": 2, "title": "改名",
	}, inv); err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if sent == nil {
		t.Fatal("未捕获到写请求")
	}
	if c, _ := sent["content"].(string); !strings.Contains(c, "163.125.126.30") {
		t.Errorf("content 应被旧值补齐（正文不能丢），实际=%#v", sent["content"])
	}
	// status 不该被补：端点硬编码为 1，补了也会被覆盖（见函数注释）
	if _, provided := sent["status"]; provided {
		t.Errorf("status 不应补齐（material.go:68 硬编码 =1），实际=%#v", sent["status"])
	}
}

// TestOverwriteTableOnlyDataEndpoints 覆盖表只应收录**数据类**端点。
//
// 纯配置类（推送地址、robots 内容、开关）被清空的代价是功能失效而非数据丢失，
// 多数已在 desc 警告；把它们也塞进表会让每次 save 都多一次 GET 回查，
// 收益不抵开销。若日后要纳入，先确认该端点确有数据字段会丢。
//
// ⚠️ 表里**只收「逐字段无条件赋值」型**端点（provider 里 grep UpdateAll 为 0）。
// 有 UpdateAll 开关的走 partialInvokePaths —— 端点自己就能只覆盖传入字段，
// 补齐多余且有风险（要回查旧值，而读端点可能返回派生值）。
// 同一个端点**不能**同时出现在两张表里，否则两套机制叠加。
func TestOverwriteTableOnlyDataEndpoints(t *testing.T) {
	// 已知会丢数据字段的端点，按机制分两组
	byPartial := []string{
		"/system/api/plugin/user/detail", // provider/user.go 15 处 req.UpdateAll
	}
	byFill := []string{
		"/system/api/plugin/material/detail",     // material.go 逐字段赋值
		"/system/api/plugin/user/group/detail",   // user.go SaveUserGroupInfo 逐字段赋值
		"/system/api/plugin/place/detail",        // place.go 逐字段赋值
	}
	for _, p := range byFill {
		if _, ok := invokeRouteOverwriteFields[p]; !ok {
			t.Errorf("%s 是逐字段无条件赋值（无 UpdateAll 开关），必须登记到 invokeRouteOverwriteFields", p)
		}
		if partialInvokePaths[p] {
			t.Errorf("%s 同时登记在 partialInvokePaths —— 两套机制会叠加", p)
		}
	}
	for _, p := range byPartial {
		if !partialInvokePaths[p] {
			t.Errorf("%s 的 provider 用 req.UpdateAll（有开关），应走 partial 而非补齐", p)
		}
		if _, ok := invokeRouteOverwriteFields[p]; ok {
			t.Errorf("%s 已走 partial，不该同时登记到补齐表（补齐多余且可能用派生值覆盖真值）", p)
		}
	}
	// 每个登记项都必须真有 GET 兄弟，否则补齐是空转
	for raw := range invokeRouteOverwriteFields {
		wantKey := "GET " + normalizeEndpointPath(raw)
		if _, found := DeclaredEndpointTargets()[wantKey]; !found {
			t.Errorf("%s 没有 GET 兄弟（找 %q），登记后补齐不会生效", raw, wantKey)
		}
	}
}


// TestInvokeRoutesPreservePlace content_place 的 save 走 invokeRoutes，
// 打到 PlaceDetailForm → provider.SavePlace（place.go:93-105 逐字段无条件赋值）。
//
// 实测（2026-10-03）：新建一条带完整字段的临时城市站（id=1），只传 id+title 改名后
// seo_title / keywords / description / template / logo / images / status / latitude
// **8 个字段全被清空**，其中 status 从 1 变 0 相当于把城市站停用。
// 是本轮发现里波及字段最多的一个。测完已删除该临时记录。
func TestInvokeRoutesPreservePlace(t *testing.T) {
	spec := mustSpec(t, "content_place")
	var sent map[string]any
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if fmt.Sprint(m["method"]) == "GET" {
			return `{"ok":true,"code":0,"data":{"code":0,"data":{
				"id":1,"title":"临时站","seo_title":"SEO标题","keywords":"kw1,kw2",
				"description":"原始描述","template":"/tpl/x.html","logo":"http://h/l.png",
				"images":["http://h/1.png"],"status":1,"latitude":31.5,"longitude":121.5,
				"timezone":"Asia/Shanghai","url_token":"tmp","content":"<p>正文</p>",
				"parent_id":0,"sort":0,"is_inherit":1}}}`, nil
		}
		p, _ := m["params"].(map[string]any)
		if p == nil {
			p = m
		}
		sent = p
		return `{"ok":true,"code":0,"data":{"code":0,"data":{"id":1}}}`, nil
	}
	if _, err := spec.Compose(context.Background(), map[string]any{
		"action": "save", "id": 1, "title": "改名",
	}, inv); err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if sent == nil {
		t.Fatal("未捕获到写请求")
	}
	// 逐个核对实测中被清空的字段
	for f, want := range map[string]string{
		"seo_title": "SEO标题", "keywords": "kw1,kw2", "description": "原始描述",
		"template": "/tpl/x.html", "logo": "http://h/l.png",
		"status": "1", "latitude": "31.5", "timezone": "Asia/Shanghai",
	} {
		if got := fmt.Sprint(sent[f]); got != want {
			t.Errorf("%s 应补齐为 %q，实际 %q（端点会清零）", f, want, got)
		}
	}
	if imgs, ok := sent["images"].([]any); !ok || len(imgs) != 1 {
		t.Errorf("images 应补齐为 1 元素数组，实际=%#v", sent["images"])
	}
}

// TestPreserveOnInvokeRouteNeedsExistingRecord 补齐依赖「能按 id 查到旧记录」，
// 查不到时端点会走新建分支（字段全清、status 被置位）。
//
// 踩坑记录（2026-10-03）：验证 content_place 补齐时，我先建站拿到 id=4，
// 却用 id=1 去改名 —— detail 端点回 record not found，补齐自然拿不到旧值，
// 端点按「新建」处理：8 个字段全清、status 被置 1。差点误判成「修复没生效」。
//
// 这条测试把「查不到旧记录就不能补齐」的行为固定下来，并说明失败时该看什么。
func TestPreserveOnInvokeRouteNeedsExistingRecord(t *testing.T) {
	spec := mustSpec(t, "content_place")
	var wrote map[string]any
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if fmt.Sprint(m["method"]) == "GET" {
			// 模拟 record not found
			return `{"ok":false,"code":-1,"msg":"record not found","data":null}`, nil
		}
		p, _ := m["params"].(map[string]any)
		if p == nil {
			p = m
		}
		wrote = p
		return `{"ok":true,"data":null}`, nil
	}
	// 查不到旧记录时不报错、也不补齐：原样把请求交给端点，由端点自己处理
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "save", "id": 999999, "title": "改名"}, inv)
	if err != nil {
		t.Fatalf("查不到旧记录时不应报错（端点自己会处理），实际：%v", err)
	}
	if res == nil {
		t.Fatal("应返回结果对象")
	}
	// 写请求照发（否则调用方会以为失败了），但**不能带补齐来的字段**
	if wrote == nil {
		t.Fatal("仍应把写请求发给端点")
	}
	for _, f := range []string{"seo_title", "template", "logo", "status", "latitude"} {
		if _, filled := wrote[f]; filled {
			t.Errorf("查不到旧记录时 %s 不该被凭空填入（无旧值可依据）", f)
		}
	}
	if fmt.Sprint(wrote["title"]) != "改名" {
		t.Errorf("调用方显式传的 title 应保留，实际=%#v", wrote["title"])
	}
}


// TestPreserveOnUpdateBothNamingAccepted 调用方直接用端点名传时也不该被覆盖。
func TestPreserveOnUpdateBothNamingAccepted(t *testing.T) {
	spec := mustSpec(t, "structure")
	var sent map[string]any
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if fmt.Sprint(m["method"]) == "GET" {
			return `{"ok":true,"data":{"code":0,"data":{
				"id":6,"from_url":"/old-test","to_url":"/new-target"}}}`, nil
		}
		p, _ := m["params"].(map[string]any)
		if p == nil {
			p = m
		}
		sent = p
		return `{"ok":true,"data":null}`, nil
	}
	// 直接用端点名 from_url 传
	if _, err := spec.Compose(context.Background(), map[string]any{
		"action": "redirect_update", "id": 6, "from_url": "/explicit",
	}, inv); err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if got := fmt.Sprint(sent["from_url"]); got != "/explicit" {
		t.Errorf("显式传的端点名 from_url 应保留，实际=%q", got)
	}
}

