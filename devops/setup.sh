#!/usr/bin/env bash
# ============================================================
# FamilySync — Armbian / Ubuntu / Debian Setup Script
# Tested on: Armbian 23.x (Debian Bookworm), Ubuntu 22.04 LTS
# RAM:  2 GB minimum
# Arch: arm64 / amd64
# ============================================================
set -euo pipefail

# ── Configuration (edit before running) ─────────────────────
# Karena Anda men-*clone* *repo* ini langsung, maka kita tidak akan memindahkannya.
# APP_DIR akan otomatis mendeteksi *folder root* *repository* Anda.
APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DB_NAME="familysync"
DB_USER="familysync"
DB_PASS="change_me_strong_password"
GO_VERSION="1.22.3"
ARCH="arm64"           # Change to "amd64" for x86 servers
# ────────────────────────────────────────────────────────────

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║      FamilySync Setup Script             ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# ── 1. System update ─────────────────────────────────────────
echo "[ 1/7 ] Updating system packages..."
apt-get update -q && apt-get upgrade -y -q

# ── 2. PostgreSQL ─────────────────────────────────────────────
echo "[ 2/7 ] Installing PostgreSQL..."
apt-get install -y postgresql postgresql-contrib

systemctl enable postgresql
systemctl start  postgresql

# Create DB user and database.
sudo -u postgres psql -c "CREATE ROLE $DB_USER WITH LOGIN PASSWORD '$DB_PASS';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE $DB_NAME OWNER $DB_USER;"              2>/dev/null || true
echo "✅  PostgreSQL ready — database: $DB_NAME"

# ── 3. Redis ──────────────────────────────────────────────────
echo "[ 3/7 ] Installing Redis..."
apt-get install -y redis-server

# Configure Redis: allow only loopback, disable persistence for speed.
sed -i 's/^# bind 127.0.0.1/bind 127.0.0.1/' /etc/redis/redis.conf
sed -i 's/^save 900/# save 900/' /etc/redis/redis.conf
sed -i 's/^save 300/# save 300/' /etc/redis/redis.conf
sed -i 's/^save 60/#  save 60/'  /etc/redis/redis.conf

systemctl enable redis-server
systemctl restart redis-server
echo "✅  Redis ready"

# ── 4. Go runtime ─────────────────────────────────────────────
echo "[ 4/7 ] Installing Go $GO_VERSION ($ARCH)..."
GO_TAR="go${GO_VERSION}.linux-${ARCH}.tar.gz"
wget -q "https://go.dev/dl/${GO_TAR}" -O /tmp/${GO_TAR}
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/${GO_TAR}
rm /tmp/${GO_TAR}

# Add to PATH for all users.
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
source /etc/profile.d/go.sh
echo "✅  Go $(go version)"

# ── 5. Deploy application ─────────────────────────────────────
echo "[ 5/7 ] Deploying FamilySync backend (In-Place)..."
echo "App Directory: $APP_DIR"

# Write .env
cat > $APP_DIR/.env <<EOF
PORT=8080
DB_DSN=postgres://$DB_USER:$DB_PASS@127.0.0.1:5432/$DB_NAME?sslmode=disable
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
JWT_SECRET=$(openssl rand -hex 32)
UPLOAD_DIR=$APP_DIR/uploads
GIN_MODE=release
EOF

mkdir -p $APP_DIR/uploads/audio

# Apply schema.
# Memaksa koneksi via localhost (TCP) untuk menghindari "Peer authentication failed"
PGPASSWORD=$DB_PASS psql -h 127.0.0.1 -U $DB_USER -d $DB_NAME -f $APP_DIR/backend/schema.sql
echo "✅  Database schema applied"

# Prevent Out-of-Memory (OOM) on 512MB/1GB VPS during compilation
if [ $(free -m | awk '/^Swap:/ {print $2}') -eq 0 ]; then
    echo "⚠️  No SWAP detected. Creating 2GB temporary SWAP file..."
    fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo "/swapfile none swap sw 0 0" >> /etc/fstab
fi

# Compile binary.
cd $APP_DIR/backend
/usr/local/go/bin/go mod tidy
# Membatasi thread kompilasi menjadi 1 agar RAM hemat
/usr/local/go/bin/go build -p 1 -o $APP_DIR/familysync-server .
echo "✅  Binary compiled: $APP_DIR/familysync-server"

# ── 6. PM2 process manager ────────────────────────────────────
echo "[ 6/7 ] Setting up PM2..."
if ! command -v npm &>/dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
    apt-get install -y nodejs
fi
npm install -g pm2 -q

# Start with PM2 from the app directory (so it finds .env and dashboard/).
cd $APP_DIR
pm2 start ./familysync-server --name familysync --cwd $APP_DIR
pm2 save

# Generate systemd startup script for PM2 auto-restart on boot.
env PATH=$PATH:/usr/local/go/bin pm2 startup systemd -u root --hp /root | tail -1 | bash
echo "✅  PM2 running (pm2 status to check)"

# ── 7. Cloudflare Tunnel ──────────────────────────────────────
echo "[ 7/7 ] Installing Cloudflare Tunnel (cloudflared)..."
if [ "$ARCH" = "arm64" ]; then
    CFURL="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64"
else
    CFURL="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
fi

wget -q "$CFURL" -O /usr/local/bin/cloudflared
chmod +x /usr/local/bin/cloudflared

echo ""
echo "════════════════════════════════════════════"
echo "  Next step: authenticate Cloudflare Tunnel"
echo ""
echo "  1. Run: cloudflared tunnel login"
echo "     (opens a browser — login to Cloudflare)"
echo ""
echo "  2. Create tunnel:"
echo "     cloudflared tunnel create familysync"
echo ""
echo "  3. Create config: /etc/cloudflared/config.yml"
echo "     tunnel: <TUNNEL_ID>"
echo "     credentials-file: /root/.cloudflared/<TUNNEL_ID>.json"
echo "     ingress:"
echo "       - hostname: familysync.yourdomain.com"
echo "         service: http://localhost:8080"
echo "       - service: http_status:404"
echo ""
echo "  4. Start tunnel:"
echo "     cloudflared service install"
echo "     systemctl enable cloudflared"
echo "     systemctl start  cloudflared"
echo "════════════════════════════════════════════"
echo ""
echo "🎉  FamilySync setup complete!"
echo "    Dashboard: http://localhost:8080"
echo "    PM2 logs:  pm2 logs familysync"
echo ""
