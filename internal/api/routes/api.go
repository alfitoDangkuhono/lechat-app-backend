package routes

import (
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/interfaces"

	"github.com/gin-gonic/gin"
)

func Routes(router *gin.Engine, authenticationController *controllers.AuthenticationController, jwt interfaces.IJwtInterface) {
	api := router.Group("api/v1")

	api.POST("/registers", authenticationController.Register)
	api.POST("/sessions", authenticationController.Login)
	api.POST("/sessions/refresh", middleware.RefreshAuthentication(jwt), authenticationController.Refresh)

	authenticated := api.Group("")
	authenticated.Use(middleware.Authentication(jwt))
	authenticated.DELETE("/sessions", authenticationController.Logout)
	authenticated.GET("/sessions/check-session", authenticationController.CheckSession)
	authenticated.GET("/profile", authenticationController.Profile)
	// Add other protected endpoints to this group to require a valid Bearer token.
}
