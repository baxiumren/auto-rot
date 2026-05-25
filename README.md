# BongBot — Auto Domain Rotator

Bot Telegram all-in-one untuk monitor nawala (TrustPositif Kominfo), kelola Cloudflare redirect rules, dan **otomatis ganti domain** di CF ketika kena blokir.

## ✨ Fitur Utama

| Menu | Fitur |
|------|-------|
| 📡 **Monitor** | Pantau domain 24/7 ke TrustPositif, multi-round check (2/3 ronde), sticky cache, force-block manual, scanner reaktif paralel |
| ⚙️ **CF Redirect** | Auto-discovery domain (V1 Page Rules + V2 Redirect Rules), register domain baru ke CF + DNS, single & bulk change URL, preserve path/query |
| 🔄 **Auto Rotator** | Setup single & bulk (banyak CF rule → 1 pool), pause/resume/force-rotate, swap history log |
| 🔧 **Settings** | CF email + Global API Key via bot, test connection, hot-reload |
| 🩺 **Health Dashboard** | Status real-time semua service di reply keyboard |
| 🔍 **Global Search** | Cari domain di Monitor / CF / Rotator / Sticky / Force sekaligus |

### 🎯 Cara Kerja Auto Swap
```
Monitor Scanner (24/7)
        ↓ detect domain1.com BLOCKED
        ↓
🚨 Spam notif berulang sampai user hapus dari Monitor
        +
🔍 Cek SEMUA CF Rule → mana current URL = domain1.com?
        ↓ match: maha66.id
🔎 Baca Rotator config: pool = STOCK-MS
📂 Pick first SAFE dari STOCK-MS → domain2.com
⚡ Update CF: https://domain1.com/?ref=x → https://domain2.com/?ref=x
              (preserve path & query)
📜 Log ke swap_history.json
📨 Notif: "AUTO-SWAP via MONITOR"
```

## 🚀 Deploy ke VPS (Ubuntu 22/24)

**One-shot install:**
```bash
curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
```

Lalu edit `.env` & start:
```bash
sudo nano /opt/bongbot/.env
sudo systemctl start bongbot
sudo journalctl -u bongbot -f
```

📖 **Panduan lengkap deploy + troubleshooting:** [`deploy/DEPLOY.md`](deploy/DEPLOY.md)

## 💻 Development Lokal (Windows)

### Requirement
- Windows 64-bit / Linux / macOS
- Go 1.21+ (cek: `go version`)
- Bot Token dari [@BotFather](https://t.me/BotFather)
- Cloudflare Global API Key

### Setup

**1. Clone / download project**
```
D:\coding\auto-change\bongbot\
```

**2. Buat file `.env`**
```bash
cp .env.example .env
```

Isi `.env`:
```env
BOT_TOKEN=token_dari_botfather
ALLOWED_CHAT_ID=-1001234567890
ADMIN_IDS=123456789

CF_EMAIL=email@example.com
CF_API_KEY=global_api_key_cloudflare

CHECK_INTERVAL=45s
```

| Key | Keterangan |
|-----|-----------|
| `BOT_TOKEN` | Token bot dari BotFather |
| `ALLOWED_CHAT_ID` | ID grup Telegram yang boleh pakai bot |
| `ADMIN_IDS` | User ID admin (pisah koma), bisa pakai bot via DM |
| `CF_EMAIL` | Email akun Cloudflare |
| `CF_API_KEY` | Global API Key Cloudflare |
| `CHECK_INTERVAL` | Interval cek nawala (default: `45s`) |

> **Cara ambil `ALLOWED_CHAT_ID`:** Forward pesan dari grup ke [@userinfobot](https://t.me/userinfobot)
>
> **Cara ambil `ADMIN_IDS`:** Kirim `/start` ke [@userinfobot](https://t.me/userinfobot)
>
> **Cara ambil CF Global API Key:** Cloudflare → My Profile → API Tokens → Global API Key

**3. Build**

Double click `build.bat` atau:
```bash
go build -o bot.exe .
```

**4. Jalankan**

Double click `start.bat` atau:
```bash
bot.exe
```

## Penggunaan

Kirim `/start` ke bot di grup yang sudah di-allowlist.

### 📡 Monitor
- **Add Domain** — tambah domain ke list monitoring dengan label/kategori
- **Cek Domain** — cek status nawala manual satu domain
- **Remove Domain** — hapus domain dari list
- **List Domain** — lihat semua domain per kategori
- **Set Interval** — ubah interval cek otomatis (min: 10s)
- **Status Blocked** — lihat domain yang sedang terblokir

### ⚙️ CF Redirect
- **Add Rule** — daftarkan CF redirect rule. **Auto-discovery**: cukup ketik label + nama domain, bot fetch Zone ID & Rule ID otomatis dari Cloudflare API. Support 2 versi:
  - **V2 — Redirect Rules** (Rulesets API, recommended) → endpoint `/zones/{zone}/rulesets/{ruleset}/rules/{rule}`
  - **V1 — Page Rules** (legacy) → endpoint `/zones/{zone}/pagerules/{rule}`
- **List Rules** — lihat semua rule yang terdaftar
- **Ganti URL** — ubah URL tujuan redirect langsung dari bot (works for both V1 & V2)
- **Hapus Rule** — hapus rule dari daftar

### 🔄 Auto Rotator
- **Setup Rotator** — hubungkan CF Rule + pool domain dari Monitor
- **List Rotator** — lihat semua rotator aktif/pause
- **Pause / Resume** — hentikan/aktifkan rotator tertentu
- **Hapus** — hapus rotator

## Cara Ambil CF IDs

> ⚡ **Note:** Sejak versi terbaru bot, kamu *gak perlu ngambil IDs secara manual*. Cukup ketik nama domain di menu **➕ Add Rule** — bot auto-discover Zone ID, Ruleset ID, & Rule ID dari Cloudflare API. Section di bawah hanya buat referensi atau debugging.

### Zone ID (auto-fetched)
```
Cloudflare → pilih domain → Overview → scroll kanan bawah → Zone ID
```

### V2 — Redirect Rules (rekomendasi Cloudflare)
```
Cloudflare → domain → Rules → Redirect Rules → klik rule → lihat URL browser
.../rulesets/{RULESET_ID}/rules/{RULE_ID}
```

### V1 — Page Rules (legacy, masih support)
```
Cloudflare → domain → Rules → Page Rules → Edit → lihat URL browser
.../page-rules/{RULE_ID}
```

## Struktur Data

Semua data disimpan otomatis di folder `data/`:

```
data/
├── domains.json        ← domain list Monitor
├── cf_rules.json       ← CF redirect rules
└── rotator_rules.json  ← Auto Rotator config
```

> Backup folder `data/` sebelum update/reinstall.

## Akses Bot

| Siapa | Akses |
|-------|-------|
| Member grup (`ALLOWED_CHAT_ID`) | ✅ Semua fitur |
| Admin (`ADMIN_IDS`) via DM | ✅ Semua fitur |
| Orang di grup lain | ⛔ Ditolak |
| Random DM | ⛔ Ditolak |

## Update Bot

1. Edit source code
2. Double click `build.bat`
3. Double click `start.bat`

Data di `data/` tidak hilang saat rebuild.
