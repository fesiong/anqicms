package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// 本文件是 T9a（后台渠道管理 API）的护栏。
//
// 两类用例：
//  1. 路由与菜单用 go/ast 解析真实源码 —— 端点集合的唯一真相源是 route/manage.go，
//     写死端点总数会在 T9b 扩充同一 party 时假失败，所以只断言「必须存在」。
//  2. 掩码 / 站点隔离 / 乐观锁 / 删除级联用真实开发库 —— 这些都不可能在 AST 层验证。

// ============================================================
// 夹具
// ============================================================

// channelAdminTestSite 复用管理器夹具（停掉渠道管理器、补齐渠道相关表），
// 再补建投递表：删除渠道要断言「投递历史保留」。
func channelAdminTestSite(t *testing.T) *Website {
	t.Helper()
	w := channelManagerTestSite(t)
	if err := w.DB.AutoMigrate(&model.NotificationDelivery{}, &model.NotificationRule{}, &model.AgentReceptionPolicy{}); err != nil {
		t.Fatalf("迁移通知/接待策略表失败：%v", err)
	}
	return w
}

func channelAdminSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// newChannelAdminBinding 建一条真实绑定行（T9a 的删除级联以绑定为轴）。
func newChannelAdminBinding(t *testing.T, w *Website, ch *model.MessageChannel) *model.ChannelBinding {
	t.Helper()
	suffix := channelAdminSuffix()
	admin := activeTestAdmin(t, w)
	binding := &model.ChannelBinding{
		SiteId:         w.Id,
		ChannelId:      ch.Id,
		OrgId:          ch.OrgId,
		PlatformUserId: "admin_del_user_" + suffix,
		AdminId:        admin.Id,
		Revision:       1,
		IdentityHash:   bindingIdentityDigest(w.Id, ch.Id, ch.OrgId, "admin_del_user_"+suffix),
	}
	if err := w.DB.Create(binding).Error; err != nil {
		t.Fatalf("创建测试绑定失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", binding.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理绑定 %d 失败：%v", binding.Id, err)
		}
	})
	return binding
}

// ============================================================
// 路由（go/ast）
// ============================================================

// channelRouteParty 是 /channels party 的解析结果。
type channelRouteParty struct {
	middlewares []string
	routes      map[string]string // "METHOD /system/api/channels/..." → handler
}

// parseChannelRouteParty 在 route/manage.go 里定位 `channels := ...Party("/channels", ...)`，
// 返回它的中间件实参（按源码写法）与紧随其后的块里注册的路由。
func parseChannelRouteParty(t *testing.T) channelRouteParty {
	t.Helper()
	path, err := FindManageRoute()
	if err != nil {
		t.Fatalf("定位 route/manage.go 失败：%v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", path, err)
	}

	result := channelRouteParty{routes: map[string]string{}}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				continue
			}
			ident, ok := assign.Lhs[0].(*ast.Ident)
			if !ok || ident.Name != "channels" {
				continue
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok {
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Party" || len(call.Args) == 0 {
				continue
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Value != `"/channels"` {
				continue
			}
			found = true
			for _, arg := range call.Args[1:] {
				result.middlewares = append(result.middlewares, channelExprString(fset, arg))
			}
			if i+1 >= len(block.List) {
				t.Fatalf("/channels party 后没有路由块")
			}
			routesBlock, ok := block.List[i+1].(*ast.BlockStmt)
			if !ok {
				t.Fatalf("/channels party 后紧跟的不是路由块")
			}
			result.routes = channelRoutesInBlock(fset, routesBlock)
		}
		return true
	})
	if !found {
		t.Fatalf("route/manage.go 里没有 channels := ...Party(\"/channels\", ...) 声明")
	}
	return result
}

func channelExprString(fset *token.FileSet, expr ast.Expr) string {
	var sb strings.Builder
	if err := printer.Fprint(&sb, fset, expr); err != nil {
		return ""
	}
	return sb.String()
}

// channelRoutesInBlock 汇总一个 `{ ... }` 块里注册的路由：变量名 + 方法 + 路径 + handler。
func channelRoutesInBlock(fset *token.FileSet, block *ast.BlockStmt) map[string]string {
	out := map[string]string{}
	for _, stmt := range block.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		routePath := strings.Trim(lit.Value, `"`)
		full := strings.TrimRight("/system/api/channels/"+strings.TrimLeft(routePath, "/"), "/")
		if full == "" {
			full = "/system/api/channels"
		}
		out[strings.ToUpper(selector.Sel.Name)+" "+full] = channelExprString(fset, call.Args[len(call.Args)-1])
	}
	return out
}

// t9aChannelRoutes 是 T9a 必须注册的端点集合（规格第 10 节的前六行）。
var t9aChannelRoutes = map[string]string{
	"GET /system/api/channels":              "manageController.ChannelList",
	"GET /system/api/channels/detail":       "manageController.ChannelDetail",
	"POST /system/api/channels/detail":      "manageController.ChannelDetailForm",
	"POST /system/api/channels/delete":      "manageController.ChannelDelete",
	"GET /system/api/channels/status":       "manageController.ChannelStatus",
	"GET /system/api/channels/capabilities": "manageController.ChannelCapabilities",
}

// TestChannelRoutesRequireAdminToken 要求 /channels 的每条路由都经过后台身份中间件。
// 只看 party 的中间件列表是否含 ParseAdminToken：路由级逐个挂 token 解析同样成立，
// 但本仓库现有 party 全部用 party 级中间件，这里锁死该形态以保持一致。
func TestChannelRoutesRequireAdminToken(t *testing.T) {
	party := parseChannelRouteParty(t)

	hasToken := false
	for _, mw := range party.middlewares {
		if mw == "middleware.ParseAdminToken" {
			hasToken = true
		}
	}
	if !hasToken {
		t.Fatalf("/channels party 未挂 middleware.ParseAdminToken（实得 %v）", party.middlewares)
	}

	for route, wantHandler := range t9aChannelRoutes {
		gotHandler, ok := party.routes[route]
		if !ok {
			t.Errorf("缺少路由 %s", route)
			continue
		}
		if gotHandler != wantHandler {
			t.Errorf("路由 %s 的处理器是 %s，期望 %s", route, gotHandler, wantHandler)
		}
	}
	if len(party.routes) == 0 {
		t.Fatal("/channels party 下没有任何路由，断言已空转")
	}
}

// TestChannelMenuRegistered 用 go/ast 校验 config/menu.go 的 plugin 分组里注册了渠道菜单，
// 且没有任何别的菜单抢占同一个 Backend 前缀。
func TestChannelMenuRegistered(t *testing.T) {
	managePath, err := FindManageRoute()
	if err != nil {
		t.Fatalf("定位 route/manage.go 失败：%v", err)
	}
	menuPath := filepath.Join(filepath.Dir(filepath.Dir(managePath)), "config", "menu.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, menuPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", menuPath, err)
	}

	var pluginMenus []*ast.CompositeLit
	var channelBackendOwners []string
	ast.Inspect(file, func(n ast.Node) bool {
		group, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		fields := channelStructFieldLiterals(fset, group)
		if fields == nil {
			return true
		}
		if fields["Key"] == `"plugin"` {
			menusLit, ok := channelCompositeValue(group, fset, "Menus")
			if ok {
				for _, elt := range menusLit.Elts {
					if menu, ok := elt.(*ast.CompositeLit); ok {
						pluginMenus = append(pluginMenus, menu)
					}
				}
			}
		}
		if strings.Contains(fields["Backend"], `"/channels"`) {
			channelBackendOwners = append(channelBackendOwners, fields["Path"])
		}
		return true
	})

	var channelsMenu *ast.CompositeLit
	for _, menu := range pluginMenus {
		fields := channelStructFieldLiterals(fset, menu)
		if fields == nil {
			continue
		}
		if fields["Path"] == `"/plugin/channel/channels"` {
			channelsMenu = menu
			if fields["GroupKey"] != `"plugin"` {
				t.Errorf("渠道菜单 GroupKey 是 %s，期望 \"plugin\"", fields["GroupKey"])
			}
			if fields["Name"] == "" {
				t.Error("渠道菜单缺少 Name")
			}
			if fields["Backend"] != `"/channels"` {
				t.Errorf("渠道菜单 Backend 是 %s，期望 \"/channels\"", fields["Backend"])
			}
		}
	}
	if channelsMenu == nil {
		t.Fatal("config/menu.go 的 plugin 分组里没有 Path=\"/plugin/channel/channels\" 的菜单")
	}
	if len(channelBackendOwners) != 1 {
		t.Fatalf("Backend=\"/channels\" 的菜单数量为 %d（%v），必须恰好一条，否则菜单权限映射会撞车",
			len(channelBackendOwners), channelBackendOwners)
	}
}

// channelStructFieldLiterals 把「字段名 → 源码字面量」抽出来；不是结构体字面量时返回 nil。
func channelStructFieldLiterals(fset *token.FileSet, lit *ast.CompositeLit) map[string]string {
	if lit == nil || len(lit.Elts) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			return nil
		}
		out[key.Name] = channelExprString(fset, kv.Value)
	}
	return out
}

func channelCompositeValue(group *ast.CompositeLit, fset *token.FileSet, field string) (*ast.CompositeLit, bool) {
	for _, elt := range group.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != field {
			continue
		}
		lit, ok := kv.Value.(*ast.CompositeLit)
		return lit, ok
	}
	return nil, false
}

// ============================================================
// 掩码与保存语义（真实库）
// ============================================================

func TestChannelDetailMasksSecrets(t *testing.T) {
	w := channelAdminTestSite(t)
	ch := newManagerTestChannel(t, w, false)

	if ChannelSecretMask != "********" {
		t.Fatalf("掩码哨兵是 %q，规格要求 \"********\"", ChannelSecretMask)
	}

	view, err := w.ChannelAdminDetail(ch.Id)
	if err != nil {
		t.Fatalf("读取渠道详情失败：%v", err)
	}
	if view.AppSecret != ChannelSecretMask {
		t.Fatalf("详情里的 app_secret 是 %q，必须掩码", view.AppSecret)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("序列化详情失败：%v", err)
	}
	if strings.Contains(string(raw), ch.AppSecret) {
		t.Fatalf("详情响应泄露了应用密钥：%s", raw)
	}
	if strings.Contains(string(raw), channelAppHash(ch)) {
		t.Fatalf("详情响应泄露了应用身份摘要：%s", raw)
	}

	items, err := w.ChannelAdminList()
	if err != nil {
		t.Fatalf("读取渠道列表失败：%v", err)
	}
	var found *ChannelListItem
	for i := range items {
		if items[i].Id == ch.Id {
			found = &items[i]
		}
	}
	if found == nil {
		t.Fatalf("列表里没有自己创建的渠道 %d", ch.Id)
	}
	if found.AppSecret != ChannelSecretMask {
		t.Fatalf("列表里的 app_secret 是 %q，必须掩码", found.AppSecret)
	}
	listRaw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("序列化列表失败：%v", err)
	}
	if strings.Contains(string(listRaw), ch.AppSecret) {
		t.Fatalf("列表响应泄露了应用密钥")
	}
	if strings.Contains(string(listRaw), "policy_json") {
		t.Fatalf("列表响应不应包含 policy_json：%s", listRaw)
	}

	// 没有密钥的渠道：详情给空串而不是掩码（G74/R84：掩码只代表「有值」）。
	empty := &model.MessageChannel{
		SiteId:   w.Id,
		Platform: model.ChannelPlatformFeishu,
		Name:     "无密钥-" + channelAdminSuffix(),
		AppId:    "no_secret_app_" + channelAdminSuffix(),
		Revision: 1,
	}
	if err := w.DB.Create(empty).Error; err != nil {
		t.Fatalf("创建无密钥渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", empty.Id, w.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理渠道 %d 失败：%v", empty.Id, err)
		}
	})
	emptyView, err := w.ChannelAdminDetail(empty.Id)
	if err != nil {
		t.Fatalf("读取无密钥渠道详情失败：%v", err)
	}
	if emptyView.AppSecret != "" {
		t.Fatalf("无密钥渠道的 app_secret 应为空串，实得 %q", emptyView.AppSecret)
	}

	if _, err := w.ChannelAdminDetail(ch.Id + 100000); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("不存在的渠道详情应报「渠道不存在」，实得 %v", err)
	}
}

func TestSaveChannelKeepsMaskedSecret(t *testing.T) {
	w := channelAdminTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	originalSecret := ch.AppSecret

	reload := func() *model.MessageChannel {
		t.Helper()
		var current model.MessageChannel
		if err := w.DB.Where("id = ? AND site_id = ?", ch.Id, w.Id).Take(&current).Error; err != nil {
			t.Fatalf("回读渠道 %d 失败：%v", ch.Id, err)
		}
		return &current
	}
	base := func() *request.Channel {
		return &request.Channel{
			Id:       ch.Id,
			Platform: ch.Platform,
			Name:     ch.Name,
			AppId:    ch.AppId,
			OrgId:    ch.OrgId,
			Revision: reload().Revision,
		}
	}

	// 1) 掩码回写：必须沿用库中真值。
	req := base()
	req.AppSecret = ChannelSecretMask
	req.Name = "改名后的渠道-" + channelAdminSuffix()
	res, err := w.ChannelAdminSave(req)
	if err != nil {
		t.Fatalf("掩码回写保存失败：%v", err)
	}
	if res.ConnectionPending {
		t.Fatalf("渠道管理器未启动时不应标记 connection_pending（管理者未运行属可忽略错误）")
	}
	saved := reload()
	if saved.AppSecret != originalSecret {
		t.Fatalf("掩码回写把密钥改成了 %q", saved.AppSecret)
	}
	if saved.Name != req.Name {
		t.Fatalf("名称没有保存：%q", saved.Name)
	}
	if saved.Revision != ch.Revision+1 || res.Revision != saved.Revision {
		t.Fatalf("revision 未按乐观锁自增：库 %d，返回 %d，原 %d", saved.Revision, res.Revision, ch.Revision)
	}

	// 2) 空串同样表示「沿用」。
	req = base()
	req.AppSecret = ""
	if _, err := w.ChannelAdminSave(req); err != nil {
		t.Fatalf("空串密钥保存失败：%v", err)
	}
	if got := reload().AppSecret; got != originalSecret {
		t.Fatalf("空串回写把密钥改成了 %q", got)
	}

	// 3) 新值覆盖。
	newSecret := "new_secret_" + channelAdminSuffix()
	req = base()
	req.AppSecret = newSecret
	if _, err := w.ChannelAdminSave(req); err != nil {
		t.Fatalf("新密钥保存失败：%v", err)
	}
	if got := reload().AppSecret; got != newSecret {
		t.Fatalf("新密钥没有落库：%q", got)
	}

	// 4) 错误 revision：拒绝，且不得有任何字段被写坏。
	stale := base()
	stale.Revision = reload().Revision - 1
	stale.Name = "不该被写入的名字"
	stale.AppSecret = "should_not_land"
	if _, err := w.ChannelAdminSave(stale); !errors.Is(err, errChannelRevisionStale) {
		t.Fatalf("错误 revision 应报「配置已被修改，请刷新后重试」，实得 %v", err)
	}
	unchanged := reload()
	if unchanged.Name == stale.Name || unchanged.AppSecret != newSecret {
		t.Fatalf("revision 冲突时仍写入了数据：name=%q secret=%q", unchanged.Name, unchanged.AppSecret)
	}

	// 5) 新建：密钥必填。
	if _, err := w.ChannelAdminSave(&request.Channel{
		Platform: model.ChannelPlatformFeishu,
		Name:     "新建缺密钥-" + channelAdminSuffix(),
		AppId:    "new_app_no_secret_" + channelAdminSuffix(),
	}); err == nil {
		t.Fatal("新建渠道缺 app_secret 必须被拒绝")
	}

	// 6) 参数校验：非法平台、非对象 policy_json、开启聊天但没选智能体、非法开关值。
	invalid := []struct {
		name string
		req  *request.Channel
	}{
		{"非法平台", &request.Channel{Platform: "wechat", Name: "x", AppId: "x", AppSecret: "x"}},
		{"缺名称", &request.Channel{Platform: model.ChannelPlatformFeishu, AppId: "x", AppSecret: "x"}},
		{"缺应用ID", &request.Channel{Platform: model.ChannelPlatformFeishu, Name: "x", AppSecret: "x"}},
		{"非法启用值", &request.Channel{Platform: model.ChannelPlatformFeishu, Name: "x", AppId: "x", AppSecret: "x", Enabled: 2}},
		{"非法聊天开关", &request.Channel{Platform: model.ChannelPlatformFeishu, Name: "x", AppId: "x", AppSecret: "x", ChatEnabled: 3}},
		{"聊天无智能体", &request.Channel{Platform: model.ChannelPlatformFeishu, Name: "x", AppId: "x", AppSecret: "x", ChatEnabled: 1}},
		{"策略非对象", &request.Channel{Platform: model.ChannelPlatformFeishu, Name: "x", AppId: "x", AppSecret: "x", PolicyJson: `[]`}},
	}
	for _, tc := range invalid {
		if _, err := w.ChannelAdminSave(tc.req); err == nil {
			t.Errorf("%s 应被拒绝", tc.name)
		}
	}

	// 7) 应用身份冲突：把别的渠道的 app_id 抢过来。
	other := newManagerTestChannel(t, w, false)
	conflict := base()
	conflict.AppId = other.AppId
	conflict.OrgId = other.OrgId
	conflict.AppSecret = ChannelSecretMask
	if _, err := w.ChannelAdminSave(conflict); !errors.Is(err, errChannelAppIdentConflict) {
		t.Fatalf("应用身份冲突应报静态文案，实得 %v", err)
	}
	if strings.Contains(errChannelAppIdentConflict.Error(), channelAppHash(other)) {
		t.Fatal("冲突文案不得包含应用身份摘要")
	}
}

func TestChannelListIsSiteScoped(t *testing.T) {
	w := channelAdminTestSite(t)
	own := newManagerTestChannel(t, w, false)

	// 异站点行：SiteId 指向一个不存在的站点，验证所有入口都按 site_id 过滤。
	suffix := channelAdminSuffix()
	foreign := &model.MessageChannel{
		SiteId:    w.Id + 90001,
		Platform:  model.ChannelPlatformFeishu,
		Name:      "异站点渠道-" + suffix,
		AppId:     "foreign_app_" + suffix,
		OrgId:     "foreign_org_" + suffix,
		AppSecret: "foreign_secret_" + suffix,
		Revision:  1,
	}
	if err := w.DB.Create(foreign).Error; err != nil {
		t.Fatalf("创建异站点渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", foreign.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理异站点渠道 %d 失败：%v", foreign.Id, err)
		}
	})

	items, err := w.ChannelAdminList()
	if err != nil {
		t.Fatalf("读取渠道列表失败：%v", err)
	}
	seenOwn, seenForeign := false, false
	for i := range items {
		switch items[i].Id {
		case own.Id:
			seenOwn = true
		case foreign.Id:
			seenForeign = true
		}
	}
	if !seenOwn {
		t.Fatalf("列表里看不到自己站点的渠道 %d", own.Id)
	}
	if seenForeign {
		t.Fatalf("列表里出现了异站点渠道 %d", foreign.Id)
	}

	if _, err := w.ChannelAdminDetail(foreign.Id); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道详情应报「渠道不存在」，实得 %v", err)
	}
	if _, err := w.ChannelAdminStatus(foreign.Id); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道状态应报「渠道不存在」，实得 %v", err)
	}
	if _, err := w.ChannelAdminSave(&request.Channel{
		Id:       foreign.Id,
		Platform: foreign.Platform,
		Name:     "被越权改名",
		AppId:    foreign.AppId,
		OrgId:    foreign.OrgId,
		Revision: foreign.Revision,
	}); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道保存应报「渠道不存在」，实得 %v", err)
	}
	var reloaded model.MessageChannel
	if err := w.DB.Where("id = ?", foreign.Id).Take(&reloaded).Error; err != nil {
		t.Fatalf("回读异站点渠道失败：%v", err)
	}
	if reloaded.Name != foreign.Name || reloaded.Revision != foreign.Revision {
		t.Fatalf("异站点渠道被越权修改：name=%q revision=%d", reloaded.Name, reloaded.Revision)
	}
	if err := w.ChannelAdminDelete(foreign.Id); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道删除应报「渠道不存在」，实得 %v", err)
	}
	if err := w.DB.Where("id = ?", foreign.Id).Take(&reloaded).Error; err != nil {
		t.Fatalf("异站点渠道被越权删除：%v", err)
	}
}

func TestChannelDeleteCancelsQueuedInbox(t *testing.T) {
	w := channelAdminTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	binding := newChannelAdminBinding(t, w, ch)
	suffix := channelAdminSuffix()

	inbox := &model.ChannelInbox{
		SiteId:    w.Id,
		ChannelId: ch.Id,
		BindingId: binding.Id,
		EventKind: "message",
		Content:   "待处理消息",
		Status:    model.InboxStatusPending,
		EventHash: channelDigest("admin_delete_inbox", suffix),
	}
	if err := w.DB.Create(inbox).Error; err != nil {
		t.Fatalf("创建入站任务失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", inbox.Id).Delete(&model.ChannelInbox{}).Error; err != nil {
			t.Logf("清理入站任务 %d 失败：%v", inbox.Id, err)
		}
	})

	target := &model.ChannelTarget{
		SiteId:           w.Id,
		ChannelId:        ch.Id,
		Kind:             TargetKindGroup,
		PlatformTargetId: "delete_group_" + suffix,
		Approved:         1,
		TargetHash:       bindingTargetDigest(w.Id, ch.Id, TargetKindGroup, "delete_group_"+suffix),
	}
	if err := w.DB.Create(target).Error; err != nil {
		t.Fatalf("创建目标失败：%v", err)
	}

	audit := &model.ChannelAudit{
		SiteId:    w.Id,
		ChannelId: ch.Id,
		Action:    "channel.test",
		Result:    "ok",
	}
	if err := w.DB.Create(audit).Error; err != nil {
		t.Fatalf("创建审计失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", audit.Id).Delete(&model.ChannelAudit{}).Error; err != nil {
			t.Logf("清理审计 %d 失败：%v", audit.Id, err)
		}
	})

	deliveryBase := uint(time.Now().UnixNano() & 0x7fffffff)
	delivery := &model.NotificationDelivery{
		SiteId:   w.Id,
		EventId:  deliveryBase,
		RuleId:   deliveryBase + 1,
		TargetId: target.Id,
		Status:   NotifyDeliveryStatusFailed,
	}
	if err := w.DB.Create(delivery).Error; err != nil {
		t.Fatalf("创建投递失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", delivery.Id).Delete(&model.NotificationDelivery{}).Error; err != nil {
			t.Logf("清理投递 %d 失败：%v", delivery.Id, err)
		}
	})

	if err := w.ChannelAdminDelete(ch.Id); err != nil {
		t.Fatalf("删除渠道失败：%v", err)
	}

	var reloadedInbox model.ChannelInbox
	if err := w.DB.Where("id = ?", inbox.Id).Take(&reloadedInbox).Error; err != nil {
		t.Fatalf("回读入站任务失败：%v", err)
	}
	if reloadedInbox.Status != model.InboxStatusCancelled {
		t.Fatalf("待处理入站任务未被取消，状态 %q", reloadedInbox.Status)
	}

	var reloadedBinding model.ChannelBinding
	if err := w.DB.Where("id = ?", binding.Id).Take(&reloadedBinding).Error; err != nil {
		t.Fatalf("回读绑定失败：%v", err)
	}
	if reloadedBinding.RevokedAt == 0 {
		t.Fatal("渠道删除后绑定未被撤销")
	}

	var targetCount int64
	if err := w.DB.Model(&model.ChannelTarget{}).Where("channel_id = ?", ch.Id).Count(&targetCount).Error; err != nil {
		t.Fatalf("统计目标失败：%v", err)
	}
	if targetCount != 0 {
		t.Fatalf("渠道删除后仍残留 %d 个目标", targetCount)
	}

	var channelCount int64
	if err := w.DB.Model(&model.MessageChannel{}).Where("id = ?", ch.Id).Count(&channelCount).Error; err != nil {
		t.Fatalf("统计渠道失败：%v", err)
	}
	if channelCount != 0 {
		t.Fatal("渠道行没有被删除")
	}

	if err := w.DB.Where("id = ?", audit.Id).Take(&model.ChannelAudit{}).Error; err != nil {
		t.Fatalf("审计历史必须保留，回读失败：%v", err)
	}
	if err := w.DB.Where("id = ?", delivery.Id).Take(&model.NotificationDelivery{}).Error; err != nil {
		t.Fatalf("投递历史必须保留，回读失败：%v", err)
	}

	if err := w.ChannelAdminDelete(ch.Id); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("重复删除应报「渠道不存在」，实得 %v", err)
	}
}

func TestChannelStatusReportsRuntime(t *testing.T) {
	w := channelAdminTestSite(t)
	ch := newManagerTestChannel(t, w, false)

	// 管理器未启动：connected 为 false，租约到期为 0，不报错。
	running, leaseExpiresAt := currentChannelRuntime(w.Id, ch.Id)
	if running || leaseExpiresAt != 0 {
		t.Fatalf("管理器未启动时运行态应为 false/0，实得 %v/%d", running, leaseExpiresAt)
	}
	status, err := w.ChannelAdminStatus(ch.Id)
	if err != nil {
		t.Fatalf("读取渠道状态失败：%v", err)
	}
	if status.Connected {
		t.Fatal("管理器未启动时 connected 必须为 false")
	}
	if status.Id != ch.Id || status.Revision != ch.Revision || status.Platform != ch.Platform {
		t.Fatalf("状态里的基础字段不对：%+v", status)
	}
	if status.PendingDeliveries != 0 || status.FailedDeliveries != 0 {
		t.Fatalf("新渠道不应有投递统计：pending=%d failed=%d", status.PendingDeliveries, status.FailedDeliveries)
	}

	// 投递统计：pending 计入待发，failed 计入失败；异站点目标不计入。
	target := &model.ChannelTarget{
		SiteId:           w.Id,
		ChannelId:        ch.Id,
		Kind:             TargetKindGroup,
		PlatformTargetId: "status_group_" + channelAdminSuffix(),
		Approved:         1,
		TargetHash:       bindingTargetDigest(w.Id, ch.Id, TargetKindGroup, "status_group_"+channelAdminSuffix()),
	}
	if err := w.DB.Create(target).Error; err != nil {
		t.Fatalf("创建目标失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", target.Id).Delete(&model.ChannelTarget{}).Error; err != nil {
			t.Logf("清理目标 %d 失败：%v", target.Id, err)
		}
	})
	base := uint(time.Now().UnixNano() & 0x7fffffff)
	for i, status := range []string{NotifyDeliveryStatusPending, NotifyDeliveryStatusRetry, NotifyDeliveryStatusFailed} {
		delivery := &model.NotificationDelivery{
			SiteId:   w.Id,
			EventId:  base + uint(i)*10,
			RuleId:   base,
			TargetId: target.Id,
			Status:   status,
		}
		if err := w.DB.Create(delivery).Error; err != nil {
			t.Fatalf("创建投递失败：%v", err)
		}
		created := delivery
		t.Cleanup(func() {
			if err := w.DB.Where("id = ?", created.Id).Delete(&model.NotificationDelivery{}).Error; err != nil {
				t.Logf("清理投递 %d 失败：%v", created.Id, err)
			}
		})
	}
	status, err = w.ChannelAdminStatus(ch.Id)
	if err != nil {
		t.Fatalf("读取渠道状态失败：%v", err)
	}
	if status.PendingDeliveries != 2 {
		t.Fatalf("待发投递统计为 %d，期望 2", status.PendingDeliveries)
	}
	if status.FailedDeliveries != 1 {
		t.Fatalf("失败投递统计为 %d，期望 1", status.FailedDeliveries)
	}
}

func TestChannelCapabilitiesDerivesFromCatalog(t *testing.T) {
	w := channelAdminTestSite(t)

	view, err := w.ChannelCapabilities(0)
	if err != nil {
		t.Fatalf("读取能力清单失败：%v", err)
	}
	if len(view.Intents) == 0 {
		t.Fatal("能力清单为空")
	}
	if len(view.BlockedBuiltins) == 0 {
		t.Fatal("内置黑名单为空：首版关闭的主机能力必须列出来")
	}
	blockedSet := map[string]bool{}
	for _, name := range view.BlockedBuiltins {
		blockedSet[name] = true
	}
	for _, want := range []string{"bash", "read_file", "write_file", "web_fetch"} {
		if !blockedSet[want] {
			t.Errorf("内置黑名单缺少 %s", want)
		}
	}

	withActions := 0
	for _, item := range view.Intents {
		if item.Name == "" || item.Title == "" {
			t.Fatalf("能力条目缺少名称或标题：%+v", item)
		}
		if item.Risk == "" || item.Domain == "" {
			t.Fatalf("%s 缺少风险等级或能力域：%+v", item.Name, item)
		}
		if item.Blocked && item.Reason == "" {
			t.Fatalf("%s 被标记 blocked 却没有原因", item.Name)
		}
		if item.Policy == "" {
			t.Fatalf("%s 缺少策略裁决", item.Name)
		}
		if len(item.Actions) > 0 {
			withActions++
		}
		if item.AgentAllowed != nil {
			t.Fatalf("没传 agent_id 时不应出现 agent_allowed（%s）", item.Name)
		}
	}
	if withActions == 0 {
		t.Fatal("没有任何意图解析出动作，能力展示对前端无用")
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("序列化能力清单失败：%v", err)
	}
	for _, forbidden := range []string{`"properties"`, `"params"`, `"app_secret"`, `"enum"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("能力清单泄露了参数 schema 或密钥（含 %s）", forbidden)
		}
	}

	// agent_id>0：给出 agent_allowed，并且真实读到接待白名单。
	agentId := uint(time.Now().UnixNano() & 0x7fffffff)
	allowedIntent := view.Intents[0].Name
	policy := &model.AgentReceptionPolicy{
		SiteId:         w.Id,
		AgentId:        agentId,
		AllowedIntents: `["` + allowedIntent + `"]`,
	}
	if err := w.DB.Create(policy).Error; err != nil {
		t.Fatalf("创建接待策略失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", policy.Id).Delete(&model.AgentReceptionPolicy{}).Error; err != nil {
			t.Logf("清理接待策略 %d 失败：%v", policy.Id, err)
		}
	})
	view, err = w.ChannelCapabilities(agentId)
	if err != nil {
		t.Fatalf("带 agent_id 读取能力清单失败：%v", err)
	}
	for _, item := range view.Intents {
		if item.AgentAllowed == nil {
			t.Fatalf("%s 缺少 agent_allowed", item.Name)
		}
		if item.Name == allowedIntent && !*item.AgentAllowed {
			t.Fatalf("白名单里的 %s 应为 agent_allowed=true", item.Name)
		}
	}

	// 没有配置行时必须回落到站点曝光：agent_allowed 全为 true，与执行侧同口径。
	// 反向对照：若这里仍按「缺策略即拒绝」，页面会显示「智能体未授权」而执行已放行，
	// 展示与执行两套结论。
	if err := w.DB.Where("id = ?", policy.Id).Delete(&model.AgentReceptionPolicy{}).Error; err != nil {
		t.Fatalf("删除接待策略失败：%v", err)
	}
	view, err = w.ChannelCapabilities(agentId)
	if err != nil {
		t.Fatalf("无接待策略时读取能力清单失败：%v", err)
	}
	for _, item := range view.Intents {
		if item.AgentAllowed == nil || !*item.AgentAllowed {
			t.Fatalf("没有接待策略时 %s 应回落为 agent_allowed=true，实际 %v", item.Name, item.AgentAllowed)
		}
	}
}
