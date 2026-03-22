package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/familysync/backend/db"
	"github.com/familysync/backend/fcm"
	redisstore "github.com/familysync/backend/redis"
	"github.com/gin-gonic/gin"
)

// CommandPayload is the JSON body for sending a command to a device.
type CommandPayload struct {
	Action string                 `json:"action" binding:"required"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// SendCommandHandler pushes a real-time command to a connected device.
// The parent dashboard calls this when the user clicks a control button.
// POST /api/v1/command/:device_id
// Requires: JWT Bearer token (parent authenticated)
//
// Supported actions:
//
//	GET_GPS       → device replies immediately with {type:"gps", lat, lng, timestamp}
//	START_CAMERA  → device begins streaming JPEG frames over WS
//	STOP_CAMERA   → device stops camera stream
//	START_SCREEN  → device begins streaming screen frames over WS
//	STOP_SCREEN   → device stops screen stream
//	START_MIC     → device begins streaming audio chunks over WS
//	STOP_MIC      → device stops microphone
func SendCommandHandler(c *gin.Context) {
	deviceID := c.Param("device_id")
	userID := c.GetInt64("user_id")

	var req CommandPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify the device belongs to the requesting parent.
	var ownerID int64
	err := db.DB.QueryRow(
		`SELECT user_id FROM devices WHERE id = $1`, deviceID,
	).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "device not found or access denied"})
		return
	}

	// Check if device is online.
	if !redisstore.IsOnline(c.Request.Context(), deviceID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "device is offline"})
		return
	}

	// Build and send the command JSON.
	payload, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode command"})
		return
	}

	if !GlobalHub.SendToDevice(deviceID, payload) {
		c.JSON(http.StatusGone, gin.H{"error": "device WebSocket not available"})
		return
	}

	// Terlepas dari dia online atau mati, setiap perintah baru yang turun dari Web, otomatis akan ditancapkan misil FCM agar si aplikasi HP tidak tertidur.
	var fcmToken string
	// Check bila FCM token eksis di baris database device
	db.DB.QueryRow(`SELECT fcm_token FROM devices WHERE id = $1`, deviceID).Scan(&fcmToken)
	if fcmToken != "" {
		// Ledakkan pelatuk Push Notification Wake-up di latar belakang dengan mode Async
		go fcm.SendWakeUpSignal(fcmToken, req.Action)
	}

	c.JSON(http.StatusOK, gin.H{"status": "command queued/sent"})
}

// DeviceStatusHandler returns the online/offline status of all parent's devices.
// GET /api/v1/device/status
func DeviceStatusHandler(c *gin.Context) {
	userID := c.GetInt64("user_id")

	rows, err := db.DB.Query(
		`SELECT id, device_name FROM devices WHERE user_id = $1`, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer rows.Close()

	type Status struct {
		ID         int64  `json:"id"`
		DeviceName string `json:"device_name"`
		Online     bool   `json:"online"`
	}

	statuses := []Status{}
	for rows.Next() {
		var s Status
		if err := rows.Scan(&s.ID, &s.DeviceName); err == nil {
			s.Online = redisstore.IsOnline(c.Request.Context(), fmt.Sprintf("%d", s.ID))
			statuses = append(statuses, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"devices": statuses})
}
