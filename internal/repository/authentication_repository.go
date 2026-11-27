package repository

import (
	"errors"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/request"

	"gorm.io/gorm"
)

type AuthenticationRepository struct {
	db *gorm.DB
}

func NewAuthenticationRepository(db *gorm.DB) *AuthenticationRepository {
	return &AuthenticationRepository{db: db}
}

func (repo *AuthenticationRepository) CreateUser(user models.Users) (models.Users, error) {
	err := repo.db.Create(&user).Error

	if err != nil {
		return user, err
	}
	return user, err
}

func (r *AuthenticationRepository) FindEmailLogin(email string) (dto.LoginDto, error) {
	var login dto.LoginDto

	err := r.db.Table("users").
		Select("id", "username", "email", "password").
		Where("email = ?", email).
		Limit(1).
		Scan(&login).Error

	return login, err
}

func (r *AuthenticationRepository) FindProfile(userID uint) (dto.ProfileDto, error) {
	var profile dto.ProfileDto
	err := r.db.Model(&models.Users{}).
		Select("id", "username", "email", "first_name", "last_name", "phone", "created_at", "updated_at", "about", "avatar_url", "public_key").
		Where("id = ?", userID).
		Take(&profile).Error
	return profile, err
}

func (r *AuthenticationRepository) UpdateAvatarURL(userID uint, avatarURL string) error {
	result := r.db.Model(&models.Users{}).
		Where("id = ?", userID).
		Update("avatar_url", avatarURL)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *AuthenticationRepository) UpdateProfile(UserId uint, req request.ProfileRequest) (dto.ProfileDto, error) {
	var profile dto.ProfileDto
	err := r.db.Model(&models.Users{}).
		Where("id = ?", UserId).
		Updates(models.Users{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Phone:     req.Phone,
			About:     req.About,
		}).Error

	if err != nil {
		return profile, err
	}

	err = r.db.Model(&models.Users{}).
		Select("id", "username", "email", "first_name", "last_name", "phone", "created_at", "updated_at", "about", "avatar_url", "public_key").
		Where("id = ?", UserId).
		Take(&profile).Error

	return profile, err
}

func (r *AuthenticationRepository) UpdatePublicKeys(userId uint, req request.UpdatePublicKeysRequest) (dto.PublicKeyDto, error) {
	var current dto.PublicKeyDto
	err := r.db.Model(&models.Users{}).
		Select("public_key", "COALESCE(public_key_updated_at, updated_at) AS updated_at").
		Where("id = ?", userId).
		Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.PublicKeyDto{}, constant.ErrNotRecordUpdate
	}
	if err != nil {
		return dto.PublicKeyDto{}, err
	}

	// No-op when the key is unchanged: key.updated_at must only move when the
	// public key actually changes, never because of a re-upload.
	if current.PublicKey == req.PublicKey {
		return current, nil
	}

	result := r.db.Model(&models.Users{}).
		Where("id = ?", userId).
		Updates(map[string]interface{}{
			"public_key":            req.PublicKey,
			"public_key_updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return dto.PublicKeyDto{}, result.Error
	}
	if result.RowsAffected == 0 {
		return dto.PublicKeyDto{}, constant.ErrNotRecordUpdate
	}

	return r.findPublicKey(userId)
}

func (r *AuthenticationRepository) findPublicKey(userId uint) (dto.PublicKeyDto, error) {
	var publicKeyDto dto.PublicKeyDto
	err := r.db.Model(&models.Users{}).
		Select("public_key", "COALESCE(public_key_updated_at, updated_at) AS updated_at").
		Where("id = ? AND public_key IS NOT NULL", userId).
		Take(&publicKeyDto).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.PublicKeyDto{}, gorm.ErrRecordNotFound
	}
	return publicKeyDto, err
}

func (r *AuthenticationRepository) FindPublicKeyByUserID(userID uint) (dto.PublicKeyDto, error) {
	return r.findPublicKey(userID)
}
