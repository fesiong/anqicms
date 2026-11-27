package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"kandaoni.com/anqicms/model"
)

// ============================================================
// 组织标识自动获取（接口优先）。
//
// 组织标识（飞书 tenant_key / 钉钉 corp_id）在平台后台没有展示页，
// 要求站长手工填写等于把「拿不到的值」变成接入门槛。建连前先向平台接口要一次：
//   - 飞书：GET /open-apis/tenant/v2/tenant/query（tenant_access_token，需 tenant:tenant:readonly）；
//   - 钉钉：GET oapi.dingtalk.com/gettoken（企业内部应用，响应含 corp_id）。
// 取不到（权限未发布、响应缺字段、网络失败）不算接入失败：调用方转为
// 「首条入站事件锚定」（见 channelWorker.validateInboundIdentity）。
// ============================================================

const (
	// channelOrgResolveTimeout 是自动获取组织标识的时限。建连前是同步调用，
	// 没有超时会把整条连接建立流程永久卡住（与飞书机器人身份拉取同理）。
	channelOrgResolveTimeout = 5 * time.Second
	// feishuTenantQueryPath 是飞书「获取企业信息」接口。
	feishuTenantQueryPath = "/open-apis/tenant/v2/tenant/query"
	// dingTalkLegacyTokenPath 是钉钉旧版企业内部应用取 token 接口，
	// 新版 /v1.0/oauth2/accessToken 不返回 corp_id，因此这里特意用旧版。
	dingTalkLegacyTokenPath = "/gettoken"
	// dingTalkLegacyOpenAPIHost 是旧版接口域名。变量而不是常量：测试替换成本地服务。
	dingTalkLegacyOpenAPIHost = "https://oapi.dingtalk.com"
	// channelOrgResponseMaxBytes 限制读取的响应体，避免异常响应打满内存。
	channelOrgResponseMaxBytes = 1 << 20
)

// platformOrgResolvers 是平台名到组织标识解析器的注册表。
// 只放已实现的平台；未登记的平台直接报错，交给事件锚定兜底。
var platformOrgResolvers = map[string]func(context.Context, PlatformCredentials) (string, error){
	model.ChannelPlatformFeishu:   feishuTenantKey,
	model.ChannelPlatformDingTalk: dingTalkCorpId,
}

// ResolvePlatformOrganization 自动获取渠道所属平台的组织标识。
//
// 调用方必须把失败当作「转为事件锚定」的信号，而不是接入失败：
// 缺少权限、企业管理员未发布权限变更、网络抖动都会走到这里。
// 错误文案只含静态中文与业务码，绝不回显 appsecret / 响应正文。
func ResolvePlatformOrganization(ctx context.Context, platform string, creds PlatformCredentials) (string, error) {
	creds = normalizePlatformCredentials(creds)
	if creds.AppId == "" || creds.AppSecret == "" {
		return "", errors.New("平台应用 ID 或密钥未配置，无法自动获取组织标识")
	}
	name := strings.ToLower(strings.TrimSpace(platform))
	resolver, ok := platformOrgResolvers[name]
	if !ok {
		return "", fmt.Errorf("平台 %q 不支持自动获取组织标识", name)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	// 只在调用方没给时限时兜底：有明确时限时不要缩短它（测试会注入更短的超时）。
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, channelOrgResolveTimeout)
		defer cancel()
	}

	org, err := resolver(ctx, creds)
	if err != nil {
		return "", err
	}
	org = strings.TrimSpace(org)
	if org == "" {
		return "", errors.New("平台未返回组织标识")
	}
	return org, nil
}

// feishuTenantKey 通过「获取企业信息」接口取 tenant_key。
//
// 该接口在 lark SDK 里没有生成，与机器人身份接口一样用 raw Get + tenant_access_token 调。
// ⚠ 需要应用具备 tenant:tenant:readonly 权限；权限变更要发布新版本才生效，
// 未生效时接口会返回权限错误——这正是事件锚定兜底的典型场景。
func feishuTenantKey(ctx context.Context, creds PlatformCredentials) (string, error) {
	resp, err := feishuClientFor(creds).Get(ctx, feishuTenantQueryPath, nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("获取飞书企业信息超时")
		}
		return "", errors.New("调用飞书企业信息接口失败：" + sanitizePlatformMessage(err.Error()))
	}
	if resp == nil {
		return "", errors.New("飞书企业信息接口无响应")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("飞书企业信息接口返回 HTTP %d", resp.StatusCode)
	}
	return parseFeishuTenantKey(resp.RawBody)
}

// parseFeishuTenantKey 解析「获取企业信息」响应。抽成纯函数便于离线断言。
// 业务码非 0、缺 tenant_key、非法 JSON 都按「没取到」处理。
func parseFeishuTenantKey(raw []byte) (string, error) {
	var payload struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Tenant struct {
			TenantKey string `json:"tenant_key"`
		} `json:"tenant"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", errors.New("飞书企业信息响应不是合法 JSON")
	}
	if payload.Code != 0 {
		return "", fmt.Errorf("飞书企业信息返回业务错误 code=%d", payload.Code)
	}
	tenantKey := strings.TrimSpace(payload.Tenant.TenantKey)
	if tenantKey == "" {
		return "", errors.New("飞书企业信息未返回 tenant_key")
	}
	return tenantKey, nil
}

// dingTalkCorpId 通过旧版 gettoken 接口取 corp_id。
func dingTalkCorpId(ctx context.Context, creds PlatformCredentials) (string, error) {
	return queryDingTalkCorpId(ctx, dingTalkLegacyOpenAPIHost, &http.Client{Timeout: channelOrgResolveTimeout}, creds)
}

// queryDingTalkCorpId 调旧版 gettoken 并解析 corp_id。host/client 可注入（测试用本地服务）。
//
// ⚠ 网络错误不能把原始 error 写进返回值：*url.Error 的文本包含完整请求 URL，
// 而 URL 的 query 里带着 appsecret。这里只给静态文案。
func queryDingTalkCorpId(ctx context.Context, host string, client *http.Client, creds PlatformCredentials) (string, error) {
	creds = normalizePlatformCredentials(creds)
	if client == nil {
		client = &http.Client{Timeout: channelOrgResolveTimeout}
	}
	endpoint := strings.TrimRight(strings.TrimSpace(host), "/") + dingTalkLegacyTokenPath
	query := url.Values{}
	query.Set("appkey", creds.AppId)
	query.Set("appsecret", creds.AppSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return "", errors.New("构造钉钉取 token 请求失败")
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("调用钉钉取 token 接口超时")
		}
		return "", errors.New("调用钉钉取 token 接口失败（网络错误）")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, channelOrgResponseMaxBytes))
	if err != nil {
		return "", errors.New("读取钉钉取 token 响应失败")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("钉钉取 token 接口返回 HTTP %d", resp.StatusCode)
	}

	var payload struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		CorpId  string `json:"corp_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", errors.New("钉钉取 token 响应不是合法 JSON")
	}
	if payload.ErrCode != 0 {
		return "", fmt.Errorf("钉钉取 token 返回业务错误 errcode=%d", payload.ErrCode)
	}
	corpId := strings.TrimSpace(payload.CorpId)
	if corpId == "" {
		return "", errors.New("钉钉取 token 响应未包含 corp_id")
	}
	return corpId, nil
}
