package sms

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

// 腾讯云 TC3-HMAC-SHA256 签名器。纯函数、无 IO：时间由调用方注入，payload 显式
// 传入（POST JSON body），便于用官方 SDK 对照向量逐字节验证（规格 §5.5／§11.1，
// 向量在 testdata/tencent_sign.json）。算法细节逐字对齐官方 SDK
// tencentcloud/common@v1.3.172/client.go:387-508 的 sendWithSignatureV3，
// 不复用 AWS SigV4 代码（派生链层数与 credential scope 字符串都不同）。

const (
	tencentSignAlgorithm = "TC3-HMAC-SHA256"
	tencentAction        = "SendSms"
	tencentAPIVersion    = "2021-01-11"
	tencentSignService   = "sms"
	tencentScopeTail     = "tc3_request"
	tencentDateLayout    = "2006-01-02"
	// TC3 只签这两个头，且顺序固定（SDK client.go:419-422 是字面量，不是排序结果）。
	tencentSignedHeaders = "content-type;host"
	tencentContentType   = "application/json"
)

// SignTencent 就地为 req 补齐 TC3 签名头（Content-Type／X-TC-Action／
// X-TC-Version／X-TC-Timestamp／X-TC-Region）并设置 Authorization。
// payload 是即将发出的 POST JSON body 原文，签名与实发 body 必须同源。
// credential scope 的日期按 UTC 从 now.Unix() 算——用本地日期是跨时区经典错法。
func SignTencent(req *http.Request, p Profile, payload []byte, now time.Time) error {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	// 与 SDK 逐字一致：先取 Unix 秒再转 UTC，而不是直接 now.Format（后者会
	// 跟着入参的时区走）。
	date := time.Unix(now.Unix(), 0).UTC().Format(tencentDateLayout)
	scope := tencentCredentialScope(date)

	req.Header.Set("Content-Type", tencentContentType)
	req.Header.Set("X-TC-Action", tencentAction)
	req.Header.Set("X-TC-Version", tencentAPIVersion)
	req.Header.Set("X-TC-Timestamp", timestamp)
	if p.CloudRegion != "" {
		req.Header.Set("X-TC-Region", p.CloudRegion)
	}

	canonical := tencentCanonicalRequest(req, tencentSHA256Hex(payload))
	stringToSign := tencentStringToSign(canonical, timestamp, scope)
	_, _, signingKey := tencentDeriveKeys(p.SecretKey, date)
	req.Header.Set("Authorization", tencentSignAlgorithm+
		" Credential="+p.SecretId+"/"+scope+
		", SignedHeaders="+tencentSignedHeaders+
		", Signature="+tencentSignature(stringToSign, signingKey))
	return nil
}

// tencentCanonicalRequest 组装规范请求：
//
//	Method\nCanonicalURI\nCanonicalQuery\nCanonicalHeaders\nSignedHeaders\nPayloadHash
//
// SendSms 是 POST JSON：CanonicalURI 恒 "/"、CanonicalQuery 恒空（SDK
// client.go:395-416 只在 GET 时构造 query）。CanonicalHeaders 自身每行以 \n
// 结尾，其后还有一个空行（与 SDK 的 fmt.Sprintf 布局逐字一致）。
// host 取 req.Host（Go 的 Host 头不在 Header map 里）。
func tencentCanonicalRequest(req *http.Request, payloadHash string) string {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	canonicalHeaders := "content-type:" + req.Header.Get("Content-Type") + "\nhost:" + host + "\n"
	return req.Method + "\n" +
		"/\n" +
		"\n" + // canonical query：POST JSON 恒空，但这一行必须在
		canonicalHeaders + "\n" +
		tencentSignedHeaders + "\n" +
		payloadHash
}

// tencentCredentialScope {UTC日期}/sms/tc3_request。
func tencentCredentialScope(date string) string {
	return date + "/" + tencentSignService + "/" + tencentScopeTail
}

// tencentStringToSign 算法名\n时间戳\ncredentialScope\nHex(SHA256(canonical))。
func tencentStringToSign(canonicalRequest, timestamp, scope string) string {
	return tencentSignAlgorithm + "\n" + timestamp + "\n" + scope + "\n" +
		tencentSHA256Hex([]byte(canonicalRequest))
}

// tencentDeriveKeys TC3 三层派生链（SDK client.go:487-493）：
//
//	SecretDate    = HMAC-SHA256(key="TC3"+SecretKey, msg=date)
//	SecretService = HMAC-SHA256(key=SecretDate,      msg="sms")
//	SecretSigning = HMAC-SHA256(key=SecretService,   msg="tc3_request")
//
// 返回原始字节（不是 hex）：下一层 HMAC 的密钥是上一层的原始摘要。
func tencentDeriveKeys(secretKey, date string) (secretDate, secretService, secretSigning []byte) {
	secretDate = tencentHMACSHA256([]byte("TC3"+secretKey), date)
	secretService = tencentHMACSHA256(secretDate, tencentSignService)
	secretSigning = tencentHMACSHA256(secretService, tencentScopeTail)
	return secretDate, secretService, secretSigning
}

// tencentSignature 用派生出的 signing key 对 stringToSign 做 HMAC，hex 小写输出。
func tencentSignature(stringToSign string, signingKey []byte) string {
	return hex.EncodeToString(tencentHMACSHA256(signingKey, stringToSign))
}

func tencentHMACSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func tencentSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
