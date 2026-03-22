package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/familysync/backend/db"
	"github.com/gin-gonic/gin"
)

// PairRequest is the expected JSON body for device pairing.
type PairRequest struct {
	DeviceName string `json:"device_name" binding:"required"`
}

// PairDeviceHandler links a new child device to the authenticated parent account.
// POST /api/v1/device/pair
// Requires: JWT Bearer token
func PairDeviceHandler(c *gin.Context) {
	var req PairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetInt64("user_id")

	// Generate a unique 32-byte random token for the device.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate device token"})
		return
	}
	deviceToken := hex.EncodeToString(tokenBytes)

	var deviceID int64
	err := db.DB.QueryRow(
		`INSERT INTO devices (user_id, device_name, device_token) VALUES ($1, $2, $3) RETURNING id`,
		userID, req.DeviceName, deviceToken,
	).Scan(&deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to pair device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"device_id": deviceID, "device_token": deviceToken})
}

// Struct untuk FCM Sync
type SyncFCMRequest struct {
	DeviceToken string `json:"device_token" binding:"required"`
	FCMToken    string `json:"fcm_token" binding:"required"`
}

// SyncFCMHandler menerima rahasia Token perangkat Firebase FCM untuk diikat ke baris Database perangkat Android
func SyncFCMHandler(c *gin.Context) {
	var req SyncFCMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.DB.Exec(`UPDATE devices SET fcm_token = $1 WHERE device_token = $2`, req.FCMToken, req.DeviceToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link fcm token to target device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "fcm_linked_securely"})
}

// ListDevicesHandler returns all devices paired to the authenticated parent.
// GET /api/v1/device/list
func ListDevicesHandler(c *gin.Context) {
	userID := c.GetInt64("user_id")

	rows, err := db.DB.Query(
		`SELECT id, device_name, device_token, paired_at FROM devices WHERE user_id = $1 ORDER BY paired_at DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list devices"})
		return
	}
	defer rows.Close()

	type Device struct {
		ID          int64  `json:"id"`
		DeviceName  string `json:"device_name"`
		DeviceToken string `json:"device_token"`
		PairedAt    string `json:"paired_at"`
	}

	devices := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.DeviceName, &d.DeviceToken, &d.PairedAt); err == nil {
			devices = append(devices, d)
		}
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}
