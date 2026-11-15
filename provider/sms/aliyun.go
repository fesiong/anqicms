package sms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
		// BuildRequest 的错误文本里带整条 URL（query 里带 TemplateParam=验证码），
		// 必须剥掉（networkErrorMessage）。
		return SendResult{}, &SendError{Class: ClassTransient, Code: "BuildRequest", Message: networkErrorMessage(err)}
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
		// ⚠️ 绝不能直接用 err.Error()：*url.Error 的文本含整条 URL，
		// 而阿里云的验证码就在 query 的 TemplateParam 里 → 会随 err_msg 落库。
		return SendResult{}, &SendError{Class: ClassTransient, Code: "NetworkError", Message: networkErrorMessage(err)}
	}
	defer resp.Body.Close()
	body, err := readSMSResponseBody(resp.Body)
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
			message = smsBodySnippet(body)
		}
		return SendResult{}, &SendError{Class: ClassifyHTTPStatus(status), Code: code, Message: message}
	}
	if jsonErr != nil {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "NonJSONResponse",
			Message: "平台返回非 JSON 响应: " + smsBodySnippet(body),
		}
	}
	if payload.Code == "" {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "EmptyResponseCode",
			Message: "平台响应缺少 Code: " + smsBodySnippet(body),
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

// 响应体上限读取与日志片段截取走 httpbody.go 的共享 helper（裁决 R38）：
// 三家适配器面对同一个敌意 body 威胁模型，逻辑只留一份。
