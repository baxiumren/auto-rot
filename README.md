# 🎰🚀 BONG BOT — ALL IN ONE FITUR BOT PALING GACOR! 🔥✨

> 🤖 *Telegram bot serba bisa buat para affiliate, agen, & domain hustler. Auto-rotate domain Cloudflare + Klikcepat saat kena nawala — biar iklan/redirect lu gak mati dan duitnya tetap ngalir!* 💸

```
   ___  ___  _  _  ___    ___  ___  _____
  | _ )/ _ \| \| |/ __|  | _ )/ _ \|_   _|
  | _ \ (_) | .` | (_ |  | _ \ (_) | | |
  |___/\___/|_|\_|\___|  |___/\___/  |_|
       AUTO-ROTATOR · MULTI-PLATFORM
```

---

## 🌟 Kenapa Pakai Bong Bot?

🇮🇩 **Khusus pejuang nawala Indonesia.** Bot ini bantu lu:
- 👀 Pantau domain 24/7 dari serbuan Kominfo (multi-source check)
- ⚡ Auto-swap target redirect Cloudflare detik itu juga pas blocked
- 🔗 Auto-swap link biolink & shortlink di klikcepat juga
- 📱 Manage semua dari Telegram — gak perlu buka dashboard CF/Klikcepat tiap saat
- 🔒 Private bot — cuma admin lu yang bisa pakai

**Bottom line:** Domain mati = duit hilang. Bot ini jaga supaya never happen. 💪

---

## 🎯 Fitur Lengkap (5 Menu Utama)

### 📡 1. Monitor — Pantau Domain Anti-Nawala
- ✅ Scan domain ke **TrustPositif Kominfo** (sumber resmi)
- ✅ Multi-source check: **Kominfo + TrustPositif API + NawalaCheck** (triple-check biar yakin)
- ✅ **Auto rotating batch** — handle 100+ domain tanpa rate-limit issue
- ✅ Sticky cache — domain udah blocked gak di-check ulang (hemat API)
- ✅ Force-block manual — paksa anggap blocked untuk testing
- ✅ Spam alert continuous sampai user hapus dari list

### ⚙️ 2. CF Redirect — Cloudflare Auto-Manage
- ✅ Auto-discovery rule pakai nama domain (gak perlu Zone ID/Rule ID manual)
- ✅ Support **V1 Page Rules** & **V2 Redirect Rules** (Rulesets API)
- ✅ Add domain baru ke Cloudflare lengkap dengan DNS placeholder
- ✅ Bulk change URL (banyak rule sekaligus)
- ✅ Preserve path & query string saat swap (`?ref=`, UTM, dll tetap utuh)

### 🔄 3. Auto Rotator — UNIFIED HUB (CF + Klikcepat!)
- ✅ **Setup Rotator** — pilih tipe (CF / Klikcepat) → pick rule/link → pool → save
- ✅ **Bulk Setup** — assign banyak CF rule / klikcepat link ke 1 pool dengan checkbox picker
- ✅ **List Rotator** — view grouped (CF + Klikcepat) dengan total counts
- ✅ Swap history log — track semua auto-rotate yang pernah jalan
- ✅ Pause / Resume / Force trigger per rotator
- ✅ Reactive auto-swap < 1 menit dari deteksi blocked

### 🔗 4. KLIKCEPAT — Bio Link & Short URL CRUD (FITUR BARU!)
- ✅ Full CRUD link (biolink + shortlink + vcard + event) via bot
- ✅ Project management (create/edit/delete project untuk grouping)
- ✅ Filter view by type (Biolink only / Shortlink only) — gak campur aduk
- ✅ Auto-detect custom domain (klikcepat.com, klikcepat.vip, atau domain custom)
- ✅ Auto-swap location_url ketika target domain blocked — biolink button tetap kerja
- ✅ Integrasi dengan Auto Rotator unified

### 🔧 5. Settings — Hub Picker
- ✅ Pilih platform yang mau di-configure (CF / Klikcepat)
- ✅ Set credentials via bot (no need edit .env)
- ✅ Test koneksi — verify API responsive
- ✅ Auto-default klikcepat.com (gak perlu set Base URL kalau pakai default)
- ✅ Credentials di-encrypt local di `data/credentials.json` (permission 0600)

---

## 🎬 Cara Kerja Auto-Swap (Flow Magic)

```
                    🤖 BOT 24/7 ONLINE
                          │
       ┌──────────────────┼──────────────────┐
       │                  │                  │
   📡 Monitor       ⚙️ CF Scanner     🔗 Klikcepat Sync
   detect BLOCKED   match current URL   match link target
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

## 🚀 Quick Start (Deploy ke VPS Ubuntu)

**One-liner install:**
```bash
curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
```

Habis install, edit `.env`:
```bash
sudo nano /opt/bongbot/.env
```

Isi minimum:
```env
BOT_TOKEN=token_dari_botfather
ALLOWED_CHAT_ID=-1003932797479
ADMIN_IDS=7626641150,881552619
CONTACT_USERNAME=hokisetahun

# Optional — bisa di-set lewat bot menu juga
CF_EMAIL=
CF_API_KEY=
KLIKCEPAT_BASE_URL=https://klikcepat.com
KLIKCEPAT_API_KEY=
TRUSTPOSITIF_API_KEY=
NAWALACHECK_API_KEY=
CHECK_INTERVAL=45s
```

Start:
```bash
sudo systemctl start bongbot
sudo journalctl -u bongbot -f
```

📖 Panduan lengkap: [`deploy/DEPLOY.md`](deploy/DEPLOY.md)

---

## 🎮 Cara Pakai

### First-time Setup
1. Login bot via DM (lu sebagai admin) → `/start`
2. `🔧 Settings → ⚙️ Cloudflare` → set email + API key
3. `🔧 Settings → 🔗 Klikcepat` → set API key
4. Test Koneksi keduanya → harus ✅

### Daily Operations
1. `📡 Monitor → ➕ Add Domain` → tambah domain ke list pantau
2. `⚙️ CF Redirect → ➕ Add Rule` → register CF rule
3. `🔄 Auto Rotator → ➕ Setup Rotator → ⚙️ CF` → link rule ke pool
4. Bot jaga 24/7 — selesai. 💯

### Bulk Setup (banyak rule sekaligus)
1. `🔄 Auto Rotator → 📦 Bulk Setup` → pilih tipe
2. Checkbox banyak rule/link → assign ke 1 pool → save batch

---

## 💻 Development Lokal (Windows / Linux / macOS)

### Requirement
- Go 1.21+
- Bot Token dari [@BotFather](https://t.me/BotFather)
- Cloudflare Global API Key (opsional, bisa di-set via bot)

### Build & Run
```bash
git clone https://github.com/baxiumren/auto-rot.git
cd auto-rot
cp .env.example .env
nano .env  # isi BOT_TOKEN, ALLOWED_CHAT_ID, ADMIN_IDS
go build -o bot .
./bot
```

Windows:
```cmd
go build -o bot.exe .
bot.exe
```

---

## 🔐 Akses Bot

| Siapa | Akses |
|-------|-------|
| Admin di `ADMIN_IDS` via DM | ✅ Full menu — CRUD semua fitur |
| Member grup `ALLOWED_CHAT_ID` | ✅ Read-only — Status + List Domain + List CF |
| Non-admin DM | 🔒 Reject dengan contact button ke owner |
| Random DM (orang asing) | ⛔ Diem aja |
| Grup lain (non-allowlist) | ⛔ Diem aja |

### Group Mode (Read-Only)
Di group, bot cuma jadi alert broadcaster + view-only:
- 🚨 Notif blocked + tombol [🗑 Hapus dari Monitor] (admin-only action)
- ⚡ Notif auto-swap (CF & Klikcepat)
- 4 tombol read-only: Status / List Domain / List CF / Setup di DM

### DM Mode (Full Power)
Admin DM bot → semua fitur CRUD + setup + management.

---

## 📂 Struktur Data

Semua data disimpan lokal di folder `data/`:

```
data/
├── domains.json              ← Monitor list (per-label grouping)
├── cf_rules.json             ← CF redirect rules
├── rotator_rules.json        ← CF Auto Rotator config
├── klikcepat_rotators.json   ← Klikcepat Auto Rotator config
├── sticky_blocked.json       ← Sticky cache (blocked domains)
├── force_blocked.json        ← Force-block list (manual)
├── history.json              ← Swap history log (max 200 entries)
└── credentials.json          ← Encrypted CF + Klikcepat creds (chmod 600)
```

> 💾 **Backup folder `data/` sebelum update/reinstall.** Install.sh sengaja gak nyentuh `data/`.

---

## 🛠 Update Bot ke Versi Terbaru

```bash
sudo bash /opt/bongbot/deploy/install.sh
```

Install.sh smart auto-restart:
- ✅ Git pull main branch
- ✅ Rebuild binary
- ✅ Detect service running → auto-restart pakai binary baru
- ✅ Tail 15 log terakhir untuk verify

Atau lewat curl direct (selalu fresh install.sh):
```bash
curl -fsSL https://raw.githubusercontent.com/baxiumren/auto-rot/main/deploy/install.sh | sudo bash
```

---

## 🎁 Bonus Features

- 🌍 **Auto rotating batch** — kalau total domain >100, bot bagi chunks 100 per tick (anti rate-limit Kominfo)
- 🔁 **Triple-source check** — TrustPositif + Kominfo direct + NawalaCheck (optional API keys)
- 📊 **Multi-page list pagination** — link/domain banyak gak masalah, ada Prev/Next
- 🎨 **Custom contact in non-admin reject** — `CONTACT_USERNAME=hokisetahun` di .env
- 🛡 **Defensive validation** — guard against session corruption, stale buttons, race conditions
- 🔧 **Smart install.sh** — auto-restart kalau service udah running
- 📦 **Multi-file structure** — bot/klikcepat_*.go per feature (gampang debug)
- ✨ **Typing indicators** — feedback langsung pas user kirim text

---

## 🐛 Troubleshooting

### Bot gak respond
```bash
sudo journalctl -u bongbot -n 50 --no-pager
```
Cek error spesifik:
- `BOT_TOKEN tidak di-set` → cek `.env`
- `Bot error: Unauthorized` → token salah/di-revoke
- `permission denied` → `sudo chown -R bongbot:bongbot /opt/bongbot`

### Markdown parse error
Underscore (`_`) di text dynamic ke-interpret sebagai italic. Wrap pakai backticks (`` ` ``) atau escape.

### Klikcepat API "Credentials belum lengkap"
Set API Key dulu via `🔧 Settings → 🔗 Klikcepat → 🔑 Set API Key`. Base URL auto-default ke `https://klikcepat.com`.

### Setup Rotator gak berfungsi
Re-deploy binary terbaru — `sudo bash /opt/bongbot/deploy/install.sh`. Mungkin masih pakai binary lama.

---

## 🤝 Contributing

This is a private bot. Untuk akses, contact: **@hokisetahun** di Telegram.

---

## 📜 License

MIT — pakai bebas, modifikasi bebas, tapi gak ada warranty. Kalau crash di production = lu yang tanggung jawab. 🫡

---

## ⭐ Credits

Built with ❤️ + ☕ + 🤖 Claude AI.

**Tech stack:**
- Go 1.21+ (binary kecil, cepet)
- `gopkg.in/telebot.v3` (Telegram bot framework)
- Cloudflare API v4
- Klikcepat platform REST API
- TrustPositif Kominfo Rest_server endpoint
- systemd (Linux service management)

**Special thanks:**
- Pejuang affiliate Indonesia yang gak nyerah sama nawala 🇮🇩
- Open source community

---

🎰 **BONG BOT — Sekali Setup, Anti Mati Selamanya** 🚀
