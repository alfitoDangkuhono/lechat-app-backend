package interfaces

import (
	"context"
	"lechat-backend/internal/database/dto"
)

type IAvatarService interface {
	CreateUploadURL(ctx context.Context, userID uint, contentType string) (dto.AvatarUploadURLDto, error)
	CreateDownloadURL(ctx context.Context, userID uint) (dto.AvatarDownloadURLDto, error)
}
