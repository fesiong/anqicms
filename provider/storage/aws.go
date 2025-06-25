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

type AwsS3Storage struct {
	bucket string
	client *s3.Client
}

func NewAwsStorage(cfg *config.PluginStorageConfig) (*AwsS3Storage, error) {
	defCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion("us-west-2"),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(defCfg)

	return &AwsS3Storage{
		bucket: cfg.S3Bucket,
		client: client,
	}, nil
}

func (s *AwsS3Storage) Put(ctx context.Context, key string, r io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

func (s *AwsS3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, err
	}

	return out.Body, nil
}

func (s *AwsS3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	return err
}

func (s *AwsS3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
