package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/interfaces"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const avatarURLExpiry = 15 * time.Minute

var (
	ErrUnsupportedAvatarContentType = errors.New("avatar content type must be image/jpeg, image/png, or image/webp")
	ErrAvatarNotFound               = errors.New("profile avatar not found")
)

type avatarRepository interface {
	FindProfile(userID uint) (dto.ProfileDto, error)
	UpdateAvatarURL(userID uint, avatarURL string) error
}

type avatarStorage interface {
	PresignUpload(ctx context.Context, key, contentType string, expires time.Duration) (*v4.PresignedHTTPRequest, error)
	PresignDownload(ctx context.Context, key string, expires time.Duration) (*v4.PresignedHTTPRequest, error)
}

type AvatarService struct {
	repository avatarRepository
	storage    avatarStorage
}

func NewAvatarService(repository avatarRepository, storage avatarStorage) interfaces.IAvatarService {
	return &AvatarService{repository: repository, storage: storage}
}

func (s *AvatarService) CreateUploadURL(ctx context.Context, userID uint, contentType string) (dto.AvatarUploadURLDto, error) {
	if userID == 0 {
		return dto.AvatarUploadURLDto{}, errors.New("invalid user ID")
	}

	contentType = strings.ToLower(strings.TrimSpace(contentType))
	extension, ok := avatarExtensions[contentType]
	if !ok {
		return dto.AvatarUploadURLDto{}, ErrUnsupportedAvatarContentType
	}

	randomKey := make([]byte, 16)
	if _, err := rand.Read(randomKey); err != nil {
		return dto.AvatarUploadURLDto{}, fmt.Errorf("generate avatar object key: %w", err)
	}
	key := fmt.Sprintf("avatars/%d/%s%s", userID, hex.EncodeToString(randomKey), extension)

	presigned, err := s.storage.PresignUpload(ctx, key, contentType, avatarURLExpiry)
	if err != nil {
		return dto.AvatarUploadURLDto{}, err
	}
	if presigned == nil || presigned.URL == "" || presigned.Method == "" {
		return dto.AvatarUploadURLDto{}, errors.New("S3 returned an incomplete upload request")
	}
	if err := s.repository.UpdateAvatarURL(userID, key); err != nil {
		return dto.AvatarUploadURLDto{}, fmt.Errorf("save profile avatar path: %w", err)
	}

	return dto.AvatarUploadURLDto{
		UploadURL: presigned.URL,
		Method:    presigned.Method,
		Headers:   presignedHeaders(presigned.SignedHeader),
		AvatarURL: key,
		ExpiresIn: int(avatarURLExpiry.Seconds()),
	}, nil
}

func (s *AvatarService) CreateDownloadURL(ctx context.Context, userID uint) (dto.AvatarDownloadURLDto, error) {
	profile, err := s.repository.FindProfile(userID)
	if err != nil {
		return dto.AvatarDownloadURLDto{}, err
	}
	if profile.AvatarURL == nil || *profile.AvatarURL == "" {
		return dto.AvatarDownloadURLDto{}, ErrAvatarNotFound
	}

	presigned, err := s.storage.PresignDownload(ctx, *profile.AvatarURL, avatarURLExpiry)
	if err != nil {
		return dto.AvatarDownloadURLDto{}, err
	}
	if presigned == nil || presigned.URL == "" {
		return dto.AvatarDownloadURLDto{}, errors.New("S3 returned an incomplete download request")
	}
	return dto.AvatarDownloadURLDto{
		DownloadURL: presigned.URL,
		ExpiresIn:   int(avatarURLExpiry.Seconds()),
	}, nil
}

var avatarExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func presignedHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for name, values := range headers {
		if strings.EqualFold(name, "host") || len(values) == 0 {
			continue
		}
		result[http.CanonicalHeaderKey(name)] = strings.Join(values, ", ")
	}
	return result
}
