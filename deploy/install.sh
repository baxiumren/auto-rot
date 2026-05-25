#!/usr/bin/env bash
#
# BongBot — One-shot installer untuk Ubuntu 22/24+
# Jalanin di VPS sebagai root atau dengan sudo:
#   curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
# ATAU clone repo dulu, lalu:
#   cd auto-rotator && sudo bash deploy/install.sh
#

set -euo pipefail

# ─── Config ──────────────────────────────────────────────────────────────────
INSTALL_DIR="/opt/bongbot"
SERVICE_USER="bongbot"
SERVICE_NAME="bongbot"
REPO_URL="https://github.com/baxiumren/auto-rot.git"
BRANCH="main"
BINARY_NAME="bot-linux-amd64"

# ─── Helpers ─────────────────────────────────────────────────────────────────
log()  { echo -e "\033[1;34m[*]\033[0m $*"; }
ok()   { echo -e "\033[1;32m[✓]\033[0m $*"; }
warn() { echo -e "\033[1;33m[!]\033[0m $*"; }
err()  { echo -e "\033[1;31m[x]\033[0m $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || err "Jalanin sebagai root: sudo bash $0"

# ─── 1. Dependencies ─────────────────────────────────────────────────────────
log "Cek & install dependency dasar..."
apt-get update -qq
apt-get install -y -qq git curl ca-certificates >/dev/null
ok "git, curl, ca-certificates terinstall"

# ─── 2. User & directory ─────────────────────────────────────────────────────
if ! id "$SERVICE_USER" &>/dev/null; then
    log "Bikin system user '$SERVICE_USER'..."
    useradd --system --shell /usr/sbin/nologin --home-dir "$INSTALL_DIR" --create-home "$SERVICE_USER"
    ok "User '$SERVICE_USER' dibuat"
else
    ok "User '$SERVICE_USER' udah ada"
fi

mkdir -p "$INSTALL_DIR/data"

# ─── 3. Source code ─────────────────────────────────────────────────────────
if [[ -d "$INSTALL_DIR/.git" ]]; then
    log "Update source code (git pull)..."
    cd "$INSTALL_DIR"
    sudo -u "$SERVICE_USER" git fetch origin "$BRANCH" -q
    sudo -u "$SERVICE_USER" git reset --hard "origin/$BRANCH"
else
    log "Clone repo ke $INSTALL_DIR..."
    # Clear directory dulu (kecuali data/)
    find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 ! -name 'data' -exec rm -rf {} +
    sudo -u "$SERVICE_USER" git clone -q --branch "$BRANCH" "$REPO_URL" "$INSTALL_DIR/.tmp_clone"
    # Move isi (termasuk dotfiles), preserve data/
    shopt -s dotglob
    mv "$INSTALL_DIR/.tmp_clone"/* "$INSTALL_DIR/"
    rm -rf "$INSTALL_DIR/.tmp_clone"
fi
ok "Source code siap"

chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"

# ─── 4. Build (kalau Go ada di sistem) atau pakai binary pre-built ───────────
cd "$INSTALL_DIR"

if command -v go &>/dev/null; then
    log "Go terdeteksi, build dari source..."
    sudo -u "$SERVICE_USER" env CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BINARY_NAME" .
    ok "Build sukses"
elif [[ -f "$BINARY_NAME" ]]; then
    ok "Pakai binary pre-built ($BINARY_NAME) dari repo"
    chmod +x "$BINARY_NAME"
else
    warn "Go gak ada di sistem dan binary pre-built ($BINARY_NAME) gak ada."
    log "Install Go..."
    apt-get install -y -qq golang-go >/dev/null
    sudo -u "$SERVICE_USER" env CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BINARY_NAME" .
    ok "Build sukses"
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

# ─── 5. .env setup ───────────────────────────────────────────────────────────
if [[ ! -f "$INSTALL_DIR/.env" ]]; then
    cp "$INSTALL_DIR/.env.example" "$INSTALL_DIR/.env"
    chown "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/.env"
    chmod 600 "$INSTALL_DIR/.env"
    warn "File .env baru dibuat dari .env.example"
    warn "WAJIB di-edit dulu sebelum start bot:"
    warn "   sudo nano $INSTALL_DIR/.env"
else
    ok "File .env udah ada (gak di-overwrite)"
fi

# ─── 6. systemd service ──────────────────────────────────────────────────────
log "Install systemd service..."
cp "$INSTALL_DIR/deploy/bongbot.service" "/etc/systemd/system/$SERVICE_NAME.service"
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
ok "Service '$SERVICE_NAME' enabled (auto-start saat boot)"

# ─── 7. Summary ──────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════════════════════════"
ok "INSTALLATION SELESAI!"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "📍 Install dir : $INSTALL_DIR"
echo "👤 Service user: $SERVICE_USER"
echo "🔧 Service     : $SERVICE_NAME"
echo ""
echo "🚀 Langkah berikutnya:"
echo ""
echo "   1. Edit .env (isi BOT_TOKEN, ALLOWED_CHAT_ID, ADMIN_IDS):"
echo "      sudo nano $INSTALL_DIR/.env"
echo ""
echo "   2. Start bot:"
echo "      sudo systemctl start $SERVICE_NAME"
echo ""
echo "   3. Cek status:"
echo "      sudo systemctl status $SERVICE_NAME"
echo ""
echo "   4. Lihat log real-time:"
echo "      sudo journalctl -u $SERVICE_NAME -f"
echo ""
echo "📋 Perintah berguna lainnya:"
echo "   sudo systemctl stop $SERVICE_NAME       # matikan bot"
echo "   sudo systemctl restart $SERVICE_NAME    # restart"
echo "   sudo systemctl disable $SERVICE_NAME    # gak auto-start saat boot"
echo "   sudo journalctl -u $SERVICE_NAME -n 100 # 100 log terakhir"
echo ""
echo "🔄 Update bot ke versi terbaru (re-run script ini):"
echo "   sudo bash $INSTALL_DIR/deploy/install.sh"
echo ""
