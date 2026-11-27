package sms

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// 阿里云 ACS3-HMAC-SHA256（V3）签名器。纯函数、无 IO：时间与 nonce 由调用方
// 注入，便于用官方 SDK 对照向量逐字节验证（规格 §5.5／§11.1，向量在
// testdata/aliyun_sign.json）。算法细节逐字对齐 darabonba-openapi/v2 v2.2.2
// 的 GetAuthorization，不复用 AWS SigV4 代码（派生链与 scope 完全不同：
// V3 没有密钥派生链，HMAC 密钥就是 AccessKeySecret 本身）。

const (
	aliyunSignAlgorithm = "ACS3-HMAC-SHA256"
	aliyunAction        = "SendSms"
	aliyunAPIVersion    = "2017-05-25"
	aliyunDateLayout    = "2006-01-02T15:04:05Z"
)

// SignAliyun 就地为 req 补齐 V3 签名头（x-acs-action／x-acs-version／x-acs-date／
// x-acs-signature-nonce／x-acs-content-sha256）并设置 Authorization。
// now 取 UTC 秒级 ISO8601；nonce 由调用方保证唯一。body 读后原样放回。
func SignAliyun(req *http.Request, p Profile, now time.Time, nonce string) error {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return fmt.Errorf("读取请求 body 失败: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])

	req.Header.Set("x-acs-action", aliyunAction)
	req.Header.Set("x-acs-version", aliyunAPIVersion)
	req.Header.Set("x-acs-date", now.UTC().Format(aliyunDateLayout))
	req.Header.Set("x-acs-signature-nonce", nonce)
	req.Header.Set("x-acs-content-sha256", payloadHash)

	canonical, signedHeaders := aliyunCanonicalRequest(req, payloadHash)
	signature := aliyunSignature(canonical, p.AccessKeySecret)
	req.Header.Set("Authorization", aliyunSignAlgorithm+
		" Credential="+p.AccessKeyId+
		",SignedHeaders="+strings.Join(signedHeaders, ";")+
		",Signature="+signature)
	return nil
}

// aliyunCanonicalRequest 组装规范请求：
//
//	Method\nCanonicalURI\nCanonicalQuery\nCanonicalHeaders\nSignedHeaders\nPayloadHash
//
// CanonicalHeaders 自身每行以 \n 结尾，其后还有一个空行（与 SDK 逐字一致）。
func aliyunCanonicalRequest(req *http.Request, payloadHash string) (string, []string) {
	canonicalHeaders, signedHeaders := aliyunCanonicalHeaders(req)
	path := req.URL.Path
	if path == "" {
		path = "/"
	}
	path = strings.ReplaceAll(path, "+", "%20")
	path = strings.ReplaceAll(path, "*", "%2A")
	path = strings.ReplaceAll(path, "%7E", "~")
	canonical := req.Method + "\n" +
		path + "\n" +
		aliyunCanonicalQuery(req.URL.Query()) + "\n" +
		canonicalHeaders + "\n" +
		strings.Join(signedHeaders, ";") + "\n" +
		payloadHash
	return canonical, signedHeaders
}

// aliyunCanonicalHeaders 只取 host、content-type 与 x-acs-* 头：键转小写、
// 值去首尾空白、按键升序；多值排序后逗号连接（与 SDK getCanonicalHeaders 一致）。
// host 取 req.Host（Go 的 Host 头不在 Header map 里）。
func aliyunCanonicalHeaders(req *http.Request) (string, []string) {
	values := make(map[string][]string)
	add := func(k, v string) {
		k = strings.ToLower(k)
		values[k] = append(values[k], strings.TrimSpace(v))
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	add("host", host)
	for k, vs := range req.Header {
		lk := strings.ToLower(k)
		if !strings.HasPrefix(lk, "x-acs-") && lk != "content-type" {
			continue
		}
		for _, v := range vs {
			add(lk, v)
		}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		vs := values[k]
		sort.Strings(vs)
		b.WriteString(k)
		b.WriteString(":")
		b.WriteString(strings.Join(vs, ","))
		b.WriteString("\n")
	}
	return b.String(), keys
}

// aliyunCanonicalQuery 按键升序、值做 RFC3986 编码（空格 %20 不是 +）。
// 与 SDK 一致：键本身不做百分号编码（SendSms 的键全是 ASCII 字母）；
// 空值渲染为 key=。适配器每个键只设单值，多值取第一个。
func aliyunCanonicalQuery(query url.Values) string {
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		vs := query[k]
		if len(vs) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("&")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(aliyunPercentEncode(vs[0]))
	}
	return b.String()
}

// aliyunPercentEncode RFC3986：空格编 %20（不是 +）、* 编 %2A、~ 不编。
func aliyunPercentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// aliyunSignature 由规范请求算签名：
// StringToSign = 算法名\nHex(SHA256(canonical))，HMAC 密钥就是 AccessKeySecret。
func aliyunSignature(canonicalRequest, accessKeySecret string) string {
	sum := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := aliyunSignAlgorithm + "\n" + hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, []byte(accessKeySecret))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}
