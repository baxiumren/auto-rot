# 📜 CHANGELOG

Semua perubahan significant di BONG BOT. Format mengikuti [Keep a Changelog](https://keepachangelog.com/) + [Semantic Versioning](https://semver.org/).

---

## 🎰 v1.0.0 — "ALL IN ONE" Release (2026-05-27)

**Production-ready milestone.** Bot udah feature-complete untuk auto-rotate CF + Klikcepat dengan multi-source nawala check.

### ✨ Major Features
- **🔗 KLIKCEPAT Integration** — Full CRUD link & project via Telegram bot
  - Add Link (5-step wizard: type → title → slug → location → project)
  - List Link dengan filter type (Biolink / Shortlink), paginated
  - Edit Link (title / slug / location_url)
  - Delete Link dengan confirmation dialog
  - Project CRUD (Add / List / Edit / Delete dengan color hex)
  - Custom domain detection (klikcepat.com, klikcepat.vip, atau custom)
  - Full URL display dengan auto-detect host
- **🔄 Unified Auto Rotator** — Hub picker untuk CF + Klikcepat
  - Setup Rotator: pick type dulu (CF / Klikcepat) baru detail
  - Bulk Setup: checkbox picker per type, assign banyak ke 1 pool
  - List Rotator: grouped view (CF section + Klikcepat section + totals)
- **⚡ Klikcepat Auto-Swap** — Monitor Scanner sekarang scan klikcepat rotators
  - Detect blocked → match link.location_url → swap via API
  - Notif group dengan "⚡ KLIKCEPAT AUTO-SWAP" template
  - History log entry untuk audit trail
- **🔧 Settings Hub** — Refactor jadi 2-button picker
  - Settings → pick CF / Klikcepat → masuk sub-menu
  - Status snapshot kedua platform di hub
  - Klikcepat credentials via bot menu (no edit .env perlu)
  - Test Koneksi per platform

### 🛡 UX Improvements
- **Typing indicators** — feedback langsung saat user input text
- **Markdown defensive escaping** — backticks untuk `location_url`, no italic di dynamic text
- **Custom CONTACT_USERNAME** — non-admin reject template configurable
- **5-button main menu** — Monitor / CF / Rotator / Klikcepat / Settings
- **Group read-only mode** — alert + view-only buttons, no wizard pollution
- **DM full power** — semua CRUD + wizard di DM admin

### 🐛 Bug Fixes
- **FlexInt type** — handle PHP backend yang return numeric fields sebagai strings
- **Auto-default Klikcepat Base URL** — gak perlu set manual kalau pakai klikcepat.com
- **CF Rule backfill** — auto-fetch domain name untuk rule lama yang field-nya kosong
- **Stale inline button defense** — validate session + domain before doAddDomain
- **Bash glob sudo permission** — install.sh edge case di reset workflow

### 🔧 Infrastructure
- **Install.sh smart-restart** — auto-detect service running, no zombie
- **Multi-file bot structure** — bot/klikcepat_*.go per feature
- **Backfill helper** — auto-detect old CF rules tanpa domain field, fetch via API
- **Spec + Plan docs** — di `docs/superpowers/` untuk traceability

---

## 🏗 v0.3.0 — Group Mode + Rotating Batch (2026-05-26)

### Added
- **Group + DM dual mode** — group untuk alert, DM untuk wizard
- **Auto rotating batch** — total >100 domain bagi chunks 100 per tick
- **Smart-nudge NO_SESSION** — soft hint kalau user kirim text tanpa wizard active
- **Group action buttons** — `🗑 Hapus dari Monitor` di setiap blocked alert
- **Non-admin DM reject** — template "private bot, contact @owner" dengan URL button

### Changed
- isAllowed() signature — return (allowed, isDM) untuk handler branching
- handleStart split antara DM vs Group flow
- Status menu nampilin chunk progress (`[chunk 2/3]`)

---

## 🛡 v0.2.0 — Multi-Source Nawala Check (2026-05-26)

### Added
- **Triple-source check** — Kominfo + TrustPositif API + NawalaCheck
- **Source picker untuk manual check** — pilih source via inline button
- **Conditional API key buttons** — hide button kalau API key gak di-set di .env

### Fixed
- HTTP 404 rate-limit dari parallel requests (turun ke 3 concurrent, 200ms delay)
- Sticky-block "treated as SAFE on ERROR" bug

---

## 🎬 v0.1.0 — Initial Production Release (earlier)

### Added
- Monitor Scanner (24/7 background)
- CF Redirect Rules CRUD (V1 Page Rules + V2 Redirect Rules)
- Auto Rotator (CF only initially)
- Bulk Setup CF
- New Domain Registration (CF + DNS)
- Bulk Change URL
- Force Block / Sticky Block management
- Swap History log
- Health Dashboard
- Global Search
- Settings (CF email + API key)
- systemd deployment

---

## 🗓 Versioning Strategy

Kita pakai [Semantic Versioning](https://semver.org/):

- **MAJOR** (X.0.0) — breaking changes (perlu migration manual)
- **MINOR** (1.X.0) — fitur baru (backward compatible)
- **PATCH** (1.0.X) — bug fixes & small improvements

## 🔗 Git Tags

```bash
# List semua versions
git tag -l

# Checkout version tertentu
git checkout v1.0.0

# Lihat diff antar version
git diff v0.3.0..v1.0.0
```
