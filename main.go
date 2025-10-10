package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"piscord-backend/config"
	"piscord-backend/handlers"
	"piscord-backend/middleware"
	"piscord-backend/services"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	mongoService := services.NewMongoService(cfg.MongoURI)
	authService := services.NewAuthService(mongoService)
	notificationService := services.NewNotificationService(mongoService)
	chatService := services.NewChatService(mongoService, notificationService)

	if err := mongoService.Connect(); err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}
	defer func() {
		if err := mongoService.Disconnect(); err != nil {
			log.Println("Error disconnecting from MongoDB: ", err)
		}
	}()

	router := gin.Default()
	router.RedirectTrailingSlash = false

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:4200"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Accept", "Origin", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authHandler := handlers.NewAuthHandler(authService, cfg)
	chatHandler := handlers.NewChatHandler(chatService, cfg)
	roomHandler := handlers.NewRoomHandler(mongoService, authService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)

	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.GET("/profile", authHandler.Profile)
	}

	router.GET("/api/ws", chatHandler.HandleWebsocket)

	auth := router.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	rooms := router.Group("/api/rooms")
	rooms.Use(middleware.AuthMiddleware())
	{
		rooms.GET("", roomHandler.GetRooms)
		rooms.GET("/my-rooms", roomHandler.GetMyRooms)
		rooms.POST("", roomHandler.CreateRoom)
		rooms.GET("/:id", roomHandler.GetRoom)
		rooms.GET("/direct/:username", roomHandler.GetDirectRoom)
		rooms.POST("/:id/join", roomHandler.JoinRoom)
		rooms.POST("/:id/leave", roomHandler.LeaveRoom)
		rooms.GET("/:id/messages", roomHandler.GetMessages)
		rooms.GET("/direct/:username/messages", roomHandler.GetDirectMessages)
	}

	notification := router.Group("/api/notifications")
	notification.Use(middleware.AuthMiddleware())
	{
		notification.GET("", notificationHandler.GetMyNotifications)
		notification.GET("/unread-count", notificationHandler.GetUnreadNotificationCount)
		notification.PUT("/:id/read", notificationHandler.MarkNotificationAsRead)
		notification.PUT("/read-all", notificationHandler.MarkAllNotificationsAsRead)
		notification.DELETE("/:id", notificationHandler.DeleteNotification)
		notification.DELETE("/delete-all", notificationHandler.DeleteAllNotifications)
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:    "0.0.0.0:" + cfg.Port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal("Error starting server: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
