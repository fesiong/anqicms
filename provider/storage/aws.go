package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
)

type AwsS3Storage struct {
	bucket    string
	region    string
	accessKey string
	secretKey string
	endpoint  string // 自定义 endpoint（R2 用），空则用 AWS 默认
	httpCli   *http.Client
}

func NewAwsStorage(cfg *config.PluginStorageConfig) (*AwsS3Storage, error) {
	return &AwsS3Storage{
		bucket:    cfg.S3Bucket,
		region:    cfg.S3Region,
		accessKey: cfg.S3AccessKey,
		secretKey: cfg.S3SecretKey,
		endpoint:  cfg.S3Endpoint,
		httpCli:   http.DefaultClient,
	}, nil
}

func (s *AwsS3Storage) host() string {
	if s.endpoint != "" {
		return strings.TrimPrefix(strings.TrimPrefix(s.endpoint, "https://"), "http://")
	}
	return fmt.Sprintf("s3.%s.amazonaws.com", s.region)
}

func (s *AwsS3Storage) scheme() string {
	if s.endpoint != "" {
		if strings.HasPrefix(s.endpoint, "https://") {
			return "https"
		}
		return "http"
	}
	return "https"
}

func (s *AwsS3Storage) objectURL(key string) string {
	return fmt.Sprintf("%s://%s/%s/%s", s.scheme(), s.host(), s.bucket, url.PathEscape(key))
}

func (s *AwsS3Storage) objectPath(key string) string {
	return fmt.Sprintf("/%s/%s", s.bucket, url.PathEscape(key))
}

// sha256Hex 计算 body 的 SHA256 十六进制
func sha256Hex(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

// hmacSHA256 HMAC-SHA256
func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// signingKey 计算 AWS SigV4 签名密钥
func signingKey(secretKey, date, region string) []byte {
	kSecret := []byte("AWS4" + secretKey)
	kDate := hmacSHA256(kSecret, date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	return hmacSHA256(kService, "aws4_request")
}

// sign 对请求进行 SigV4 签名，设置 Authorization 等头部
func (s *AwsS3Storage) sign(req *http.Request, bodyHash string) {
	now := time.Now().UTC()
	date := now.Format("20060102")
	dateTime := now.Format("20060102T150405Z")

	// 补全必要头部
	req.Header.Set("x-amz-date", dateTime)
	req.Header.Set("x-amz-content-sha256", bodyHash)

	// 收集要签名的头部，按小写字母排序
	var signedHeaders []string
	headersMap := make(map[string]string)

	// host 取自 req.Host（Go 中 Host 头不存储在 Header 中）
	headersMap["host"] = req.Host
	signedHeaders = append(signedHeaders, "host")

	// x-amz-* 头部
	for _, h := range []string{"x-amz-date", "x-amz-content-sha256", "x-amz-copy-source"} {
		if v := req.Header.Get(h); v != "" {
			kl := strings.ToLower(h)
			headersMap[kl] = strings.TrimSpace(v)
			signedHeaders = append(signedHeaders, kl)
		}
	}
	sort.Strings(signedHeaders)

	// 构建 canonical headers
	var canonicalHeaders strings.Builder
	for _, h := range signedHeaders {
		canonicalHeaders.WriteString(h)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(headersMap[h])
		canonicalHeaders.WriteString("\n")
	}

	// 1. Canonical Request
	canonicalRequest := req.Method + "\n" +
		req.URL.Path + "\n" +
		"" + "\n" + // query string（S3 简单操作无 query）
		canonicalHeaders.String() + "\n" +
		strings.Join(signedHeaders, ";") + "\n" +
		bodyHash

	// 2. String to Sign
	credentialScope := date + "/" + s.region + "/s3/aws4_request"
	canonicalHash := sha256Hex([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" +
		dateTime + "\n" +
		credentialScope + "\n" +
		canonicalHash

	// 3. Signature
	sKey := signingKey(s.secretKey, date, s.region)
	signature := hex.EncodeToString(hmacSHA256(sKey, stringToSign))

	// 4. Authorization
	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, strings.Join(signedHeaders, ";"), signature)
	req.Header.Set("Authorization", auth)
}

func (s *AwsS3Storage) Put(ctx context.Context, key string, r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	bodyHash := sha256Hex(body)

	req, err := http.NewRequestWithContext(ctx, "PUT", s.objectURL(key), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Host = s.host()
	req.Header.Set("Content-Type", mime.TypeByExtension(path.Ext(key)))

	s.sign(req, bodyHash)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 put failed (status=%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (s *AwsS3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	emptyHash := sha256Hex(nil)

	req, err := http.NewRequestWithContext(ctx, "GET", s.objectURL(key), nil)
	if err != nil {
		return nil, err
	}
	req.Host = s.host()

	s.sign(req, emptyHash)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 get failed (status=%d): %s", resp.StatusCode, string(respBody))
	}
	return resp.Body, nil
}

func (s *AwsS3Storage) Delete(ctx context.Context, key string) error {
	emptyHash := sha256Hex(nil)

	req, err := http.NewRequestWithContext(ctx, "DELETE", s.objectURL(key), nil)
	if err != nil {
		return err
	}
	req.Host = s.host()

	s.sign(req, emptyHash)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 delete failed (status=%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (s *AwsS3Storage) Exists(ctx context.Context, key string) (bool, error) {
	emptyHash := sha256Hex(nil)

	req, err := http.NewRequestWithContext(ctx, "HEAD", s.objectURL(key), nil)
	if err != nil {
		return false, err
	}
	req.Host = s.host()

	s.sign(req, emptyHash)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return resp.StatusCode == 200, nil
}

func (s *AwsS3Storage) Move(ctx context.Context, src, dest string) error {
	emptyHash := sha256Hex(nil)

	copySource := "/" + s.bucket + "/" + url.PathEscape(src)

	req, err := http.NewRequestWithContext(ctx, "PUT", s.objectURL(dest), nil)
	if err != nil {
		return err
	}
	req.Host = s.host()
	req.Header.Set("x-amz-copy-source", copySource)

	s.sign(req, emptyHash)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 copy failed (status=%d): %s", resp.StatusCode, string(respBody))
	}

	// 删除源文件
	return s.Delete(ctx, src)
}

// 校验接口实现
var _ Storage = (*AwsS3Storage)(nil)
