package services

import (
	"errors"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/repository"
	"lechat-backend/internal/request"
	"lechat-backend/utils"
)

type AuthenticationService struct {
	authenticationRepository *repository.AuthenticationRepository
}

func NewAuthenticationService(AuthenticatuonRepository *repository.AuthenticationRepository) interfaces.IAuthentications {
	return &AuthenticationService{
		authenticationRepository: AuthenticatuonRepository,
	}
}

func (s *AuthenticationService) Registration(req request.AddUserRequest) (models.Users, error) {
	newUser := models.Users{
		Username: req.Username,
		Email:    req.Email,
		Password: utils.HashPassword(req.Password),
		Phone:    req.Phone,
	}
	return s.authenticationRepository.CreateUser(newUser)
}

func (s *AuthenticationService) CredentialLogin(email string, password string) (dto.LoginDto, error) {

	login, err := s.authenticationRepository.FindEmailLogin(email)

	if err != nil {
		return dto.LoginDto{}, errors.New("email_incorrect")
	}
	if !utils.ComparePassword(login.Password, password) {
		return dto.LoginDto{}, errors.New("password_inccorrect")
	}

	return login, nil
}

func (s *AuthenticationService) GetProfile(userID uint) (dto.ProfileDto, error) {
	return s.authenticationRepository.FindProfile(userID)
}

func (s *AuthenticationService) UpdateProfile(UserId uint, req request.ProfileRequest) (dto.ProfileDto, error) {

	profile, err := s.authenticationRepository.FindProfile(UserId)
	// check if the user exists
	if err != nil {
		return dto.ProfileDto{}, errors.New("user_not_found")
	}
	// checking what is changed
	if profile.FirstName == req.FirstName && profile.LastName == req.LastName && profile.Phone == req.Phone && profile.About == req.About {
		return dto.ProfileDto{}, errors.New("no_changes_detected")
	}
	return s.authenticationRepository.UpdateProfile(UserId, req)
}
