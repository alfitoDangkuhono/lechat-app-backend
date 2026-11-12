package main

import (
	"context"
	"lechat-backend/config"
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/routes"
	"lechat-backend/internal/database"
	"lechat-backend/internal/helpers"
	"lechat-backend/internal/observability"
	"lechat-backend/internal/repository"
	"lechat-backend/internal/services"
	"lechat-backend/internal/storage"
	"lechat-backend/utils"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	config := config.LoadConfig()
	s3Storage, err := storage.NewS3StorageFromConfig(config.S3_CONFIG)
	if err != nil {
		helpers.IsPanic("failed to initialize S3 storage", err)
	}
	shutdownTelemetry, err := observability.Init(context.Background())
	if err != nil {
		helpers.IsPanic("failed to initialize OpenTelemetry", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(ctx); err != nil {
			appLogger.Error(err)
		}
	}()

	requestMetrics, err := observability.GinMetrics()
	if err != nil {
		helpers.IsPanic("failed to initialize HTTP metrics", err)
	}

	router := gin.New()
	router.Use(gin.Logger(), observability.GinTracing(), requestMetrics, gin.Recovery())
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
	avatarService := services.NewAvatarService(authenticationRepository, s3Storage)
	chatService := services.NewChatService(chatRepository)
	contactService := services.NewContactService(contactRepository)
	realtimeHub := services.NewRealtimeHub()
	// Controllers
	authenticationController := controllers.NewAuthenticationController(authenticationService, jwtService, avatarService)
	avatarController := controllers.NewAvatarController(avatarService)
	chatController := controllers.NewChatController(chatService, realtimeHub)
	contactController := controllers.NewContactController(contactService)
	realtimeController := controllers.NewRealtimeController(
		chatService,
		jwtService,
		realtimeHub,
		config.CORS_CONFIG.AllowOrigins,
	)
	// Routes
	routes.Routes(router, authenticationController, jwtService, chatController)
	routes.ProfileAvatarRoutes(router, jwtService, avatarController)
	routes.ContactRoutes(router, jwtService, contactController)
	routes.RealtimeRoutes(router, realtimeController)

	server := &http.Server{
		Addr:    ":" + config.APP_CONFIG.APP_PORT,
		Handler: router,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	stop, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case <-stop.Done():
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			appLogger.Error(err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error(err)
	}
}
