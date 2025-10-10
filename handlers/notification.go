package handlers

import (
	"net/http"
	"strconv"

	"piscord-backend/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationHandler struct {
	NotificationService *services.NotificationService
	AuthService         *services.AuthService
}

func NewNotificationHandler(notificationService *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		NotificationService: notificationService,
	}
}

func (h *NotificationHandler) GetUnreadNotificationCount(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	count, err := h.NotificationService.GetUnreadNotificationCount(userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread notification count"})
		return
	}

	c.JSON(http.StatusOK, count)
}

func (h *NotificationHandler) GetMyNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	notifications, err := h.NotificationService.GetMyNotifications(page, limit, userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  notifications,
		"total": len(notifications),
	})
}

func (h *NotificationHandler) MarkNotificationAsRead(c *gin.Context) {
	notificationID, _ := c.Params.Get("id")
	notificationObjectID, err := primitive.ObjectIDFromHex(notificationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.NotificationService.MarkNotificationAsRead(notificationObjectID, userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

func (h *NotificationHandler) MarkAllNotificationsAsRead(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.NotificationService.MarkAllNotificationsAsRead(userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all notifications as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	notificationID, _ := c.Params.Get("id")
	notificationObjectID, err := primitive.ObjectIDFromHex(notificationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.NotificationService.DeleteNotification(notificationObjectID, userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

func (h *NotificationHandler) DeleteAllNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.NotificationService.DeleteAllNotifications(userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}
