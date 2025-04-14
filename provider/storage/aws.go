package storage

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"kandaoni.com/anqicms/config"
)

type AwsS3Storage struct {
	bucket string
	sess   *session.Session
}

func NewAwsStorage(cfg *config.PluginStorageConfig) (*AwsS3Storage, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(cfg.S3Region),
		Credentials: credentials.NewStaticCredentials(cfg.S3AccessKey, cfg.S3SecretKey, ""),
	})
	if err != nil {
		return nil, err
	}

	return &AwsS3Storage{
		bucket: cfg.S3Bucket,
		sess:   sess,
	}, nil
}

func (s *AwsS3Storage) Put(ctx context.Context, key string, r io.Reader) error {
	_, err := s3manager.NewUploader(s.sess).UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

func (s *AwsS3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
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

func (s *AwsS3Storage) Delete(ctx context.Context, key string) error {
	_, err := s3.New(s.sess).DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *AwsS3Storage) Exists(ctx context.Context, key string) (bool, error) {
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
