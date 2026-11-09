package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

// ============================================================
// 组织标识自动获取（接口优先）与首条事件锚定（TOFU）。
//
// 背景：组织标识（飞书 tenant_key / 钉钉 corp_id）在平台后台没有展示页，
// 要求站长手工填写等于把一个「拿不到的值」变成接入门槛。策略改为：
//   1. 建连前先调平台接口自动获取；
//   2. 接口取不到（权限未发布 / 响应不含该字段）时，用首条入站事件锚定；
//   3. 两者都要求非空——空组织事件仍然拒绝，不能退化成「放行一切」。
// ============================================================

// TestValidatePlatformCredentialsAllowsEmptyOrganization 固化建连前校验的边界：
// 组织标识不再是必填项（自动获取/事件锚定会补上），应用 ID 与密钥仍然必填。
func TestValidatePlatformCredentialsAllowsEmptyOrganization(t *testing.T) {
	if err := validatePlatformCredentials(PlatformCredentials{
		AppId:     "app_1",
		AppSecret: "secret_1",
	}); err != nil {
		t.Fatalf("组织标识留空时建连校验必须通过（由自动获取/事件锚定补齐），实际报错：%v", err)
	}

	// 反向对照：应用 ID 与密钥仍然必填，否则校验器变成了空转。
	if err := validatePlatformCredentials(PlatformCredentials{AppSecret: "secret_1"}); err == nil {
		t.Fatal("缺少应用 ID 仍必须拒绝")
	}
	if err := validatePlatformCredentials(PlatformCredentials{AppId: "app_1"}); err == nil {
		t.Fatal("缺少应用密钥仍必须拒绝")
	}
}

// TestInboundEventValidateIdentityAdoptionMode 固化「首条事件锚定」阶段的身份语义：
// 渠道还没拿到组织标识时（expectedOrgId 为空），非空事件组织必须放行，
// 但空组织仍然拒绝；一旦有了基准，后续必须严格相等。
func TestInboundEventValidateIdentityAdoptionMode(t *testing.T) {
	const appId = "feishu_app_1"

	event := InboundEvent{
		Kind:           InboundKindText,
		AppId:          appId,
		OrganizationId: "tenant_first",
		UserId:         "ou_1",
		ChatId:         "oc_1",
		ChatType:       ChatTypeP2P,
	}
	if err := event.ValidateIdentity(appId, ""); err != nil {
		t.Fatalf("锚定阶段（期望组织为空）不能拒绝非空事件组织：%v", err)
	}

	// 空组织事件在任何阶段都必须拒绝：锚定不能锚在空值上。
	empty := event
	empty.OrganizationId = ""
	if err := empty.ValidateIdentity(appId, ""); err == nil {
		t.Fatal("锚定阶段也必须拒绝空组织事件")
	}

	// 已有基准后仍严格相等（防止跨企业事件混入）。
	other := event
	other.OrganizationId = "tenant_other"
	if err := other.ValidateIdentity(appId, "tenant_first"); err == nil {
		t.Fatal("已有基准后，不匹配的事件组织必须拒绝")
	}
}

// TestResolvePlatformOrganizationDispatch 固化解析入口：只认已实现平台，
// 凭据不全时直接报错（不发网络请求），平台解析器通过注册表分派。
func TestResolvePlatformOrganizationDispatch(t *testing.T) {
	creds := PlatformCredentials{AppId: "app_1", AppSecret: "secret_1"}

	if _, err := ResolvePlatformOrganization(context.Background(), "wechat", creds); err == nil {
		t.Fatal("未实现的平台必须报错，不能返回空组织继续建连")
	}
	if _, err := ResolvePlatformOrganization(context.Background(), model.ChannelPlatformFeishu, PlatformCredentials{AppId: "app_1"}); err == nil {
		t.Fatal("缺少应用密钥时必须报错，不能发网络请求")
	}

	// 注册表分派：换掉飞书解析器，确认入口把平台名映射到了对应实现。
	original := platformOrgResolvers[model.ChannelPlatformFeishu]
	platformOrgResolvers[model.ChannelPlatformFeishu] = func(_ context.Context, got PlatformCredentials) (string, error) {
		if got.AppId != "app_1" {
			t.Errorf("解析器收到的凭据不对：%+v", got)
		}
		return "  tenant_from_api  ", nil
	}
	t.Cleanup(func() { platformOrgResolvers[model.ChannelPlatformFeishu] = original })

	org, err := ResolvePlatformOrganization(context.Background(), model.ChannelPlatformFeishu, creds)
	if err != nil {
		t.Fatalf("分派到平台解析器后应返回组织标识：%v", err)
	}
	if org != "tenant_from_api" {
		t.Fatalf("组织标识必须 TrimSpace 后返回，实得 %q", org)
	}

	// 解析器返回空串等同于「没取到」：必须报错，交给事件锚定兜底。
	platformOrgResolvers[model.ChannelPlatformFeishu] = func(context.Context, PlatformCredentials) (string, error) {
		return "   ", nil
	}
	if _, err := ResolvePlatformOrganization(context.Background(), model.ChannelPlatformFeishu, creds); err == nil {
		t.Fatal("解析器返回空组织时必须报错")
	}
}

// TestParseFeishuTenantKey 固化飞书 tenant/query 响应解析：
// 正常取 tenant_key；业务错误、空值、非法 JSON 都必须报错（交给事件锚定兜底），
// 且错误文案不得回显响应正文（可能含企业名称等敏感信息）。
func TestParseFeishuTenantKey(t *testing.T) {
	valid := []byte(`{"code":0,"msg":"success","tenant":{"tenant_key":"tk_1","name":"某企业"}}`)
	org, err := parseFeishuTenantKey(valid)
	if err != nil || org != "tk_1" {
		t.Fatalf("合法响应应解出 tenant_key，实得 org=%q err=%v", org, err)
	}

	cases := []struct {
		name string
		body string
	}{
		{name: "业务错误", body: `{"code":99991672,"msg":"access denied"}`},
		{name: "缺少 tenant_key", body: `{"code":0,"tenant":{"name":"某企业"}}`},
		{name: "空白 tenant_key", body: `{"code":0,"tenant":{"tenant_key":"  "}}`},
		{name: "非法 JSON", body: `not-json`},
	}
	for _, tc := range cases {
		_, err := parseFeishuTenantKey([]byte(tc.body))
		if err == nil {
			t.Errorf("%s：必须报错", tc.name)
			continue
		}
		if strings.Contains(err.Error(), "某企业") || strings.Contains(err.Error(), "access denied") {
			t.Errorf("%s：错误文案回显了响应正文：%v", tc.name, err)
		}
	}
}

// TestQueryDingTalkCorpId 固化钉钉旧版 gettoken 取 corp_id 的链路：
// 成功解出 corp_id；errcode 非 0、缺字段、非 2xx、网络失败都必须报错。
// 网络错误文案绝不能回显 URL（URL 的 query 里带着 appsecret）。
func TestQueryDingTalkCorpId(t *testing.T) {
	const appId, appSecret = "ding_app_1", "ding_secret_1"

	cases := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{name: "正常返回 corp_id", status: 200, body: `{"errcode":0,"errmsg":"ok","access_token":"t","expires_in":7200,"corp_id":"corp_1"}`, want: "corp_1"},
		{name: "业务错误", status: 200, body: `{"errcode":40001,"errmsg":"invalid appkey"}`, wantErr: true},
		{name: "缺少 corp_id", status: 200, body: `{"errcode":0,"access_token":"t"}`, wantErr: true},
		{name: "非 2xx", status: 500, body: `server error`, wantErr: true},
		{name: "非法 JSON", status: 200, body: `not-json`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != dingTalkLegacyTokenPath {
					t.Errorf("请求路径 = %q，期望 %q", r.URL.Path, dingTalkLegacyTokenPath)
				}
				if got := r.URL.Query().Get("appkey"); got != appId {
					t.Errorf("appkey = %q，期望 %q", got, appId)
				}
				if got := r.URL.Query().Get("appsecret"); got != appSecret {
					t.Errorf("appsecret = %q，期望 %q", got, appSecret)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			org, err := queryDingTalkCorpId(context.Background(), server.URL, server.Client(),
				PlatformCredentials{AppId: appId, AppSecret: appSecret})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际得到 %q", org)
				}
				return
			}
			if err != nil || org != tc.want {
				t.Fatalf("实得 org=%q err=%v，期望 org=%q", org, err, tc.want)
			}
		})
	}

	// 网络失败：连一个必然拒绝的地址，错误文案不得带出 appsecret。
	_, err := queryDingTalkCorpId(context.Background(), "http://127.0.0.1:1", &http.Client{},
		PlatformCredentials{AppId: appId, AppSecret: appSecret})
	if err == nil {
		t.Fatal("网络失败必须报错")
	}
	if strings.Contains(err.Error(), appSecret) || strings.Contains(err.Error(), "appsecret") {
		t.Fatalf("网络错误文案泄露了密钥：%v", err)
	}
}

// ============================================================
// 建连集成：接口优先 → 落库；取不到 → 首条事件锚定。
// ============================================================

// newOrglessTestChannel 建一条组织标识为空的渠道。
// 清理按自己创建的 ID 精确删，不按站点批量删。
func newOrglessTestChannel(t *testing.T, w *Website, appId string) *model.MessageChannel {
	t.Helper()
	ch := &model.MessageChannel{
		SiteId:    w.Id,
		Platform:  model.ChannelPlatformFeishu,
		Name:      "组织锚定测试-" + appId,
		Enabled:   1,
		AppId:     appId,
		OrgId:     "",
		AppSecret: "orgless_secret_" + appId,
		Revision:  1,
	}
	if err := w.DB.Create(ch).Error; err != nil {
		t.Fatalf("创建空组织渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("channel_id = ?", ch.Id).Delete(&model.ChannelInbox{}).Error; err != nil {
			t.Logf("清理渠道 %d 的入站任务失败：%v", ch.Id, err)
		}
		if err := w.DB.Where("channel_id = ?", ch.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理渠道 %d 的绑定失败：%v", ch.Id, err)
		}
		if err := w.DB.Where("id = ? AND site_id = ?", ch.Id, w.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理渠道 %d 失败：%v", ch.Id, err)
		}
	})
	return ch
}

// cleanupChannelLeases 按「平台+应用+候选组织」逐个删租约。
// 组织标识可能在建连过程中变化，所以调用方要把「空值 / 自动获取值 / 锚定值」都列出来。
func cleanupChannelLeases(t *testing.T, w *Website, platform, appId string, orgs ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, org := range orgs {
			hash := model.ComputeAppIdentityHash(platform, appId, org)
			if err := w.DB.Where("app_identity_hash = ?", hash).Delete(&model.ChannelLease{}).Error; err != nil {
				t.Logf("清理租约 %s 失败：%v", org, err)
			}
		}
	})
}

// waitStoredChannel 轮询等待渠道行满足条件。组织标识的写入是异步于连接建立的。
func waitStoredChannel(t *testing.T, w *Website, channelId uint, match func(*model.MessageChannel) bool) *model.MessageChannel {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last model.MessageChannel
	for time.Now().Before(deadline) {
		err := w.DB.Where("id = ? AND site_id = ?", channelId, w.Id).Take(&last).Error
		if err == nil && match(&last) {
			return &last
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待渠道 %d 落库状态超时，最后读到：%+v", channelId, last)
	return nil
}

// TestWorkerResolvesOrganizationBeforeConnect 固化接口优先路径：
// 组织标识为空时建连前先自动获取，拿到后落库（含应用身份摘要）并把值传给适配器。
func TestWorkerResolvesOrganizationBeforeConnect(t *testing.T) {
	w := channelManagerTestSite(t)
	const appId = "org_api_app"
	ch := newOrglessTestChannel(t, w, appId)
	cleanupChannelLeases(t, w, ch.Platform, appId, "", "tenant_auto")

	factory := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(factory)
	var resolved atomic.Int32
	opts.resolveOrg = func(_ context.Context, platform string, creds PlatformCredentials) (string, error) {
		// 测试管理器会接管库里所有已启用渠道（含开发库里的真实渠道），
		// 注入值必须只作用于本用例创建的应用，避免污染真实渠道配置。
		if creds.AppId != appId {
			return "", errors.New("测试未注入此应用")
		}
		resolved.Add(1)
		if platform != model.ChannelPlatformFeishu {
			t.Errorf("解析器收到的平台 = %q", platform)
		}
		return "tenant_auto", nil
	}
	startTestChannelManager(t, w, opts)

	adapter := factory.waitForRun(t, appId, 1)
	if got := adapter.creds().OrganizationId; got != "tenant_auto" {
		t.Fatalf("适配器收到的组织标识 = %q，期望自动获取值 tenant_auto", got)
	}

	stored := waitStoredChannel(t, w, ch.Id, func(c *model.MessageChannel) bool {
		return strings.TrimSpace(c.OrgId) == "tenant_auto"
	})
	wantHash := model.ComputeAppIdentityHash(stored.Platform, stored.AppId, "tenant_auto")
	if stored.AppIdentityHash != wantHash {
		t.Fatalf("落库的应用身份摘要未随组织标识更新：got %s want %s", stored.AppIdentityHash, wantHash)
	}
	if got := resolved.Load(); got != 1 {
		// 重建后的 worker 已经从库里读到组织标识，不应再调接口。
		t.Fatalf("自动获取被调用 %d 次，期望恰好 1 次", got)
	}

	// 自动获取写入组织标识后，扫描循环不得把它当成外部配置变更再重连一次：
	// 本连接建立时已按落库后的摘要持有租约，配置并没有被外部改动。
	// 负向对照：删掉 run 里的 wk.signature.Store 后，这里会看到第 2 次构造。
	time.Sleep(150 * time.Millisecond)
	if got := factory.adapterCount(appId); got != 1 {
		t.Fatalf("自动获取组织标识后发生了多余重连：适配器构造 %d 次，期望 1 次", got)
	}
}

// TestWorkerSkipsResolverWhenOrganizationConfigured 固化手工填写的优先级：
// 组织标识已有值时绝不调平台接口（站长显式配置就是最终值）。
func TestWorkerSkipsResolverWhenOrganizationConfigured(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	factory := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(factory)
	var resolved atomic.Int32
	opts.resolveOrg = func(_ context.Context, _ string, creds PlatformCredentials) (string, error) {
		// 只对本用例的渠道计数并返回注入值；其它渠道（开发库里的真实渠道）
		// 返回错误，确保测试不会把假组织标识写进真实配置。
		if creds.AppId != ch.AppId {
			return "", errors.New("测试未注入此应用")
		}
		resolved.Add(1)
		return "tenant_should_not_be_used", nil
	}
	startTestChannelManager(t, w, opts)

	adapter := factory.waitForRun(t, ch.AppId, 1)
	if got := adapter.creds().OrganizationId; got != ch.OrgId {
		t.Fatalf("适配器收到的组织标识 = %q，期望手工配置值 %q", got, ch.OrgId)
	}
	if got := resolved.Load(); got != 0 {
		t.Fatalf("已有组织标识时不应自动获取，实际调用 %d 次", got)
	}
}

// TestWorkerAdoptsOrganizationFromFirstInboundEvent 固化兜底路径：
// 接口取不到时连接照常建立（不终止），首条非空组织事件成为校验基准并落库；
// 之后不同组织的事件、空组织事件一律丢弃，入站身份摘要用锚定值计算。
func TestWorkerAdoptsOrganizationFromFirstInboundEvent(t *testing.T) {
	w := channelManagerTestSite(t)
	const appId = "org_adopt_app"
	ch := newOrglessTestChannel(t, w, appId)
	cleanupChannelLeases(t, w, ch.Platform, appId, "", "tenant_first", "tenant_other")

	factory := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(factory)
	opts.resolveOrg = func(context.Context, string, PlatformCredentials) (string, error) {
		return "", errors.New("未授予 tenant:tenant:readonly")
	}
	startTestChannelManager(t, w, opts)

	adapter := factory.waitForRun(t, appId, 1)
	if got := adapter.creds().OrganizationId; got != "" {
		t.Fatalf("接口取不到组织标识时必须按锚定模式建连（空值），实际 %q", got)
	}
	sink := adapter.waitSink(t)

	event := InboundEvent{
		EventId:        "ev_adopt_1",
		Kind:           InboundKindText,
		AppId:          appId,
		OrganizationId: "tenant_first",
		UserId:         "user_1",
		ChatId:         "chat_1",
		ChatType:       ChatTypeP2P,
		Text:           "你好",
	}
	if err := sink(context.Background(), event); err != nil {
		t.Fatalf("锚定事件处理失败：%v", err)
	}

	stored := waitStoredChannel(t, w, ch.Id, func(c *model.MessageChannel) bool {
		return strings.TrimSpace(c.OrgId) == "tenant_first"
	})
	wantHash := model.ComputeAppIdentityHash(stored.Platform, stored.AppId, "tenant_first")
	if stored.AppIdentityHash != wantHash {
		t.Fatalf("锚定后应用身份摘要未更新：got %s want %s", stored.AppIdentityHash, wantHash)
	}

	var inbox model.ChannelInbox
	if err := w.DB.Where("channel_id = ? AND site_id = ? AND event_hash = ?",
		ch.Id, w.Id, channelDigest(fmt.Sprint(w.Id), fmt.Sprint(ch.Id), ch.Platform, "ev_adopt_1")).
		Take(&inbox).Error; err != nil {
		t.Fatalf("锚定事件应落库为入站任务：%v", err)
	}
	wantIdentity := bindingIdentityDigest(w.Id, ch.Id, "tenant_first", "user_1")
	if inbox.IdentityHash != wantIdentity {
		t.Fatalf("入站身份摘要必须用锚定组织计算：got %s want %s", inbox.IdentityHash, wantIdentity)
	}

	// 跨企业事件：锚定之后必须丢弃（不落 Inbox）。
	other := event
	other.EventId = "ev_adopt_2"
	other.OrganizationId = "tenant_other"
	if err := sink(context.Background(), other); err != nil {
		t.Fatalf("丢弃跨企业事件不应返回错误：%v", err)
	}
	// 空组织事件：任何阶段都丢弃。
	empty := event
	empty.EventId = "ev_adopt_3"
	empty.OrganizationId = ""
	if err := sink(context.Background(), empty); err != nil {
		t.Fatalf("丢弃空组织事件不应返回错误：%v", err)
	}

	var count int64
	if err := w.DB.Model(&model.ChannelInbox{}).Where("channel_id = ? AND site_id = ?", ch.Id, w.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("统计入站任务失败：%v", err)
	}
	if count != 1 {
		t.Fatalf("锚定后只允许首条事件落库，实际 %d 条", count)
	}
}

// TestHandleInboundKeepsAdoptionRestartFlag 锁住「锚定后必须请求重建」的机制：
// handleInbound 处理完锚定事件后会取消当前连接，但 restartAfterAdoption 必须
// 保留到 run 退出，供 run 发起重建请求。若在处理函数里就把标记 Swap 清零，
// 连接会停、重建请求却丢了，只能等下一轮扫描补救。
//
// 负向对照：把 handleInbound 的 defer 改回 `restartAfterAdoption.Swap(false)`，
// 本测试的第二条断言必红。
func TestHandleInboundKeepsAdoptionRestartFlag(t *testing.T) {
	w := channelManagerTestSite(t)
	const appId = "org_adopt_flag_app"
	ch := newOrglessTestChannel(t, w, appId)
	cleanupChannelLeases(t, w, ch.Platform, appId, "", "tenant_flag")

	opts := testChannelManagerOptions(newFakeChannelFactory(nil))
	opts.resolveOrg = func(context.Context, string, PlatformCredentials) (string, error) {
		return "", errors.New("取不到")
	}
	manager := newChannelManager(w, opts)

	var canceled atomic.Bool
	worker := &channelWorker{
		manager: manager,
		channel: *ch,
		ctx:     context.Background(),
		cancel:  func() { canceled.Store(true) },
	}
	worker.orgId.Store("")

	event := InboundEvent{
		EventId:        "ev_flag_1",
		Kind:           InboundKindText,
		AppId:          appId,
		OrganizationId: "tenant_flag",
		UserId:         "user_flag",
		ChatId:         "chat_flag",
		ChatType:       ChatTypeP2P,
		Text:           "你好",
	}
	if err := worker.handleInbound(context.Background(), nil, event); err != nil {
		t.Fatalf("锚定事件处理失败：%v", err)
	}
	if !canceled.Load() {
		t.Fatal("锚定后必须取消当前连接")
	}
	if !worker.restartAfterAdoption.Load() {
		t.Fatal("锚定标记必须保留到 run 退出，否则重建请求会丢失")
	}
	stored := waitStoredChannel(t, w, ch.Id, func(c *model.MessageChannel) bool {
		return strings.TrimSpace(c.OrgId) == "tenant_flag"
	})
	if want := model.ComputeAppIdentityHash(stored.Platform, stored.AppId, "tenant_flag"); stored.AppIdentityHash != want {
		t.Fatalf("锚定后应用身份摘要未更新：got %s want %s", stored.AppIdentityHash, want)
	}
}
