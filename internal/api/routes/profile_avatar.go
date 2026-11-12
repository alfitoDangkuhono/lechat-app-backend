package routes

import (
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/interfaces"

	"github.com/gin-gonic/gin"
)

func ProfileAvatarRoutes(router *gin.Engine, jwt interfaces.IJwtInterface, avatarController *controllers.AvatarController) {
	api := router.Group("api/v1")
	authenticated := api.Group("")
	authenticated.Use(middleware.Authentication(jwt))
	authenticated.POST("/profile/avatar/upload-url", avatarController.PresignUpload)
	authenticated.GET("/profile/avatar/download-url", avatarController.PresignDownload)
}
