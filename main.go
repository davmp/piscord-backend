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

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	mongoService := services.NewMongoService(cfg.MongoURI)
	authService := services.NewAuthService(mongoService)
	chatService := services.NewChatService(mongoService)

	if err := mongoService.Connect(); err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}
	defer func() {
		if err := mongoService.Disconnect(); err != nil {
			log.Println("Error disconnecting from MongoDB: ", err)
		}
	}()

	router := gin.Default()

	authHandler := handlers.NewAuthHandler(authService, cfg)
	chatHandler := handlers.NewChatHandler(chatService, cfg)
	roomHandler := handlers.NewRoomHandler(mongoService, authService)

	auth := router.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.GET("/profile", authHandler.Profile)

		api.GET("/rooms", roomHandler.GetRooms)
		api.POST("/rooms", roomHandler.CreateRoom)
		api.GET("/rooms/:id", roomHandler.GetRoom)
		api.GET("/direct/:username", roomHandler.GetDirectRoom)
		api.POST("/rooms/:id/join", roomHandler.JoinRoom)
		api.POST("/rooms/:id/leave", roomHandler.LeaveRoom)
		api.GET("/rooms/:id/messages", roomHandler.GetMessages)
		api.GET("/direct/:username/messages", roomHandler.GetDirectMessages)
	}

	api.GET("/api/ws", chatHandler.HandleWebsocket)

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
