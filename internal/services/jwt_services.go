package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"lechat-backend/config"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/helpers"
	IJWtInterface "lechat-backend/internal/interfaces"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwtService struct {
	secret          string
	issuer          string
	revocationStore IJWtInterface.ITokenRevocationStore
}

func NewJwtService(revocationStore IJWtInterface.ITokenRevocationStore) IJWtInterface.IJwtInterface {
	return &jwtService{
		secret:          config.LoadConfig().SECURITY_CONFIG.JwtSecret,
		issuer:          config.LoadConfig().SECURITY_CONFIG.IssuerSecretJwt,
		revocationStore: revocationStore,
	}
}

func (s *jwtService) GenerateToken(idUser uint, userName string, email string, isRefreshToken bool) (string, error) {

	var timeExpire time.Duration

	switch isRefreshToken {
	case true:
		timeExpire = time.Hour * config.LoadConfig().SECURITY_CONFIG.ExpireRefreshToken
	default:
		timeExpire = time.Second * config.LoadConfig().SECURITY_CONFIG.ExpireToken
	}

	tokenIDBytes := make([]byte, 16)
	if _, err := rand.Read(tokenIDBytes); err != nil {
		return "", fmt.Errorf("generate token ID: %w", err)
	}

	claims := &models.JwtClaims{
		UserId:    idUser,
		Username:  userName,
		Email:     email,
		TokenType: models.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(tokenIDBytes),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(timeExpire)),
		},
	}
	if isRefreshToken {
		claims.TokenType = models.TokenTypeRefresh
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *jwtService) ValidateToken(tokenString string) (*models.JwtClaims, error) {

	token, err := jwt.ParseWithClaims(tokenString, &models.JwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*models.JwtClaims)

	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	if claims.ExpiresAt == nil {
		return nil, errors.New("token is missing expiration")
	}
	tokenID := claims.ID
	if tokenID == "" {
		tokenID = helpers.TokenRevocationID(tokenString)
	}
	revoked, err := s.revocationStore.IsRevoked(tokenID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", constant.ErrTokenRevocationDB, err)
	}
	if revoked {
		return nil, constant.ErrTokenRevoked
	}
	return claims, nil
}

func (s *jwtService) RevokeToken(tokenID string, expiresAt time.Time) error {
	if tokenID == "" || expiresAt.IsZero() {
		return errors.New("token ID and expiration are required for revocation")
	}
	return s.revocationStore.Revoke(tokenID, expiresAt)
}
