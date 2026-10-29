package repository

import (
	"lechat-backend/internal/database/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenRevocationRepository struct {
	db *gorm.DB
}

func NewTokenRevocationRepository(db *gorm.DB) *TokenRevocationRepository {
	return &TokenRevocationRepository{db: db}
}

func (r *TokenRevocationRepository) Revoke(tokenID string, expiresAt time.Time) error {
	if err := r.db.Where("expires_at <= ?", time.Now()).Delete(&models.RevokedToken{}).Error; err != nil {
		return err
	}

	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.RevokedToken{
		TokenID:   tokenID,
		ExpiresAt: expiresAt,
	}).Error
}

func (r *TokenRevocationRepository) IsRevoked(tokenID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.RevokedToken{}).
		Where("token_id = ? AND expires_at > ?", tokenID, time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
