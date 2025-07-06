package storage

import (
	"context"
	"github.com/tencentyun/cos-go-sdk-v5"
	"io"
	"kandaoni.com/anqicms/config"
	"net/http"
	"net/url"
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
	_, err := s.client.Object.Put(context.Background(), key, r, nil)
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
	_, _, err := s.client.Object.Copy(ctx, dest, src, nil)
	if err != nil {
		return err
	}

	return s.Delete(ctx, src)
}
