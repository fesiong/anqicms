package storage

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"

	"github.com/tencentyun/cos-go-sdk-v5"
	"kandaoni.com/anqicms/config"
)

type TencentStorage struct {
	client *cos.Client
}

func NewTencentStorage(cfg *config.PluginStorageConfig) (*TencentStorage, error) {
	u, _ := url.Parse(cfg.TencentBucketUrl)
	b := &cos.BaseURL{BucketURL: u}
	// Permanent key
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.TencentSecretId,
			SecretKey: cfg.TencentSecretKey,
		},
	})

	return &TencentStorage{
		client: client,
	}, nil
}

func (s *TencentStorage) Put(ctx context.Context, key string, r io.Reader) error {
	// 根据文件后缀设置ContentType
	contentType := mime.TypeByExtension(path.Ext(key))
	_, err := s.client.Object.Put(context.Background(), key, r, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		}})
	if err != nil {
		return err
	}

	return nil
}

func (s *TencentStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, err
	}

	return resp.Body, err
}

func (s *TencentStorage) Delete(ctx context.Context, key string) error {
	_, err := s.client.Object.Delete(ctx, key)
	if err != nil {
		return err
	}

	return nil
}

func (s *TencentStorage) Exists(ctx context.Context, key string) (bool, error) {
	exist, err := s.client.Object.IsExist(ctx, key)

	return exist, err
}

func (s *TencentStorage) Move(ctx context.Context, src, dest string) error {
	// sourceURL 格式: sourcebucket-appid.cos.region.myqcloud.com/sourcekey
	// 不能包含 http:// 或 https:// 前缀
	sourceURL := s.client.BaseURL.BucketURL.Host + "/" + src
	_, _, err := s.client.Object.Copy(ctx, dest, sourceURL, nil)
	if err != nil {
		return err
	}

	return s.Delete(ctx, src)
}
