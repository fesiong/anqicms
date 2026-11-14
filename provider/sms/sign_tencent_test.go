package sms

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 本文件锁死「手写 TC3-HMAC-SHA256 签名器 ≡ 官方 SDK」这条契约（规格 §11.1）。
// 向量由 tmp/smsvec 一次性采集（官方 SDK tencentcloud/sms v1.3.172 +
// common v1.3.172，httptest 抓取，时间戳取自 SDK 实际请求头 X-TC-Timestamp，
// 未注入），冻结在 testdata/tencent_sign.json。仓内测试只读向量，永不联网、
// 不依赖 SDK。
// canonical 中间量（canonicalRequest / stringToSign / canonicalHeaders /
// payloadHash / credentialScope）与派生密钥链（secretDate / secretService /
// secretSigning）由同一隔离模块里 SDK client.go:387-508 TC3 签名块的逐字副本
// 对冻结输入构造，并与 SDK 实抓 Authorization 逐字节交叉验证一致后冻结。
// 期望值不来自本仓 SignTencent——自证等于空转。

type tencentVectorInput struct {
	To          string `json:"to"`
	Code        string `json:"code"`
	OutId       string `json:"outId"`
	SignName    string `json:"signName"`
	TemplateId  string `json:"templateId"`
	SmsSdkAppId string `json:"smsSdkAppId"`
}

type tencentVector struct {
	Name          string             `json:"name"`
	Endpoint      string             `json:"endpoint"`
	SecretId      string             `json:"secretId"`
	SecretKey     string             `json:"secretKey"`
	CloudRegion   string             `json:"cloudRegion"`
	Input         tencentVectorInput `json:"input"`
	Method        string             `json:"method"`
	Path          string             `json:"path"`
	RawQuery      string             `json:"rawQuery"`
	Headers       map[string]string  `json:"headers"`
	Body          string             `json:"body"`
	Authorization string             `json:"authorization"`

	CanonicalRequest string `json:"canonicalRequest"`
	StringToSign     string `json:"stringToSign"`
	CanonicalURI     string `json:"canonicalUri"`
	CanonicalQuery   string `json:"canonicalQuery"`
	CanonicalHeaders string `json:"canonicalHeaders"`
	SignedHeaders    string `json:"signedHeaders"`
	PayloadHash      string `json:"payloadHash"`
	CredentialScope  string `json:"credentialScope"`
	Timestamp        string `json:"timestamp"`
	Date             string `json:"date"`
	SecretDate       string `json:"secretDate"`
	SecretService    string `json:"secretService"`
	SecretSigning    string `json:"secretSigning"`
}

func loadTencentVectors(t *testing.T) []tencentVector {
	t.Helper()
	data, err := os.ReadFile("testdata/tencent_sign.json")
	if err != nil {
		t.Fatalf("读取向量文件失败: %v", err)
	}
	var file struct {
		Vectors []tencentVector `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("解析向量文件失败: %v", err)
	}
	// 反空转：没有向量时后面的循环全部空转假绿。
	if len(file.Vectors) == 0 {
		t.Fatal("向量文件为空：护栏会空转")
	}
	for _, v := range file.Vectors {
		// 反空转：缺 canonical 中间量时逐字节比对会退化成「空串==空串」。
		// 用有序切片而不是 map：失败信息里的字段名必须确定（AGENTS.md §11.5）。
		for _, field := range []struct {
			name string
			got  string
		}{
			{"canonicalRequest", v.CanonicalRequest},
			{"stringToSign", v.StringToSign},
			{"canonicalUri", v.CanonicalURI},
			{"canonicalHeaders", v.CanonicalHeaders},
			{"signedHeaders", v.SignedHeaders},
			{"payloadHash", v.PayloadHash},
			{"credentialScope", v.CredentialScope},
			{"timestamp", v.Timestamp},
			{"date", v.Date},
			{"secretDate", v.SecretDate},
			{"secretService", v.SecretService},
			{"secretSigning", v.SecretSigning},
			{"authorization", v.Authorization},
			{"body", v.Body},
		} {
			if field.got == "" {
				t.Fatalf("向量 %s 缺少 %s：中间量护栏会空转", v.Name, field.name)
			}
		}
		// canonicalQuery 是 POST JSON 形态下**合法的空值**（SDK client.go:398
		// 只在 GET 时构造），不能要求非空；但必须冻成空串而不是漏字段。
		if v.CanonicalQuery != "" {
			t.Fatalf("向量 %s 的 canonicalQuery 应为空（SendSms 是 POST JSON）, got %q", v.Name, v.CanonicalQuery)
		}
		if v.Method != http.MethodPost {
			t.Fatalf("向量 %s 的 method 应为 POST, got %q", v.Name, v.Method)
		}
	}
	return file.Vectors
}

// tencentVectorRequest 按向量重建请求（含 SDK 实际使用的全部请求头）。
func tencentVectorRequest(t *testing.T, v tencentVector) *http.Request {
	t.Helper()
	u := "https://" + v.Endpoint + v.Path
	if v.RawQuery != "" {
		u += "?" + v.RawQuery
	}
	req, err := http.NewRequest(v.Method, u, strings.NewReader(v.Body))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Host = v.Endpoint
	for k, val := range v.Headers {
		req.Header.Set(k, val)
	}
	return req
}

func tencentVectorProfile(v tencentVector) Profile {
	return Profile{
		Provider:    "tencent",
		Endpoint:    v.Endpoint,
		SecretId:    v.SecretId,
		SecretKey:   v.SecretKey,
		CloudRegion: v.CloudRegion,
		SmsSdkAppId: v.Input.SmsSdkAppId,
		SignName:    v.Input.SignName,
		TemplateId:  v.Input.TemplateId,
	}
}

func tencentVectorTime(t *testing.T, v tencentVector) time.Time {
	t.Helper()
	ts, err := strconv.ParseInt(v.Timestamp, 10, 64)
	if err != nil {
		t.Fatalf("解析向量 timestamp 失败: %v", err)
	}
	return time.Unix(ts, 0)
}

func tencentVectorSignature(t *testing.T, v tencentVector) string {
	t.Helper()
	idx := strings.LastIndex(v.Authorization, "Signature=")
	if idx < 0 {
		t.Fatalf("向量 Authorization 缺少 Signature 段: %q", v.Authorization)
	}
	return v.Authorization[idx+len("Signature="):]
}

// tencentTestSHA256Hex 测试侧独立算哈希，不复用生产 helper（避免自证）。
func tencentTestSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// tencentTestHMAC 测试侧独立算 HMAC，返回原始字节。
func tencentTestHMAC(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// TestTencentSignMatchesOfficialVector 用向量里 SDK 实际使用的时间戳喂
// SignTencent，逐字节比对 Authorization 与 X-TC-Timestamp，并比对全部
// canonical 中间量与派生密钥链（规格 §11.1 步骤 6）。
// 抓它的破坏：canonical request 构造（header 集合/大小写/排序、空 query 行、
// payload hash）、credential scope 拼装、stringToSign 拼装、三层派生链任何一处
// 与 SDK 不一致。
func TestTencentSignMatchesOfficialVector(t *testing.T) {
	for _, v := range loadTencentVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			req := tencentVectorRequest(t, v)
			payload := []byte(v.Body)
			if err := SignTencent(req, tencentVectorProfile(v), payload, tencentVectorTime(t, v)); err != nil {
				t.Fatalf("SignTencent 返回错误: %v", err)
			}
			if got := req.Header.Get("Authorization"); got != v.Authorization {
				t.Fatalf("Authorization 与官方 SDK 向量不一致\n got: %s\nwant: %s", got, v.Authorization)
			}
			for _, h := range []struct {
				name string
				want string
			}{
				{"X-TC-Timestamp", v.Headers["x-tc-timestamp"]},
				{"X-TC-Action", v.Headers["x-tc-action"]},
				{"X-TC-Version", v.Headers["x-tc-version"]},
				{"X-TC-Region", v.Headers["x-tc-region"]},
				{"Content-Type", v.Headers["content-type"]},
			} {
				if got := req.Header.Get(h.name); got != h.want {
					t.Fatalf("%s 与 SDK 向量不一致\n got: %q\nwant: %q", h.name, got, h.want)
				}
			}
			tencentAssertCanonicalMatchesVector(t, req, v, payload)
		})
	}
}

// tencentAssertCanonicalMatchesVector 把签名器构造的 canonical request、
// stringToSign 与派生密钥链同 SDK 冻结的中间量逐字节比对，并逐段比对
// （method／URI／query／headers／SignedHeaders／payloadHash）——签名不一致时
// 这是定位「哪一段偏了」的唯一调试资产。
func tencentAssertCanonicalMatchesVector(t *testing.T, req *http.Request, v tencentVector, payload []byte) {
	t.Helper()
	payloadHash := tencentTestSHA256Hex(payload)
	if payloadHash != v.PayloadHash {
		t.Fatalf("payloadHash 与 SDK 中间量不一致\n got: %s\nwant: %s", payloadHash, v.PayloadHash)
	}
	canonical := tencentCanonicalRequest(req, payloadHash)
	if canonical != v.CanonicalRequest {
		t.Fatalf("canonical request 与官方 SDK 中间量不一致\n got: %q\nwant: %q", canonical, v.CanonicalRequest)
	}
	// 布局：method\nURI\nquery\nheader 行…\n\nSignedHeaders\npayloadHash
	// （canonicalHeaders 每行自带 \n，其后还有一个空行）
	lines := strings.Split(canonical, "\n")
	if len(lines) < 6 {
		t.Fatalf("canonical request 分段异常（%d 行）: %q", len(lines), canonical)
	}
	last := len(lines) - 1
	for _, s := range []struct {
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
	} {
		if s.got != s.want {
			t.Fatalf("canonical 段 %s 与 SDK 中间量不一致\n got: %q\nwant: %q", s.name, s.got, s.want)
		}
	}

	ts := req.Header.Get("X-TC-Timestamp")
	date := time.Unix(mustParseInt(t, ts), 0).UTC().Format("2006-01-02")
	scope := tencentCredentialScope(date)
	if scope != v.CredentialScope {
		t.Fatalf("credentialScope 不一致\n got: %s\nwant: %s", scope, v.CredentialScope)
	}
	stringToSign := tencentStringToSign(canonical, ts, scope)
	if stringToSign != v.StringToSign {
		t.Fatalf("stringToSign 与 SDK 中间量不一致\n got: %q\nwant: %q", stringToSign, v.StringToSign)
	}
	secretDate, secretService, secretSigning := tencentDeriveKeys(v.SecretKey, date)
	for _, s := range []struct {
		name string
		got  string
		want string
	}{
		{"secretDate", hex.EncodeToString(secretDate), v.SecretDate},
		{"secretService", hex.EncodeToString(secretService), v.SecretService},
		{"secretSigning", hex.EncodeToString(secretSigning), v.SecretSigning},
	} {
		if s.got != s.want {
			t.Fatalf("派生密钥 %s 与 SDK 中间量不一致\n got: %s\nwant: %s", s.name, s.got, s.want)
		}
	}
	if got := tencentSignature(stringToSign, secretSigning); got != tencentVectorSignature(t, v) {
		t.Fatalf("签名与 SDK 向量不一致\n got: %s\nwant: %s", got, tencentVectorSignature(t, v))
	}
}

func mustParseInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("解析整数失败 %q: %v", s, err)
	}
	return n
}

// tencentVectorChainError 只用 stdlib + 冻结数据校验向量自洽（不碰被测签名器）：
// body → payloadHash → canonicalRequest → stringToSign，以及
// "TC3"+SecretKey → date → sms → tc3_request 三层派生链 → Authorization 里的
// Signature。这证明冻进向量的中间量正是产出 SDK 签名的那一份，不是随便一个字符串。
// 返回 nil 表示自洽；反向对照靠喂它一份被改坏的向量副本。
func tencentVectorChainError(v tencentVector) error {
	if v.PayloadHash != tencentTestSHA256Hex([]byte(v.Body)) {
		return fmt.Errorf("payloadHash 不等于 body 的 sha256")
	}
	wantHeaders := "content-type:" + v.Headers["content-type"] + "\nhost:" + v.Endpoint + "\n"
	if v.CanonicalHeaders != wantHeaders {
		return fmt.Errorf("canonicalHeaders 与 content-type/host 不符\n got: %q\nwant: %q", v.CanonicalHeaders, wantHeaders)
	}
	if v.SignedHeaders != "content-type;host" {
		return fmt.Errorf("signedHeaders 不是 content-type;host: %q", v.SignedHeaders)
	}
	rebuilt := v.Method + "\n" + v.CanonicalURI + "\n" + v.CanonicalQuery + "\n" +
		v.CanonicalHeaders + "\n" + v.SignedHeaders + "\n" + v.PayloadHash
	if rebuilt != v.CanonicalRequest {
		return fmt.Errorf("分段拼不回 canonicalRequest（有人只改了其中一段）")
	}
	ts, err := strconv.ParseInt(v.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("timestamp 不是整数: %q", v.Timestamp)
	}
	date := time.Unix(ts, 0).UTC().Format("2006-01-02")
	if v.Date != date {
		return fmt.Errorf("date 不是 timestamp 的 UTC 日期: got %s want %s", v.Date, date)
	}
	scope := date + "/sms/tc3_request"
	if v.CredentialScope != scope {
		return fmt.Errorf("credentialScope 不符: got %s want %s", v.CredentialScope, scope)
	}
	stringToSign := "TC3-HMAC-SHA256\n" + v.Timestamp + "\n" + scope + "\n" + tencentTestSHA256Hex([]byte(v.CanonicalRequest))
	if stringToSign != v.StringToSign {
		return fmt.Errorf("stringToSign 与 sha256(canonicalRequest) 不符\n got: %s\nwant: %s", v.StringToSign, stringToSign)
	}
	secretDate := tencentTestHMAC([]byte("TC3"+v.SecretKey), date)
	if hex.EncodeToString(secretDate) != v.SecretDate {
		return fmt.Errorf("secretDate 与 HMAC(TC3+SecretKey, date) 不符")
	}
	secretService := tencentTestHMAC(secretDate, "sms")
	if hex.EncodeToString(secretService) != v.SecretService {
		return fmt.Errorf("secretService 与 HMAC(secretDate, sms) 不符")
	}
	secretSigning := tencentTestHMAC(secretService, "tc3_request")
	if hex.EncodeToString(secretSigning) != v.SecretSigning {
		return fmt.Errorf("secretSigning 与 HMAC(secretService, tc3_request) 不符")
	}
	signature := hex.EncodeToString(tencentTestHMAC(secretSigning, stringToSign))
	wantAuth := "TC3-HMAC-SHA256 Credential=" + v.SecretId + "/" + scope +
		", SignedHeaders=content-type;host, Signature=" + signature
	if v.Authorization != wantAuth {
		return fmt.Errorf("Authorization 与派生链重算结果不符\n got: %s\nwant: %s", v.Authorization, wantAuth)
	}
	if v.Headers["authorization"] != v.Authorization {
		return fmt.Errorf("headers.authorization 与 authorization 字段不一致")
	}
	if v.Headers["x-tc-timestamp"] != v.Timestamp {
		return fmt.Errorf("headers.x-tc-timestamp 与 timestamp 字段不一致")
	}
	return nil
}

// TestTencentVectorCanonicalChainSelfConsistent 锁死「冻结的 canonical 中间量与
// 派生密钥链确实是产出 SDK 签名的那一份」。抓它的破坏：把 canonicalRequest /
// secretDate 换成任何别的字符串（哪怕 Authorization 仍对得上），或只改一个分段。
func TestTencentVectorCanonicalChainSelfConsistent(t *testing.T) {
	for _, v := range loadTencentVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			// 正锚：先证明原始向量自洽，再证明校验器抓得住错。
			if err := tencentVectorChainError(v); err != nil {
				t.Fatalf("向量链条不自洽: %v", err)
			}
			for _, field := range []struct {
				name string
				get  func(*tencentVector) *string
			}{
				{"canonicalRequest", func(x *tencentVector) *string { return &x.CanonicalRequest }},
				{"stringToSign", func(x *tencentVector) *string { return &x.StringToSign }},
				{"secretDate", func(x *tencentVector) *string { return &x.SecretDate }},
				{"secretService", func(x *tencentVector) *string { return &x.SecretService }},
				{"secretSigning", func(x *tencentVector) *string { return &x.SecretSigning }},
				{"payloadHash", func(x *tencentVector) *string { return &x.PayloadHash }},
				{"credentialScope", func(x *tencentVector) *string { return &x.CredentialScope }},
			} {
				corrupted := v
				ptr := field.get(&corrupted)
				*ptr = tencentFlipLastByte(*ptr)
				if *ptr == *field.get(&v) {
					t.Fatalf("改坏 %s 后仍与原值相同：反向对照空转", field.name)
				}
				if err := tencentVectorChainError(corrupted); err == nil {
					t.Fatalf("%s 改坏一个字节后链条校验仍通过：校验不是字节级的", field.name)
				}
			}
		})
	}
}

// tencentFlipLastByte 把最后一个字节改掉（用于反向对照，保证结果一定不同）。
func tencentFlipLastByte(s string) string {
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

// tencentZoneWithDifferentDate 构造一个时区，使同一时刻的本地日期与 UTC 日期
// 不同（不依赖测试机的 time.Local，CI 上 TZ=UTC 也照样成立）。
// 偏移取 -(当日已过秒数+1)，本地时间落在前一天 23:59:59。
func tencentZoneWithDifferentDate(ts int64) (*time.Location, string, string) {
	utc := time.Unix(ts, 0).UTC()
	secsIntoDay := int64(utc.Hour())*3600 + int64(utc.Minute())*60 + int64(utc.Second())
	zone := time.FixedZone("SMS-TEST", -int(secsIntoDay+1))
	return zone, utc.Format("2006-01-02"), time.Unix(ts, 0).In(zone).Format("2006-01-02")
}

// TestTencentSignVectorNegativeControls 证明向量比对是字节级敏感的（规格 §11.1 步骤 9）：
// ⓪ 正锚：签名器输出必须等于 SDK 向量（缺了它 ① 对任意垃圾都恒绿）；
// ① 向量签名改一个字节必须不等；
// ② 派生密钥与 credential scope 的日期串从 UTC 日期换成本地日期必须不等
// （TC3 的 scope 日期必须按 UTC 算，跨时区是经典错法）；
// ③ canonical request 中间量改一个字节必须不等；
// ④ payload 改一个字节必须不等（证明签的是真实 body 而不是空串）。
func TestTencentSignVectorNegativeControls(t *testing.T) {
	for _, v := range loadTencentVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			req := tencentVectorRequest(t, v)
			now := tencentVectorTime(t, v)
			payload := []byte(v.Body)
			if err := SignTencent(req, tencentVectorProfile(v), payload, now); err != nil {
				t.Fatalf("SignTencent 返回错误: %v", err)
			}
			got := req.Header.Get("Authorization")

			// ⓪ 正锚：先证明签名器输出**等于** SDK 向量，再做翻转对照。
			if got != v.Authorization {
				t.Fatalf("正锚失败：签名器输出与 SDK 向量不一致\n got: %s\nwant: %s", got, v.Authorization)
			}

			// ① 签名改一个字节 → 必须不等。
			if got == tencentFlipLastByte(v.Authorization) {
				t.Fatal("向量签名改一个字节后仍然相等：比对不是字节级的")
			}

			// ③ canonical request 中间量改一个字节 → 必须不等。
			// 同样先立正锚，再证明比对抓得住单字节偏差。
			payloadHash := tencentTestSHA256Hex(payload)
			canonical := tencentCanonicalRequest(req, payloadHash)
			if canonical != v.CanonicalRequest {
				t.Fatalf("正锚失败：canonical request 与 SDK 中间量不一致\n got: %q\nwant: %q", canonical, v.CanonicalRequest)
			}
			if canonical == tencentFlipLastByte(v.CanonicalRequest) {
				t.Fatal("canonical request 改一个字节后仍然相等：中间量比对不是字节级的")
			}

			// ② 日期串换成「本地日期」→ 签名必须变。走与 SignTencent 相同的
			// canonical/stringToSign/派生链管线，只变异日期口径。
			ts := now.Unix()
			zone, utcDate, localDate := tencentZoneWithDifferentDate(ts)
			if localDate == utcDate {
				t.Fatal("构造的时区没能让本地日期与 UTC 日期不同：对照空转")
			}
			if gotDate := req.Header.Get("X-TC-Timestamp"); gotDate != v.Timestamp {
				t.Fatalf("正锚失败：X-TC-Timestamp = %q, want %q", gotDate, v.Timestamp)
			}
			mutatedScope := localDate + "/sms/tc3_request"
			mutatedSTS := tencentStringToSign(canonical, v.Timestamp, mutatedScope)
			_, _, mutatedKey := tencentDeriveKeys(v.SecretKey, localDate)
			if tencentSignature(mutatedSTS, mutatedKey) == tencentVectorSignature(t, v) {
				t.Fatal("把派生密钥日期换成本地日期后签名仍与 SDK 一致：UTC 口径未被锁死")
			}
			// 同一时刻换个时区表示，SignTencent 的输出必须一字不变。
			// 这条直接钉住生产实现（上面的管线变异只钉住 helper 的日期敏感性）。
			zoneReq := tencentVectorRequest(t, v)
			if err := SignTencent(zoneReq, tencentVectorProfile(v), payload, now.In(zone)); err != nil {
				t.Fatalf("SignTencent（非 UTC 时区）返回错误: %v", err)
			}
			if zoneReq.Header.Get("Authorization") != v.Authorization {
				t.Fatalf("同一时刻用非 UTC 时区表示就签出不同结果：credential scope 用了本地日期\n got: %s\nwant: %s",
					zoneReq.Header.Get("Authorization"), v.Authorization)
			}

			// ④ payload 改一个字节 → 签名必须变。
			mutatedPayload := append(append([]byte{}, payload...), ' ')
			payloadReq := tencentVectorRequest(t, v)
			if err := SignTencent(payloadReq, tencentVectorProfile(v), mutatedPayload, now); err != nil {
				t.Fatalf("SignTencent（变异 payload）返回错误: %v", err)
			}
			if payloadReq.Header.Get("Authorization") == v.Authorization {
				t.Fatal("payload 改一个字节后签名不变：签的不是真实 body")
			}
		})
	}
}

// TestTencentSignDerivesScopeDateFromTimestamp 直接断言 credential scope 的日期
// 来自 X-TC-Timestamp 的 UTC 日期，而不是调用方的本地日期或 time.Now()。
// 用一个人工构造的跨日时刻（UTC 2026-11-14T23:30:00Z，UTC+8 已是 11-15）。
// 抓它的破坏：date 用 now.Local()、或用 time.Now() 而不是入参 now。
func TestTencentSignDerivesScopeDateFromTimestamp(t *testing.T) {
	v := loadTencentVectors(t)[0]
	const ts = int64(1794699000) // 2026-11-14T23:30:00Z
	utc := time.Unix(ts, 0).UTC()
	if utc.Format("2006-01-02T15:04:05Z") != "2026-11-14T23:30:00Z" {
		t.Fatalf("构造的时刻不符: %s", utc.Format(time.RFC3339))
	}
	plus8 := time.FixedZone("UTC+8", 8*3600)
	if utc.In(plus8).Format("2006-01-02") != "2026-11-15" {
		t.Fatal("构造的时刻在 UTC+8 没有跨日：对照空转")
	}

	for _, loc := range []*time.Location{time.UTC, plus8} {
		req := tencentVectorRequest(t, v)
		if err := SignTencent(req, tencentVectorProfile(v), []byte(v.Body), time.Unix(ts, 0).In(loc)); err != nil {
			t.Fatalf("SignTencent 返回错误: %v", err)
		}
		auth := req.Header.Get("Authorization")
		if !strings.Contains(auth, "Credential="+v.SecretId+"/2026-11-14/sms/tc3_request,") {
			t.Fatalf("时区 %s 下 credential scope 的日期不是 UTC 日期 2026-11-14: %s", loc, auth)
		}
		if strings.Contains(auth, "/2026-11-15/") {
			t.Fatalf("时区 %s 下 credential scope 用了本地日期: %s", loc, auth)
		}
		if got := req.Header.Get("X-TC-Timestamp"); got != strconv.FormatInt(ts, 10) {
			t.Fatalf("X-TC-Timestamp = %q, want %d（必须来自入参 now）", got, ts)
		}
	}
}
