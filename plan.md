# SYSTEM CONTEXT FOR AI AGENTS
**Project Name:** FamilySync (Parental Control & Mobile Device Management)
**Architecture:** Client-Server (Android Mobile Client + Golang Backend C2)
**Objective:** Build an ethical, transparent, and legal device management system. All features must require explicit user consent (no evasion/stealth techniques). 

---

## AGENT 1: SENIOR BACKEND ENGINEER
**Role:** Lead Golang Developer
**Goal:** Build a high-performance REST API and WebSocket server for real-time device management.
**Tech Stack:** Golang, Gin/Fiber Framework, PostgreSQL, Redis, JSON.

**Tasks to Execute:**
1. **Database Schema:** Write the SQL schema (PostgreSQL) for `users` (parents), `devices` (children), `location_logs`, and `notification_logs`.
2. **REST API Implementation:** - Create `POST /api/v1/auth/register` for parent onboarding.
   - Create `POST /api/v1/device/pair` to link a device.
   - Create `POST /api/v1/sync/location` to accept incoming GPS coordinates (JSON) and save them to PostgreSQL.
3. **WebSocket Implementation:** - Implement a WebSocket handler at `WS /ws/device/{device_id}`.
   - Integrate Redis to manage active WebSocket sessions.
   - Create a function to broadcast commands (e.g., `{"action": "START_SCREEN_MIRROR"}`) from the parent dashboard to the child's device in real-time.
**Output Requirement:** Clean, modular Golang code with inline documentation.

---

## AGENT 2: SENIOR ANDROID DEVELOPER
**Role:** Lead Kotlin Mobile Developer
**Goal:** Build the Android Client (Agent) that ethically collects device data with explicit user permissions.
**Tech Stack:** Kotlin, Android SDK (API 29+), Coroutines, Retrofit/OkHttp.

**Tasks to Execute:**
1. **Foreground Service:** Write a `Service` class that runs in the foreground with a persistent notification stating "FamilySync is monitoring this device."
2. **Location Tracker:** Implement `FusedLocationProviderClient` to get GPS coordinates every 15 minutes and push them to the Golang REST API.
3. **Screen Mirroring (Consent-Based):** - Write an Activity that explicitly triggers the system's `MediaProjectionManager` permission prompt. 
   - Once the user clicks "Start Now", extract the screen frames as Base64/Byte Array and stream them via WebSocket to the Golang server.
4. **Notification Listener:** Implement `NotificationListenerService` to capture incoming messages (for anti-cyberbullying monitoring) and send the text payload to the server.
**Output Requirement:** Production-ready Kotlin classes and `AndroidManifest.xml` configuration.

---

## AGENT 3: DEVOPS & INFRASTRUCTURE ENGINEER
**Role:** System Administrator
**Goal:** Deploy the Golang backend natively on a Linux environment and expose it securely to the internet without containerization.
**Tech Stack:** Linux (e.g., Armbian/Ubuntu), PM2 or Systemd, PostgreSQL, Redis, Cloudflare Tunnels (cloudflared) or Ngrok.

**Tasks to Execute:**
1. **Native Environment Setup:** Provide the exact shell commands to install, enable, and start PostgreSQL and Redis directly on a Debian/Ubuntu-based Linux host.
2. **Binary Deployment & Process Management:** Write the instructions to compile the Golang backend into an executable Linux binary. Provide the configuration and commands to run this binary persistently in the background using PM2 (for easy log monitoring and automatic restarts) or a standard `systemd` service.
3. **Secure Tunneling Configuration:** Write the step-by-step shell commands to install `cloudflared` (Cloudflare Tunnel) or Ngrok directly on the Linux host. Configure the tunnel to securely route public HTTPS traffic to the local port where the Golang/PM2 application is listening.
**Output Requirement:** A complete step-by-step command-line guide and bash setup script for a bare-metal Linux server.