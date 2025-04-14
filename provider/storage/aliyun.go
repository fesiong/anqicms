package storage

import (
	"context"
	"io"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"kandaoni.com/anqicms/config"
)

type AliyunStorage struct {
	client *oss.Bucket
}

func NewAliyunStorage(cfg *config.PluginStorageConfig) (*AliyunStorage, error) {
	client, err := oss.New(cfg.AliyunEndpoint, cfg.AliyunAccessKeyId, cfg.AliyunAccessKeySecret)
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(cfg.AliyunBucketName)
	if err != nil {
		return nil, err
	}

	return &AliyunStorage{
		client: bucket,
	}, nil
}

func (s *AliyunStorage) Put(ctx context.Context, key string, r io.Reader) error {
	err := s.client.PutObject(key, r)
	if err != nil {
		return err
	}

	return nil
}

func (s *AliyunStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	reader, err := s.client.GetObject(key)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (s *AliyunStorage) Delete(ctx context.Context, key string) error {
	err := s.client.DeleteObject(key)

	return err
}

func (s *AliyunStorage) Exists(ctx context.Context, key string) (bool, error) {
	exist, err := s.client.IsObjectExist(key)

	return exist, err
}
