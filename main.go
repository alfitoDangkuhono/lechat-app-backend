package main

import (
	"lechat-backend/config"
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/routes"
	"lechat-backend/internal/database"
	"lechat-backend/internal/helpers"
	"lechat-backend/internal/repository"
	"lechat-backend/internal/services"
	"lechat-backend/utils"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		helpers.IsPanic("env not found", err)
	}

	appLogger := utils.NewLogger()
	router := gin.Default()
	config := config.LoadConfig()
	router.SetTrustedProxies([]string{"100.91.195.119"})
	router.Use(cors.New(cors.Config{
		AllowOrigins:     config.CORS_CONFIG.AllowOrigins,
		AllowMethods:     config.CORS_CONFIG.AllowMethods,
		AllowHeaders:     config.CORS_CONFIG.AllowHeaders,
		ExposeHeaders:    config.CORS_CONFIG.ExposeHeaders,
		AllowCredentials: config.CORS_CONFIG.AllowCredentials,
		MaxAge:           config.CORS_CONFIG.MaxAge,
	}))

	database.Connect(config, appLogger.Writer())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})

	// repository
	authenticationRepository := repository.NewAuthenticationRepository(database.DB)
	chatRepository := repository.NewChatRepository(database.DB)
	contactRepository := repository.NewContactRepository(database.DB)
	tokenRevocationRepository := repository.NewTokenRevocationRepository(database.DB)
	// services
	jwtService := services.NewJwtService(tokenRevocationRepository)
	authenticationService := services.NewAuthenticationService(authenticationRepository)
	chatService := services.NewChatService(chatRepository)
	contactService := services.NewContactService(contactRepository)
	// Controllers
	authenticationController := controllers.NewAuthenticationController(authenticationService, jwtService)
	chatController := controllers.NewChatController(chatService)
	contactController := controllers.NewContactController(contactService)
	realtimeController := controllers.NewRealtimeController(
		chatService,
		jwtService,
		services.NewRealtimeHub(),
		config.CORS_CONFIG.AllowOrigins,
	)
	// Routes
	routes.Routes(router, authenticationController, jwtService, chatController)
	routes.ContactRoutes(router, jwtService, contactController)
	routes.RealtimeRoutes(router, realtimeController)

	router.Run(":" + config.APP_CONFIG.APP_PORT)
}
