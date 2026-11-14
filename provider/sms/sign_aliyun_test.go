package sms

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// 本文件锁死「手写 ACS3-HMAC-SHA256 签名器 ≡ 官方 SDK」这条契约（规格 §11.1）。
// 向量由 tmp/smsvec 一次性采集（官方 SDK dysmsapi v5.6.0 + darabonba-openapi v2.2.2，
// httptest 抓取，时间与 nonce 取自 SDK 实际请求头，未注入），冻结在
// testdata/aliyun_sign.json。仓内测试只读向量，永不联网、不依赖 SDK。

type aliyunVectorInput struct {
	To         string `json:"to"`
	Code       string `json:"code"`
	OutId      string `json:"outId"`
	SignName   string `json:"signName"`
	TemplateId string `json:"templateId"`
	ParamName  string `json:"paramName"`
}

type aliyunVector struct {
	Name            string            `json:"name"`
	Endpoint        string            `json:"endpoint"`
	AccessKeyId     string            `json:"accessKeyId"`
	AccessKeySecret string            `json:"accessKeySecret"`
	Input           aliyunVectorInput `json:"input"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	RawQuery        string            `json:"rawQuery"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Authorization   string            `json:"authorization"`
}

func loadAliyunVectors(t *testing.T) []aliyunVector {
	t.Helper()
	data, err := os.ReadFile("testdata/aliyun_sign.json")
	if err != nil {
		t.Fatalf("读取向量文件失败: %v", err)
	}
	var file struct {
		Vectors []aliyunVector `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("解析向量文件失败: %v", err)
	}
	// 反空转：没有向量时后面的循环全部空转假绿。
	if len(file.Vectors) == 0 {
		t.Fatal("向量文件为空：护栏会空转")
	}
	return file.Vectors
}

// aliyunVectorRequest 按向量重建请求（含 SDK 实际使用的全部请求头）。
func aliyunVectorRequest(t *testing.T, v aliyunVector) *http.Request {
	t.Helper()
	u := "https://" + v.Endpoint + v.Path
	if v.RawQuery != "" {
		u += "?" + v.RawQuery
	}
	var body io.Reader
	if v.Body != "" {
		body = strings.NewReader(v.Body)
	}
	req, err := http.NewRequest(v.Method, u, body)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Host = v.Endpoint
	for k, val := range v.Headers {
		req.Header.Set(k, val)
	}
	return req
}

func aliyunVectorProfile(v aliyunVector) Profile {
	return Profile{AccessKeyId: v.AccessKeyId, AccessKeySecret: v.AccessKeySecret}
}

func aliyunVectorSignature(t *testing.T, v aliyunVector) string {
	t.Helper()
	idx := strings.LastIndex(v.Authorization, "Signature=")
	if idx < 0 {
		t.Fatalf("向量 Authorization 缺少 Signature 段: %q", v.Authorization)
	}
	return v.Authorization[idx+len("Signature="):]
}

// TestAliyunSignMatchesOfficialVector 用向量里 SDK 实际使用的时间与 nonce 喂
// SignAliyun，逐字节比对 Authorization 与 x-acs-content-sha256。
// 抓它的破坏：canonical request 构造（query 排序/编码、header 集合与大小写、
// payload hash）、stringToSign 拼装、HMAC 密钥任何一处与 SDK 不一致。
func TestAliyunSignMatchesOfficialVector(t *testing.T) {
	for _, v := range loadAliyunVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			req := aliyunVectorRequest(t, v)
			date, err := time.Parse("2006-01-02T15:04:05Z", v.Headers["x-acs-date"])
			if err != nil {
				t.Fatalf("解析向量 x-acs-date 失败: %v", err)
			}
			nonce := v.Headers["x-acs-signature-nonce"]
			if nonce == "" {
				t.Fatal("向量缺少 x-acs-signature-nonce")
			}
			if err := SignAliyun(req, aliyunVectorProfile(v), date, nonce); err != nil {
				t.Fatalf("SignAliyun 返回错误: %v", err)
			}
			if got := req.Header.Get("Authorization"); got != v.Authorization {
				t.Fatalf("Authorization 与官方 SDK 向量不一致\n got: %s\nwant: %s", got, v.Authorization)
			}
			if got, want := req.Header.Get("x-acs-content-sha256"), v.Headers["x-acs-content-sha256"]; got != want {
				t.Fatalf("x-acs-content-sha256 不一致\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// TestAliyunSignVectorNegativeControls 证明向量比对是字节级敏感的（规格 §11.1 步骤 9）：
// ① 向量签名改一个字节必须不等；② canonical query 的 %20 换成 + 必须不等
// （阿里云 V3 用 %20，写成 + 是经典错法）。
func TestAliyunSignVectorNegativeControls(t *testing.T) {
	for _, v := range loadAliyunVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			req := aliyunVectorRequest(t, v)
			date, err := time.Parse("2006-01-02T15:04:05Z", v.Headers["x-acs-date"])
			if err != nil {
				t.Fatalf("解析向量 x-acs-date 失败: %v", err)
			}
			if err := SignAliyun(req, aliyunVectorProfile(v), date, v.Headers["x-acs-signature-nonce"]); err != nil {
				t.Fatalf("SignAliyun 返回错误: %v", err)
			}
			got := req.Header.Get("Authorization")

			// ① 签名改一个字节 → 必须不等。
			corrupted := []byte(v.Authorization)
			last := corrupted[len(corrupted)-1]
			if last == '0' {
				corrupted[len(corrupted)-1] = '1'
			} else {
				corrupted[len(corrupted)-1] = '0'
			}
			if got == string(corrupted) {
				t.Fatal("向量签名改一个字节后仍然相等：比对不是字节级的")
			}

			// ② canonical query 的 %20 换成 + → 签名必须变。
			// 走与 SignAliyun 相同的 canonical/signature 管线，只变异编码规则。
			payloadHash := req.Header.Get("x-acs-content-sha256")
			canonical, _ := aliyunCanonicalRequest(req, payloadHash)
			mutated := strings.ReplaceAll(canonical, "%20", "+")
			if mutated == canonical {
				t.Skip("该向量 canonical query 不含 %20，此对照由 international 向量覆盖")
			}
			if aliyunSignature(mutated, v.AccessKeySecret) == aliyunVectorSignature(t, v) {
				t.Fatal("canonical query 的 %20 换成 + 后签名仍与 SDK 一致：编码规则未被锁死")
			}
		})
	}
}

// TestAliyunCanonicalQueryEncodesSpaceAsPercent20 直接断言 canonical query 的
// 编码形态（手工推导的字面量，不经过被测签名器计算期望值）：
// 国际向量 SignName="AnQi Test" → canonical 必须是 AnQi%20Test 而不是 AnQi+Test。
// 抓它的破坏：把 url.QueryEscape 的 + 直接当 canonical（经典错法）。
func TestAliyunCanonicalQueryEncodesSpaceAsPercent20(t *testing.T) {
	found := false
	for _, v := range loadAliyunVectors(t) {
		if !strings.Contains(v.Input.SignName, " ") {
			continue
		}
		found = true
		t.Run(v.Name, func(t *testing.T) {
			req := aliyunVectorRequest(t, v)
			canonical, _ := aliyunCanonicalRequest(req, req.Header.Get("x-acs-content-sha256"))
			if !strings.Contains(canonical, "SignName=AnQi%20Test") {
				t.Fatalf("canonical request 未包含 SignName=AnQi%%20Test:\n%s", canonical)
			}
			if strings.Contains(canonical, "SignName=AnQi+Test") {
				t.Fatalf("canonical request 把空格编成了 +:\n%s", canonical)
			}
		})
	}
	// 反空转：向量里必须至少有一个带空格的 SignName，否则本护栏从未执行。
	if !found {
		t.Fatal("向量中没有带空格的 SignName：%20 编码护栏会空转")
	}
}
