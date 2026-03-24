// Package handlers — WebSocket hub for real-time device command and frame relay.
//
// Two WebSocket endpoints:
//   - WS /ws/device/{device_id}  → Android client connects here (persistent)
//   - WS /ws/parent/{device_id}  → Parent dashboard connects to receive live frames
//
// Flow:
//  1. Android connects → registered in hub.devices map + Redis session
//  2. Parent sends POST /api/v1/command/{device_id} → command pushed to device WS
//  3. Android responds with JSON frames/GPS → hub relays to parent WS
package handlers

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/familysync/backend/db"
	redisstore "github.com/familysync/backend/redis"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ─── WebSocket upgrader ──────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Allow all origins — in production, restrict to your domain.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─── Hub ─────────────────────────────────────────────────────────────────────

// Hub tracks active device and parent WebSocket connections in memory.
// For a single-node Armbian deployment, in-memory is sufficient.
// Redis is used as a lightweight presence flag alongside.
type Hub struct {
	mu      sync.RWMutex
	devices map[string]*websocket.Conn // device_id → device connection
	parents map[string]*websocket.Conn // device_id → parent dashboard connection
}

// GlobalHub is the singleton hub instance shared across all handlers.
var GlobalHub = &Hub{
	devices: make(map[string]*websocket.Conn),
	parents: make(map[string]*websocket.Conn),
}

func (h *Hub) registerDevice(id string, c *websocket.Conn) {
	h.mu.Lock()
	h.devices[id] = c
	h.mu.Unlock()
}

func (h *Hub) unregisterDevice(id string) {
	h.mu.Lock()
	delete(h.devices, id)
	h.mu.Unlock()
}

func (h *Hub) registerParent(id string, c *websocket.Conn) {
	h.mu.Lock()
	h.parents[id] = c
	h.mu.Unlock()
}

func (h *Hub) unregisterParent(id string) {
	h.mu.Lock()
	delete(h.parents, id)
	h.mu.Unlock()
}

// SendToDevice pushes a text command to the connected device.
// Returns false if the device is not currently connected.
func (h *Hub) SendToDevice(deviceID string, msg []byte) bool {
	h.mu.RLock()
	conn, ok := h.devices[deviceID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, msg) == nil
}

// relayToParent forwards a raw frame payload to the watching parent dashboard.
func (h *Hub) relayToParent(deviceID string, data []byte) {
	h.mu.RLock()
	conn, ok := h.parents[deviceID]
	h.mu.RUnlock()
	if !ok {
		return // No parent watching — drop silently.
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		log.Printf("relay→parent[%s]: %v", deviceID, err)
	}
}

// ─── Device WebSocket handler ─────────────────────────────────────────────────

// DeviceWSHandler handles the persistent WebSocket from the Android client.
// WS /ws/device/:device_id?token=<device_token>
func DeviceWSHandler(c *gin.Context) {
	deviceID := c.Param("device_id")
	token := c.Query("token")

	// Validate device_token against the database.
	var dbID int64
	err := db.DB.QueryRow(
		`SELECT id FROM devices WHERE device_token = $1`, token,
	).Scan(&dbID)
	if err != nil || dbID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid device token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("device WS upgrade [%s]: %v", deviceID, err)
		return
	}
	defer func() {
		conn.Close()
		GlobalHub.unregisterDevice(deviceID)
		// JANGAN hapus session Redis di sini — biarkan TTL 60 detik expire sendiri.
		// Ini memberi waktu device untuk reconnect setelah restart/swipe-close
		// tanpa status berubah ke offline di dashboard.
		log.Printf("device [%s] disconnected — session TTL will expire naturally", deviceID)
	}()

	GlobalHub.registerDevice(deviceID, conn)
	redisstore.RegisterSession(c.Request.Context(), deviceID)
	log.Printf("device [%s] connected", deviceID)

	// Configuration.
	conn.SetReadLimit(10 * 1024 * 1024) // 10 MB max (for video frames)
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		redisstore.RefreshSession(c.Request.Context(), deviceID)
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Keepalive ping goroutine.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}()

	// Main read loop — forward every incoming message (frame/GPS response) to the parent.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		GlobalHub.relayToParent(deviceID, msg)
	}
}

// ─── Parent WebSocket handler ─────────────────────────────────────────────────

// ParentWSHandler opens a receive-only WebSocket for the parent dashboard to watch live feeds.
// WS /ws/parent/:device_id  — JWT validated upstream by middleware.
func ParentWSHandler(c *gin.Context) {
	deviceID := c.Param("device_id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("parent WS upgrade [%s]: %v", deviceID, err)
		return
	}
	defer func() {
		conn.Close()
		GlobalHub.unregisterParent(deviceID)
		log.Printf("parent viewer [%s] disconnected", deviceID)
	}()

	GlobalHub.registerParent(deviceID, conn)
	log.Printf("parent viewer [%s] connected", deviceID)

	// Keep alive — parent only receives; discard any messages it sends.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
