// Package redis wraps go-redis and provides session management helpers.
// Each connected Android device is tracked by its device_id as a Redis key.
package redisstore

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client is the shared Redis client.
var Client *redis.Client

// sessionTTL is how long a session key lives without a heartbeat.
const sessionTTL = 60 * time.Second

// Connect initialises the Redis client using REDIS_ADDR env var.
func Connect() {
	Client = redis.NewClient(&redis.Options{
		Addr:     getenv("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: failed to connect: %v", err)
	}
	fmt.Println("✅  Redis connected")
}

// RegisterSession marks a device as online.
func RegisterSession(ctx context.Context, deviceID string) error {
	return Client.Set(ctx, sessionKey(deviceID), "online", sessionTTL).Err()
}

// RemoveSession marks a device as offline.
func RemoveSession(ctx context.Context, deviceID string) error {
	return Client.Del(ctx, sessionKey(deviceID)).Err()
}

// IsOnline returns true if the device currently has an active session.
func IsOnline(ctx context.Context, deviceID string) bool {
	val, err := Client.Get(ctx, sessionKey(deviceID)).Result()
	return err == nil && val == "online"
}

// RefreshSession resets the TTL for an existing session (heartbeat).
func RefreshSession(ctx context.Context, deviceID string) {
	Client.Expire(ctx, sessionKey(deviceID), sessionTTL)
}

// Publish sends a raw message to the device's pub/sub channel.
// The backend WebSocket hub subscribes to this channel and forwards
// the payload to the appropriate parent WebSocket connection.
func Publish(ctx context.Context, deviceID string, payload []byte) error {
	return Client.Publish(ctx, frameChannel(deviceID), payload).Err()
}

// Subscribe returns a *redis.PubSub subscription for the device frame channel.
// Callers must call .Close() when done.
func Subscribe(ctx context.Context, deviceID string) *redis.PubSub {
	return Client.Subscribe(ctx, frameChannel(deviceID))
}

// ─── helpers ────────────────────────────────────────────────────────────────

func sessionKey(deviceID string) string   { return "device:" + deviceID + ":online" }
func frameChannel(deviceID string) string { return "frames:" + deviceID }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
