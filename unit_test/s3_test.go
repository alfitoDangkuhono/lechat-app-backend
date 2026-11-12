package controllers_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"lechat-backend/config"
	"lechat-backend/internal/storage"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func testS3Storage(t *testing.T) *storage.S3Storage {
	t.Helper()

	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String("http://localhost:9000"),
		UsePathStyle: true,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test-access-key",
				SecretAccessKey: "test-secret-key",
			}, nil
		}),
	})
	store, err := storage.NewS3Storage(client, "test-bucket")
	if err != nil {
		t.Fatalf("NewS3Storage() error = %v", err)
	}
	return store
}

func TestNewS3StorageRequiresClientAndBucket(t *testing.T) {
	if _, err := storage.NewS3Storage(nil, "test-bucket"); err == nil {
		t.Fatal("NewS3Storage(nil, bucket) expected an error")
	}

	client := testS3Storage(t).Client
	if _, err := storage.NewS3Storage(client, " "); err == nil {
		t.Fatal("NewS3Storage(client, empty bucket) expected an error")
	}
}

func TestNewS3StorageFromConfig(t *testing.T) {
	store, err := storage.NewS3StorageFromConfig(config.S3Config{
		EndpointURL:     "http://localhost:9000",
		Region:          "us-east-1",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Bucket:          "test-bucket",
	})
	if err != nil {
		t.Fatalf("NewS3StorageFromConfig() error = %v", err)
	}

	request, err := store.PresignUpload(context.Background(), "avatars/42/avatar.png", "image/png", time.Minute)
	if err != nil {
		t.Fatalf("PresignUpload() after config initialization error = %v", err)
	}
	if !strings.HasPrefix(request.URL, "http://localhost:9000/test-bucket/avatars/42/avatar.png?") {
		t.Errorf("PresignUpload() URL = %q, want path-style S3 URL", request.URL)
	}
}

func TestNewS3StorageFromConfigRejectsInvalidSettings(t *testing.T) {
	tests := []struct {
		name   string
		config config.S3Config
	}{
		{
			name: "missing credentials",
			config: config.S3Config{
				Region: "us-east-1",
				Bucket: "test-bucket",
			},
		},
		{
			name: "invalid endpoint",
			config: config.S3Config{
				EndpointURL:     "ftp://localhost:9000",
				Region:          "us-east-1",
				AccessKeyID:     "test-access-key",
				SecretAccessKey: "test-secret-key",
				Bucket:          "test-bucket",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := storage.NewS3StorageFromConfig(tt.config); err == nil {
				t.Fatal("NewS3StorageFromConfig() expected an error")
			}
		})
	}
}

func TestPresignUploadAndDownload(t *testing.T) {
	storage := testS3Storage(t)
	expires := 15 * time.Minute

	upload, err := storage.PresignUpload(context.Background(), "users/42/avatar.png", "image/png", expires)
	if err != nil {
		t.Fatalf("PresignUpload() error = %v", err)
	}
	if upload.Method != "PUT" {
		t.Errorf("PresignUpload() method = %q, want PUT", upload.Method)
	}
	if !strings.HasPrefix(upload.URL, "http://localhost:9000/test-bucket/users/42/avatar.png?") {
		t.Errorf("PresignUpload() URL = %q, want path-style S3 URL", upload.URL)
	}
	if got := upload.SignedHeader.Get("Content-Type"); got != "image/png" {
		t.Errorf("PresignUpload() Content-Type header = %q, want image/png", got)
	}

	download, err := storage.PresignDownload(context.Background(), "users/42/avatar.png", expires)
	if err != nil {
		t.Fatalf("PresignDownload() error = %v", err)
	}
	if download.Method != "GET" {
		t.Errorf("PresignDownload() method = %q, want GET", download.Method)
	}
	if !strings.HasPrefix(download.URL, "http://localhost:9000/test-bucket/users/42/avatar.png?") {
		t.Errorf("PresignDownload() URL = %q, want path-style S3 URL", download.URL)
	}
}

func TestPresignRejectsInvalidRequest(t *testing.T) {
	storage := testS3Storage(t)

	tests := []struct {
		name    string
		key     string
		expires time.Duration
	}{
		{name: "empty key", expires: time.Minute},
		{name: "zero expiry", key: "object", expires: 0},
		{name: "subsecond expiry", key: "object", expires: time.Millisecond},
		{name: "expiry over seven days", key: "object", expires: 7*24*time.Hour + time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := storage.PresignDownload(context.Background(), tt.key, tt.expires); err == nil {
				t.Fatal("PresignDownload() expected an error")
			}
		})
	}
}
