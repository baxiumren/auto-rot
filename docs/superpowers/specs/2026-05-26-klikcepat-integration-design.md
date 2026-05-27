# KLIKCEPAT Integration — Design Spec

**Date:** 2026-05-26
**Status:** Design approved, ready for implementation plan
**Related:** BongBot Auto Domain Rotator

## Background & Goal

User punya 2 sistem domain protection:
1. **CF Redirect Rules** — existing, sudah ada di bot (auto-swap on nawala block)
2. **Klikcepat.com** — bio link / short URL platform (66biolinks by AltumCode)

Klikcepat punya link-link (biolink pages, short URLs) yang `location_url` (target redirect) bisa kena nawala block. Saat ini user harus manual update link target via klikcepat dashboard.

**Goal:** Integrate klikcepat ke BongBot supaya:
- Bot bisa CRUD (create/read/update/delete) link klikcepat dari Telegram
- Auto-swap `location_url` ke domain backup pas target ke-detect blocked oleh Monitor Scanner

## Constraints & Discoveries

### 66biolinks API Architecture

- **Auth:** Bearer token in `Authorization` header
- **Base URL:** `https://klikcepat.com/api/`
- **Per-user API key** (bukan global) — bot operates AS specific user account

### Endpoints Relevant

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/api/links` | List semua user's links (paginated) |
| GET | `/api/links/{id}` | Get detail single link |
| POST | `/api/links` | Create new link |
| POST | `/api/links/{id}` | UPDATE link (termasuk `location_url`) |
| DELETE | `/api/links/{id}` | Delete link |

### Link Types Supported

- `biolink` — bio link page
- `link` — short URL
- `file` — file hosting link
- `vcard` — vCard
- `event` — event link
- `static` — static link

### Auth Strategy Decision

User punya master account + normal admin account di klikcepat:
- **Master**: untuk manage users (gak ada link)
- **Normal admin**: punya link-link (yang akan di-manage bot)

**Decision:** Master enable API di plan setting → normal admin generate API key → bot pakai normal admin's API key.

Bot operates AS normal admin via API.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       BongBot (Go)                           │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Cloudflare  │  │   Monitor    │  │    KLIKCEPAT     │  │
│  │   Client     │  │   Scanner    │  │   API Client     │  │
│  │  (existing)  │  │              │  │      (NEW)       │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│         │                  │                   │             │
│         │       triggerAutoSwap (extended)     │             │
│         └──────────────────┴───────────────────┘             │
└─────────────────────────────────────────────────────────────┘
            │                              │
            ▼                              ▼
   ┌──────────────────┐         ┌────────────────────┐
   │ Cloudflare API   │         │ klikcepat.com/api  │
   │ (Page/Redirect)  │         │ Bearer token auth  │
   └──────────────────┘         └────────────────────┘
```

## Components

### 1. `klikcepat/` package (new)

API client mirror pattern `cloudflare/client.go`.

**File:** `klikcepat/client.go`

```go
package klikcepat

type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

type Link struct {
    ID          int    `json:"id"`
    Type        string `json:"type"`
    Title       string `json:"title"`
    URL         string `json:"url"`              // slug
    LocationURL string `json:"location_url"`     // target redirect (SWAP TARGET)
    DomainID    int    `json:"domain_id"`
    IsEnabled   int    `json:"is_enabled"`
}

func New(baseURL, apiKey string) *Client
func (c *Client) HasCredentials() bool
func (c *Client) Ping() error
func (c *Client) ListLinks() ([]Link, error)
func (c *Client) GetLink(id int) (*Link, error)
func (c *Client) CreateLink(linkType, title, url, locationURL string) (*Link, error)
func (c *Client) UpdateLink(id int, locationURL string) error  // primary swap method
func (c *Client) UpdateLinkFull(id int, fields map[string]string) error  // full update
func (c *Client) DeleteLink(id int) error
```

### 2. `store/store.go` extensions

```go
// KlikcepatRotatorStore — mapping klikcepat link → pool label
type KlikcepatRotatorStore struct {
    mu   sync.Mutex
    data []KlikcepatRotator
}

type KlikcepatRotator struct {
    ID        string `json:"id"`
    Label     string `json:"label"`      // user-friendly identifier
    LinkID    int    `json:"link_id"`    // klikcepat link ID
    LinkURL   string `json:"link_url"`   // displayed slug for UI
    PoolLabel string `json:"pool_label"` // monitor label for backup pool
    Active    bool   `json:"active"`
    CreatedAt time.Time `json:"created_at"`
}

func NewKlikcepatRotatorStore() *KlikcepatRotatorStore
func (s *KlikcepatRotatorStore) GetAll() []KlikcepatRotator
func (s *KlikcepatRotatorStore) GetByLinkID(id int) (*KlikcepatRotator, bool)
func (s *KlikcepatRotatorStore) Add(r KlikcepatRotator) error
func (s *KlikcepatRotatorStore) Delete(id string) bool
func (s *KlikcepatRotatorStore) Toggle(id string) bool
```

Stored di `data/klikcepat_rotators.json`.

### 3. `config/config.go` extension

```go
type Config struct {
    // ... existing fields ...
    KlikcepatBaseURL string  // dari KLIKCEPAT_BASE_URL env
    KlikcepatAPIKey  string  // dari KLIKCEPAT_API_KEY env
}
```

`.env` additions:
```env
KLIKCEPAT_BASE_URL=https://klikcepat.com
KLIKCEPAT_API_KEY=<normal-admin-api-key>
```

### 4. `store/credentials.go` extension (opsional)

Klikcepat credentials bisa juga di-store di `data/credentials.json` (sama kayak CF), supaya bisa di-set via bot menu.

### 5. Bot Menu (REVISED — 5 button layout, unified Rotator hub)

**Main menu (DM admin) — 5 button total:**
```
🏠 MENU UTAMA
[📡 Monitor]      [⚙️ CF Redirect]
[🔄 Auto Rotator] [🔗 KLIKCEPAT]
[🔧 Settings]
```

`🔄 Auto Rotator` jadi **unified hub** untuk CF + Klikcepat auto-swap.
`🔗 KLIKCEPAT` standalone untuk CRUD link/project (mirror `⚙️ CF Redirect`).

**Sub-menu `🔗 KLIKCEPAT`:**
```
🔗 KLIKCEPAT — Bio Link & Short URL

[➕ Tambah Link]   [📋 List Link]
[✏️ Edit Link]    [🗑 Hapus Link]
[📁 Projects]     [🌐 Open Dashboard]
[🔙 Kembali]
```

`📁 Projects` punya sub-menu CRUD project sendiri. Auto-Swap *tidak* di sini — di Auto Rotator.

**Sub-menu `📁 Projects`:**
```
📁 KLIKCEPAT PROJECTS

[➕ Tambah Project]  [📋 List Project]
[✏️ Edit Project]   [🗑 Hapus Project]
[🔙 Kembali]
```

**Sub-menu `🔄 Auto Rotator` (UNIFIED — CF + Klikcepat):**
```
🔄 AUTO ROTATOR — Setup Auto-Swap

[➕ Setup Rotator]   [📦 Bulk Setup]
[📋 List Rotator]    [📜 Swap History]
[🔙 Kembali]
```

**Wizard `➕ Setup Rotator` (UNIFIED ENTRY):**

Step 1 — Pilih tipe rotator:
```
🔄 Setup Rotator — Pilih Tipe

[⚙️ CF Redirect]    [🔗 KLIKCEPAT]
[❌ Batal]
```

- Klik `⚙️ CF Redirect` → flow existing (pick CF Rule → pool → label)
- Klik `🔗 KLIKCEPAT` → flow baru (pick klikcepat link → pool → label)

**Wizard `📋 List Rotator` (UNIFIED VIEW):**
Tampilkan SEMUA rotator (CF + Klikcepat) grouped by type:
```
📋 ROTATOR LIST

═══ ⚙️ CF Redirect ═══
✅ MAHA66-ROT
   🔧 Rule: MAHA66.ID
   📂 Pool: STOCK-MS
   [Toggle] [Delete] [Force]

═══ 🔗 KLIKCEPAT ═══
✅ PROMO-MAHA-ROT
   🔗 Link: klikcepat.com/promo-maha
   📂 Pool: MONEYSITE
   [Toggle] [Delete] [Force]

Total: 6 CF + 3 Klikcepat = 9 active rotators
```

**Wizard flows (CRUD link):**

**A. Tambah Link (Add):**
1. Pilih type (biolink/link/file/vcard/event/static) — inline buttons
2. Input title (text)
3. Input slug (text, auto-generate kalau kosong)
4. Input location_url (text)
5. (Opsional) Pilih project — picker dari /api/projects, atau "Skip"
6. Confirm → POST /api/links → success message

**B. Edit Link:**
1. Pick link dari list (inline button per link, max 100 paginated)
2. Pilih field yang mau di-edit (title / slug / location_url / project / is_enabled)
3. Input new value
4. Confirm → POST /api/links/{id}

**C. Hapus Link:**
1. Pick link dari list
2. Confirmation dialog
3. DELETE /api/links/{id}

**D. Project CRUD (mirror Link CRUD):**
1. Tambah: input name + color (hex) → POST /api/projects
2. List: GET /api/projects → tampilkan + count link per project
3. Edit: pick → input new name/color → POST /api/projects/{id}
4. Hapus: pick → confirmation → DELETE /api/projects/{id}

**E. Auto-Swap Setup (UNIFIED di Auto Rotator menu, BUKAN di Klikcepat menu):**

Dari `🔄 Auto Rotator → ➕ Setup Rotator`:

Step 1 — User klik `🔗 KLIKCEPAT` (tipe rotator):

Step 2 — Pick klikcepat link:
- Bot fetch GET /api/links (filter by type: link/biolink)
- Tampilkan inline button per link (pagination kalau >50)

Step 3 — Pick pool label:
- Dari h.domains.Labels() (Monitor pool)

Step 4 — Input rotator label:
- Free text identifier

Step 5 — Save → KlikcepatRotatorStore.Add()

### 6. Auto-Swap Integration

Extend `rotator/monitor_scanner.go` → `triggerAutoSwap`:

```go
func (ms *MonitorScanner) triggerAutoSwap(blockedDomain, blockedLabel string) {
    // EXISTING: scan CF rules
    ms.triggerCFAutoSwap(blockedDomain, blockedLabel)

    // NEW: scan klikcepat rotators
    if ms.klikcepat != nil && ms.klikcepat.HasCredentials() {
        ms.triggerKlikcepatAutoSwap(blockedDomain, blockedLabel)
    }
}

func (ms *MonitorScanner) triggerKlikcepatAutoSwap(blockedDomain, blockedLabel string) {
    rotators := ms.klikcepatStore.GetAll()
    for _, rot := range rotators {
        if !rot.Active {
            continue
        }
        // Get current link state from klikcepat
        link, err := ms.klikcepat.GetLink(rot.LinkID)
        if err != nil {
            log.Printf("[KLIKCEPAT-SWAP] gagal fetch link %d: %v", rot.LinkID, err)
            continue
        }
        currentHost := extractHost(link.LocationURL)
        if !strings.EqualFold(currentHost, blockedDomain) {
            continue
        }
        // Pick next safe domain from pool
        pool := ms.domains.GetByLabel(rot.PoolLabel)
        nextDomain := ms.pickNextSafe(pool, blockedDomain)
        if nextDomain == "" {
            ms.notify.Notify(fmt.Sprintf("🚨 *Klikcepat swap gagal — pool kosong*\n🔗 Link: `%s`\n📂 Pool: `%s`", rot.LinkURL, rot.PoolLabel))
            continue
        }
        // Build new URL (preserve path & query)
        newLocationURL := buildSwapURL(link.LocationURL, nextDomain)
        if err := ms.klikcepat.UpdateLink(rot.LinkID, newLocationURL); err != nil {
            ms.notify.Notify(fmt.Sprintf("❌ *Klikcepat swap GAGAL*\n🔗 Link: `%s`\nError: %v", rot.LinkURL, err))
            continue
        }
        // Notif sukses
        ms.notify.Notify(fmt.Sprintf(
            "⚡ *KLIKCEPAT AUTO-SWAP*\n"+
                "🔗 Link: `%s` (%s)\n"+
                "🚫 Sebelum: `%s` *(BLOCKED — label: %s)*\n"+
                "   URL: `%s`\n"+
                "✅ Sekarang: `%s`\n"+
                "   URL: `%s`\n"+
                "📂 Pool: `%s`\n"+
                "🕐 %s",
            rot.LinkURL, link.Type,
            blockedDomain, blockedLabel, link.LocationURL,
            nextDomain, newLocationURL,
            rot.PoolLabel,
            time.Now().Format("02/01/2006 15:04:05"),
        ))
        if ms.history != nil {
            ms.history.LogSwap("klikcepat-scan", rot.Label, rot.LinkURL, link.LocationURL, newLocationURL, true, "")
        }
    }
}
```

### 7. Settings Integration

`🔧 Settings` menu tambah section baru:

```
🔧 Settings
├── 🔧 Cloudflare (existing)
└── 🔗 Klikcepat (NEW)
    [📧 Set Base URL]
    [🔑 Set API Key]
    [✅ Test Koneksi]
    [🗑 Hapus Credentials]
```

`Test Koneksi` call `Klikcepat.Ping()` (GET /api/user) → konfirmasi credentials valid.

### 8. Group Menu Extension (opsional)

Di group, tambah tombol `🔗 List Klikcepat` di groupMenu (parallel sama List CF):

```
[🩺 Status Bot]    [📋 List Domain]
[🔄 List CF]       [🔗 List Klikcepat]   ← BARU
[🤖 Setup di DM →]
```

Nampilin list klikcepat links (read-only) di group.

## Data Flow Examples

### Scenario 1: User add new link via bot DM

```
1. User klik 🔗 KLIKCEPAT → ➕ Tambah Link
2. Bot: "Pilih type:" [biolink] [link] [file] [vcard] [event] [static]
3. User klik [link]
4. Bot: "Ketik title:" → user "Promo Maha"
5. Bot: "Ketik slug (kosong = auto):" → user "promo-maha"
6. Bot: "Ketik location URL:" → user "https://maha-supreme.com/daftar"
7. Bot: POST /api/links {type: "link", title: "Promo Maha", url: "promo-maha", location_url: "..."}
8. Klikcepat return new link object
9. Bot: "✅ Link dibuat!
        🔗 klikcepat.com/promo-maha
        🎯 https://maha-supreme.com/daftar
        📌 Type: Short URL"
```

### Scenario 2: Setup auto-swap (UNIFIED via Auto Rotator)

```
1. User: 🔄 Auto Rotator → ➕ Setup Rotator
2. Bot: "Pilih tipe rotator:" [⚙️ CF Redirect] [🔗 KLIKCEPAT]
3. User klik [🔗 KLIKCEPAT]
4. Bot fetch list klikcepat links → tampilkan picker
5. User pilih link "promo-maha"
6. Bot tampilkan picker pool label (dari Monitor)
7. User pilih "MONEYSITE"
8. Bot: "Ketik label rotator:" → user "PROMO-MAHA-ROT"
9. Save ke KlikcepatRotatorStore
10. Bot: "✅ Rotator dibuat!
         Type: Klikcepat
         🔗 Link: klikcepat.com/promo-maha
         📂 Pool: MONEYSITE
         ✅ Active"
```

### Scenario 3: Domain blocked → auto-swap

```
1. Monitor cycle: detect maha-supreme.com BLOCKED
2. triggerAutoSwap:
   - CF Rules check (existing) → swap CF rules yang current = maha-supreme.com
   - Klikcepat check (NEW):
     - Iterate KlikcepatRotators yang Active
     - Rotator "PROMO-MAHA-ROT" (LinkID=42):
       - GET /api/links/42 → location_url = "https://maha-supreme.com/daftar"
       - currentHost = "maha-supreme.com" → MATCH
       - pickNextSafe(pool="MONEYSITE", current="maha-supreme.com") → "mahasupreme.xyz"
       - newURL = "https://mahasupreme.xyz/daftar" (preserve path/query)
       - POST /api/links/42 {location_url: newURL}
3. Notif group:
   ⚡ KLIKCEPAT AUTO-SWAP
   🔗 Link: klikcepat.com/promo-maha (link)
   🚫 Sebelum: maha-supreme.com (BLOCKED)
   ✅ Sekarang: mahasupreme.xyz
```

## Error Handling

| Error | Detection | Action |
|---|---|---|
| API key invalid (401) | HTTP status from klikcepat | Log + notif admin "❌ Klikcepat API key invalid — disabled auto-swap" + skip cycle (don't crash) |
| Link not found (404) | HTTP status when GET/POST | Log + mark rotator inactive automatically + notif "⚠️ Klikcepat link X tidak ada, rotator di-pause" |
| Network error / timeout | http.Client error | Retry 3x with exponential backoff (1s, 2s, 4s); skip cycle if all fail |
| Pool kosong | pickNextSafe returns "" | Notif "🚨 Pool kosong untuk klikcepat link X" |
| Rate limit (429) | HTTP status | Wait 30s + retry once |
| Klikcepat server error (5xx) | HTTP status | Log + skip cycle, retry next monitor tick |

## Testing Plan

### Unit tests
- `klikcepat/client_test.go` — mock HTTP responses, test parse Link struct, test UpdateLink request body
- `store/klikcepat_test.go` — CRUD operations on KlikcepatRotatorStore
- `rotator/monitor_scanner_test.go` — test triggerKlikcepatAutoSwap with mock klikcepat client

### Integration tests
- Manual test via bot UI:
  1. Set API key via Settings menu
  2. Test Koneksi → konfirm OK
  3. Create link via wizard
  4. List link → confirm muncul
  5. Edit link target → confirm update
  6. Delete link → confirm hilang
  7. Setup auto-swap → confirm di list rotator
  8. Force-block domain di Monitor → confirm klikcepat link target ke-update otomatis
  9. Verify di klikcepat.com dashboard

### Edge cases to test
- Klikcepat down during scan → bot tetep jalan (no crash)
- API key removed mid-operation → graceful error
- Pool kosong → notif yg bener (gak crash)
- Domain match dengan path/query → preserve path/query saat swap
- Multiple klikcepat rotator untuk pool yang sama → semua jalan paralel

## Migration / Rollout

Karena fitur baru (gak modify existing), gak ada migration script. Rollout:

1. Deploy code update
2. User add `KLIKCEPAT_BASE_URL` + `KLIKCEPAT_API_KEY` ke `.env` (atau via Settings menu)
3. Restart bot
4. Bot auto-detect klikcepat credentials → enable klikcepat menu
5. Backward compat: kalau credentials kosong, klikcepat menu tetep muncul tapi greyed out / show "Setup credentials dulu"

## Out of Scope (Future)

- **Biolink block-level swap** (individual blocks/buttons inside biolink page) — 66biolinks SENGAJA gak expose API untuk blocks. Block management harus via dashboard klikcepat.com (web UI).
- Analytics/statistics dari klikcepat (perlu /api/statistics, bisa di-add nanti)
- Bulk operations (bulk create link / bulk delete)
- Klikcepat custom domains management (perlu /api/domains)
- QR code, splash pages (perlu /api/qr-codes, /api/splash-pages)
- Multi-account support (manage multiple klikcepat user accounts dalam 1 bot)

## Open Questions

(none — design approved by user)

## Revision History

- **2026-05-26 v1** — Initial design (separate Klikcepat menu dengan Auto-Swap di dalam)
- **2026-05-26 v2** — Restructure: Auto-Swap dipindah ke `🔄 Auto Rotator` (unified hub CF+Klikcepat). `🔗 KLIKCEPAT` jadi fokus CRUD link/project doang. Menu utama 5 button (2x2 + Settings).
