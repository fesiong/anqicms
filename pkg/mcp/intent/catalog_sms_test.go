package intent

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// apiCatalogFile 是生成物 provider/api_catalog.json 的相对路径。
//
// pkg/mcp/intent 不能 import provider（provider 已 import 本包，会循环），
// 所以「意图声明的参数必须被落点端点认识」只能对着这份生成物核对——
// 它同时是 go:embed 进生产二进制的那一份，是端点契约的真相源。
// 相对路径是仓库根（pkg/mcp/intent → ../../..）下的 provider/api_catalog.json。
const apiCatalogFile = "../../../provider/api_catalog.json"

type smsCatalogDoc struct {
	Endpoints []smsCatalogEndpoint `json:"endpoints"`
}

type smsCatalogEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Params []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"params"`
}

func loadSmsCatalog(t *testing.T) *smsCatalogDoc {
	t.Helper()
	raw, err := os.ReadFile(apiCatalogFile)
	if err != nil {
		t.Fatalf("读取 %s 失败（生成物缺失？先跑 go run ./cmd/apicatalog）：%v", apiCatalogFile, err)
	}
	var doc smsCatalogDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("解析 %s 失败：%v", apiCatalogFile, err)
	}
	if len(doc.Endpoints) == 0 {
		t.Fatalf("%s 里没有任何端点，本测试会空转", apiCatalogFile)
	}
	return &doc
}

// smsCatalogParams 返回端点的「参数名 → 声明类型」集合；端点不存在直接 Fatal。
//
// 必须先确认端点存在：路径写错时后续的逐参数核对会一个都不命中（或全部
// 按"无参数"通过），测试恒绿却什么都没守住。
func smsCatalogParams(t *testing.T, doc *smsCatalogDoc, method, path string) map[string]string {
	t.Helper()
	for _, ep := range doc.Endpoints {
		if ep.Method == method && ep.Path == path {
			out := make(map[string]string, len(ep.Params))
			for _, p := range ep.Params {
				out[p.Name] = p.Type
			}
			return out
		}
	}
	t.Fatalf("api_catalog.json 缺少端点 %s %s：路径写错或未重新生成", method, path)
	return nil
}

// TestSmsIntentStaysGated 短信渠道是高危面：配置含三家厂商凭据，
// test 动作会真实付费给任意号码发短信，必须默认关闭并四处名单同时表态。
func TestSmsIntentStaysGated(t *testing.T) {
	spec, ok := specByName("channel_sms")
	if !ok {
		t.Fatal("意图 channel_sms 不存在：短信配置/测试发送/发送记录没有模型面入口")
	}
	if !spec.DefaultOff {
		t.Error("channel_sms 必须 DefaultOff：配置含厂商凭据，test 会真实付费发短信给任意号码")
	}
	if spec.Domain != DomainChannel {
		t.Errorf("channel_sms 归域应为 %s，实得 %s", DomainChannel, spec.Domain)
	}
	if spec.Risk != RiskWrite {
		t.Errorf("channel_sms 风险等级应为 %s，实得 %s", RiskWrite, spec.Risk)
	}
	if strings.TrimSpace(gatedDomainIntents[spec.Name]) == "" {
		t.Error("channel_sms 未在 gatedDomainIntents 登记非空理由")
	}
	for _, n := range defaultOpenDomainIntents {
		if n == spec.Name {
			t.Errorf("channel_sms 出现在 defaultOpenDomainIntents，与 DefaultOff 自相矛盾")
		}
	}
	for _, n := range RecommendedExposed() {
		if n == spec.Name {
			t.Errorf("推荐白名单包含了 DefaultOff 的 channel_sms，与「保守起步」定位相反")
		}
	}

	action, ok := spec.Params["action"]
	if !ok {
		t.Fatal("channel_sms 未声明 action 参数")
	}
	if !action.Required {
		t.Error("channel_sms 的 action 必须是必填参数")
	}
	wantEnum := []string{"setting_get", "setting_save", "test", "logs"}
	if strings.Join(action.Enum, ",") != strings.Join(wantEnum, ",") {
		t.Errorf("channel_sms 的 action 枚举应为 %v，实得 %v", wantEnum, action.Enum)
	}
	if len(spec.Caps) != 1 || spec.Caps[0] != "api_invoke" {
		t.Errorf("channel_sms 的能力集应为 [api_invoke]，实得 %v", spec.Caps)
	}
}

// TestSmsIntentParamsMatchEndpoints 核对意图声明的每个参数都能被对应
// action 的落点端点认识。
//
// 漏名/改名的后果是静默的：端点 ReadJSON 直接忽略不认识的字段，
// 调用方看到 code=0 却发现筛选没生效。所以逐 action 核对而不是看意图级并集。
//
// 类型口径（R130）：urlparam 端点在 catalog 里天然没有类型，不能拿意图的
// integer 与 catalog 的 string 做严格相等；只显式钉住数值语义字段必须是
// integer（模型据此传时间戳/状态码，而不是"昨天"这类任意文本）。
func TestSmsIntentParamsMatchEndpoints(t *testing.T) {
	spec, ok := specByName("channel_sms")
	if !ok {
		t.Fatal("意图 channel_sms 不存在")
	}
	doc := loadSmsCatalog(t)

	const (
		settingPath = "/system/api/plugin/sms/setting"
		testPath    = "/system/api/plugin/sms/test"
		logsPath    = "/system/api/plugin/sms/logs"
	)

	// 第一步：四个端点必须全部在 catalog 里（R131）。
	endpointParams := map[string]map[string]string{
		"setting_get":  smsCatalogParams(t, doc, "GET", settingPath),
		"setting_save": smsCatalogParams(t, doc, "POST", settingPath),
		"test":         smsCatalogParams(t, doc, "POST", testPath),
		"logs":         smsCatalogParams(t, doc, "GET", logsPath),
	}

	// 第二步：Compose 的落点必须与接口约定逐字一致。
	wantRoutes := map[string]string{
		"setting_get":  "GET " + settingPath,
		"setting_save": "POST " + settingPath,
		"test":         "POST " + testPath,
		"logs":         "GET " + logsPath,
	}
	routes, ok := intentRouteRegistry[spec.Name]
	if !ok || len(routes) == 0 {
		t.Fatalf("%s 未用 invokeRoutes 登记任何路由：覆盖审计与审批门都会失真", spec.Name)
	}
	for action, want := range wantRoutes {
		target, ok := routes[action]
		if !ok {
			t.Errorf("action %s 未登记落点（期望 %s）", action, want)
			continue
		}
		if got := normalizeTarget(target); got != want {
			t.Errorf("action %s 落点应为 %s，实得 %s", action, want, got)
		}
	}
	if len(routes) != len(wantRoutes) {
		t.Errorf("channel_sms 登记了 %d 个 action，接口约定是 %d 个", len(routes), len(wantRoutes))
	}

	// 第三步：逐 action 核对参数名。page/page_size 先按分页词表换算成
	// 端点真正读的 current/pageSize；action/values 豁免（分别是分派字段与
	// 运行时展开的对象通配）。
	forwarded := map[string][]string{
		"setting_get":  nil,
		"setting_save": {"provider"},
		"test":         nil,
		"logs":         {"page", "page_size", "scene", "provider", "status", "phone", "start_time", "end_time"},
	}
	for action, names := range forwarded {
		recognized := endpointParams[action]
		for _, name := range names {
			if _, ok := spec.Params[name]; !ok {
				t.Errorf("意图未声明 %s 参数（action %s 依赖它）", name, action)
				continue
			}
			target := name
			if renamed, ok := paginationRenames[name]; ok {
				target = renamed
			}
			if _, ok := recognized[target]; !ok {
				t.Errorf("action %s 的目标端点不认识参数 %s（换算后 %s）：值会被静默忽略", action, name, target)
			}
		}
	}

	// 每个声明参数都必须至少被一个 action 消费，否则就是摆设。
	covered := map[string]bool{"action": true, "values": true}
	for _, names := range forwarded {
		for _, name := range names {
			covered[name] = true
		}
	}
	var orphans []string
	for name := range spec.Params {
		if !covered[name] {
			orphans = append(orphans, name)
		}
	}
	if len(orphans) > 0 {
		sort.Strings(orphans)
		t.Errorf("channel_sms 声明了未被任何 action 消费的参数: %v", orphans)
	}

	// 数值语义字段必须是 integer。
	for _, name := range []string{"status", "start_time", "end_time", "page", "page_size"} {
		p, ok := spec.Params[name]
		if !ok {
			t.Errorf("channel_sms 未声明 %s 参数", name)
			continue
		}
		if p.Type != "integer" {
			t.Errorf("%s 应为 integer（时间戳/状态码），实得 %s", name, p.Type)
		}
	}
	if values, ok := spec.Params["values"]; !ok || values.Type != "object" {
		t.Errorf("values 应为 object（setting_save 传完整配置、test 传号码），实得 %+v", values)
	}
}

// TestSmsSecretFieldNamesCovered 锁住短信配置里的凭据字段全部被出口掩码。
//
// 反向对照的价值在这一行：把 _token 加宽成后缀会让 token_expire/token_type
// 变成 ********，调用方会误判「配置没填」——所以 token_expire 必须为 false。
func TestSmsSecretFieldNamesCovered(t *testing.T) {
	secret := []string{
		"auth_token",     // Twilio 账号令牌
		"account_sid",    // Twilio 账号 SID：Basic auth 用户名侧
		"api_key_sid",    // Twilio API Key SID：Basic auth 用户名侧
		"api_key_secret", // Twilio API Key 密码侧（由 _secret 后缀覆盖）
		"access_key_id",  // 阿里云 AK：用户名侧
		"access_key_secret",
		"secret_id", // 腾讯云 SecretId：用户名侧
		"secret_key",
	}
	notSecret := []string{
		"token_expire", "token_type", "sign_name", "templates", "texts",
		"from", "messaging_service_sid", "sms_sdk_app_id", "provider",
	}
	for _, name := range secret {
		if !IsSecretField(name) {
			t.Errorf("%s 是凭据，必须被掩码", name)
		}
	}
	for _, name := range notSecret {
		if IsSecretField(name) {
			t.Errorf("%s 不是凭据，被掩码会让调用方误判配置状态", name)
		}
	}
}

// TestSmsDomainMapping 短信端点必须归入触达渠道域，否则 provider 侧
// TestAllEndpointsHaveDomain 会把四个端点报成 unknown。
func TestSmsDomainMapping(t *testing.T) {
	if got := domainByPlugin["sms"]; got != DomainChannel {
		t.Errorf("domainByPlugin[\"sms\"] 应为 %s，实得 %s", DomainChannel, got)
	}
	if got := DomainOfPath("/system/api/plugin/sms/logs"); got != DomainChannel {
		t.Errorf("DomainOfPath(/system/api/plugin/sms/logs) 应为 %s，实得 %s", DomainChannel, got)
	}
}
