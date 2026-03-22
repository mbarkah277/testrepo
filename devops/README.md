# FamilySync — Deployment Guide

## Architecture Overview
```
Parent Dashboard (Web UI)
        │  POST /api/v1/command/{device_id}
        ▼
Golang Backend (Armbian)
  ├── PostgreSQL  (persistence)
  ├── Redis       (device sessions + frame relay)
  └── Cloudflare Tunnel → public HTTPS
        │  WS /ws/device/{id}?token=<token>
        ▼
Android Client (child device)
```

---

## Quick Start (Armbian / Ubuntu)

### 1. Clone / copy project
```bash
git clone <your-repo> /opt/familysync-src
cd /opt/familysync-src
```

### 2. Run setup script
```bash
chmod +x devops/setup.sh
sudo bash devops/setup.sh
```
This installs PostgreSQL, Redis, Go 1.22, compiles the binary, starts PM2, and installs cloudflared.

### 3. Edit .env
```bash
nano /opt/familysync/.env
# Set JWT_SECRET, DB_PASS, PORT as needed
pm2 restart familysync
```

### 4. Apply DB schema (done by setup.sh, but manually if needed)
```bash
PGPASSWORD=yourpass psql -U familysync -d familysync -f backend/schema.sql
```

---

## Cloudflare Tunnel (Expose to Internet)
```bash
# Authenticate (opens browser)
cloudflared tunnel login

# Create tunnel
cloudflared tunnel create familysync

# Create /etc/cloudflared/config.yml:
tunnel: <TUNNEL_ID>
credentials-file: /root/.cloudflared/<TUNNEL_ID>.json
ingress:
  - hostname: familysync.yourdomain.com
    service: http://localhost:8080
  - service: http_status:404

# Start as system service
cloudflared service install
systemctl enable cloudflared
systemctl start  cloudflared
```

---

## Alternative: systemd (instead of PM2)
```bash
cp devops/familysync.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable familysync
systemctl start  familysync
journalctl -u familysync -f   # live logs
```

---

## PM2 Commands
```bash
pm2 status             # check running
pm2 logs familysync    # live logs
pm2 restart familysync # restart after config change
pm2 stop familysync    # stop
```

---

## Android Client Setup
1. Open `android/` in Android Studio
2. Set your tunnel URL in `ApiClient.kt` → `BASE_URL`
3. Build and install APK on child device
4. Open app → register parent account → pair device
5. Grant all requested permissions (location, camera, microphone, notifications)
6. The service starts automatically and shows a persistent notification

---

## Parent Dashboard
Open `https://familysync.yourdomain.com` in any browser.
- **Login** with your parent account
- Select a child device from the sidebar
- Use buttons to send real-time commands

| Button | Action |
|---|---|
| 📍 Get GPS | Instant location fix on map |
| 📷 Start Camera | Live camera stream (5fps) |
| 🖥️ Start Screen | Live screen capture stream |
| 🎙️ Start Mic | Live microphone audio |
| 🔔 Load Notifications | Table of captured notifications |

---

## RAM Usage (Armbian 2GB)
| Service | Idle RAM |
|---|---|
| familysync binary | ~15–30 MB |
| PostgreSQL | ~50 MB |
| Redis | ~5 MB |
| **Total** | **~70–85 MB** |

Plenty of headroom for 2GB RAM. ✅
