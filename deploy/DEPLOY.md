# 🚀 Deploy BongBot ke VPS Ubuntu

Panduan lengkap deploy bot ke VPS Ubuntu (22.04 / 24.04) supaya **running 24/7** auto-restart kalau crash.

---

## ⚡ Quick Install (1 command)

Setelah repo udah di-push ke GitHub:

```bash
curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
```

Atau clone manual:

```bash
git clone https://github.com/baxiumren/auto-rot.git /tmp/bongbot
sudo bash /tmp/bongbot/deploy/install.sh
```

---

## 📋 Manual Install (kalau gak mau pakai script)

### 1. Install Go (opsional — kalau mau build sendiri)

```bash
sudo apt update
sudo apt install -y golang-go git
go version  # Pastikan Go 1.21+
```

### 2. Bikin user khusus + clone repo

```bash
sudo useradd --system --shell /usr/sbin/nologin --home-dir /opt/bongbot --create-home bongbot
sudo -u bongbot git clone https://github.com/baxiumren/auto-rot.git /opt/bongbot/.tmp
sudo mv /opt/bongbot/.tmp/* /opt/bongbot/.tmp/.* /opt/bongbot/ 2>/dev/null || true
sudo rm -rf /opt/bongbot/.tmp
sudo chown -R bongbot:bongbot /opt/bongbot
```

### 3. Build binary

```bash
cd /opt/bongbot
sudo -u bongbot env CGO_ENABLED=0 go build -ldflags="-s -w" -o bot-linux-amd64 .
sudo chmod +x bot-linux-amd64
```

Atau kalau udah ada `bot-linux-amd64` di repo, skip step ini.

### 4. Setup .env

```bash
sudo cp /opt/bongbot/.env.example /opt/bongbot/.env
sudo chown bongbot:bongbot /opt/bongbot/.env
sudo chmod 600 /opt/bongbot/.env
sudo nano /opt/bongbot/.env
```

Isi:
```env
BOT_TOKEN=8791752562:AAE18m...
ALLOWED_CHAT_ID=-1003932797479
ADMIN_IDS=7626641150,881552619
CF_EMAIL=
CF_API_KEY=
CHECK_INTERVAL=45s
```

> CF_EMAIL & CF_API_KEY boleh kosong — bisa di-set lewat menu `🔧 Settings` di bot.

### 5. Install systemd service

```bash
sudo cp /opt/bongbot/deploy/bongbot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable bongbot
sudo systemctl start bongbot
```

### 6. Verifikasi

```bash
sudo systemctl status bongbot
sudo journalctl -u bongbot -f
```

Outputnya harus muncul:
```
✅ BongBot started | interval=45s | admins=2 | sticky=0
[ROTATOR] Service dimulai
[MONITOR-SCAN] service dimulai
```

---

## 🛠️ Operasional Commands

| Aksi | Command |
|------|---------|
| Start | `sudo systemctl start bongbot` |
| Stop | `sudo systemctl stop bongbot` |
| Restart | `sudo systemctl restart bongbot` |
| Status | `sudo systemctl status bongbot` |
| Log real-time | `sudo journalctl -u bongbot -f` |
| Log 100 terakhir | `sudo journalctl -u bongbot -n 100` |
| Log dari hari ini | `sudo journalctl -u bongbot --since today` |
| Disable auto-start | `sudo systemctl disable bongbot` |
| Enable auto-start | `sudo systemctl enable bongbot` |

---

## 🔄 Update Bot ke Versi Terbaru

### Cara 1 — re-run install script (RECOMMENDED, auto-restart):
```bash
sudo bash /opt/bongbot/deploy/install.sh
```

Install.sh otomatis:
- git pull dari main branch
- Rebuild binary
- **Auto-restart service kalau sedang running** (no zombie)
- Tail 15 log terakhir untuk verify

### Cara 2 — manual git pull + rebuild:
```bash
cd /opt/bongbot
sudo -u bongbot git pull
sudo -u bongbot env CGO_ENABLED=0 go build -ldflags="-s -w" -o bot-linux-amd64 .
sudo systemctl restart bongbot
```

---

## 🔐 Security Notes

### Yang udah di-handle service file:
- ✅ Service jalan sebagai non-root user (`bongbot`)
- ✅ `NoNewPrivileges=true` (gak bisa escalation)
- ✅ `ProtectSystem=strict` (read-only filesystem)
- ✅ `ProtectHome=true` (gak bisa akses /home)
- ✅ `PrivateTmp=true` (isolated /tmp)
- ✅ `ReadWritePaths=/opt/bongbot/data` (cuma data/ yang writable)
- ✅ `MemoryMax=512M` (cegah memory leak nge-blow up server)

### File permissions:
```bash
sudo chmod 600 /opt/bongbot/.env                    # cuma owner bisa baca
sudo chmod 600 /opt/bongbot/data/credentials.json   # cuma owner bisa baca
```

### Firewall (UFW):
Bot ini *outbound only* (Telegram API + CF API), jadi gak butuh open port. Kalau pakai UFW:
```bash
sudo ufw allow ssh
sudo ufw enable
# port 22 aja udah cukup
```

---

## 💾 Backup & Restore

### Backup manual:
```bash
sudo tar -czf bongbot-backup-$(date +%Y%m%d-%H%M%S).tar.gz \
    -C /opt/bongbot data .env
```

### Restore:
```bash
sudo systemctl stop bongbot
sudo tar -xzf bongbot-backup-YYYYMMDD-HHMMSS.tar.gz -C /opt/bongbot/
sudo chown -R bongbot:bongbot /opt/bongbot/data /opt/bongbot/.env
sudo systemctl start bongbot
```

### Auto-backup harian via cron:
```bash
sudo crontab -e
```
Tambah baris:
```
0 3 * * * tar -czf /root/bongbot-backup-$(date +\%Y\%m\%d).tar.gz -C /opt/bongbot data .env && find /root/bongbot-backup-*.tar.gz -mtime +7 -delete
```
(Backup tiap jam 3 pagi, hapus backup > 7 hari.)

---

## 🔍 Troubleshooting

### Bot gak start
```bash
sudo journalctl -u bongbot -n 50 --no-pager
```

**Penyebab umum:**
- `BOT_TOKEN tidak di-set` → cek `.env`
- `Bot error: Unauthorized` → token salah / di-revoke di BotFather
- `permission denied` → jalanin `sudo chown -R bongbot:bongbot /opt/bongbot`

### Bot crash terus
```bash
sudo systemctl status bongbot
# Cek "Restart=" — kalau "limit hit" → ada bug, hentikan dulu
sudo systemctl stop bongbot
# Cek log:
sudo journalctl -u bongbot -n 200 --no-pager
```

### CF API error
Kalau di log muncul `CF error 6003: Invalid request headers`:
- Pastikan pake **Global API Key** (bukan API Token)
- Cek di Settings bot → 🔧 Test Koneksi

### Memory creep
Kalau RAM > 500 MB, systemd auto-restart bot (`MemoryMax=512M`). Cek dengan:
```bash
sudo systemctl status bongbot
# Lihat "Memory: XXX MB"
```

---

## 🌍 Multi-bot (deploy 2+ bot di 1 server)

Kalau mau jalanin 2 bot terpisah di server yang sama:

```bash
# Copy service file dengan nama baru
sudo cp /etc/systemd/system/bongbot.service /etc/systemd/system/bongbot-2.service

# Edit ganti WorkingDirectory & ExecStart
sudo nano /etc/systemd/system/bongbot-2.service
# Ganti: /opt/bongbot → /opt/bongbot-2

# Install instance kedua
sudo cp -r /opt/bongbot /opt/bongbot-2
sudo chown -R bongbot:bongbot /opt/bongbot-2
sudo nano /opt/bongbot-2/.env  # ganti BOT_TOKEN ke token bot lain

sudo systemctl daemon-reload
sudo systemctl enable --now bongbot-2
```

---

## 📞 Need Help?

- Cek log: `sudo journalctl -u bongbot -f`
- File issue di [GitHub repo](https://github.com/baxiumren/auto-rot)

## 🔗 KLIKCEPAT Integration (Optional)

Bot bisa integrate dengan klikcepat.com untuk:
- CRUD link & project via Telegram bot
- Auto-swap `location_url` ketika domain target kena nawala (parallel ke CF Rotator)

### Setup di Klikcepat (one-time)

**Sebagai master admin di klikcepat:**

1. **Admin Panel → Settings** → ✅ Enable users API system
2. **Admin Panel → Plans** → edit plan untuk normal admin → ✅ API access

**Sebagai normal admin:**

3. **Account → API** → klik **Generate API Key**
4. Copy API key (format: panjang random string)

### Setup di Bot

**Opsi A — via .env (restart bot setelah edit):**

```env
KLIKCEPAT_BASE_URL=https://klikcepat.com
KLIKCEPAT_API_KEY=<paste-api-key-disini>
```

Lalu restart bot atau jalanin `sudo bash /opt/bongbot/deploy/install.sh`.

**Opsi B — via bot menu (no restart):**

1. DM bot → 🔧 Settings → 🔗 Klikcepat Settings
2. Set Base URL → ketik `https://klikcepat.com`
3. Set API Key → paste API key (pesan auto-deleted untuk security)
4. ✅ Test Koneksi → harus respon "Test BERHASIL"

### Auto-Swap Setup

Setelah credentials OK:

1. DM bot → 🔄 Auto Rotator → ➕ Setup Rotator
2. Pilih **🔗 KLIKCEPAT** (instead of CF Redirect)
3. Pick klikcepat link (yang mau di-auto-swap)
4. Pick pool label (dari Monitor domains)
5. Kasih label → save → done

Sekarang ketika MonitorScanner detect domain target kena nawala:
- Bot scan CF rules → swap (existing)
- Bot scan klikcepat rotators → swap location_url ke domain backup dari pool

Notif group:
```
⚡ KLIKCEPAT AUTO-SWAP
🔗 Link: /promo-maha (link)
🚫 Sebelum: maha-supreme.com (BLOCKED)
✅ Sekarang: mahasupreme.xyz
```

### Limitasi Saat Ini

- **Biolink blocks** (button2 inside biolink page) gak bisa di-CRUD via bot — klikcepat platform gak expose API untuknya. Manage via klikcepat dashboard web.
- API hard cap: 1000 link per fetch. Kalau lu punya >1000 link, perlu pagination support (future enhancement).
- Klikcepat rotators belum punya toggle/delete UI di bot (List view aja). Future enhancement.

### Troubleshooting

**"Klikcepat HTTP 401":** API key invalid/expired. Re-generate di klikcepat → set ulang via bot.

**"Klikcepat HTTP 404" saat update link:** Link sudah dihapus di dashboard. Hapus rotator config-nya manual via filesystem (data/klikcepat_rotators.json) atau via future delete UI.

**Test Koneksi sukses tapi auto-swap gak jalan:** Pastikan rotator config Active=true. Cek log bot saat ada blocked detected — harus muncul `[KLIKCEPAT-SWAP] ...`.
