package sms

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
// R35 起向量还冻了 SDK 构造的 canonical request 中间量（canonicalRequest /
// stringToSign / canonicalUri / canonicalQuery / canonicalHeaders / signedHeaders /
// payloadHash），由同一隔离模块里 SDK utils.GetAuthorization 的逐字副本对冻结输入
// 构造，并与上游导出函数、SDK 实抓 Authorization 三方交叉验证一致。

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

	// R35（规格 §11.1 步骤 6「逐字节比对 canonical request 中间量」）：
	// 由官方 SDK darabonba-openapi/v2@v2.2.2 utils.GetAuthorization 的逐字副本
	// 对同一组冻结输入构造，并与上游导出函数、SDK 实抓 Authorization 三方交叉
	// 验证一致后冻结。签名不一致时靠这几段定位是 query／header／payload 哪段偏了。
	CanonicalRequest string `json:"canonicalRequest"`
	StringToSign     string `json:"stringToSign"`
	CanonicalURI     string `json:"canonicalUri"`
	CanonicalQuery   string `json:"canonicalQuery"`
	CanonicalHeaders string `json:"canonicalHeaders"`
	SignedHeaders    string `json:"signedHeaders"`
	PayloadHash      string `json:"payloadHash"`
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
	// 反空转（R35）：缺 canonical 中间量时，逐字节比对会退化成「空串==空串」。
	// 用有序切片而不是 map：失败信息里的字段名必须确定（AGENTS.md §11.5）。
	for _, v := range file.Vectors {
		for _, field := range []struct {
			name string
			got  string
		}{
			{"canonicalRequest", v.CanonicalRequest},
			{"stringToSign", v.StringToSign},
			{"canonicalUri", v.CanonicalURI},
			{"canonicalQuery", v.CanonicalQuery},
			{"canonicalHeaders", v.CanonicalHeaders},
			{"signedHeaders", v.SignedHeaders},
			{"payloadHash", v.PayloadHash},
		} {
			if field.got == "" {
				t.Fatalf("向量 %s 缺少 %s：canonical 中间量护栏会空转（R35）", v.Name, field.name)
			}
		}
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
			// 规格 §11.1 步骤 6：canonical request 中间量也要逐字节比对。
			aliyunAssertCanonicalMatchesVector(t, req, v)
		})
	}
}

// aliyunAssertCanonicalMatchesVector 把签名器构造的 canonical request 与 SDK 冻结
// 的中间量逐字节比对，并逐段比对（method／URI／query／headers／SignedHeaders／
// payloadHash）——签名不一致时这是定位「哪一段偏了」的唯一调试资产（R35）。
func aliyunAssertCanonicalMatchesVector(t *testing.T, req *http.Request, v aliyunVector) {
	t.Helper()
	payloadHash := req.Header.Get("x-acs-content-sha256")
	canonical, signedHeaders := aliyunCanonicalRequest(req, payloadHash)
	if canonical != v.CanonicalRequest {
		t.Fatalf("canonical request 与官方 SDK 中间量不一致\n got: %q\nwant: %q", canonical, v.CanonicalRequest)
	}
	if got := strings.Join(signedHeaders, ";"); got != v.SignedHeaders {
		t.Fatalf("SignedHeaders 不一致\n got: %s\nwant: %s", got, v.SignedHeaders)
	}
	// 布局：method\nURI\nquery\nheader 行…\n\nSignedHeaders\npayloadHash
	// （canonicalHeaders 每行自带 \n，其后还有一个空行）
	lines := strings.Split(canonical, "\n")
	if len(lines) < 6 {
		t.Fatalf("canonical request 分段异常（%d 行）: %q", len(lines), canonical)
	}
	last := len(lines) - 1
	segments := []struct {
		name string
		got  string
		want string
	}{
		{"method", lines[0], v.Method},
		{"canonicalUri", lines[1], v.CanonicalURI},
		{"canonicalQuery", lines[2], v.CanonicalQuery},
		{"canonicalHeaders", strings.Join(lines[3:last-2], "\n") + "\n", v.CanonicalHeaders},
		{"空行分隔", lines[last-2], ""},
		{"signedHeaders", lines[last-1], v.SignedHeaders},
		{"payloadHash", lines[last], v.PayloadHash},
	}
	for _, s := range segments {
		if s.got != s.want {
			t.Fatalf("canonical 段 %s 与 SDK 中间量不一致\n got: %q\nwant: %q", s.name, s.got, s.want)
		}
	}
}

// aliyunVectorChainError 只用 stdlib + 冻结数据校验向量自洽（不碰被测签名器）：
// sha256(canonicalRequest) → stringToSign → HMAC-SHA256(SK) → Authorization 里的
// Signature，且分段能拼回整串、payloadHash 等于 body 的 sha256。
// 这证明冻进向量的 canonical request 正是 SDK 签名的那一份，不是随便一个字符串。
// 返回 nil 表示自洽；反向对照靠喂它一份被改坏的向量副本。
func aliyunVectorChainError(v aliyunVector) error {
	sum := sha256.Sum256([]byte(v.CanonicalRequest))
	stringToSign := "ACS3-HMAC-SHA256\n" + hex.EncodeToString(sum[:])
	if stringToSign != v.StringToSign {
		return fmt.Errorf("stringToSign 与 sha256(canonicalRequest) 不符\n got: %s\nwant: %s", v.StringToSign, stringToSign)
	}
	mac := hmac.New(sha256.New, []byte(v.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))
	if !strings.HasSuffix(v.Authorization, ",Signature="+signature) {
		return fmt.Errorf("HMAC(stringToSign) 与向量 Authorization 的 Signature 不符: %s", signature)
	}
	rebuilt := v.Method + "\n" + v.CanonicalURI + "\n" + v.CanonicalQuery + "\n" +
		v.CanonicalHeaders + "\n" + v.SignedHeaders + "\n" + v.PayloadHash
	if rebuilt != v.CanonicalRequest {
		return fmt.Errorf("分段拼不回 canonicalRequest（有人只改了其中一段）")
	}
	bodySum := sha256.Sum256([]byte(v.Body))
	if hex.EncodeToString(bodySum[:]) != v.PayloadHash {
		return fmt.Errorf("payloadHash 不等于 body 的 sha256")
	}
	if v.PayloadHash != v.Headers["x-acs-content-sha256"] {
		return fmt.Errorf("payloadHash 与 x-acs-content-sha256 头不一致")
	}
	return nil
}

// TestAliyunVectorCanonicalChainSelfConsistent 锁死「冻结的 canonical 中间量确实
// 是产出 SDK 签名的那一份」。抓它的破坏：把 canonicalRequest 换成任何别的字符串
// （哪怕 Authorization 仍对得上），或只改一个分段。
func TestAliyunVectorCanonicalChainSelfConsistent(t *testing.T) {
	for _, v := range loadAliyunVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			if err := aliyunVectorChainError(v); err != nil {
				t.Fatalf("向量 canonical 链条不自洽: %v", err)
			}
			// 反向对照：改坏 canonicalRequest 一个字节，链条校验必须报错。
			// 先立正锚（上面已 err==nil），再证明校验器抓得住错，避免空转。
			corrupted := v
			corrupted.CanonicalRequest = aliyunFlipLastByte(v.CanonicalRequest)
			if corrupted.CanonicalRequest == v.CanonicalRequest {
				t.Fatal("改坏 canonicalRequest 后仍与原值相同：反向对照空转")
			}
			if err := aliyunVectorChainError(corrupted); err == nil {
				t.Fatal("canonicalRequest 改坏一个字节后链条校验仍通过：校验不是字节级的")
			}
		})
	}
}

// aliyunFlipLastByte 把最后一个字节改掉（用于反向对照，保证结果一定不同）。
func aliyunFlipLastByte(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[len(b)-1] == '0' {
		b[len(b)-1] = '1'
	} else {
		b[len(b)-1] = '0'
	}
	return string(b)
}

// TestAliyunSignVectorNegativeControls 证明向量比对是字节级敏感的（规格 §11.1 步骤 9）：
// ⓪ 正锚：签名器输出必须等于 SDK 向量（R32，缺了它 ① 对任意垃圾都恒绿）；
// ① 向量签名改一个字节必须不等；③ canonical request 中间量改一个字节必须不等（R35）；
// ② canonical query 的 %20 换成 + 必须不等（阿里云 V3 用 %20，写成 + 是经典错法）。
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

			// ⓪ 正锚（R32）：先证明签名器输出**等于** SDK 向量，再做翻转对照。
			// 没有这一条，下面的「got != corrupted」对任意垃圾输出都恒绿——
			// 它证明的只是「字符串不等即不等」，抓不住签名器整体跑偏。
			if got != v.Authorization {
				t.Fatalf("正锚失败：签名器输出与 SDK 向量不一致\n got: %s\nwant: %s", got, v.Authorization)
			}

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

			// ③ canonical request 中间量改一个字节 → 必须不等（R35）。
			// 同样先立正锚：逐字节等于 SDK 冻结值，再证明比对抓得住单字节偏差。
			// 放在 ② 之前：② 对不含 %20 的向量会 t.Skip，不能把 ③ 一起跳过。
			payloadHash := req.Header.Get("x-acs-content-sha256")
			canonical, _ := aliyunCanonicalRequest(req, payloadHash)
			if canonical != v.CanonicalRequest {
				t.Fatalf("正锚失败：canonical request 与 SDK 中间量不一致\n got: %q\nwant: %q", canonical, v.CanonicalRequest)
			}
			if canonical == aliyunFlipLastByte(v.CanonicalRequest) {
				t.Fatal("canonical request 改一个字节后仍然相等：中间量比对不是字节级的")
			}

			// ② canonical query 的 %20 换成 + → 签名必须变。
			// 走与 SignAliyun 相同的 canonical/signature 管线，只变异编码规则。
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
