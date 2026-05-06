package storage

import (
	storage2 "cloud.google.com/go/storage"
	"context"
	"google.golang.org/api/option"
	"io"
	"kandaoni.com/anqicms/config"
)

type GoogleStorage struct {
	cfg    *config.PluginStorageConfig
	bucket *storage2.BucketHandle
}

func NewGoogleStorage(cfg *config.PluginStorageConfig) (*GoogleStorage, error) {
	client, err := storage2.NewClient(context.Background(), option.WithCredentialsJSON([]byte(cfg.GoogleCredentialsJson)))
	if err != nil {
		return nil, err
	}
	return &GoogleStorage{
		cfg:    cfg,
		bucket: client.Bucket(cfg.GoogleBucketName),
	}, nil
}

func (s *GoogleStorage) Put(ctx context.Context, key string, r io.Reader) error {
	w := s.bucket.Object(key).NewWriter(ctx)
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (s *GoogleStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.bucket.Object(key).NewReader(ctx)
}

func (s *GoogleStorage) Delete(ctx context.Context, key string) error {
	return s.bucket.Object(key).Delete(ctx)
}

func (s *GoogleStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.bucket.Object(key).Attrs(ctx)
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (s *GoogleStorage) Move(ctx context.Context, src, dest string) error {
	_, err := s.bucket.Object(dest).CopierFrom(s.bucket.Object(src)).Run(ctx)
	if err != nil {
		return err
	}

	return s.Delete(ctx, src)
}
