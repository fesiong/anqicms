package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/oauth2/google"
	"kandaoni.com/anqicms/config"
)

type GoogleStorage struct {
	cfg     *config.PluginStorageConfig
	httpCli *http.Client
	bucket  string
}

func NewGoogleStorage(cfg *config.PluginStorageConfig) (*GoogleStorage, error) {
	conf, err := google.JWTConfigFromJSON([]byte(cfg.GoogleCredentialsJson),
		"https://www.googleapis.com/auth/devstorage.read_write")
	if err != nil {
		return nil, err
	}
	return &GoogleStorage{
		cfg:     cfg,
		httpCli: conf.Client(context.Background()),
		bucket:  cfg.GoogleBucketName,
	}, nil
}

func (s *GoogleStorage) gcsUrl(key string) string {
	return fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s", s.bucket, url.PathEscape(key))
}

func (s *GoogleStorage) gcsUploadUrl(key string) string {
	return fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s", s.bucket, url.QueryEscape(key))
}

func (s *GoogleStorage) Put(ctx context.Context, key string, r io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "POST", s.gcsUploadUrl(key), r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mime.TypeByExtension(path.Ext(key)))

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gcs upload failed (status=%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *GoogleStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.gcsUrl(key)+"?alt=media", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gcs get failed (status=%d): %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (s *GoogleStorage) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", s.gcsUrl(key), nil)
	if err != nil {
		return err
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gcs delete failed (status=%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *GoogleStorage) Exists(ctx context.Context, key string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.gcsUrl(key), nil)
	if err != nil {
		return false, err
	}
	// 只需要知道元数据是否存在，不需要下载内容
	req.Header.Set("Content-Range", "bytes */0")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return resp.StatusCode == 200, nil
}

func (s *GoogleStorage) Move(ctx context.Context, src, dest string) error {
	// Copy: POST /b/{bucket}/o/{src}/copyTo/b/{bucket}/o/{dest}
	copyUrl := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s/copyTo/b/%s/o/%s",
		s.bucket, url.PathEscape(src), s.bucket, url.PathEscape(dest))
	req, err := http.NewRequestWithContext(ctx, "POST", copyUrl, strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gcs copy failed (status=%d): %s", resp.StatusCode, string(body))
	}

	// 删除源文件
	return s.Delete(ctx, src)
}

// 确保实现了 Storage 接口
var _ Storage = (*GoogleStorage)(nil)
