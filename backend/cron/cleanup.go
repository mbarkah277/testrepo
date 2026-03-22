package cron

import (
	"log"
	"os"
	"time"

	"github.com/familysync/backend/db"
)

// StartCleanupJob initiates a scheduled daemon to permanently delete outdated logs
// (audio files, location data, parsing notifications) to prevent the disk and DB from filling up.
func StartCleanupJob() {
	// The job runs automatically every 24 hours.
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			cleanupOldData()
		}
	}()

	// To ensure the server stays clean on restarts, also perform a swift clean
	// 10 seconds after every backend boot.
	go func() {
		time.Sleep(10 * time.Second)
		cleanupOldData()
	}()
}

func cleanupOldData() {
	log.Println("[CRON] Running automatic data cleanup for old records...")

	// ----------------------------------------------------
	// Retention policy: Keep tracking data for the last 7 days.
	// You can change 'retentionDays' to whatever limits you like.
	// ----------------------------------------------------
	retentionDays := 7
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// 1. Delete physical .aac audio files from disks.
	// We read the paths from the DB first before we delete their DB records.
	rows, err := db.DB.Query(`SELECT file_path FROM audio_logs WHERE recorded_at < $1`, cutoff)
	if err == nil {
		for rows.Next() {
			var filePath string
			if err := rows.Scan(&filePath); err == nil {
				// Delete the physical file permanently
				if err := os.Remove(filePath); err == nil {
					log.Printf("[CRON] Deleted expired audio file from disk: %s", filePath)
				}
			}
		}
		rows.Close()
	} else {
		log.Printf("[CRON] Warning: Could not scan old audio records: %v", err)
	}

	// 2. Delete database rows (PostgreSQL)
	if _, err := db.DB.Exec(`DELETE FROM audio_logs WHERE recorded_at < $1`, cutoff); err != nil {
		log.Printf("[CRON] Failed to purge audio_logs table: %v", err)
	}
	if _, err := db.DB.Exec(`DELETE FROM location_logs WHERE recorded_at < $1`, cutoff); err != nil {
		log.Printf("[CRON] Failed to purge location_logs table: %v", err)
	}
	if _, err := db.DB.Exec(`DELETE FROM notification_logs WHERE received_at < $1`, cutoff); err != nil {
		log.Printf("[CRON] Failed to purge notification_logs table: %v", err)
	}

	log.Printf("[CRON] Mission Accomplished. All tracker data older than %d days has been securely wiped.", retentionDays)
}
