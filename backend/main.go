// FamilySync Backend — main entry point.
// Loads .env, connects to PostgreSQL and Redis, then starts the Gin HTTP server.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/familysync/backend/cron"
	"github.com/familysync/backend/db"
	"github.com/familysync/backend/fcm"
	"github.com/familysync/backend/handlers"
	"github.com/familysync/backend/middleware"
	redisstore "github.com/familysync/backend/redis"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (ignore error in production where env vars are set directly).
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — using system environment variables")
	}

	// Connect to PostgreSQL and Redis.
	db.Connect()
	redisstore.Connect()

	// Nyalakan peluncur Firebase
	fcm.InitFirebase()

	// Start background cleanup cron job (auto-delete old data).
	cron.StartCleanupJob()

	// Set Gin to release mode in production; debug mode shows route table.
	if os.Getenv("GIN_MODE") != "release" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// ── Serve parent dashboard as static files ────────────────────────────────
	// Automatic Working Directory detection (In-Place deploy via PM2 vs Local Go run)
	dashboardDir := "./dashboard"
	if _, err := os.Stat(dashboardDir); os.IsNotExist(err) {
		dashboardDir = "../dashboard"
	}

	r.StaticFile("/", dashboardDir+"/index.html")
	r.StaticFile("/app.js", dashboardDir+"/app.js")
	r.StaticFile("/style.css", dashboardDir+"/style.css")

	// ── Public auth routes ────────────────────────────────────────────────────
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", handlers.RegisterHandler)
		auth.POST("/login", handlers.LoginHandler)
	}

	// ── Device data ingestion (authenticated by device_token, not JWT) ────────
	sync := r.Group("/api/v1/sync")
	{
		sync.POST("/location", handlers.SyncLocationHandler)
		sync.POST("/audio", handlers.SyncAudioHandler)
		sync.POST("/notification", handlers.SyncNotificationHandler)
		sync.POST("/fcm", handlers.SyncFCMHandler)
	}

	// ── Parent-authenticated routes (JWT required) ────────────────────────────
	api := r.Group("/api/v1", middleware.AuthRequired())
	{
		// Device management
		api.POST("/device/pair", handlers.PairDeviceHandler)
		api.GET("/device/list", handlers.ListDevicesHandler)
		api.GET("/device/status", handlers.DeviceStatusHandler)

		// History queries
		api.GET("/device/:device_id/location", handlers.GetLocationHistoryHandler)
		api.GET("/device/:device_id/notifications", handlers.GetNotificationsHandler)

		// Real-time command dispatch → pushes command to device WebSocket
		api.POST("/command/:device_id", handlers.SendCommandHandler)

		// Parent live-feed WebSocket (receives frames relayed from device)
		api.GET("/ws/parent/:device_id", handlers.ParentWSHandler)
	}

	// ── Device WebSocket (authenticated by device_token query param) ──────────
	r.GET("/ws/device/:device_id", handlers.DeviceWSHandler)

	// ── Start server ──────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("\n🚀  FamilySync server running on http://0.0.0.0:%s\n", port)
	fmt.Printf("📊  Dashboard:   http://localhost:%s/\n", port)
	fmt.Printf("🔌  Device WS:   ws://localhost:%s/ws/device/{id}?token=<token>\n", port)
	fmt.Printf("📡  Parent WS:   ws://localhost:%s/api/v1/ws/parent/{id}\n\n", port)

	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
