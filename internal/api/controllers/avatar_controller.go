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
	"lechat-backend/internal/services"
	"lechat-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AvatarController struct {
	avatarService interfaces.IAvatarService
}

func NewAvatarController(avatarService interfaces.IAvatarService) *AvatarController {
	return &AvatarController{avatarService: avatarService}
}

func (c *AvatarController) PresignUpload(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		return
	}

	var req request.AvatarUploadURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helpers.CustomValidationInput(ctx, err)
		return
	}

	result, err := c.avatarService.CreateUploadURL(ctx.Request.Context(), userID, req.ContentType)
	if err != nil {
		if errors.Is(err, services.ErrUnsupportedAvatarContentType) {
			utils.ErrorResponse(ctx, http.StatusBadRequest, services.ErrUnsupportedAvatarContentType, nil)
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorResponse(ctx, http.StatusNotFound, constant.ErrNotFoundData, nil)
			return
		}
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrInternalServer, nil)
		return
	}

	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func (c *AvatarController) PresignDownload(ctx *gin.Context) {
	userID, ok := authenticatedUserID(ctx)
	if !ok {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		return
	}

	result, err := c.avatarService.CreateDownloadURL(ctx.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, services.ErrAvatarNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorResponse(ctx, http.StatusNotFound, constant.ErrNotFoundData, nil)
			return
		}
		utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrInternalServer, nil)
		return
	}

	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func authenticatedUserID(ctx *gin.Context) (uint, bool) {
	claimsValue, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	claims, ok := claimsValue.(*models.JwtClaims)
	if !exists || !ok || claims == nil || claims.UserId == 0 {
		return 0, false
	}
	return claims.UserId, true
}
