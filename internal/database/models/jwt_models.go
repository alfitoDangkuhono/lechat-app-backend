package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JwtClaims struct {
	UserId    uint   `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	TokenType string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type RevokedToken struct {
	TokenID   string    `gorm:"primaryKey;column:token_id"`
	ExpiresAt time.Time `gorm:"column:expires_at;index"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RevokedToken) TableName() string {
	return "token_revocations"
}
