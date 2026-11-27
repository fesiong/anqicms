package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"kandaoni.com/anqicms/config"
)

// 腾讯云短信适配器（规格 §5.3）。只读 Profile，不读站点配置、不做校验兜底：
// 凭据／签名／模板缺失在 Resolve 阶段就拒了。请求形态与官方 SDK 逐字一致：
// POST /、JSON body、PhoneNumberSet 带 +、TemplateParamSet 是位置数组
// （由 testdata/tencent_sign.json 向量锁死）。

// tencentSendTimeout 发送超时 10s；不自动重试——短信按条计费，重试等于重复扣费。
const tencentSendTimeout = 10 * time.Second

// tencentPermanentCodes 已知 permanent 错误码 → 中文提示（规格 §5.6 腾讯云行：
// 签名未审核、模板未审核、SecretId 无效、SmsSdkAppId 不存在、余额不足、
// 号码格式非法）。产品特有码核验自官方 SDK sms@v1.3.172/v20210111/errors.go；
// AuthFailure.* 是云 API 网关级标准码（公共错误码文档，2026-11-14 核验）。
// 未列入的码一律 transient，且原样保留进 SendError.Code（排查唯一线索，
// 不得吞成「发送失败」）。平台消息为空时回落这里的中文提示。
// ⚠️ 新增码必须同步 tencent_test.go 的 tencentPermanentCases（go/ast 双向穷举护栏）。
var tencentPermanentCodes = map[string]string{
	// 签名未审核／不存在
	"FailedOperation.SignatureIncorrectOrUnapproved": "签名未审核或格式错误",
	"FailedOperation.MissingSignatureList":           "签名不存在或未通过审核",
	// 模板未审核／不存在
	"FailedOperation.TemplateIncorrectOrUnapproved": "模板未审核或内容不匹配",
	"FailedOperation.MissingTemplateList":           "模板不存在或未通过审核",
	"FailedOperation.TemplateUnapprovedOrNotExist":  "模板未审核或不存在",
	// SecretId 无效／签名错误／未授权（网关级）
	"AuthFailure.SecretIdNotFound":      "SecretId 不存在",
	"AuthFailure.InvalidSecretId":       "SecretId 非法（不是云 API 密钥）",
	"AuthFailure.SignatureFailure":      "签名错误（SecretKey 不正确）",
	"AuthFailure.UnauthorizedOperation": "请求未授权（CAM 权限不足）",
	// SmsSdkAppId 不存在／不属于该密钥
	"InvalidParameterValue.SdkAppIdNotExist":      "SmsSdkAppId 不存在",
	"UnauthorizedOperation.SmsSdkAppIdVerifyFail": "SmsSdkAppId 与云 API 密钥不匹配",
	// 余额不足／欠费停服
	"FailedOperation.InsufficientBalanceInSmsPackage":  "短信套餐包余量不足",
	"UnauthorizedOperation.ServiceSuspendDueToArrears": "欠费停服",
	// 号码格式非法
	"InvalidParameterValue.IncorrectPhoneNumber": "手机号格式错误",
	"FailedOperation.PhoneNumberParseFail":       "号码解析失败（不符合 E.164）",
}

// tencentSendPayload 字段顺序与官方 SDK SendSmsRequest 的声明顺序一致
// （sms@v1.3.172/v20210111/models.go:1732-1764），保证 json.Marshal 出来的
// body 与 SDK 逐字节相同——TC3 签的就是这段 body 的 SHA256。
// ExtendCode／SenderId 本期不用，SDK 侧为 nil 时同样不出现，故不声明。
type tencentSendPayload struct {
	PhoneNumberSet   []string `json:"PhoneNumberSet"`
	SmsSdkAppId      string   `json:"SmsSdkAppId"`
	TemplateId       string   `json:"TemplateId"`
	SignName         string   `json:"SignName,omitempty"` // 国际为空时整个键不出现
	TemplateParamSet []string `json:"TemplateParamSet"`   // 位置数组，不是对象
	SessionContext   string   `json:"SessionContext,omitempty"`
}

// tencentSendStatus 是 Response.SendStatusSet 的元素。
type tencentSendStatus struct {
	SerialNo       string `json:"SerialNo"`
	PhoneNumber    string `json:"PhoneNumber"`
	IsoCode        string `json:"IsoCode"`
	Code           string `json:"Code"`
	Message        string `json:"Message"`
	SessionContext string `json:"SessionContext"`
}

// tencentAPIError 是 Response.Error（接口/网关级失败，HTTP 200 也可能带它）。
// 形状核验自官方 SDK common@v1.3.172/http/response.go:53-61 的 ErrorResponse。
type tencentAPIError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type tencentSender struct {
	client *http.Client
}

// NewTencentSender 返回腾讯云发送器。任务 5 的 NewSender(provider) 工厂会调它。
func NewTencentSender() Sender {
	return &tencentSender{client: &http.Client{Timeout: tencentSendTimeout}}
}

func (s *tencentSender) Name() string { return config.SmsProviderTencent }

func (s *tencentSender) Send(ctx context.Context, p Profile, req SendRequest) (SendResult, error) {
	// PhoneNumberSet 用带 + 的 E.164（规格 §5.3）。一次只发一个号码，
	// 因此不会触发平台「单次请求不能混发国内与国际」的限制。
	payload, err := json.Marshal(tencentSendPayload{
		PhoneNumberSet:   []string{req.To},
		SmsSdkAppId:      p.SmsSdkAppId,
		TemplateId:       p.TemplateId,
		SignName:         p.SignName,
		TemplateParamSet: []string{req.Code},
		SessionContext:   req.OutId,
	})
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "BuildPayload", Message: err.Error()}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+p.Endpoint+"/", bytes.NewReader(payload))
	if err != nil {
		// 剥掉 URL（networkErrorMessage）：构造请求失败的文本里带整条 URL。
		return SendResult{}, &SendError{Class: ClassTransient, Code: "BuildRequest", Message: networkErrorMessage(err)}
	}
	if err := SignTencent(httpReq, p, payload, time.Now()); err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "Sign", Message: err.Error()}
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		// 剥掉 URL（networkErrorMessage）：*url.Error 的文本含整条 URL。
		return SendResult{}, &SendError{Class: ClassTransient, Code: "NetworkError", Message: networkErrorMessage(err)}
	}
	defer resp.Body.Close()
	body, err := readSMSResponseBody(resp.Body)
	if err != nil {
		return SendResult{}, &SendError{Class: ClassTransient, Code: "ReadResponse", Message: err.Error()}
	}
	return parseTencentResponse(resp.StatusCode, body)
}

// parseTencentResponse 成功判据只有一个：HTTP 200 且
// Response.SendStatusSet[0].Code == "Ok"（大小写与阿里云的 "OK" 不同）。
// 空数组视为失败；Response.Error 是接口/网关级失败，优先于 SendStatusSet。
// HTTP 200 但 body 不是 JSON（网关插 HTML 错误页、CDN 拦截）判失败，
// 原始 body 片段截断 200 字节留进 Message（审查重点 5），绝不伪装成功。
func parseTencentResponse(status int, body []byte) (SendResult, error) {
	var payload struct {
		Response struct {
			SendStatusSet []tencentSendStatus `json:"SendStatusSet"`
			RequestId     string              `json:"RequestId"`
			Error         *tencentAPIError    `json:"Error"`
		} `json:"Response"`
	}
	jsonErr := json.Unmarshal(body, &payload)

	if status != http.StatusOK {
		code, message := "", ""
		if jsonErr == nil && payload.Response.Error != nil {
			code, message = payload.Response.Error.Code, payload.Response.Error.Message
		}
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", status)
		}
		if message == "" {
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
	if e := payload.Response.Error; e != nil && e.Code != "" {
		class, message := tencentErrorClass(e.Code, e.Message)
		return SendResult{}, &SendError{Class: class, Code: e.Code, Message: message}
	}
	if len(payload.Response.SendStatusSet) == 0 {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "EmptySendStatusSet",
			Message: "平台响应缺少 SendStatusSet: " + smsBodySnippet(body),
		}
	}
	status0 := payload.Response.SendStatusSet[0]
	if status0.Code == "" {
		return SendResult{}, &SendError{
			Class:   ClassTransient,
			Code:    "EmptyStatusCode",
			Message: "平台响应缺少下发状态码: " + smsBodySnippet(body),
		}
	}
	if status0.Code == "Ok" {
		return SendResult{BizId: status0.SerialNo, RawCode: status0.Code, RawMsg: status0.Message}, nil
	}
	class, message := tencentErrorClass(status0.Code, status0.Message)
	return SendResult{}, &SendError{Class: class, Code: status0.Code, Message: message}
}

// tencentErrorClass 已知 permanent 码回落中文提示；未列入的一律 transient，
// 且平台原始消息原样保留（排查唯一线索）。
func tencentErrorClass(code, message string) (ErrorClass, string) {
	if hint, ok := tencentPermanentCodes[code]; ok {
		if message == "" {
			message = hint
		}
		return ClassPermanent, message
	}
	return ClassTransient, message
}
