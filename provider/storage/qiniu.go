package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	storage2 "github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/downloader"
	"github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/objects"
	"io"
	"kandaoni.com/anqicms/config"
)

type QiniuStorage struct {
	token  string
	cfg    *config.PluginStorageConfig
	client *qbox.Mac
}

func NewQiniuStorage(cfg *config.PluginStorageConfig) (*QiniuStorage, error) {
	mac := qbox.NewMac(cfg.QiniuAccessKey, cfg.QiniuSecretKey)

	return &QiniuStorage{
		client: mac,
		cfg:    cfg,
	}, nil
}

func (s *QiniuStorage) Put(ctx context.Context, key string, r io.Reader) error {
	putPolicy := storage2.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", s.cfg.QiniuBucket, key),
	}
	upToken := putPolicy.UploadToken(s.client)

	cfg := storage2.Config{}
	// 是否使用https域名
	//cfg.UseHTTPS = true
	// 上传是否使用CDN上传加速
	cfg.UseCdnDomains = false
	region, _ := storage2.GetRegionByID(storage2.RegionID(s.cfg.QiniuRegion))
	cfg.Zone = &region

	formUploader := storage2.NewFormUploader(&cfg)
	ret := storage2.PutRet{}
	putExtra := storage2.PutExtra{}
	err := formUploader.Put(context.Background(), &ret, upToken, key, r, -1, &putExtra)
	if err != nil {
		return err
	}
	return nil
}

func (s *QiniuStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	urlsProvider := downloader.SignURLsProvider(downloader.NewDefaultSrcURLsProvider(s.client.AccessKey, nil), downloader.NewCredentialsSigner(s.client), nil)
	r := bytes.NewBuffer(nil)
	downloadManager := downloader.NewDownloadManager(&downloader.DownloadManagerOptions{})
	_, err := downloadManager.DownloadToWriter(context.Background(), key, r, &downloader.ObjectOptions{
		GenerateOptions:      downloader.GenerateOptions{BucketName: s.cfg.QiniuBucket},
		DownloadURLsProvider: urlsProvider,
	})
	if err != nil {
		return nil, err
	}
	return io.NopCloser(r), err
}

func (s *QiniuStorage) Delete(ctx context.Context, key string) error {
	objectsManager := objects.NewObjectsManager(&objects.ObjectsManagerOptions{
		Options: http_client.Options{Credentials: s.client},
	})
	bucket := objectsManager.Bucket(s.cfg.QiniuBucket)
	err := bucket.Object(key).Delete().Call(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (s *QiniuStorage) Exists(ctx context.Context, key string) (bool, error) {
	objectsManager := objects.NewObjectsManager(&objects.ObjectsManagerOptions{
		Options: http_client.Options{Credentials: s.client},
	})
	bucket := objectsManager.Bucket(s.cfg.QiniuBucket)
	_, err := bucket.Object(key).Stat().Call(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *QiniuStorage) Move(ctx context.Context, src, dest string) error {
	objectsManager := objects.NewObjectsManager(&objects.ObjectsManagerOptions{
		Options: http_client.Options{Credentials: s.client},
	})
	bucket := objectsManager.Bucket(s.cfg.QiniuBucket)
	err := bucket.Object(src).MoveTo(s.cfg.QiniuBucket, dest).Call(context.Background())
	if err != nil {
		return err
	}

	return nil
}
