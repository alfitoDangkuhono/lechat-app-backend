package interfaces

import (
	"lechat-backend/internal/database/models"
	"time"
)

type IJwtInterface interface {
	GenerateToken(idUser uint, userName string, email string, isRefreshToken bool) (string, error)
	ValidateToken(token string) (*models.JwtClaims, error)
	RevokeToken(tokenID string, expiresAt time.Time) error
}
