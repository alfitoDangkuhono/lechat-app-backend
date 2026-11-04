package routes

import (
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/interfaces"

	"github.com/gin-gonic/gin"
)

func ContactRoutes(router *gin.Engine, jwt interfaces.IJwtInterface, contactController *controllers.ContactController) {
	api := router.Group("api/v1")
	authenticated := api.Group("")
	authenticated.Use(middleware.Authentication(jwt))
	authenticated.GET("/contacts", contactController.ListContacts)
	authenticated.GET("/contacts/requests", contactController.ListRequests)
	authenticated.POST("/contacts/:user_id/requests", contactController.RequestContact)
	authenticated.POST("/contacts/:user_id/accept", contactController.AcceptContact)
	authenticated.DELETE("/contacts/:user_id/requests", contactController.RejectContact)
	authenticated.DELETE("/contacts/:user_id", contactController.RemoveContact)
}
