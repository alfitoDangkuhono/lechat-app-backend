package interfaces

import (
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/request"
)

type IAuthentications interface {
	Registration(req request.AddUserRequest) (models.Users, error)
	CredentialLogin(email string, password string) (dto.LoginDto, error)
	GetProfile(userID uint) (dto.ProfileDto, error)
}
