package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"lechat-backend/internal/database/dto"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

type avatarRepositoryStub struct {
	profile     dto.ProfileDto
	findErr     error
	updateErr   error
	updatedID   uint
	updatedPath string
}

func (s *avatarRepositoryStub) FindProfile(uint) (dto.ProfileDto, error) {
	return s.profile, s.findErr
}

func (s *avatarRepositoryStub) UpdateAvatarURL(userID uint, avatarURL string) error {
	s.updatedID = userID
	s.updatedPath = avatarURL
	return s.updateErr
}

type avatarStorageStub struct {
	uploadRequest   *v4.PresignedHTTPRequest
	downloadRequest *v4.PresignedHTTPRequest
	uploadErr       error
	downloadErr     error
	uploadKey       string
	uploadType      string
	uploadExpiry    time.Duration
	downloadKey     string
	downloadExpiry  time.Duration
}

func (s *avatarStorageStub) PresignUpload(_ context.Context, key, contentType string, expires time.Duration) (*v4.PresignedHTTPRequest, error) {
	s.uploadKey = key
	s.uploadType = contentType
	s.uploadExpiry = expires
	return s.uploadRequest, s.uploadErr
}

func (s *avatarStorageStub) PresignDownload(_ context.Context, key string, expires time.Duration) (*v4.PresignedHTTPRequest, error) {
	s.downloadKey = key
	s.downloadExpiry = expires
	return s.downloadRequest, s.downloadErr
}

func TestCreateAvatarUploadURLStoresKeyAndReturnsSignedHeaders(t *testing.T) {
	repository := &avatarRepositoryStub{}
	objectStorage := &avatarStorageStub{
		uploadRequest: &v4.PresignedHTTPRequest{
			URL:    "https://s3.example/upload",
			Method: http.MethodPut,
			SignedHeader: http.Header{
				"Content-Type": []string{"image/png"},
				"Host":         []string{"s3.example"},
			},
		},
	}
	service := NewAvatarService(repository, objectStorage)

	result, err := service.CreateUploadURL(context.Background(), 42, " image/png ")
	if err != nil {
		t.Fatalf("CreateUploadURL() error = %v", err)
	}
	if !strings.HasPrefix(result.AvatarURL, "avatars/42/") || !strings.HasSuffix(result.AvatarURL, ".png") {
		t.Fatalf("avatar_url = %q, want generated user-scoped PNG object key", result.AvatarURL)
	}
	if repository.updatedID != 42 || repository.updatedPath != result.AvatarURL {
		t.Fatalf("stored avatar path = (%d, %q), want (42, %q)", repository.updatedID, repository.updatedPath, result.AvatarURL)
	}
	if objectStorage.uploadKey != result.AvatarURL || objectStorage.uploadType != "image/png" {
		t.Fatalf("presigned upload request = (%q, %q), want (%q, image/png)", objectStorage.uploadKey, objectStorage.uploadType, result.AvatarURL)
	}
	if result.Method != http.MethodPut || result.Headers["Content-Type"] != "image/png" {
		t.Fatalf("upload method/headers = (%q, %#v), want PUT with Content-Type", result.Method, result.Headers)
	}
	if _, ok := result.Headers["Host"]; ok {
		t.Fatal("Host must not be returned as a browser-settable header")
	}
	if result.ExpiresIn != 900 || objectStorage.uploadExpiry != 15*time.Minute {
		t.Fatalf("upload expiry = %d seconds / %s, want 900 seconds / 15m", result.ExpiresIn, objectStorage.uploadExpiry)
	}
}

func TestCreateAvatarUploadURLRejectsUnsupportedTypeWithoutSideEffects(t *testing.T) {
	repository := &avatarRepositoryStub{}
	objectStorage := &avatarStorageStub{}
	service := NewAvatarService(repository, objectStorage)

	_, err := service.CreateUploadURL(context.Background(), 42, "image/gif")
	if !errors.Is(err, ErrUnsupportedAvatarContentType) {
		t.Fatalf("CreateUploadURL() error = %v, want unsupported content type", err)
	}
	if objectStorage.uploadKey != "" || repository.updatedPath != "" {
		t.Fatal("unsupported content type must not presign or update the profile")
	}
}

func TestCreateAvatarDownloadURLUsesStoredObjectKey(t *testing.T) {
	key := "avatars/42/avatar.png"
	repository := &avatarRepositoryStub{profile: dto.ProfileDto{AvatarURL: &key}}
	objectStorage := &avatarStorageStub{
		downloadRequest: &v4.PresignedHTTPRequest{
			URL:    "https://s3.example/download",
			Method: http.MethodGet,
		},
	}
	service := NewAvatarService(repository, objectStorage)

	result, err := service.CreateDownloadURL(context.Background(), 42)
	if err != nil {
		t.Fatalf("CreateDownloadURL() error = %v", err)
	}
	if objectStorage.downloadKey != key || result.DownloadURL != "https://s3.example/download" {
		t.Fatalf("download key/URL = (%q, %q), want (%q, signed URL)", objectStorage.downloadKey, result.DownloadURL, key)
	}
	if result.ExpiresIn != 900 || objectStorage.downloadExpiry != 15*time.Minute {
		t.Fatalf("download expiry = %d seconds / %s, want 900 seconds / 15m", result.ExpiresIn, objectStorage.downloadExpiry)
	}
}
