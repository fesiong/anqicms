package storage

import (
	"bytes"
	"context"
	"github.com/upyun/go-sdk/v3/upyun"
	"io"
	"kandaoni.com/anqicms/config"
)

type UpyunStorage struct {
	client *upyun.UpYun
}

func NewUpyunStorage(cfg *config.PluginStorageConfig) (*UpyunStorage, error) {
	up := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   cfg.UpyunBucket,
		Operator: cfg.UpyunOperator,
		Password: cfg.UpyunPassword,
	})

	return &UpyunStorage{
		client: up,
	}, nil
}

func (s *UpyunStorage) Put(ctx context.Context, key string, r io.Reader) error {
	err := s.client.Put(&upyun.PutObjectConfig{
		Path:   key,
		Reader: r,
	})

	if err != nil {
		return err
	}

	return nil
}

func (s *UpyunStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	r := bytes.NewBuffer(nil)
	_, err := s.client.Get(&upyun.GetObjectConfig{
		Path:   key,
		Writer: r,
	})
	if err != nil {
		return nil, err
	}

	return io.NopCloser(r), nil
}

func (s *UpyunStorage) Delete(ctx context.Context, key string) error {
	err := s.client.Delete(&upyun.DeleteObjectConfig{
		Path: key,
	})

	return err
}

func (s *UpyunStorage) Exists(ctx context.Context, key string) (bool, error) {
	info, err := s.client.GetInfo(key)
	if err != nil {
		return false, err
	}

	if info != nil {
		return true, nil
	}
	return false, nil
}

func (s *UpyunStorage) Move(ctx context.Context, src, dest string) error {
	return s.client.Move(&upyun.MoveObjectConfig{
		SrcPath:  src,
		DestPath: dest,
	})
}
