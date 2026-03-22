-- FamilySync PostgreSQL Schema
-- Run with: psql -U postgres -d familysync -f schema.sql

CREATE TABLE IF NOT EXISTS users (
    id            SERIAL PRIMARY KEY,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS devices (
    id           SERIAL PRIMARY KEY,
    user_id      INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name  VARCHAR(255) NOT NULL,
    device_token VARCHAR(64) UNIQUE NOT NULL,   -- used by Android client to auth WS/REST
    fcm_token    VARCHAR(255) DEFAULT '',       -- token for Firebase Cloud Messaging WakeUp
    paired_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS location_logs (
    id          SERIAL PRIMARY KEY,
    device_id   INT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_logs (
    id          SERIAL PRIMARY KEY,
    device_id   INT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    app_package VARCHAR(255) NOT NULL,
    content     TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audio_logs (
    id          SERIAL PRIMARY KEY,
    device_id   INT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    file_path   TEXT NOT NULL,
    duration_s  INT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for query performance
CREATE INDEX IF NOT EXISTS idx_location_device    ON location_logs(device_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_device ON notification_logs(device_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_audio_device        ON audio_logs(device_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_device_token        ON devices(device_token);
