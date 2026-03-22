# FamilySync — Local Testing Guide (macOS + Android Emulator)

> **Goal**: Run the backend on your Mac, test with an Android Virtual Device (AVD) in Android Studio.
> The `devops/setup.sh` is Linux-only — use this guide instead.

---

## Prerequisites

| Tool | Install |
|---|---|
| [Homebrew](https://brew.sh) | `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"` |
| [Go 1.22+](https://go.dev) | `brew install go` |
| Android Studio | [Download](https://developer.android.com/studio) — includes AVD Manager |

---

## Step 1 — Run the setup script (one-time)

```bash
cd /Users/barkah/androidProject
chmod +x guided/local-setup.sh
bash guided/local-setup.sh
```

This installs **PostgreSQL 16**, **Redis**, and **Go** via Homebrew, creates the database, applies the schema, and writes `backend/.env`.

---

## Step 2 — Start the backend

```bash
cd /Users/barkah/androidProject/backend
go mod tidy          # download dependencies (first time only)
go run ./...
```

You should see:
```
[GIN-debug] Listening on :8080
```

Open **http://localhost:8080** → you'll see the parent dashboard login page.

---

## Step 3 — Set up Android Virtual Device (AVD)

1. Open **Android Studio → More Actions → Virtual Device Manager**
2. Create a device:
   - **Phone**: Pixel 6 (or any)
   - **API**: 30+ (Android 11+) — required for `foregroundServiceType`
3. Click ▶ to start the emulator

> **Important:** In the Android emulator, `localhost` refers to the emulator itself.
> Use **`10.0.2.2`** to reach your Mac's localhost.

---

## Step 3.5 — (Optional) Testing with a Physical Android Device on Wi-Fi

If you want to test on a real Android phone instead of an emulator:
1. Ensure your Mac and Android phone are connected to the **same Wi-Fi network**.
2. Find your Mac's local IP address (e.g. by running `ipconfig getifaddr en0` in the terminal).
3. When configuring the Android app onboarding, use your Mac's IP address instead of `10.0.2.2`. 
   - Example: `http://192.168.18.171:8080`
   - *(Note: We enabled `usesCleartextTraffic` in AndroidManifest so HTTP connections to local IPs are permitted).*
4. The rest of the steps remain the same.

---

## Step 4 — Configure the Android app

When you launch the app on the emulator, the **onboarding screen** will ask for:

| Field | Value for local testing |
|---|---|
| Server URL | `http://10.0.2.2:8080` |
| Parent email | anything, e.g. `parent@test.com / admin@familysync.com` |
| Password | anything, e.g. `password123` |
| Device name | anything, e.g. `Emulator` |

Tap **Activate FamilySync** → the app will register, pair the device, and start the monitoring service.

---

## Step 5 — Test in the dashboard

Open **http://localhost:8080** in your Mac browser.

1. **Register** with the same email/password you used in the app
2. Select the device from the sidebar
3. Click buttons to test commands:

| Button | Expected result |
|---|---|
| 📍 Get GPS | Map updates with emulator's mock location |
| 📷 Start Camera | Live frames appear in the canvas |
| 🖥️ Start Screen | Emulator screen mirrors in the canvas |
| 🎙️ Start Mic | Audio activity bar animates |
| 🔔 Load | Notification log fills in |

---

## Step 6 — Set a mock GPS location (emulator)

Android emulators don't have real GPS. To simulate a location:

1. In the running emulator: click the **⋮ (More)** button → **Location**
2. Search for any address → click **Set Location**
3. Tap **📍 Get GPS** in the dashboard → the pin appears on the map

---

## Daily Workflow

```bash
# Start services (if not already running)
brew services start postgresql@16
brew services start redis

# Run backend
cd /Users/barkah/androidProject/backend
go run ./...
```

```bash
# Stop services when done
brew services stop postgresql@16
brew services stop redis
```

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `connection refused` on app startup | Backend not running — run `go run ./...` |
| App says "Auth failed" | Check backend terminal for the error; DB might not be running |
| Dashboard shows device "Offline" | WebSocket didn't connect — check emulator has internet & server URL is `http://10.0.2.2:8080` |
| No camera/screen frames | Emulator may not support Camera2 API; use a physical device for camera testing |
| `go mod tidy` fails | Run `go env GOPATH` and ensure `$GOPATH/bin` is in PATH |
| PostgreSQL won't start | Run `brew services restart postgresql@16` |

---

## File Locations

```
androidProject/
├── backend/
│   ├── .env          ← Created by local-setup.sh
│   └── uploads/      ← Audio files saved here
├── dashboard/        ← Served at http://localhost:8080
└── guided/           ← This guide + macOS setup script
```
