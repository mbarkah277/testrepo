package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/familysync/backend/db"
	"github.com/gin-gonic/gin"
)

// SyncAudioRequest is the JSON payload from the Android client.
type SyncAudioRequest struct {
	DeviceToken string `json:"device_token" binding:"required"`
	AudioB64    string `json:"audio_b64"    binding:"required"` // Base64-encoded AAC bytes
	DurationSec int    `json:"duration_s"`
	RecordedAt  string `json:"recorded_at"` // ISO-8601
}

// SyncAudioHandler receives an audio chunk from the Android client,
// decodes it, saves to disk, and logs metadata in the database.
// POST /api/v1/sync/audio
func SyncAudioHandler(c *gin.Context) {
	var req SyncAudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve device.
	var deviceID int64
	err := db.DB.QueryRow(
		`SELECT id FROM devices WHERE device_token = $1`, req.DeviceToken,
	).Scan(&deviceID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown device token"})
		return
	}

	// Decode audio bytes.
	audioBytes, err := base64.StdEncoding.DecodeString(req.AudioB64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 audio data"})
		return
	}

	// Build file path: uploads/audio/{device_id}/{timestamp}.aac
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	dirPath := filepath.Join(uploadDir, "audio", strconv.FormatInt(deviceID, 10))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload directory"})
		return
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	fileName := fmt.Sprintf("%s.aac", timestamp)
	filePath := filepath.Join(dirPath, fileName)

	if err := os.WriteFile(filePath, audioBytes, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save audio file"})
		return
	}

	// Log metadata.
	recordedAt := time.Now()
	if req.RecordedAt != "" {
		if t, err := time.Parse(time.RFC3339, req.RecordedAt); err == nil {
			recordedAt = t
		}
	}

	_, err = db.DB.Exec(
		`INSERT INTO audio_logs (device_id, file_path, duration_s, recorded_at) VALUES ($1, $2, $3, $4)`,
		deviceID, filePath, req.DurationSec, recordedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log audio metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "file": filePath})
}

// SyncNotificationHandler saves a notification captured by the Android client.
// POST /api/v1/sync/notification
type SyncNotificationRequest struct {
	DeviceToken string `json:"device_token"  binding:"required"`
	AppPackage  string `json:"app_package"   binding:"required"`
	Content     string `json:"content"       binding:"required"`
}

func SyncNotificationHandler(c *gin.Context) {
	var req SyncNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var deviceID int64
	err := db.DB.QueryRow(
		`SELECT id FROM devices WHERE device_token = $1`, req.DeviceToken,
	).Scan(&deviceID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown device token"})
		return
	}

	_, err = db.DB.Exec(
		`INSERT INTO notification_logs (device_id, app_package, content) VALUES ($1, $2, $3)`,
		deviceID, req.AppPackage, req.Content,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetNotificationsHandler returns recent notification logs.
// GET /api/v1/device/:device_id/notifications?limit=100
func GetNotificationsHandler(c *gin.Context) {
	deviceID := c.Param("device_id")
	rows, err := db.DB.Query(
		`SELECT app_package, content, received_at FROM notification_logs
		 WHERE device_id = $1 ORDER BY received_at DESC LIMIT 100`,
		deviceID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer rows.Close()

	type Notif struct {
		AppPackage string `json:"app_package"`
		Content    string `json:"content"`
		ReceivedAt string `json:"received_at"`
	}

	notifs := []Notif{}
	for rows.Next() {
		var n Notif
		if err := rows.Scan(&n.AppPackage, &n.Content, &n.ReceivedAt); err == nil {
			notifs = append(notifs, n)
		}
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifs})
}
