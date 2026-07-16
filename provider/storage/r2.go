package storage

import (
	"kandaoni.com/anqicms/config"
)

// R2Storage Cloudflare R2（S3 兼容），复用 AwsS3Storage
type R2Storage = AwsS3Storage

func NewR2Storage(cfg *config.PluginStorageConfig) (*R2Storage, error) {
	return NewAwsStorage(cfg)
}

var _ Storage = (*R2Storage)(nil)
