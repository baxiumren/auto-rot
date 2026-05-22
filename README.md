# BongBot

Bot Telegram all-in-one untuk monitor nawala, kelola Cloudflare redirect, dan auto-rotate domain ketika kena blokir Kominfo.

## Fitur

| Menu | Fitur |
|------|-------|
| 📡 Monitor | Pantau domain nawala 24/7, cek manual, kelola list domain per kategori |
| ⚙️ CF Redirect | Kelola Cloudflare redirect rules (v1 & v2), ganti URL redirect |
| 🔄 Auto Rotator | Otomatis ganti domain di CF ketika kena nawala, pool domain dari Monitor |

### Cara Kerja Auto Rotator
```
Monitor deteksi domain kena nawala
        ↓
Ambil domain berikutnya dari pool (label Monitor)
        ↓
Update CF redirect rule otomatis
        ↓
Notifikasi Telegram
```

## Instalasi

### Requirement
- Windows 64-bit
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
- **Add Rule** — daftarkan CF redirect rule (butuh Zone ID, Ruleset ID, Rule ID)
- **List Rules** — lihat semua rule yang terdaftar
- **Ganti URL** — ubah URL tujuan redirect langsung dari bot
- **Hapus Rule** — hapus rule dari daftar

### 🔄 Auto Rotator
- **Setup Rotator** — hubungkan CF Rule + pool domain dari Monitor
- **List Rotator** — lihat semua rotator aktif/pause
- **Pause / Resume** — hentikan/aktifkan rotator tertentu
- **Hapus** — hapus rotator

## Cara Ambil CF IDs

### Zone ID
```
Cloudflare → pilih domain → Overview → scroll kanan bawah → Zone ID
```

### Ruleset ID & Rule ID (Redirect Rules v1)
```
Cloudflare → domain → Rules → Redirect Rules → klik rule → lihat URL browser
.../rulesets/{RULESET_ID}/rules/{RULE_ID}
```

### Page Rule ID (Page Rules v2)
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
