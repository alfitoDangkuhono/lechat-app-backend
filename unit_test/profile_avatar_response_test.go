package controllers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
)

type profileAvatarServiceStub struct {
	downloadURL string
	expiresIn   int
	userID      uint
	err         error
}

func (s *profileAvatarServiceStub) CreateUploadURL(context.Context, uint, string) (dto.AvatarUploadURLDto, error) {
	return dto.AvatarUploadURLDto{}, nil
}

func (s *profileAvatarServiceStub) CreateDownloadURL(_ context.Context, userID uint) (dto.AvatarDownloadURLDto, error) {
	s.userID = userID
	return dto.AvatarDownloadURLDto{
		DownloadURL: s.downloadURL,
		ExpiresIn:   s.expiresIn,
	}, s.err
}

func TestProfileReturnsPresignedAvatarURL(t *testing.T) {
	avatarKey := "avatars/42/avatar.png"
	auth := &authenticationStub{
		profile: dto.ProfileDto{
			Id:        42,
			AvatarURL: &avatarKey,
		},
	}
	jwtService := &jwtStub{
		claims: &models.JwtClaims{
			UserId:    42,
			TokenType: models.TokenTypeAccess,
		},
	}
	avatarService := &profileAvatarServiceStub{
		downloadURL: "https://storage.example/presigned-avatar",
		expiresIn:   900,
	}

	recorder := profileRequest(auth, jwtService, "Bearer valid-access-token", avatarService)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data dto.ProfileDto `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.AvatarURL == nil || *response.Data.AvatarURL != avatarService.downloadURL {
		t.Fatalf("avatar_url = %v, want presigned URL %q", response.Data.AvatarURL, avatarService.downloadURL)
	}
	if response.Data.AvatarURLExpiresIn != avatarService.expiresIn {
		t.Fatalf("avatar_url_expires_in = %d, want %d", response.Data.AvatarURLExpiresIn, avatarService.expiresIn)
	}
	if avatarService.userID != 42 {
		t.Fatalf("avatar signing user ID = %d, want 42", avatarService.userID)
	}
}
