package middleware

import (
	"errors"
	"net/http"
	"strings"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/helpers"
	"lechat-backend/internal/interfaces"
	"lechat-backend/utils"

	"github.com/gin-gonic/gin"
)

const AuthenticatedClaimsKey = "authenticatedClaims"

func Authentication(jwt interfaces.IJwtInterface) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		claims, _, ok := validateBearerToken(ctx, jwt)
		if !ok {
			return
		}
		if claims.TokenType != "" && claims.TokenType != models.TokenTypeAccess {
			utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
			ctx.Abort()
			return
		}

		ctx.Set(AuthenticatedClaimsKey, claims)
		ctx.Next()
	}
}

func RefreshAuthentication(jwt interfaces.IJwtInterface) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		claims, _, ok := validateBearerToken(ctx, jwt)
		if !ok {
			return
		}
		if claims.TokenType != models.TokenTypeRefresh {
			utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidRefreshToken, nil)
			ctx.Abort()
			return
		}

		ctx.Set(AuthenticatedClaimsKey, claims)
		ctx.Next()
	}
}

func validateBearerToken(ctx *gin.Context, jwt interfaces.IJwtInterface) (*models.JwtClaims, string, bool) {
	parts := strings.Fields(ctx.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		ctx.Abort()
		return nil, "", false
	}

	claims, err := jwt.ValidateToken(parts[1])
	if err != nil {
		if errors.Is(err, constant.ErrTokenRevocationDB) {
			utils.ErrorResponse(ctx, http.StatusInternalServerError, constant.ErrInternalServer, nil)
		} else {
			utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		}
		ctx.Abort()
		return nil, "", false
	}
	if claims == nil {
		utils.ErrorResponse(ctx, http.StatusUnauthorized, constant.ErrInvalidToken, nil)
		ctx.Abort()
		return nil, "", false
	}
	if claims.ID == "" {
		claims.ID = helpers.TokenRevocationID(parts[1])
	}
	return claims, parts[1], true
}
