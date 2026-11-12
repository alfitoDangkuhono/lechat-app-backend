package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"lechat-backend/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const maxPresignedURLExpiry = 7 * 24 * time.Hour

type S3Storage struct {
	Client        *s3.Client
	PresignClient *s3.PresignClient
	Bucket        string
}

func NewS3StorageFromConfig(cfg config.S3Config) (*S3Storage, error) {
	if strings.TrimSpace(cfg.Region) == "" ||
		strings.TrimSpace(cfg.AccessKeyID) == "" ||
		strings.TrimSpace(cfg.SecretAccessKey) == "" ||
		strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("S3 region, credentials, and bucket must be configured")
	}

	options := s3.Options{
		Region: cfg.Region,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     cfg.AccessKeyID,
				SecretAccessKey: cfg.SecretAccessKey,
			}, nil
		}),
	}
	if cfg.EndpointURL != "" {
		endpoint, err := url.Parse(cfg.EndpointURL)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return nil, errors.New("S3 endpoint must be an absolute HTTP or HTTPS URL")
		}
		options.BaseEndpoint = aws.String(cfg.EndpointURL)
		options.UsePathStyle = true
	}

	return NewS3Storage(s3.New(options), cfg.Bucket)
}

func NewS3Storage(client *s3.Client, bucket string) (*S3Storage, error) {
	if client == nil {
		return nil, errors.New("S3 client must not be nil")
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("S3 bucket must not be empty")
	}

	return &S3Storage{
		Client:        client,
		PresignClient: s3.NewPresignClient(client),
		Bucket:        bucket,
	}, nil
}

func (s *S3Storage) PresignUpload(ctx context.Context, key, contentType string, expires time.Duration) (*v4.PresignedHTTPRequest, error) {
	if err := s.validatePresignRequest(ctx, key, expires); err != nil {
		return nil, err
	}

	input := &s3.PutObjectInput{
		Bucket: &s.Bucket,
		Key:    &key,
	}
	if contentType != "" {
		input.ContentType = &contentType
	}

	request, err := s.PresignClient.PresignPutObject(ctx, input, func(options *s3.PresignOptions) {
		options.Expires = expires
	})
	if err != nil {
		return nil, fmt.Errorf("presign S3 upload: %w", err)
	}
	return request, nil
}

func (s *S3Storage) PresignDownload(ctx context.Context, key string, expires time.Duration) (*v4.PresignedHTTPRequest, error) {
	if err := s.validatePresignRequest(ctx, key, expires); err != nil {
		return nil, err
	}

	request, err := s.PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.Bucket,
		Key:    &key,
	}, func(options *s3.PresignOptions) {
		options.Expires = expires
	})
	if err != nil {
		return nil, fmt.Errorf("presign S3 download: %w", err)
	}
	return request, nil
}

func (s *S3Storage) validatePresignRequest(ctx context.Context, key string, expires time.Duration) error {
	if s == nil || s.PresignClient == nil || strings.TrimSpace(s.Bucket) == "" {
		return errors.New("S3 storage is not initialized")
	}
	if ctx == nil {
		return errors.New("S3 request context must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" {
		return errors.New("S3 object key must not be empty")
	}
	if expires < time.Second || expires > maxPresignedURLExpiry {
		return fmt.Errorf("presigned URL expiry must be at least one second and at most %s", maxPresignedURLExpiry)
	}
	return nil
}
