package repository

import (
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
		Select("id", "username", "email", "first_name", "last_name", "phone", "created_at", "updated_at", "about", "avatar_url").
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
		Select("id", "username", "email", "first_name", "last_name", "phone", "created_at", "updated_at", "about", "avatar_url").
		Where("id = ?", UserId).
		Take(&profile).Error

	return profile, err
}
