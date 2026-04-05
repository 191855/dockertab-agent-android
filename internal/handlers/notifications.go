package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type registerTokenRequest struct {
	DeviceToken string `json:"device_token" binding:"required"`
}

func (h *Handler) RegisterDeviceToken(c *gin.Context) {
	var req registerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_token is required"})
		return
	}

	deviceID := c.GetString("device_id")

	if h.RelayRegisterFCMToken != nil {
		h.RelayRegisterFCMToken(deviceID, req.DeviceToken)
	}

	c.JSON(http.StatusOK, gin.H{"registered": true})
}

func (h *Handler) UnregisterDeviceToken(c *gin.Context) {
	var req registerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_token is required"})
		return
	}

	deviceID := c.GetString("device_id")

	if h.RelayUnregisterFCMToken != nil {
		h.RelayUnregisterFCMToken(deviceID, req.DeviceToken)
	}

	c.JSON(http.StatusOK, gin.H{"unregistered": true})
}
