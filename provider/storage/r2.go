package storage

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"kandaoni.com/anqicms/config"
)

type R2Storage struct {
	bucket string
	client *s3.Client
}

func NewR2Storage(cfg *config.PluginStorageConfig) (*R2Storage, error) {
	defCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion("auto"),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(defCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
	})

	return &R2Storage{
		bucket: cfg.S3Bucket,
		client: client,
	}, nil
}

func (s *R2Storage) Put(ctx context.Context, key string, r io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

func (s *R2Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, err
	}

	return out.Body, nil
}

func (s *R2Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	return err
}

func (s *R2Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
