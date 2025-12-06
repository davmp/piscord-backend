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
	redisService := services.NewRedisService(cfg.RedisURI)
	authService := services.NewAuthService(mongoService, redisService)
	roomService := services.NewRoomService(mongoService, redisService)
	notificationService := services.NewNotificationService(mongoService, redisService)
	chatService := services.NewChatService(roomService, mongoService, notificationService, authService, redisService)

	if err := mongoService.Connect(); err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}
	defer func() {
		if err := mongoService.Disconnect(); err != nil {
			log.Println("Error disconnecting from MongoDB: ", err)
		}
	}()

	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	router.RedirectTrailingSlash = false

	authHandler := handlers.NewAuthHandler(authService, chatService, roomService, redisService, cfg)
	chatHandler := handlers.NewChatHandler(chatService, cfg)
	roomHandler := handlers.NewRoomHandler(authService, chatService, roomService, mongoService, redisService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)

	profile := router.Group("/api/profile")
	profile.Use(middleware.AuthMiddleware())
	{
		profile.GET("", authHandler.GetProfile)
		profile.PUT("", authHandler.UpdateProfile)
		profile.GET("/:id", authHandler.GetProfileByID)
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
		rooms.PUT("/:id", roomHandler.UpdateRoom)
		rooms.GET("/direct/:id", roomHandler.GetDirectRoom)
		rooms.POST("/:id/join", roomHandler.JoinRoom)
		rooms.POST("/:id/leave", roomHandler.LeaveRoom)
		rooms.POST("/:id/kick/:memberId", roomHandler.KickMember)
		rooms.GET("/:id/messages", roomHandler.GetMessages)
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
