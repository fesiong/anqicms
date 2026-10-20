package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

func hexOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type authInfo struct {
	date, region, signedHeaders, signature string
}

func parseAuth(auth string) (authInfo, error) {
	var p authInfo
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return p, fmt.Errorf("缺少 AWS4-HMAC-SHA256 前缀")
	}
	cred := ""
	for _, seg := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ", ") {
		switch {
		case strings.HasPrefix(seg, "Credential="):
			cred = strings.TrimPrefix(seg, "Credential=")
		case strings.HasPrefix(seg, "SignedHeaders="):
			p.signedHeaders = strings.TrimPrefix(seg, "SignedHeaders=")
		case strings.HasPrefix(seg, "Signature="):
			p.signature = strings.TrimPrefix(seg, "Signature=")
		}
	}
	cp := strings.Split(cred, "/")
	if len(cp) != 5 {
		return p, fmt.Errorf("credential scope 段数应为 5，实际 %d: %q", len(cp), cred)
	}
	if p.signedHeaders == "" || p.signature == "" {
		return p, fmt.Errorf("缺少 SignedHeaders 或 Signature")
	}
	p.date, p.region = cp[1], cp[2]
	return p, nil
}

func signWith(secret, date, region, stringToSign string) string {
	k := func(data string) []byte { return hmacSHA256([]byte("AWS4"+secret), data) }
	kRegion := hmacSHA256(k(date), region)
	kService := hmacSHA256(kRegion, "s3")
	return hex.EncodeToString(hmacSHA256(hmacSHA256(kService, "aws4_request"), stringToSign))
}

func buildCanonical(method, uri, signedHeaders string, hdr map[string]string, bodyHash string) string {
	var cb strings.Builder
	for _, h := range strings.Split(signedHeaders, ";") {
		cb.WriteString(h + ":" + strings.TrimSpace(hdr[h]) + "\n")
	}
	return method + "\n" + uri + "\n" + "" + "\n" +
		cb.String() + "\n" + signedHeaders + "\n" + bodyHash
}

// strictS3 只接受「编码后路径」这一种口径 —— 这才是 AWS SigV4 规范要求，
// 也是真实 S3/R2 服务端的行为（服务端按收到的原始 URI 规范化后校验）。
type strictS3 struct {
	t      *testing.T
	secret string
}

func (s *strictS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ai, err := parseAuth(r.Header.Get("Authorization"))
	if err != nil {
		s.t.Errorf("客户端 Authorization 格式异常: %v", err)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	bodyHash := r.Header.Get("x-amz-content-sha256")
	hdr := map[string]string{
		"host":                 r.Host,
		"x-amz-date":           r.Header.Get("x-amz-date"),
		"x-amz-content-sha256": bodyHash,
	}
	rawURI := r.RequestURI
	if i := strings.Index(rawURI, "?"); i >= 0 {
		rawURI = rawURI[:i]
	}

	// 真实 S3/R2 会先对收到的 URI 做规范化（%2F → /，逐段解码再按 segment 编码），
	// 然后用规范化结果校验签名。AWS SigV4 规范原文：canonical URI 是
	// 「URI-encoded path」，且 S3 会对每个 path segment 单独编码、保留分隔符。
	canonicalURI := normalizeS3URI(rawURI)

	cr := buildCanonical(r.Method, canonicalURI, ai.signedHeaders, hdr, bodyHash)
	sts := "AWS4-HMAC-SHA256\n" + hdr["x-amz-date"] + "\n" +
		ai.date + "/" + ai.region + "/s3/aws4_request\n" + hexOf([]byte(cr))

	if signWith(s.secret, ai.date, ai.region, sts) == ai.signature {
		s.t.Logf("    ✅ 签名通过 (canonical uri=%s)", canonicalURI)
		w.WriteHeader(http.StatusOK)
		return
	}
	s.t.Logf("    ❌ SignatureDoesNotMatch")
	s.t.Logf("       线上原始 URI      = %s", rawURI)
	s.t.Logf("       规范化 canonical  = %s", canonicalURI)
	s.t.Logf("       canonical request = %q", cr)
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, "SignatureDoesNotMatch")
}

// normalizeS3URI 模拟 S3/R2 服务端的 URI 规范化：
// 整条 path 先解码，再按 segment 逐个编码，斜杠保留为分隔符。
func normalizeS3URI(rawURI string) string {
	decoded, err := url.PathUnescape(rawURI)
	if err != nil {
		return rawURI
	}
	segs := strings.Split(decoded, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// 反向对照：错误 secret 必须被拒。
func TestNegativeControl(t *testing.T) {
	s := &strictS3{t: t, secret: "secretkey"}
	srv := httptest.NewServer(s)
	defer srv.Close()
	bad, _ := NewAwsStorage(&config.PluginStorageConfig{
		S3Endpoint: srv.URL, S3Bucket: "mybucket", S3Region: "auto",
		S3AccessKey: "ak", S3SecretKey: "WRONG",
	})
	if err := bad.Put(t.Context(), "uploads/x.txt", strings.NewReader("payload")); err == nil {
		t.Fatal("反向对照失败：错误 secret 被接受")
	}
	t.Log("✅ 反向对照通过")
}

func TestStrictPutFilenames(t *testing.T) {
	cases := []struct{ name, key string }{
		{"纯ASCII无特殊字符", "uploads/202610/20/photo.jpg"},
		{"含括号(zip归一化后)", "uploads/202610/20/zombie-nation-(usa).zip"},
		{"含空格", "uploads/202610/20/a b.zip"},
		{"含中文", "uploads/202610/20/测试文档.zip"},
		{"含&号", "uploads/202610/20/a&b.zip"},
		{"含+号", "uploads/202610/20/a+b.zip"},
		{"含%号", "uploads/202610/20/100%.zip"},
		{"含#号", "uploads/202610/20/a#b.zip"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &strictS3{t: t, secret: "secretkey"}
			srv := httptest.NewServer(s)
			defer srv.Close()
			st, _ := NewAwsStorage(&config.PluginStorageConfig{
				S3Endpoint: srv.URL, S3Bucket: "mybucket", S3Region: "auto",
				S3AccessKey: "ak", S3SecretKey: "secretkey",
			})
			t.Logf("  key = %s", c.key)
			if err := st.Put(t.Context(), c.key, strings.NewReader("payload")); err != nil {
				t.Errorf("  ⇒ 上传失败: %v", err)
			}
		})
	}
}

// 反向对照：修复改变了「线上路径」形态，必须确认对象 key 语义没变。
// 断言：斜杠仍是分隔符（不是 %2F），修复前能传成功的文件路径形态保持一致。
func TestObjectKeySemanticsUnchanged(t *testing.T) {
	cases := []struct{ key, want string }{
		{"uploads/202610/20/photo.jpg", "uploads/202610/20/photo.jpg"},
		{"uploads/202610/20/zombie-nation-(usa).zip", "uploads/202610/20/zombie-nation-%28usa%29.zip"},
		{"a.txt", "a.txt"},
		{"uploads/2026/中文 文件.zip", "uploads/2026/%E4%B8%AD%E6%96%87%20%E6%96%87%E4%BB%B6.zip"},
	}
	for _, c := range cases {
		got := escapeKey(c.key)
		t.Logf("key=%-44s → escaped=%s", c.key, got)
		if got != c.want {
			t.Errorf("key=%q\n  期望 %q\n  实际 %q", c.key, c.want, got)
		}
		if strings.Contains(got, "%2F") {
			t.Errorf("key=%q 的斜杠被编码成 %%2F，对象会从 uploads/x 变成单段名", c.key)
		}
	}

	// 关键：修复前 PathEscape 会把斜杠编码掉，导致对象 key 语义被破坏。
	// 断言新实现在同样输入下不产生 %2F。
	if strings.Contains(url.PathEscape("uploads/x.txt"), "%2F") == false {
		t.Fatal("前提失效：url.PathEscape 已不再编码斜杠，本测试失去意义")
	}
	t.Log("✅ 前提成立：url.PathEscape 确实会把 / 编码成 %2F（这正是原 bug）")
}
