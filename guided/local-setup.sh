#!/usr/bin/env bash
# ============================================================
# FamilySync — Local Dev Setup (macOS + Homebrew)
# Run once to install and configure PostgreSQL + Redis
# ============================================================
set -euo pipefail

# ── Config ───────────────────────────────────────────────────
DB_NAME="familysync"
DB_USER="familysync"
DB_PASS="localdevpass"
# ─────────────────────────────────────────────────────────────

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  FamilySync Local Dev Setup (macOS)      ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# 1. Homebrew
if ! command -v brew &>/dev/null; then
  echo "Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# 2. PostgreSQL
echo "[ 1/3 ] Installing PostgreSQL..."
brew install postgresql@16
brew services start postgresql@16
export PATH="$(brew --prefix postgresql@16)/bin:$PATH"
sleep 2  # wait for pg to start

# Create user + database (ignore errors if already exists)
createuser  --createdb "$DB_USER"       2>/dev/null || true
createdb    -O "$DB_USER" "$DB_NAME"    2>/dev/null || true
psql -d "$DB_NAME" -c "ALTER USER $DB_USER WITH PASSWORD '$DB_PASS';" 2>/dev/null || true

# Apply schema
PGPASSWORD=$DB_PASS psql -U "$DB_USER" -d "$DB_NAME" \
  -f "$(dirname "$0")/../backend/schema.sql"
echo "✅  PostgreSQL ready"

# 3. Redis
echo "[ 2/3 ] Installing Redis..."
brew install redis
brew services start redis
echo "✅  Redis ready"

# 4. Go
echo "[ 3/3 ] Checking Go installation..."
if ! command -v go &>/dev/null; then
  brew install go
fi
echo "✅  Go $(go version)"

# 5. Write .env
ENV_FILE="$(dirname "$0")/../backend/.env"
if [ ! -f "$ENV_FILE" ]; then
  cat > "$ENV_FILE" <<EOF
PORT=8080
DB_DSN=postgres://$DB_USER:$DB_PASS@localhost:5432/$DB_NAME?sslmode=disable
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
JWT_SECRET=local_dev_secret_change_in_prod
UPLOAD_DIR=$(realpath "$(dirname "$0")/../backend")/uploads
GIN_MODE=debug
EOF
  mkdir -p "$(dirname "$0")/../backend/uploads/audio"
  echo "✅  .env written"
else
  echo "ℹ️   .env already exists — skipping"
fi

echo ""
echo "════════════════════════════════════════════"
echo "  Setup complete! Now run:"
echo "  cd backend && go run ./..."
echo "  Dashboard → http://localhost:8080"
echo "  Android emulator URL → http://10.0.2.2:8080"
echo "════════════════════════════════════════════"
echo ""
