package handlers

import (
	"net/http"
	"time"

	"github.com/familysync/backend/db"
	"github.com/gin-gonic/gin"
)

// SyncLocationRequest is sent by the Android client.
type SyncLocationRequest struct {
	DeviceToken string  `json:"device_token" binding:"required"`
	Latitude    float64 `json:"lat"          binding:"required"`
	Longitude   float64 `json:"lng"          binding:"required"`
	Timestamp   string  `json:"timestamp"`   // ISO-8601, optional override
}

// SyncLocationHandler saves GPS coordinates received from the Android client.
// POST /api/v1/sync/location
func SyncLocationHandler(c *gin.Context) {
	var req SyncLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve device_id from device_token.
	var deviceID int64
	err := db.DB.QueryRow(
		`SELECT id FROM devices WHERE device_token = $1`, req.DeviceToken,
	).Scan(&deviceID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown device token"})
		return
	}

	// Use provided timestamp or default to now.
	recordedAt := time.Now()
	if req.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			recordedAt = t
		}
	}

	_, err = db.DB.Exec(
		`INSERT INTO location_logs (device_id, latitude, longitude, recorded_at) VALUES ($1, $2, $3, $4)`,
		deviceID, req.Latitude, req.Longitude, recordedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save location"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetLocationHistoryHandler returns recent GPS logs for a device.
// GET /api/v1/device/:device_id/location?limit=50
func GetLocationHistoryHandler(c *gin.Context) {
	deviceID := c.Param("device_id")
	limit := 50

	rows, err := db.DB.Query(
		`SELECT latitude, longitude, recorded_at FROM location_logs
		 WHERE device_id = $1 ORDER BY recorded_at DESC LIMIT $2`,
		deviceID, limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch locations"})
		return
	}
	defer rows.Close()

	type Point struct {
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
		RecordedAt string  `json:"recorded_at"`
	}

	points := []Point{}
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.Lat, &p.Lng, &p.RecordedAt); err == nil {
			points = append(points, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{"locations": points})
}
