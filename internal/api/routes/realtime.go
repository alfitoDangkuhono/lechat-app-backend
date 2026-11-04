package routes

import (
	"lechat-backend/internal/api/controllers"

	"github.com/gin-gonic/gin"
)

func RealtimeRoutes(router *gin.Engine, controller *controllers.RealtimeController) {
	router.GET("/api/v1/ws", controller.Serve)
}
