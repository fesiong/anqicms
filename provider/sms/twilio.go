package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
)

// Twilio 短信适配器（规格 §5.4）。只读 Profile，不读站点配置：凭据／发件方／文案
// 缺失在 Resolve 阶段就拒了，这里只补两条「提前拒能给出可读原因」的本地校验
// （发件方二选一、Endpoint 形状）。Twilio 无签名算法，HTTP Basic 就是全部鉴权。
//
// ⚠️ 裁决 R41：Profile.Endpoint 是**完整 URL**（resolve.go 的 twilioEndpointFormat
// 已把 AccountSid 填进去），不是 host。阿里云／腾讯云那套 "https://" + p.Endpoint
// 的形状照抄过来会双重前缀——Twilio 是三家里唯一的例外。

// twilioSendTimeout 发送超时 10s；不自动重试——短信按条计费，重试等于重复扣费。
const twilioSendTimeout = 10 * time.Second

// twilioPermanentCodes 已知 permanent 错误码 → 中文提示（规格 §5.6 Twilio 行：
// 20003 鉴权失败、21211 To 非法、21606 From 非法、21610 已退订、余额不足）。
// 码值核验自官方 Error and Warning Dictionary（2026-11-14），逐条链接见
// twilio_test.go 的 twilioPermanentCases 注释。
//
// 「余额不足」规格没给码：官方字典里没有任何一条标题叫 insufficient funds，
// 余额耗尽被停机落在 20005（Account not active，请求即拒）与 30002
// （Account suspended，已入队但投递前被停）两条上，二者的官方描述都提到
// 余额为负／欠费。
// ⚠️ 但**余额只是常见原因之一**（裁决 R48，2026-11-15 订正）：20005 另有
// 「试用未激活／父账户被停／ToS 审核」，30002 另有「AUP 违规／欺诈流量」。
// 故中文提示降级为「常见原因：…」，不把余额说成定论。分类仍判 permanent
// （两码都要人工充值或申诉）。未列入的码一律 transient，且原样保留进
// SendError.Code（排查唯一线索，不得吞成「发送失败」）。平台消息为空时回落
// 这里的中文提示。
// ⚠️ 新增码必须同步 twilio_test.go 的 twilioPermanentCases（go/ast 双向穷举护栏）。
var twilioPermanentCodes = map[string]string{
	// 鉴权失败（Permission Denied）
	"20003": "鉴权失败（AccountSid／AuthToken 或 API Key 不正确）",
	// To 非法
	"21211": "收件号码非法（不是合法 E.164）",
	// From 非法
	"21606": "发件号码非法（不是本账户可发消息的 Twilio 号码）",
	// 收件人已退订
	"21610": "收件人已退订（回复过 STOP）",
	// 账户未激活／被暂停（常见原因：余额问题）
	"20005": "账户未激活（常见原因：余额降到 $0 或以下）",
	"30002": "账户被暂停（常见原因：余额为负或欠费；已入队但未投递）",
}

// twilioMessage 是 2xx 的 Message 资源。error_code 是 integer|null、
// error_message 是 string|null（核验自官方 Message 资源文档，2026-11-14）；
// 用 *int64 才能把「null＝没有错误」与「0」区分开。
// 官方文档提醒 error_code/error_message 的取值可能随平台改进而变，因此这张表
// 只作分类提示：未识别的码兜底 transient，绝不因为查不到表就判成功。
type twilioMessage struct {
	Sid          string `json:"sid"`
	Status       string `json:"status"`
	ErrorCode    *int64 `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// twilioRestException 是非 2xx 的异常体（核验自 twilio.com/docs/usage/twilios-response）。
// ⚠️ 它的 status 是**整数** HTTP 码，与 Message 资源的字符串 status 同名不同型：
// 用一个结构体同时解两种形状会让整个 body 解析失败，平台 code 被吞成 HTTP_4xx。
// 所以两种形状各自一个结构体，不共用。
type twilioRestException struct {
	Code    *int64 `json:"code"`
	Message string `json:"message"`
}

type twilioSender struct {
	client *http.Client
}

// NewTwilioSender 返回 Twilio 发送器。NewSender(provider) 工厂会调它。
func NewTwilioSender() Sender {
	return &twilioSender{client: &http.Client{Timeout: twilioSendTimeout}}
}

func (s *twilioSender) Name() string { return config.SmsProviderTwilio }

func (s *twilioSender) Send(ctx context.Context, p Profile, req SendRequest) (SendResult, error) {
	// 裁决 R41②：发请求前校验 Endpoint 是 https 绝对 URL，畸形值 permanent 提前拒，
	// 不让它走到网络层才以 401/404 暴露。消息不回显 Endpoint 原值——它内嵌
	// AccountSid，而 AccountSid 是 §3.4 的掩码凭据，err_msg 不该把它反脱敏。
	u, err := url.Parse(p.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return SendResult{}, &SendError{
			Class:   ClassPermanent,
			Code:    "InvalidEndpoint",
			Message: "Twilio Endpoint 不是合法的 https 绝对 URL（应形如 https://api.twilio.com/2010-04-01/Accounts/{AccountSid}/Messages.json）",
		}
	}
	// 发件方二选一（规格 §5.4）。Resolve 已拒过，这里再拒一次是为了让直接构造
	// Profile 的调用方（后台测试发送）也拿到可读原因，而不是平台的一句英文报错。
	// errors.Join 同时满足两条契约：errors.Is 认得出 ErrSmsCredentialMissing，
	// errors.As 仍取得到 *SendError（sender.go「Send 的错误一律包成 *SendError」）。
	if p.From == "" && p.MessagingServiceSid == "" {
		return SendResult{}, errors.Join(ErrSmsCredentialMissing, &SendError{
			Class:   ClassPermanent,
			Code:    ErrSmsCredentialMissing.Error(),
			Message: "发件方未配置：From 与 MessagingServiceSid 至少填一个",
		})
	}

	form := url.Values{}
	// To 带 +（E.164）；Body 取已渲染的本地文案，不是 TemplateId 槽里的 {{code}} 原文。
	form.Set("To", req.To)
	form.Set("Body", req.Text)
	// 两者都填时只带 MessagingServiceSid（规格 §5.4）。
	if p.MessagingServiceSid != "" {
		form.Set("MessagingServiceSid", p.MessagingServiceSid)
	} else {
		form.Set("From", p.From)
	}
	// 本期不用 ContentSid 与 StatusCallback（规格 §3.2 YAGNI）；OutId 也不透传——
	// Twilio 这两个可选参数都被排除后，表单里没有承载它的位置。

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		// 剥掉 URL（networkErrorMessage）：Twilio 的 URL 路径里带 AccountSid。
		return SendResult{}, &SendError{Class: ClassTransient, Code: "BuildRequest", Message: networkErrorMessage(err)}
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Basic 凭据对二选一，API Key 优先；半对不算配齐，回落 AuthToken。
	// 两种情况下 URL 路径里的 AccountSid 都仍是 AccountSid（它恒必填）。
	user, pass := p.AccountSid, p.AuthToken
	if p.ApiKeySid != "" && p.ApiKeySecret != "" {
		user, pass = p.ApiKeySid, p.ApiKeySecret
	}
	httpReq.SetBasicAuth(user, pass)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		// 剥掉 URL（networkErrorMessage）：*url.Error 的文本含整条 URL（路径里有 AccountSid）。
		return SendResult{}, &SendError{Class: ClassTransient, Code: "NetworkError", Message: networkErrorMessage(err)}
	}
	defer resp.Body.Close()
	body, err := readSMSResponseBody(resp.Body)
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "ReadResponse", Message: err.Error()}
	}
	return parseTwilioResponse(resp.StatusCode, body)
}

// parseTwilioResponse 成功判据（规格 §5.4）：平台受理（HTTP 200／201）、
// status 不为 failed/undelivered、且 error_code 为空。error_code 非空时即使
// status == "queued" 也算失败，所以它优先于 status 判定。BizId 取 sid。
// HTTP 2xx 但 body 不是 JSON（网关插 HTML 错误页、CDN 拦截）判失败，
// 原始 body 片段截断 200 字节留进 Message（审查重点 5），绝不伪装成功。
func parseTwilioResponse(status int, body []byte) (SendResult, error) {
	// ⚠️ Twilio 的 POST 创建返回 **201 Created**，不是 200（官方「Twilio API
	// responses」状态码表：201 Created (For POST requests only)）。照抄阿里云／
	// 腾讯云的「只认 200」会把每一次真实成功发送判成 HTTP_201 失败。
	// 204 之类没有 Message 资源体的 2xx 不算受理，仍走下面的异常路径。
	if status != http.StatusOK && status != http.StatusCreated {
		var ex twilioRestException
		code, message := "", ""
		if err := json.Unmarshal(body, &ex); err == nil {
			if ex.Code != nil {
				code = strconv.FormatInt(*ex.Code, 10)
			}
			message = ex.Message
		}
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", status)
		}
		if message == "" {
			message = smsBodySnippet(body)
		}
		return SendResult{}, &SendError{Class: ClassifyHTTPStatus(status), Code: code, Message: message}
	}

	var msg twilioMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "NonJSONResponse",
			Message: "平台返回非 JSON 响应: " + smsBodySnippet(body),
		}
	}
	if msg.ErrorCode != nil {
		code := strconv.FormatInt(*msg.ErrorCode, 10)
		class, message := twilioErrorClass(code, msg.ErrorMessage)
		return SendResult{}, &SendError{Class: class, Code: code, Message: message}
	}
	switch msg.Status {
	case "":
		// 缺 status 就无从判断是否真的发出去了，必须落失败让站长可查。
		// 这里**不**回显 body 片段：Twilio 的 Message 资源带 body 字段（渲染正文
		// 含验证码），而规格 §10.5 明确「日志不存码、不存渲染正文」。
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "EmptyStatus",
			Message: "平台响应缺少 status，无法确认是否已下发",
		}
	case "failed", "undelivered":
		// 没有 error_code 的失败状态：无码可查 → transient（规格 §10.1「未知」），
		// Code 保留 status 原值当线索。
		message := msg.ErrorMessage
		if message == "" {
			message = "平台下发状态为 " + msg.Status
		}
		return SendResult{}, &SendError{Class: ClassTransient, Code: msg.Status, Message: message}
	}
	return SendResult{BizId: msg.Sid, RawCode: msg.Status, RawMsg: msg.ErrorMessage}, nil
}

// twilioErrorClass 已知 permanent 码回落中文提示；未列入的一律 transient，
// 且平台原始消息原样保留（排查唯一线索）。
func twilioErrorClass(code, message string) (ErrorClass, string) {
	if hint, ok := twilioPermanentCodes[code]; ok {
		if message == "" {
			message = hint
		}
		return ClassPermanent, message
	}
	return ClassTransient, message
}

// 响应体上限读取与日志片段截取走 httpbody.go 的共享 helper（裁决 R38）：
// 三家适配器面对同一个敌意 body 威胁模型，逻辑只留一份。
