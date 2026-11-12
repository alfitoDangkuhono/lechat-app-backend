package controllers

import (
	"errors"
	"net/http"

	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/helpers"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/request"
	"lechat-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthenticationController struct {
	iauthentication interfaces.IAuthentications
	ijwt            interfaces.IJwtInterface
}

func NewAuthenticationController(iauthentication interfaces.IAuthentications, ijwt interfaces.IJwtInterface) *AuthenticationController {
	return &AuthenticationController{
		iauthentication: iauthentication,
		ijwt:            ijwt,
	}
}

func (c *AuthenticationController) Register(ctx *gin.Context) {
	var request request.AddUserRequest

	if err := ctx.ShouldBindBodyWithJSON(&request); err != nil {
		helpers.CustomValidationInput(ctx, err)
		return
	}

	createdUser, err := c.iauthentication.Registration(request)
	if err != nil {
		utils.ErrorResponse(ctx, 500, constant.ErrInternalServer, nil)
		return
	}

	token, err := c.ijwt.GenerateToken(uint(createdUser.Id), createdUser.Username, createdUser.Email, false)

	if err != nil {
		utils.ErrorResponse(ctx, 500, constant.ErrFailedCreateToken, nil)
		return
	}
	refreshToken, err := c.ijwt.GenerateToken(uint(createdUser.Id), createdUser.Username, createdUser.Email, true)
	if err != nil {
		utils.ErrorResponse(ctx, 500, constant.ErrFailedCreateToken, nil)
		return
	}

	utils.SuccessResponse(ctx, 201, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
	})
}

func (c *AuthenticationController) Login(ctx *gin.Context) {

	var req request.LoginRequest
	message := constant.ErrNotFoundEmail
	code := 404

	if err := ctx.ShouldBindJSON(&req); err != nil {
		helpers.CustomValidationInput(ctx, err)
		return
	}

	login, err := c.iauthentication.CredentialLogin(req.Email, req.Password)
	if err != nil {
		if err.Error() == "password_inccorrect" {
			message = constant.ErrIncorectPassword
			code = 500
		}
		utils.ErrorResponse(ctx, code, message, nil)
		return
	}

	token, err := c.ijwt.GenerateToken(login.Id, login.Username, login.Email, false)

	if err != nil {
		utils.ErrorResponse(ctx, 500, constant.ErrFailedCreateToken, nil)
		return
	}
	refreshToken, err := c.ijwt.GenerateToken(login.Id, login.Username, login.Email, true)
	if err != nil {
		utils.ErrorResponse(ctx, 500, constant.ErrFailedCreateToken, nil)
		return
	}

	utils.SuccessResponse(ctx, 200, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
	})
}

func (c *AuthenticationController) Refresh(ctx *gin.Context) {
	claimsValue, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	claims, ok := claimsValue.(*models.JwtClaims)
	if !exists || !ok || claims == nil || claims.ExpiresAt == nil {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidRefreshToken, nil)
		return
	}

	accessToken, err := c.ijwt.GenerateToken(claims.UserId, claims.Username, claims.Email, false)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrFailedCreateToken, nil)
		return
	}
	refreshToken, err := c.ijwt.GenerateToken(claims.UserId, claims.Username, claims.Email, true)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrFailedCreateToken, nil)
		return
	}
	if err := c.ijwt.RevokeToken(claims.ID, claims.ExpiresAt.Time); err != nil {
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrInternalServer, nil)
		return
	}

	utils.SuccessResponse(ctx, http.StatusOK, gin.H{
		"token":         accessToken,
		"refresh_token": refreshToken,
	})
}

func (c *AuthenticationController) Logout(ctx *gin.Context) {
	claimsValue, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	claims, ok := claimsValue.(*models.JwtClaims)
	if !exists || !ok || claims == nil || claims.ExpiresAt == nil {
		utils.ErrorResponse(ctx, 401, constant.ErrInvalidToken, nil)
		return
	}

	if err := c.ijwt.RevokeToken(claims.ID, claims.ExpiresAt.Time); err != nil {
		utils.ErrorResponse(ctx, 500, constant.ErrInternalServer, nil)
		return
	}

	utils.SuccessResponse(ctx, 200, gin.H{
		"logged_out": true,
	})
}

func (c *AuthenticationController) Profile(ctx *gin.Context) {
	claimsValue, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	claims, ok := claimsValue.(*models.JwtClaims)
	if !exists || !ok || claims == nil {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		return
	}

	profile, err := c.iauthentication.GetProfile(claims.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorResponse(ctx, http.StatusNotFound, constant.ErrNotFoundData, nil)
			return
		}
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrInternalServer, nil)
		return
	}

	utils.SuccessResponse(ctx, http.StatusOK, profile)
}

func (c *AuthenticationController) CheckSession(ctx *gin.Context) {
	claimsValue, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	claims, ok := claimsValue.(*models.JwtClaims)
	if !exists || !ok || claims == nil {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		return
	}

	utils.SuccessResponse(ctx, http.StatusOK, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":       claims.UserId,
			"username": claims.Username,
			"email":    claims.Email,
		},
		"expires_at": claims.ExpiresAt,
	})
}

func (c *AuthenticationController) UpdateProfile(ctx *gin.Context) {
	claimsValue, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	claims, ok := claimsValue.(*models.JwtClaims)
	if !exists || !ok || claims == nil {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		return
	}

	var req request.ProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helpers.CustomValidationInput(ctx, err)
		return
	}

	profile, err := c.iauthentication.UpdateProfile(claims.UserId, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorResponse(ctx, http.StatusNotFound, constant.ErrNotFoundData, nil)
			return
		}
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrInternalServer, nil)
		return
	}

	utils.SuccessResponse(ctx, http.StatusOK, profile)
}
