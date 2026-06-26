# 📜 CHANGELOG

Semua perubahan significant di BONG BOT.

## 🗓 Versioning Scheme

Custom simple scheme (bukan strict SemVer):

```
v1.0 → v1.1 → v1.2 → v1.3 → ... → v1.9 → v2.0 → v2.1 → ...
                                          ↑
                       Naik major pas mau "double digit"
```

- **Minor bump** (`v1.X`): Setiap release significant — fitur baru / refactor / bug fix gede
- **Major bump** (`v2.0`, `v3.0`): Setelah v1.9 (atau emang ada breaking changes yg massive)
- **Patch** (skip): Gak pake third digit — too noisy

---

## 🚀 v1.8 — "JASEBKING UI + LinkFB + Info Tools" (2026-06-10)

**Massive UI revamp + multi-shortener foundation + utility tools.**

### 🎨 UI Refactor — JASEBKING Style (semua section)
Format konsisten di seluruh bot: title diamond (`💎 X X X 💎`), section header ALL CAPS dengan emoji, sub-items pakai `└` arrow, separator `|` antar section, semua URL/slug/value dalam backtick (copy-friendly).

Refactored:
- 🏠 Welcome message (DM admin + group)
- 🩺 Health Dashboard
- 📡 Monitor (entry + list + status + sticky + add/remove/check)
- ⚙️ CF Redirect (entry + add + list + change URL)
- 🔄 Auto Rotator (entry + setup + list per-type)
- 🔗 KLIKCEPAT (entry + add 5-step + edit + delete + projects)
- 🔧 Settings (CF + Klikcepat + Group Commands)
- 📋 Group views (Status Bot + List Domain + List CF)

### 🔗 NEW: LinkFB Integration
Pixly engine kedua — paralel sama Klikcepat. Scope: shortlink + projects (no biolink edit per design).

Backend:
- New `linkfb` instance dari klikcepat.Client (generic Pixly engine)
- Credentials: LinkfbBaseURL, LinkfbAPIKey, LinkfbDomainMap
- Default base URL: `https://linkfb.io`

UI:
- 🏠 Main menu → 🔗 LINKFB sub-section
- 🔧 Settings → 🔗 LinkFB (Set URL/Key/Test/Clear)
- LinkFB sub-menu: Add (4-step) / List / Edit / Delete / Projects / Dashboard URL
- Projects full CRUD (Add wizard 2-step / List / Edit / Delete with confirm)
- Pagination 10/page di Edit + Delete pickers

### 🛠 NEW: Info Tools (Telegram utility)
5 commands via tombol + slash:

Tombol (DM bot → 🏠 MENU → 🛠 Info Tools):
- 🆔 Get User ID dari @username
- 📨 Get Chat ID dari t.me link (support /c/ private)
- 👤 User Info (name, bio, dll)
- 💬 Chat Info (title, type, member count, description)
- 📚 Help

Slash commands (works DM admin + group):
- `/id @username`
- `/cekid <link>`
- `/info @username`
- `/cinfo @username`
- `/help`

### ✨ Klikcepat Polish
- ✏️ Edit Link: search by slug/title (case-insensitive, paginated)
- 📋 List Rotator: split picker bertingkat (CF | KLIKCEPAT → BIOLINK | SHORTLINK)
- 🩺 Health Dashboard: section Klikcepat baru (status, mapping, rotator counts)
- 🔍 Global Search: scan klikcepat juga (domain mapping + rotator destination)

### 🐛 Critical Bug Fixes
- Username dengan underscore (`@lupis_keju`) bikin Telegram reject bot reply
- Italic wrap code block (`_text `code` text_`) di banyak prompt → fixed
- Klikcepat sticky-blocked stuck — retry swap juga buat sticky/force-blocked
- Klikcepat API broken untuk update (`email_reports` decode + slug preservation)

### 🔧 Backend
- New stores: KlikcepatBlockRotatorStore, GroupCommandStore
- Klikcepat client: ListBiolinkBlocks, GetBiolinkBlock, UpdateBiolinkBlockLocation
- Custom Pixly endpoint `ApiBiolinkBlocks.php` (build from scratch — vendor patch)
- LinkFB.io API verified compatible tanpa patch

---

## 🎰 v1.5 — "Group Commands + Full UX Polish" (2026-05-29)

**Big release.** Member di group sekarang bisa pake bot tanpa harus admin — slash command langsung kasih link aktif.

### 💬 NEW: Group Commands
- **Member-facing slash commands** — admin set di DM, member pake di group
- Wizard 3-step: command name → pick klikcepat project → description
- Auto-sync ke Telegram `setMyCommands(AllGroupChats)` — autocomplete muncul saat user ketik `/`
- Group handler: detect `/cmd` → fetch klikcepat link by `project_id` → reply inline URL buttons
- **Smart SAFE filter** — auto-skip link yg destination-nya sticky-blocked Kominfo
- Format reply: `🎰 PROJECT — LINK TERSEDIA` + N link aktif + tombol klik
- Group welcome message di-update — section terpisah buat MEMBER vs ADMIN

### 🐛 Bug Fixes (Markdown Parse Errors)
- **Username dengan underscore** (Suriani `@lupis_keju`) bikin Telegram reject bot reply
  - Fix: escape `_`, `*`, `` ` `` di username sebelum prepend
- **Italic wrap code block** (`_text `code` text_`) bikin parser confused
  - Fix di banyak tempat: monitor source picker, group cmd success, group cmd list, dll
- **Literal underscore in display** (`project_id=5`, `id=N`) ditafsirkan italic delimiter
  - Fix: rephrase ke `(id N)` clean spacing

### 🎨 UI Refactor (sebelumnya di v1.1-1.4 — di-merge)
- **List Rotator picker bertingkat** — CF | KLIKCEPAT → BIOLINK BLOCK | SHORTLINK
- **Bulk Setup label prompt** — user kasih prefix custom, gak auto-generate doang
- **Edit Link pagination + subtype picker** (BIOLINK | SHORTLINK, full URL display)
- **Health Status** — section Klikcepat baru (status, mapping, rotator counts)
- **Global Search extended** — scan klikcepat domain mapping + shortlink/block rotator destination
- **Auto-swap notif** — pake full URL klikcepat (`klikcepat.lat/slug`) sesuai domain mapping

### ✨ Biolink Block Rotator (sebelumnya di v1.1)
- Custom Pixly endpoint `ApiBiolinkBlocks.php` (build from scratch)
- GET list/single + POST update location_url
- Bot wizard: Setup Rotator → KLIKCEPAT → 📄 BIOLINK → pick biolink → pick block → pool → label
- Bulk variant: multi-select blocks dalam 1 biolink → 1 pool → save N rotators
- Monitor scanner trigger `triggerKlikcepatBlockAutoSwap` tiap cycle BLOCKED (idempotent)

### 🔧 Backend
- New `klikcepat.BiolinkBlock` type + `BuildShortlinkURL` helper
- New `store.KlikcepatBlockRotatorStore`, `store.GroupCommandStore`
- Klikcepat client: `ListBiolinkBlocks`, `GetBiolinkBlock`, `UpdateBiolinkBlockLocation`
- Refactor `triggerKlikcepatAutoSwap` — decouple dari CF (jalan independent)
- Client-side filter `type=biolink/link` (Pixly API filter di-ignore)
- Klikcepat swap retry 1x setelah 3s kalau 5xx (transient backend)

### 🐛 Critical Bug Fixes (Klikcepat Vendor)
- **Pixly `ApiLinks.php`** patched (host: lo deploy edit sendiri):
  - `email_reports` decode missing → `array_filter` on string → PHP fatal 500
  - Slug auto-randomize on partial update → `$_POST['url'] = false` fallback
- **Klikcepat auto-swap sticky-blocked stuck** — sebelumnya skip retry kalau domain udah sticky
  - Fix: trigger swap juga buat sticky/force-blocked domain (idempotent skip kalau host udah beda)

### 🧪 Infrastructure
- LinkFB.io API verified compatible (sama Pixly, no patch needed)
- Multi-shortener foundation ready (Phase 2)
- ASCII art README diperbaiki (sebelumnya oleng di GitHub render)

---

## 🚀 v1.1 — "Biolink Block + UI Polish" (2026-05-28)

**Highlight:** Bug fix vendor API + biolink block rotator dari nol.

### 🐛 Critical Bug Fixes
- **Klikcepat API broken** — Patched Pixly source code (`ApiLinks.php`):
  - `email_reports` decode missing → `array_filter` on string → PHP fatal 500
  - Slug auto-randomize on partial update → bug `$_POST['url'] = false` fallback
  - Patch line 578 + line 615 (lihat dokumentasi di-repo)

### ✨ New Features
- **📄 Biolink Block Rotator** — Auto-swap destination button di biolink (LOGIN, DAFTAR, dll)
  - Custom API endpoint `ApiBiolinkBlocks.php` (build from scratch)
  - GET list/single + POST update location_url
  - Bot: Setup Rotator → KLIKCEPAT → 📄 BIOLINK → pick biolink → pick block → pool → label
  - Bulk variant: multi-select blocks dalam 1 biolink
- **🔍 Global Search extended** — sekarang scan Klikcepat juga:
  - Domain mapping (klikcepat.com/.lat/.vip)
  - Shortlink rotator destination
  - Biolink block rotator destination
- **🩺 Status Dashboard** — Section Klikcepat baru (status, mapping, rotator counts)

### 🎨 UI/UX Improvements
- **List Rotator** — Picker bertingkat: CF | KLIKCEPAT → BIOLINK BLOCK | SHORTLINK
  - Counter per tipe di tombol picker
  - Tiap sub-list ada Back button
- **Edit Link** — Pagination 10/page + subtype picker + full URL display
- **Bulk Setup** — Tanya prefix label setelah pilih pool (sebelumnya auto-label)
- **Setup Rotator Klikcepat** — Display full URL (`klikcepat.lat/slug`) bukan slug doang
- **Auto-swap notif** — Show full URL klikcepat sesuai domain mapping
- **Klikcepat swap retry** — 1x setelah 3s kalau 5xx (transient backend)
- **Block rotator integration ke monitor scanner** — trigger tiap cycle BLOCKED (idempotent)

### 🔧 Backend
- New `klikcepat.BiolinkBlock` type + `BuildShortlinkURL` helper
- New `store.KlikcepatBlockRotatorStore`
- Refactor `triggerKlikcepatAutoSwap` — decouple dari CF (jalan independent)
- Client-side filter type=biolink/link (Pixly API filter di-ignore)

---

## 🎰 v1.0 — "ALL IN ONE" Initial Release (2026-05-27)

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

## 🏗 v0.3 — Group Mode + Rotating Batch (2026-05-26, pre-release)

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

## 🛡 v0.2 — Multi-Source Nawala Check (2026-05-26, pre-release)

### Added
- **Triple-source check** — Kominfo + TrustPositif API + NawalaCheck
- **Source picker untuk manual check** — pilih source via inline button
- **Conditional API key buttons** — hide button kalau API key gak di-set di .env

### Fixed
- HTTP 404 rate-limit dari parallel requests (turun ke 3 concurrent, 200ms delay)
- Sticky-block "treated as SAFE on ERROR" bug

---

## 🎬 v0.1 — Initial Production Release (earlier, pre-release)

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

## 🔗 Git Tags

```bash
# List semua versions
git tag -l

# Checkout version tertentu
git checkout v1.0

# Lihat diff antar version
git diff v0.3..v1.0
```

## 🎯 Next Release Roadmap

- **v1.1** — TBD (custom API biolink blocks? bulk delete? analytics? user pick)
- **v1.2** — ...
- ...
- **v1.9** — last of v1.x series
- **v2.0** — next major leap
