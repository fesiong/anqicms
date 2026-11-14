package sms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
)

// 阿里云短信适配器（规格 §5.2）。只读 Profile，不读站点配置、不做校验兜底：
// 凭据／签名／模板缺失在 Resolve 阶段就拒了。请求形态与官方 SDK 逐字一致：
// POST /、参数在 query、空 body（由 testdata/aliyun_sign.json 向量锁死）。

// aliyunSendTimeout 发送超时 10s；不自动重试——短信按条计费，重试等于重复扣费。
const aliyunSendTimeout = 10 * time.Second

// aliyunMaxResponseBytes 响应体读取上限 64 KiB。本适配器明确预设了「网关插 HTML／
// CDN 拦截」这种敌意 body，而 10s 超时内高速链路能灌进数百 MB：先 io.ReadAll 再
// 截 200 字节等于把整条响应收进内存。片段只需要 200 字节，64 KiB 足够留现场。
const aliyunMaxResponseBytes = 64 << 10

// aliyunPermanentCodes 已知 permanent 错误码 → 中文提示（规格 §5.6）。
// 码值核验自官方 API 错误码文档（2026-11-14）；InvalidAccessKeyId.NotFound／
// Forbidden.AccessKeyDisabled 是 POP 网关级标准码。未列入的码一律 transient，
// 且原样保留进 SendError.Code（排查唯一线索，不得吞成「发送失败」）。
// 平台消息为空时回落这里的中文提示。
// ⚠️ 新增码必须同步 aliyun_test.go 的 aliyunPermanentCases（go/ast 双向穷举护栏）。
var aliyunPermanentCodes = map[string]string{
	// 签名不存在／未审核／禁用
	"isv.SMS_SIGNATURE_ILLEGAL": "签名不存在或未审核",
	"isv.SIGN_STATE_ILLEGAL":    "签名状态不可用",
	"isv.SMS_SIGN_ILLEGAL":      "签名禁止使用",
	// 模板不存在／未审核
	"isv.SMS_TEMPLATE_ILLEGAL": "模板不存在或未审核",
	// AK 无效或无权限
	"InvalidAccessKeyId.NotFound": "AccessKeyId 不存在",
	"Forbidden.AccessKeyDisabled": "AccessKey 已被禁用",
	"SignatureDoesNotMatch":       "签名不匹配（AccessKeySecret 错误）",
	"isp.RAM_PERMISSION_DENY":     "RAM 权限不足",
	"isv.ACCOUNT_NOT_EXISTS":      "账户不存在",
	"isv.PRODUCT_UN_SUBSCRIPT":    "未开通短信服务",
	// 账户余额不足
	"isv.AMOUNT_NOT_ENOUGH": "账户余额不足",
	"isv.OUT_OF_SERVICE":    "欠费停机",
	// 号码格式非法
	"isv.MOBILE_NUMBER_ILLEGAL": "手机号码格式非法",
	// 平台级流控
	"isv.BUSINESS_LIMIT_CONTROL": "触发平台流控",
	"isv.DAY_LIMIT_CONTROL":      "触发日发送限额",
	"isv.MONTH_LIMIT_CONTROL":    "触发月发送限额",
	"MOBILE_SEND_LIMIT":          "单号码发送超限",
}

type aliyunSender struct {
	client *http.Client
}

// NewAliyunSender 返回阿里云发送器。任务 5 的 NewSender(provider) 工厂会调它。
func NewAliyunSender() Sender {
	return &aliyunSender{client: &http.Client{Timeout: aliyunSendTimeout}}
}

func (s *aliyunSender) Name() string { return config.SmsProviderAliyun }

func (s *aliyunSender) Send(ctx context.Context, p Profile, req SendRequest) (SendResult, error) {
	templateParam, err := json.Marshal(map[string]string{p.ParamName: req.Code})
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "TemplateParam", Message: err.Error()}
	}
	query := url.Values{}
	// 裁决 R29：国内国际同一规则去 +。官方 SDK 原文：国内允许 86／+86／0086 前缀，
	// 国际及港澳台要求 [国家码][手机号]（不带 +），8613900001111 与 85200001234 都合法。
	query.Set("PhoneNumbers", strings.TrimPrefix(req.To, "+"))
	query.Set("SignName", p.SignName)
	query.Set("TemplateCode", p.TemplateId)
	query.Set("TemplateParam", string(templateParam))
	if req.OutId != "" {
		query.Set("OutId", req.OutId)
	}
	endpoint := "https://" + p.Endpoint + "/?" + query.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "BuildRequest", Message: err.Error()}
	}
	httpReq.Header.Set("accept", "application/json")
	var nonceBuf [16]byte
	if _, err := rand.Read(nonceBuf[:]); err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "Nonce", Message: err.Error()}
	}
	if err := SignAliyun(httpReq, p, time.Now(), hex.EncodeToString(nonceBuf[:])); err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "Sign", Message: err.Error()}
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "NetworkError", Message: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, aliyunMaxResponseBytes))
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "ReadResponse", Message: err.Error()}
	}
	return parseAliyunResponse(resp.StatusCode, body)
}

// parseAliyunResponse 成功判据只有一个：HTTP 200 且 Code == "OK"。
// HTTP 200 但 body 不是 JSON（网关插 HTML 错误页、CDN 拦截）判失败，
// 原始 body 片段截断 200 字节留进 Message（审查重点 5），绝不伪装成功。
func parseAliyunResponse(status int, body []byte) (SendResult, error) {
	var payload struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		BizId     string `json:"BizId"`
		RequestId string `json:"RequestId"`
	}
	jsonErr := json.Unmarshal(body, &payload)

	if status != http.StatusOK {
		code := payload.Code
		if jsonErr != nil || code == "" {
			code = fmt.Sprintf("HTTP_%d", status)
		}
		message := payload.Message
		if jsonErr != nil || message == "" {
			message = aliyunBodySnippet(body)
		}
		return SendResult{}, &SendError{Class: ClassifyHTTPStatus(status), Code: code, Message: message}
	}
	if jsonErr != nil {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "NonJSONResponse",
			Message: "平台返回非 JSON 响应: " + aliyunBodySnippet(body),
		}
	}
	if payload.Code == "" {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "EmptyResponseCode",
			Message: "平台响应缺少 Code: " + aliyunBodySnippet(body),
		}
	}
	if payload.Code == "OK" {
		return SendResult{BizId: payload.BizId, RawCode: payload.Code, RawMsg: payload.Message}, nil
	}
	class := ClassTransient
	message := payload.Message
	if hint, ok := aliyunPermanentCodes[payload.Code]; ok {
		class = ClassPermanent
		if message == "" {
			message = hint
		}
	}
	return SendResult{}, &SendError{Class: class, Code: payload.Code, Message: message}
}

// aliyunBodySnippet 原始 body 截断到 200 字节，供日志保留现场。
// 200 不是 UTF-8 rune 边界：国内网关／WAF／CDN 插进来的中文拦截页是 3 字节 rune，
// 硬切会留下半个汉字。这条 Message 的唯一目的地是 sms_logs.err_msg（规格 §10.4
// 「失败也留痕」），utf8mb4 列拒收非法字节序列时整条发送流水写入失败——最需要
// 留痕的「HTTP 200 非 JSON」反而什么线索都没留下。所以切完必须回到合法边界。
// 用 ToValidUTF8 而不是「整段 utf8.Valid 就退一字节」的循环：后者会在第一个非法
// 字节处停住，把其后的现场全部丢掉（非法字节在开头时更是整段清空）；ToValidUTF8
// 只丢弃非法字节（含被切开的半个 rune），其余现场原样保留，且字节数只减不增。
func aliyunBodySnippet(body []byte) string {
	const max = 200
	if len(body) > max {
		body = body[:max]
	}
	return strings.ToValidUTF8(string(body), "")
}
