package storage

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"kandaoni.com/anqicms/config"
)

type R2Storage struct {
	bucket string
	sess   *session.Session
}

func NewR2Storage(cfg *config.PluginStorageConfig) (*R2Storage, error) {
	// R2使用S3兼容API，但需要特定的endpoint
	sess, err := session.NewSession(&aws.Config{
		Endpoint:    aws.String(cfg.S3Endpoint),
		Region:      aws.String(cfg.S3Region),
		Credentials: credentials.NewStaticCredentials(cfg.S3AccessKey, cfg.S3SecretKey, ""),
	})
	if err != nil {
		return nil, err
	}

	return &R2Storage{
		bucket: cfg.S3Bucket,
		sess:   sess,
	}, nil
}

func (s *R2Storage) Put(ctx context.Context, key string, r io.Reader) error {
	_, err := s3manager.NewUploader(s.sess).UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

func (s *R2Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := s3manager.NewDownloader(s.sess).DownloadWithContext(ctx, buf, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func (s *R2Storage) Delete(ctx context.Context, key string) error {
	_, err := s3.New(s.sess).DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *R2Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s3.New(s.sess).HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var aerr awserr.Error
		if errors.As(err, &aerr) && aerr.Code() == "NotFound" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
