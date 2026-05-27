<div align="center">

```
██████╗  ██████╗ ███╗   ██╗ ██████╗     ██████╗  ██████╗ ████████╗
██╔══██╗██╔═══██╗████╗  ██║██╔════╝     ██╔══██╗██╔═══██╗╚══██╔══╝
██████╔╝██║   ██║██╔██╗ ██║██║  ███╗    ██████╔╝██║   ██║   ██║
██╔══██╗██║   ██║██║╚██╗██║██║   ██║    ██╔══██╗██║   ██║   ██║
██████╔╝╚██████╔╝██║ ╚████║╚██████╔╝    ██████╔╝╚██████╔╝   ██║
╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝     ╚═════╝  ╚═════╝    ╚═╝
```

# 🎰🚀 BONG BOT — ALL IN ONE FITUR BOT PALING GACOR! 🔥✨

**Telegram bot serba bisa buat para affiliate Indonesia.**
Auto-rotate domain Cloudflare + Klikcepat saat kena nawala — biar duit gak putus! 💸

[![Version](https://img.shields.io/badge/version-v1.0-blue)](https://github.com/baxiumren/auto-rot/releases)
[![Go](https://img.shields.io/badge/go-1.21+-cyan)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](#-license)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows-orange)]()

</div>

---

## 📑 Table of Contents

1. [Kenapa Bong Bot?](#-kenapa-bong-bot)
2. [Fitur Lengkap](#-fitur-lengkap)
3. [Cara Kerja](#-cara-kerja-auto-swap)
4. [Quick Start (Deploy)](#-quick-start)
5. [Cara Pakai](#-cara-pakai)
6. [Konfigurasi](#-konfigurasi)
7. [Update Bot](#-update-bot)
8. [Troubleshooting](#-troubleshooting)
9. [Contributing](#-contributing--license)

---

## 💎 Kenapa Bong Bot?

🇮🇩 **Khusus pejuang nawala Indonesia.** Bot ini bantu lu:

| ✅ | Yang Bot Bisa |
|---|---|
| 👀 | Pantau domain 24/7 dari serbuan Kominfo (multi-source check) |
| ⚡ | Auto-swap target redirect Cloudflare detik itu juga pas blocked |
| 🔗 | Auto-swap link biolink & shortlink di klikcepat |
| 📱 | Manage semua dari Telegram — gak perlu buka dashboard tiap saat |
| 🔒 | Private bot — cuma admin lu yang bisa pakai |

**Bottom line:** Domain mati = duit hilang. Bot ini jaga supaya never happen. 💪

---

## 🎯 Fitur Lengkap

### 1️⃣ 📡 Monitor — Pantau Domain Anti-Nawala

- ✅ Scan ke **TrustPositif Kominfo** (sumber resmi)
- ✅ Multi-source check: **Kominfo + TrustPositif API + NawalaCheck** (triple-check)
- ✅ **Auto rotating batch** — handle 100+ domain tanpa rate-limit
- ✅ Sticky cache — domain blocked gak di-check ulang (hemat API)
- ✅ Force-block manual untuk testing
- ✅ Spam alert continuous sampai user hapus dari list

### 2️⃣ ⚙️ CF Redirect — Cloudflare Auto-Manage

- ✅ Auto-discovery rule pakai nama domain (no Zone ID manual)
- ✅ Support **V1 Page Rules** & **V2 Redirect Rules**
- ✅ Add domain baru ke Cloudflare lengkap dengan DNS placeholder
- ✅ Bulk change URL (banyak rule sekaligus)
- ✅ Preserve path & query string saat swap

### 3️⃣ 🔄 Auto Rotator — Unified Hub (CF + Klikcepat)

- ✅ Setup Rotator — pilih tipe (CF / Klikcepat) → pick rule → pool
- ✅ **Bulk Setup** — checkbox picker, assign banyak rule ke 1 pool
- ✅ List Rotator grouped view (CF + Klikcepat dengan totals)
- ✅ Swap history log
- ✅ Reactive auto-swap < 1 menit dari blocked detection

### 4️⃣ 🔗 KLIKCEPAT — Bio Link & Short URL CRUD

- ✅ Full CRUD link (biolink + shortlink + vcard + event)
- ✅ Project management (grouping link)
- ✅ Filter view by type (Biolink only / Shortlink only)
- ✅ Auto-detect custom domain (klikcepat.com, klikcepat.vip, custom)
- ✅ Auto-swap location_url ketika target blocked
- ✅ Integrasi dengan Auto Rotator unified

### 5️⃣ 🔧 Settings — Hub Picker

- ✅ Pilih platform (CF / Klikcepat)
- ✅ Set credentials via bot (no edit .env)
- ✅ Test koneksi per platform
- ✅ Auto-default klikcepat.com
- ✅ Credentials encrypted di `data/credentials.json`

---

## 🎬 Cara Kerja Auto-Swap

```
                       🤖 BOT 24/7 ONLINE
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
       📡 Monitor       ⚙️ CF Scanner     🔗 Klikcepat Sync
       detect BLOCKED   match current      match link target
           │                  │                  │
           └────────────┬─────┴──────────────────┘
                        ▼
            🔎 Cari Rotator config
                        │
           ┌────────────┴────────────┐
           ▼                         ▼
       ⚙️ CF Rotator             🔗 KLC Rotator
       Update CF rule URL        Update link.location_url
       via Cloudflare API        via Klikcepat API
           │                         │
           └────────────┬────────────┘
                        ▼
           ⚡ AUTO-SWAP — < 1 menit total
                        │
                        ▼
       📨 Notif group: "DOMAIN SWAPPED"
       📜 Log ke history.json
       ✅ Iklan/redirect tetap jalan, duit tetep ngalir 💰
```

---

## 🚀 Quick Start

### 📦 One-Liner Install (Ubuntu 22/24)

```bash
curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
```

### ⚙️ Setup .env

```bash
sudo nano /opt/bongbot/.env
```

**Minimum required:**
```env
BOT_TOKEN=token_dari_botfather
ALLOWED_CHAT_ID=-1003932797479
ADMIN_IDS=7626641150,881552619
CONTACT_USERNAME=hokisetahun
```

**Optional (bisa di-set via bot menu juga):**
```env
CF_EMAIL=
CF_API_KEY=
KLIKCEPAT_BASE_URL=https://klikcepat.com
KLIKCEPAT_API_KEY=
TRUSTPOSITIF_API_KEY=
NAWALACHECK_API_KEY=
CHECK_INTERVAL=45s
```

### ▶️ Start Bot

```bash
sudo systemctl start bongbot
sudo journalctl -u bongbot -f
```

📖 **Detail deploy:** [`deploy/DEPLOY.md`](deploy/DEPLOY.md)

---

## 🎮 Cara Pakai

### 🆕 First-time Setup

1. **Login bot via DM** (admin) → `/start`
2. `🔧 Settings → ⚙️ Cloudflare` → set email + API key
3. `🔧 Settings → 🔗 Klikcepat` → set API key
4. **Test Koneksi** keduanya → harus ✅

### 📅 Daily Operations

```
1. 📡 Monitor → ➕ Add Domain        → tambah domain ke pantau
2. ⚙️ CF Redirect → ➕ Add Rule      → register CF rule
3. 🔄 Auto Rotator → ➕ Setup Rotator → link rule ke pool
4. ✅ Done — bot jaga 24/7
```

### 📦 Bulk Setup (banyak rule sekaligus)

```
1. 🔄 Auto Rotator → 📦 Bulk Setup → pilih tipe (CF / Klikcepat)
2. ✓ Checkbox pilih rule/link → 📂 Pick pool → 💾 Save batch
```

---

## 🔧 Konfigurasi

### 🔑 Variabel Environment

| Variable | Required | Keterangan |
|----------|----------|-----------|
| `BOT_TOKEN` | ✅ Yes | Token dari [@BotFather](https://t.me/BotFather) |
| `ALLOWED_CHAT_ID` | ✅ Yes | Chat ID grup untuk alerts |
| `ADMIN_IDS` | ✅ Yes | User IDs admin (comma-separated) |
| `CONTACT_USERNAME` | ✅ Yes | Handle Telegram owner (untuk non-admin reject) |
| `CF_EMAIL` | ⚪ Optional | Email Cloudflare (bisa set via bot) |
| `CF_API_KEY` | ⚪ Optional | Global API Key Cloudflare |
| `KLIKCEPAT_BASE_URL` | ⚪ Optional | Default `https://klikcepat.com` |
| `KLIKCEPAT_API_KEY` | ⚪ Optional | API key normal admin klikcepat |
| `TRUSTPOSITIF_API_KEY` | ⚪ Optional | Premium TrustPositif API key |
| `NAWALACHECK_API_KEY` | ⚪ Optional | NawalaCheck.com API key |
| `CHECK_INTERVAL` | ⚪ Optional | Default `45s` (min 10s) |

### 🔐 Mode Akses

| Siapa | Akses |
|-------|-------|
| Admin di `ADMIN_IDS` via DM | ✅ Full CRUD |
| Member grup `ALLOWED_CHAT_ID` | ✅ Read-only (Status, List) |
| Non-admin DM | 🔒 Reject + contact button |
| Random DM / grup lain | ⛔ Ignored |

### 📂 Struktur Data

```
data/
├── domains.json              ← Monitor list (per-label grouping)
├── cf_rules.json             ← CF redirect rules
├── rotator_rules.json        ← CF Auto Rotator config
├── klikcepat_rotators.json   ← Klikcepat Auto Rotator config
├── sticky_blocked.json       ← Sticky cache
├── force_blocked.json        ← Force-block list
├── history.json              ← Swap history log
└── credentials.json          ← CF + Klikcepat creds (chmod 600)
```

> 💾 **Backup folder `data/` sebelum update.** Install.sh sengaja gak nyentuh `data/`.

---

## 🔄 Update Bot

### Standard Update (paling sering)

```bash
sudo bash /opt/bongbot/deploy/install.sh
```

✨ Smart features:
- 🔄 Auto git pull
- 🔨 Auto rebuild binary
- ⚡ Auto-restart kalau service running
- 📋 Tail 15 log terakhir untuk verify
- 🎰 **Banner "BONG BOT READY"** pas selesai

### Safe Update (always works)

Pakai curl method kalau install.sh sendiri ke-update:

```bash
curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
```

---

## 🐛 Troubleshooting

<details>
<summary><b>Bot gak respond ke /start</b></summary>

```bash
sudo journalctl -u bongbot -n 50 --no-pager
```

**Cek error spesifik:**
- `BOT_TOKEN tidak di-set` → cek `.env`
- `Bot error: Unauthorized` → token salah / di-revoke di BotFather
- `permission denied` → `sudo chown -R bongbot:bongbot /opt/bongbot`
</details>

<details>
<summary><b>Klikcepat "Credentials belum lengkap"</b></summary>

Set API Key via `🔧 Settings → 🔗 Klikcepat → 🔑 Set API Key`.
Base URL auto-default ke `https://klikcepat.com`, gak perlu set manual.
</details>

<details>
<summary><b>Setup Rotator gak berfungsi</b></summary>

Deploy binary terbaru:
```bash
sudo bash /opt/bongbot/deploy/install.sh
```
Mungkin masih pakai binary lama dari sebelum FlexInt fix.
</details>

<details>
<summary><b>Markdown parse error di Telegram</b></summary>

Underscore (`_`) di text dynamic ke-interpret sebagai italic.
Reproduce: kalau pesan dynamic punya kata dengan `_`, escape via backticks.
</details>

<details>
<summary><b>Zombie bot process setelah reset</b></summary>

```bash
sudo pkill -9 -f bot-linux-amd64
sudo systemctl restart bongbot
```
Penyebab: `rm -rf /opt/bongbot` tanpa stop service dulu → process keep running pakai deleted binary.
</details>

---

## 🎁 Bonus Features

- 🌍 **Auto rotating batch** — handle 100+ domain anti rate-limit
- 🔁 **Triple-source check** — Kominfo + TrustPositif + NawalaCheck
- 📊 **Pagination** — list panjang gak masalah
- 🛡 **Defensive validation** — guard against stale buttons & race conditions
- 🔧 **Smart install.sh** — auto-restart + BONG BOT READY banner
- 📦 **Multi-file structure** — `bot/klikcepat_*.go` per feature
- ✨ **Typing indicators** — feedback langsung di Telegram

---

## 💻 Development Lokal

### Requirements
- Go 1.21+
- Bot Token dari [@BotFather](https://t.me/BotFather)

### Setup
```bash
git clone https://github.com/baxiumren/auto-rot.git
cd auto-rot
cp .env.example .env
nano .env
go build -o bot .
./bot
```

**Windows:**
```cmd
go build -o bot.exe .
bot.exe
```

---

## 🤝 Contributing & License

### 🔒 Private Bot

This is a private bot. Untuk akses, contact: **[@hokisetahun](https://t.me/hokisetahun)** di Telegram.

### 📜 License

MIT — pakai bebas, modifikasi bebas, tapi gak ada warranty.
Kalau crash di production = lu yang tanggung jawab. 🫡

### ⭐ Tech Stack

- **Go 1.21+** (binary kecil, cepat)
- **gopkg.in/telebot.v3** (Telegram bot framework)
- **Cloudflare API v4**
- **Klikcepat platform REST API**
- **TrustPositif Kominfo Rest_server**
- **systemd** (Linux service management)

### 🙏 Special Thanks

- 🇮🇩 Pejuang affiliate Indonesia yang gak nyerah sama nawala
- 🌐 Open source community

---

<div align="center">

```
   ╔═══════════════════════════════════════════════╗
   ║   🎰  BONG BOT — Sekali Setup, Anti Mati 🚀   ║
   ╚═══════════════════════════════════════════════╝
```

**[⬆ Back to top](#-table-of-contents)**

</div>
