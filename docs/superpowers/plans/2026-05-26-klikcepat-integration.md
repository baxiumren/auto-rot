# KLIKCEPAT Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate klikcepat.com (klikcepat) ke BongBot: CRUD link & project via Telegram + auto-swap location_url ketika domain blocked, semua via unified Auto Rotator menu.

**Architecture:**
- New `klikcepat/` package mirror pattern `cloudflare/` (HTTP client + types)
- New `KlikcepatRotatorStore` in `store/` (persist link→pool mappings)
- Bot menu refactor: 5 button utama (`Monitor`, `CF Redirect`, `Auto Rotator`, `KLIKCEPAT`, `Settings`)
- `Auto Rotator` jadi unified hub — Setup Rotator pick type dulu (CF / Klikcepat) sebelum config detail
- `KLIKCEPAT` menu fokus CRUD link/project (auto-swap setup di Auto Rotator)
- Extend `monitor_scanner.go` triggerAutoSwap untuk scan klikcepat rotators

**Tech Stack:** Go 1.21+, gopkg.in/telebot.v3, klikcepat REST API (Bearer token auth)

**Reference:** Design spec — `docs/superpowers/specs/2026-05-26-klikcepat-integration-design.md`

---

## File Structure (Map)

**NEW files:**
- `klikcepat/client.go` — HTTP client, Bearer auth, all API methods
- `klikcepat/types.go` — Link, Project struct definitions
- `klikcepat/client_test.go` — Unit tests for HTTP client (mocked server)
- `store/klikcepat.go` — KlikcepatRotatorStore (CRUD rotator mapping, JSON persistence)
- `bot/klikcepat.go` — Menu + CRUD link handlers
- `bot/klikcepat_projects.go` — Project CRUD handlers
- `bot/klikcepat_rotator.go` — Klikcepat-specific rotator wizard (called from Auto Rotator)
- `bot/settings_klikcepat.go` — Klikcepat credentials section in Settings

**MODIFIED files:**
- `config/config.go` — add `KlikcepatBaseURL`, `KlikcepatAPIKey` fields + env parsing
- `store/store.go` — add fields ke Credentials struct (KlikcepatBaseURL, KlikcepatAPIKey)
- `bot/menu.go` — add ~20 new callback constants + new menu functions
- `bot/bot.go` — route new callbacks + add klikcepat client to Handler struct
- `bot/session.go` — add new Step constants for klikcepat wizards
- `bot/autorotate.go` — refactor handleRotatorAdd to show "pick type" step
- `bot/settings.go` — add Klikcepat section button + handler
- `rotator/monitor_scanner.go` — add KlikcepatRotatorStore field + triggerKlikcepatAutoSwap
- `main.go` — initialize klikcepat client + store + pass to handlers
- `.env.example` — add `KLIKCEPAT_BASE_URL`, `KLIKCEPAT_API_KEY`

---

## Phase 1: Klikcepat API Client Package

### Task 1: Klikcepat types + client constructor + Ping

**Files:**
- Create: `klikcepat/types.go`
- Create: `klikcepat/client.go`

- [ ] **Step 1: Create `klikcepat/types.go`**

```go
package klikcepat

// Link represents a klikcepat link object (biolink, shortlink, vcard, etc).
type Link struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	ProjectID   int    `json:"project_id"`
	DomainID    int    `json:"domain_id"`
	Type        string `json:"type"`         // biolink, link, file, vcard, event, static
	Title       string `json:"title"`
	URL         string `json:"url"`          // slug (klikcepat.com/SLUG)
	LocationURL string `json:"location_url"` // target redirect — what we swap
	IsEnabled   int    `json:"is_enabled"`
	Datetime    string `json:"datetime"`
}

// Project represents a klikcepat project (link grouping).
type Project struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
```

- [ ] **Step 2: Create `klikcepat/client.go` with constructor + Ping**

```go
package klikcepat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	mu      sync.RWMutex
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) SetCredentials(baseURL, apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.apiKey = apiKey
}

func (c *Client) HasCredentials() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL != "" && c.apiKey != ""
}

// Ping verifies credentials by calling /api/user endpoint.
func (c *Client) Ping() error {
	_, err := c.do(http.MethodGet, "/api/user", nil)
	return err
}

// do is the low-level HTTP helper.
// body can be nil (GET/DELETE), or url.Values (form POST), or []byte JSON.
func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	c.mu.RLock()
	base, key := c.baseURL, c.apiKey
	c.mu.RUnlock()
	if base == "" || key == "" {
		return nil, fmt.Errorf("klikcepat credentials belum di-set")
	}
	var bodyReader io.Reader
	contentType := ""
	switch v := body.(type) {
	case nil:
		// no body
	case url.Values:
		bodyReader = strings.NewReader(v.Encode())
		contentType = "application/x-www-form-urlencoded"
	case []byte:
		bodyReader = bytes.NewReader(v)
		contentType = "application/json"
	default:
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(raw)
		contentType = "application/json"
	}

	req, err := http.NewRequest(method, base+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("klikcepat request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		// Try parse error message
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(data, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = errResp.Error
		}
		if msg == "" {
			msg = string(data)
		}
		return nil, fmt.Errorf("klikcepat HTTP %d: %s", resp.StatusCode, msg)
	}
	return data, nil
}
```

- [ ] **Step 3: Verify build**

Run: `cd /e/CODING/auto-rotator && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
cd /e/CODING/auto-rotator
git add klikcepat/
git commit -m "feat(klikcepat): add API client foundation + types"
```

---

### Task 2: Klikcepat Link CRUD methods

**Files:**
- Modify: `klikcepat/client.go`

- [ ] **Step 1: Add Link CRUD methods**

Append to `klikcepat/client.go`:

```go
// ─── Links API ────────────────────────────────────────────────────────────────

// ListLinks fetches all user's links. Optional filter by type ("link", "biolink", etc.)
// Empty linkType returns all types.
func (c *Client) ListLinks(linkType string) ([]Link, error) {
	path := "/api/links?results_per_page=1000"
	if linkType != "" {
		path += "&type=" + url.QueryEscape(linkType)
	}
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse links: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) GetLink(id int) (*Link, error) {
	data, err := c.do(http.MethodGet, fmt.Sprintf("/api/links/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// CreateLink creates a new link.
// linkType: "link" (shortlink), "biolink" (biolink page)
// title: display title
// slug: custom slug (empty = auto-generate)
// locationURL: target redirect URL
// projectID: 0 = no project, >0 = assign to project
func (c *Client) CreateLink(linkType, title, slug, locationURL string, projectID int) (*Link, error) {
	form := url.Values{}
	form.Set("type", linkType)
	form.Set("title", title)
	if slug != "" {
		form.Set("url", slug)
	}
	form.Set("location_url", locationURL)
	if projectID > 0 {
		form.Set("project_id", fmt.Sprintf("%d", projectID))
	}
	data, err := c.do(http.MethodPost, "/api/links", form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateLink updates fields of an existing link.
// Pass non-empty values for fields to update.
// Use UpdateLinkLocation for swap-only operations (more efficient).
func (c *Client) UpdateLink(id int, fields map[string]string) (*Link, error) {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	data, err := c.do(http.MethodPost, fmt.Sprintf("/api/links/%d", id), form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateLinkLocation is the primary swap method — updates location_url only.
func (c *Client) UpdateLinkLocation(id int, locationURL string) error {
	form := url.Values{}
	form.Set("location_url", locationURL)
	_, err := c.do(http.MethodPost, fmt.Sprintf("/api/links/%d", id), form)
	return err
}

func (c *Client) DeleteLink(id int) error {
	_, err := c.do(http.MethodDelete, fmt.Sprintf("/api/links/%d", id), nil)
	return err
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add klikcepat/client.go
git commit -m "feat(klikcepat): add Links CRUD methods"
```

---

### Task 3: Klikcepat Project CRUD methods

**Files:**
- Modify: `klikcepat/client.go`

- [ ] **Step 1: Add Project CRUD methods**

Append to `klikcepat/client.go`:

```go
// ─── Projects API ─────────────────────────────────────────────────────────────

func (c *Client) ListProjects() ([]Project, error) {
	data, err := c.do(http.MethodGet, "/api/projects?results_per_page=1000", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Project `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetProject(id int) (*Project, error) {
	data, err := c.do(http.MethodGet, fmt.Sprintf("/api/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Project `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) CreateProject(name, color string) (*Project, error) {
	if color == "" {
		color = "#000000"
	}
	form := url.Values{}
	form.Set("name", name)
	form.Set("color", color)
	data, err := c.do(http.MethodPost, "/api/projects", form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Project `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) UpdateProject(id int, name, color string) (*Project, error) {
	form := url.Values{}
	if name != "" {
		form.Set("name", name)
	}
	if color != "" {
		form.Set("color", color)
	}
	data, err := c.do(http.MethodPost, fmt.Sprintf("/api/projects/%d", id), form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Project `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteProject(id int) error {
	_, err := c.do(http.MethodDelete, fmt.Sprintf("/api/projects/%d", id), nil)
	return err
}
```

- [ ] **Step 2: Verify build + commit**

```bash
go build ./...
git add klikcepat/client.go
git commit -m "feat(klikcepat): add Projects CRUD methods"
```

---

### Task 4: Klikcepat client unit test (HTTP mocked)

**Files:**
- Create: `klikcepat/client_test.go`

- [ ] **Step 1: Write unit test**

Create `klikcepat/client_test.go`:

```go
package klikcepat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Write([]byte(`{"data": {"id": 1, "email": "test@test.com"}}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	if err := c.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestListLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/links") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": 1, "type": "link", "title": "Test", "url": "test", "location_url": "https://example.com"},
				{"id": 2, "type": "biolink", "title": "Bio", "url": "bio", "location_url": ""},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	links, err := c.ListLinks("")
	if err != nil {
		t.Fatalf("ListLinks failed: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
	if links[0].LocationURL != "https://example.com" {
		t.Errorf("link[0].LocationURL = %q", links[0].LocationURL)
	}
}

func TestUpdateLinkLocation(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/links/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		r.ParseForm()
		receivedBody = r.PostForm.Encode()
		w.Write([]byte(`{"data": {"id": 42, "location_url": "https://new.com"}}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	if err := c.UpdateLinkLocation(42, "https://new.com"); err != nil {
		t.Fatalf("UpdateLinkLocation failed: %v", err)
	}
	if !strings.Contains(receivedBody, "location_url=https") {
		t.Errorf("body missing location_url: %s", receivedBody)
	}
}

func TestHasCredentials(t *testing.T) {
	c := New("", "")
	if c.HasCredentials() {
		t.Error("expected false for empty creds")
	}
	c.SetCredentials("https://klikcepat.com", "key")
	if !c.HasCredentials() {
		t.Error("expected true after SetCredentials")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd /e/CODING/auto-rotator && go test ./klikcepat/...`
Expected: PASS — 4 tests OK

- [ ] **Step 3: Commit**

```bash
git add klikcepat/client_test.go
git commit -m "test(klikcepat): unit tests for HTTP client"
```

---

## Phase 2: Storage & Config

### Task 5: Config + .env.example + Credentials store

**Files:**
- Modify: `config/config.go`
- Modify: `store/store.go` (Credentials struct)
- Modify: `.env.example`

- [ ] **Step 1: Add fields to Config struct**

Edit `config/config.go`. Add to `Config` struct after `NawalaCheckKey`:

```go
	// Klikcepat (klikcepat) integration — optional
	KlikcepatBaseURL string // dari KLIKCEPAT_BASE_URL
	KlikcepatAPIKey  string // dari KLIKCEPAT_API_KEY
```

In `Load()`, add before `return`:

```go
	klikcepatBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("KLIKCEPAT_BASE_URL")), "/")
	klikcepatAPIKey := strings.TrimSpace(os.Getenv("KLIKCEPAT_API_KEY"))
```

In the returned `&Config{...}`, add:

```go
		KlikcepatBaseURL: klikcepatBaseURL,
		KlikcepatAPIKey:  klikcepatAPIKey,
```

- [ ] **Step 2: Extend Credentials struct**

Open `store/store.go`. Find `Credentials` struct (search for `type Credentials struct`). Add fields:

```go
type Credentials struct {
	CFEmail          string `json:"cf_email"`
	CFAPIKey         string `json:"cf_api_key"`
	KlikcepatBaseURL string `json:"klikcepat_base_url"`
	KlikcepatAPIKey  string `json:"klikcepat_api_key"`
}
```

Find existing setter methods (`SetEmail`, `SetAPIKey`). Add similar:

```go
func (s *CredentialStore) SetKlikcepatBaseURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.KlikcepatBaseURL = strings.TrimRight(strings.TrimSpace(url), "/")
	go s.save()
}

func (s *CredentialStore) SetKlikcepatAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.KlikcepatAPIKey = strings.TrimSpace(key)
	go s.save()
}

func (s *CredentialStore) ClearKlikcepat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.KlikcepatBaseURL = ""
	s.data.KlikcepatAPIKey = ""
	go s.save()
}
```

- [ ] **Step 3: Update .env.example**

Append to `.env.example`:

```env

# ─── Optional: KLIKCEPAT (klikcepat) Integration ─────────────────────────
# Bot bisa CRUD link/project + auto-swap location_url ketika domain blocked
# Setup: enable API di plan klikcepat → normal admin generate API key
# Atau set via menu Settings → 🔗 Klikcepat di bot
KLIKCEPAT_BASE_URL=
KLIKCEPAT_API_KEY=
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add config/config.go store/store.go .env.example
git commit -m "feat(config): add klikcepat config + credentials store fields"
```

---

### Task 6: KlikcepatRotatorStore

**Files:**
- Create: `store/klikcepat.go`

- [ ] **Step 1: Create KlikcepatRotatorStore**

Create `store/klikcepat.go`:

```go
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// KlikcepatRotator maps a klikcepat link to a Monitor pool label
// for auto-swap when target domain gets blocked.
type KlikcepatRotator struct {
	ID        string    `json:"id"`         // slug from Label
	Label     string    `json:"label"`      // user-friendly name
	LinkID    int       `json:"link_id"`    // klikcepat link ID
	LinkURL   string    `json:"link_url"`   // displayed slug for UI
	LinkType  string    `json:"link_type"`  // "link" or "biolink"
	PoolLabel string    `json:"pool_label"` // Monitor label for backup pool
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type KlikcepatRotatorStore struct {
	mu       sync.Mutex
	data     []KlikcepatRotator
	filepath string
}

func NewKlikcepatRotatorStore() *KlikcepatRotatorStore {
	s := &KlikcepatRotatorStore{
		filepath: "data/klikcepat_rotators.json",
	}
	s.load()
	return s
}

func (s *KlikcepatRotatorStore) load() {
	raw, err := os.ReadFile(s.filepath)
	if err != nil {
		return // empty store
	}
	_ = json.Unmarshal(raw, &s.data)
}

func (s *KlikcepatRotatorStore) save() {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.filepath, raw, 0644)
}

func (s *KlikcepatRotatorStore) GetAll() []KlikcepatRotator {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]KlikcepatRotator, len(s.data))
	copy(out, s.data)
	return out
}

func (s *KlikcepatRotatorStore) GetByID(id string) (*KlikcepatRotator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].ID == id {
			r := s.data[i]
			return &r, true
		}
	}
	return nil, false
}

func (s *KlikcepatRotatorStore) GetByLinkID(linkID int) (*KlikcepatRotator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].LinkID == linkID {
			r := s.data[i]
			return &r, true
		}
	}
	return nil, false
}

// Add creates a new rotator. Returns error if Label already exists.
func (s *KlikcepatRotatorStore) Add(r KlikcepatRotator) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = slugify(r.Label)
	for _, existing := range s.data {
		if existing.ID == r.ID {
			return fmt.Errorf("rotator dengan label %q udah ada", r.Label)
		}
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	r.Active = true
	s.data = append(s.data, r)
	go s.save()
	return nil
}

func (s *KlikcepatRotatorStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data {
		if r.ID == id {
			s.data = append(s.data[:i], s.data[i+1:]...)
			go s.save()
			return true
		}
	}
	return false
}

// Toggle flips Active. Returns new state, and whether found.
func (s *KlikcepatRotatorStore) Toggle(id string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].ID == id {
			s.data[i].Active = !s.data[i].Active
			go s.save()
			return s.data[i].Active, true
		}
	}
	return false, false
}
```

> Note: `slugify` already exists in store package — uses same helper as CFRuleStore.

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add store/klikcepat.go
git commit -m "feat(store): add KlikcepatRotatorStore for auto-swap mapping"
```

---

## Phase 3: Wire Bot Infrastructure

### Task 7: Add session steps + menu callbacks + main menu refactor

**Files:**
- Modify: `bot/session.go`
- Modify: `bot/menu.go`

- [ ] **Step 1: Add Step constants for klikcepat wizards**

Edit `bot/session.go`. All existing Step constants use type `Step` (e.g. `StepMonitorAddDomain Step = "..."`). Add at end of the const block:

```go
	// Klikcepat Settings
	StepSettingsKlikcepatURL Step = "settings_klikcepat_url"
	StepSettingsKlikcepatKey Step = "settings_klikcepat_key"

	// Klikcepat Link CRUD
	StepKlikcepatAddType        Step = "klikcepat_add_type"
	StepKlikcepatAddTitle       Step = "klikcepat_add_title"
	StepKlikcepatAddSlug        Step = "klikcepat_add_slug"
	StepKlikcepatAddLocationURL Step = "klikcepat_add_location"
	StepKlikcepatAddProject     Step = "klikcepat_add_project"

	StepKlikcepatEditPickField Step = "klikcepat_edit_pickfield"
	StepKlikcepatEditValue     Step = "klikcepat_edit_value"

	// Klikcepat Project CRUD
	StepKlikcepatProjectAddName  Step = "klikcepat_project_add_name"
	StepKlikcepatProjectAddColor Step = "klikcepat_project_add_color"
	StepKlikcepatProjectEditName Step = "klikcepat_project_edit_name"

	// Klikcepat Rotator wizard (entry from Auto Rotator)
	StepKlikcepatRotatorPickLink Step = "klikcepat_rot_picklink"
	StepKlikcepatRotatorPickPool Step = "klikcepat_rot_pickpool"
	StepKlikcepatRotatorAddLabel Step = "klikcepat_rot_label"
```

- [ ] **Step 2: Add callback constants**

Edit `bot/menu.go`. Add after the existing const block (before `// ─── Menu Texts ─`):

```go
const (
	// Klikcepat root menu
	cbKlikcepat = "klikcepat"

	// Klikcepat Settings
	cbSettingsKlikcepat        = "settings_klikcepat"
	cbSettingsKlikcepatSetURL  = "settings_klc_url"
	cbSettingsKlikcepatSetKey  = "settings_klc_key"
	cbSettingsKlikcepatTest    = "settings_klc_test"
	cbSettingsKlikcepatClear   = "settings_klc_clear"

	// Klikcepat Link CRUD
	cbKlikcepatAdd        = "klc_add"
	cbKlikcepatAddType    = "klc_add_type"     // param = link type
	cbKlikcepatAddPickProject = "klc_add_proj" // param = project_id (0 = skip)
	cbKlikcepatList       = "klc_list"          // param = page index
	cbKlikcepatListByProj = "klc_list_proj"     // param = "projectID|page"
	cbKlikcepatEdit       = "klc_edit"          // param = link ID picker entry
	cbKlikcepatEditPick   = "klc_edit_pick"     // param = link ID
	cbKlikcepatEditField  = "klc_edit_field"    // param = field name
	cbKlikcepatDelete     = "klc_delete"        // entry → list picker
	cbKlikcepatDeletePick = "klc_delete_pick"   // param = link ID
	cbKlikcepatDeleteConfirm = "klc_del_yes"    // param = link ID
	cbKlikcepatOpenDashboard = "klc_dashboard"

	// Klikcepat Projects
	cbKlikcepatProjects     = "klc_projects"
	cbKlikcepatProjectAdd   = "klc_proj_add"
	cbKlikcepatProjectList  = "klc_proj_list"
	cbKlikcepatProjectEdit  = "klc_proj_edit"        // entry
	cbKlikcepatProjectEditPick = "klc_proj_edit_pick" // param = project ID
	cbKlikcepatProjectDelete = "klc_proj_del"
	cbKlikcepatProjectDeletePick = "klc_proj_del_pick"
	cbKlikcepatProjectDeleteConfirm = "klc_proj_del_yes"

	// Auto Rotator unified — pick type
	cbRotatorAddPickType    = "rotator_add_pick"      // entry to pick CF vs Klikcepat
	cbRotatorAddTypeCF      = "rotator_add_cf"
	cbRotatorAddTypeKlikcepat = "rotator_add_klc"

	// Klikcepat Rotator
	cbKlikcepatRotPickLink = "klc_rot_picklink"  // param = link ID
	cbKlikcepatRotPickPool = "klc_rot_pickpool"  // param = pool label
	cbKlikcepatRotToggle   = "klc_rot_toggle"    // param = rotator ID
	cbKlikcepatRotDelete   = "klc_rot_delete"    // param = rotator ID
	cbKlikcepatRotForce    = "klc_rot_force"     // param = rotator ID
)
```

- [ ] **Step 3: Refactor mainMenu() to 5-button layout**

Replace `mainMenu()` in `bot/menu.go`:

```go
func mainMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📡 Monitor", cbMonitor),
			m.Data("⚙️ CF Redirect", cbCF),
		),
		m.Row(
			m.Data("🔄 Auto Rotator", cbRotator),
			m.Data("🔗 KLIKCEPAT", cbKlikcepat),
		),
		m.Row(
			m.Data("🔧 Settings", cbSettings),
		),
	)
	return m
}
```

- [ ] **Step 4: Add klikcepatMenu() helper**

In `bot/menu.go`, add after `rotatorMenu()`:

```go
func klikcepatMenu(botUsername string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	rows := []tele.Row{
		m.Row(
			m.Data("➕ Tambah Link", cbKlikcepatAdd),
			m.Data("📋 List Link", cbKlikcepatList, "0"),
		),
		m.Row(
			m.Data("✏️ Edit Link", cbKlikcepatEdit),
			m.Data("🗑 Hapus Link", cbKlikcepatDelete),
		),
		m.Row(
			m.Data("📁 Projects", cbKlikcepatProjects),
		),
	}
	if botUsername != "" {
		// Dashboard quick-access (URL button)
		rows = append(rows, m.Row(
			m.URL("🌐 Open Dashboard", "https://klikcepat.com"),
		))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbMain)))
	m.Inline(rows...)
	return m
}

func backToKlikcepat() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbKlikcepat)))
	return m
}

func klikcepatProjectsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Tambah Project", cbKlikcepatProjectAdd),
			m.Data("📋 List Project", cbKlikcepatProjectList),
		),
		m.Row(
			m.Data("✏️ Edit Project", cbKlikcepatProjectEdit),
			m.Data("🗑 Hapus Project", cbKlikcepatProjectDelete),
		),
		m.Row(m.Data("🔙 Kembali", cbKlikcepat)),
	)
	return m
}

func backToKlikcepatProjects() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbKlikcepatProjects)))
	return m
}
```

- [ ] **Step 5: Add textKlikcepat constant**

In `bot/menu.go` near `textRotator`:

```go
	textKlikcepat = "🔗 *KLIKCEPAT — Bio Link & Short URL*\n\n" +
		"Manage link & project di klikcepat.com langsung dari bot.\n\n" +
		"*Yang bisa dilakuin:*\n" +
		"• ➕ *Tambah Link* — bikin shortlink/biolink page baru\n" +
		"• 📋 *List Link* — liat semua link kamu (paginated)\n" +
		"• ✏️ *Edit Link* — update title/slug/target URL/project\n" +
		"• 🗑 *Hapus Link* — delete link permanent\n" +
		"• 📁 *Projects* — manage project grouping\n\n" +
		"_💡 Auto-Swap setup ada di menu *🔄 Auto Rotator* (unified untuk CF + Klikcepat)._"

	textKlikcepatProjects = "📁 *KLIKCEPAT — Projects*\n\n" +
		"Group link berdasarkan project (misal: KONTAK, PROMO, RTP).\n\n" +
		"Project bisa di-assign saat create link, dan bisa difilter di List Link."
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`
Expected: no errors (Step type might need adjustment based on existing pattern — check session.go)

- [ ] **Step 7: Commit**

```bash
git add bot/menu.go bot/session.go
git commit -m "feat(bot): add klikcepat menu structure + session steps + 5-button main menu"
```

---

### Task 8: Wire klikcepat client + store into Handler + main.go

**Files:**
- Modify: `bot/bot.go`
- Modify: `main.go`
- Modify: `rotator/monitor_scanner.go` (add field — actual logic in Task 16)
- Create: `bot/klikcepat.go` (stub handlers for routing)

- [ ] **Step 1: Extend Handler struct**

In `bot/bot.go`, find `type Handler struct`. Add fields:

```go
type Handler struct {
	cfg            *config.Config
	bot            *tele.Bot
	sessions       *sessionStore
	domains        *store.DomainStore
	cfrules        *store.CFRuleStore
	rotators       *store.RotatorStore
	creds          *store.CredentialStore
	cf             *cloudflare.Client
	rotSvc         *rotator.Service
	monScanner     *rotator.MonitorScanner
	history        *store.HistoryStore
	klikcepat      *klikcepat.Client            // NEW
	klikcepatRotators *store.KlikcepatRotatorStore // NEW
}
```

Add import: `"bongbot/klikcepat"`

Update `New()` function signature:

```go
func New(
	b *tele.Bot,
	cfg *config.Config,
	domains *store.DomainStore,
	cfrules *store.CFRuleStore,
	rotators *store.RotatorStore,
	creds *store.CredentialStore,
	cf *cloudflare.Client,
	rotSvc *rotator.Service,
	monScanner *rotator.MonitorScanner,
	history *store.HistoryStore,
	klc *klikcepat.Client,
	klcRotators *store.KlikcepatRotatorStore,
) *Handler {
	return &Handler{
		cfg:               cfg,
		bot:               b,
		sessions:          newSessionStore(),
		domains:           domains,
		cfrules:           cfrules,
		rotators:          rotators,
		creds:             creds,
		cf:                cf,
		rotSvc:            rotSvc,
		monScanner:        monScanner,
		history:           history,
		klikcepat:         klc,
		klikcepatRotators: klcRotators,
	}
}
```

- [ ] **Step 2: Create stub `bot/klikcepat.go` with handleKlikcepat root handler**

```go
package bot

import (
	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Root Handler ──────────────────────────────────────────────────

func (h *Handler) handleKlikcepat(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\n"+
				"Set Base URL & API Key dulu lewat menu *🔧 Settings → 🔗 Klikcepat* sebelum pakai fitur ini.",
			backToMain(), tele.ModeMarkdown,
		)
	}
	return c.Edit(textKlikcepat, klikcepatMenu(h.cfg.BotUsername), tele.ModeMarkdown)
}
```

- [ ] **Step 3: Route cbKlikcepat in handleCallback**

In `bot/bot.go`, find the `switch unique {` in `handleCallback`. Add case (after `cbRotator`):

```go
		// Klikcepat
		case cbKlikcepat:
			return h.handleKlikcepat(c)
```

- [ ] **Step 4: Wire in main.go**

In `main.go`, after `cf := cloudflare.New(...)`, add:

```go
	// Klikcepat (klikcepat) integration — credentials prioritas: credentials.json > .env
	klcBaseURL := cfg.KlikcepatBaseURL
	klcAPIKey := cfg.KlikcepatAPIKey
	if cred := creds.Get(); cred.KlikcepatBaseURL != "" || cred.KlikcepatAPIKey != "" {
		if cred.KlikcepatBaseURL != "" {
			klcBaseURL = cred.KlikcepatBaseURL
		}
		if cred.KlikcepatAPIKey != "" {
			klcAPIKey = cred.KlikcepatAPIKey
		}
		log.Printf("✅ Klikcepat credentials loaded dari data/credentials.json")
	}
	klc := klikcepat.New(klcBaseURL, klcAPIKey)
	if klc.HasCredentials() {
		log.Printf("✅ Klikcepat client siap (base=%s)", klcBaseURL)
	} else {
		log.Printf("⚠️  Klikcepat credentials belum di-set — fitur klikcepat disabled. Pakai menu 🔧 Settings → 🔗 Klikcepat.")
	}

	klcRotators := store.NewKlikcepatRotatorStore()
```

Update `bot.New(...)` call to pass new args:

```go
	h := bot.New(b, cfg, domains, cfrules, rotators, creds, cf, rotSvc, monScanner, history, klc, klcRotators)
```

Add import to `main.go`: `"bongbot/klikcepat"`

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add bot/bot.go bot/klikcepat.go main.go
git commit -m "feat(bot): wire klikcepat client + rotator store into Handler"
```

---

## Phase 4: Settings Integration

### Task 9: Klikcepat settings handlers

**Files:**
- Create: `bot/settings_klikcepat.go`
- Modify: `bot/settings.go` (add tombol Klikcepat di settings menu)
- Modify: `bot/bot.go` (route callbacks)

- [ ] **Step 1: Add tombol Klikcepat di settings menu**

Edit `bot/menu.go`, find `settingsMenu()`. Add a row after existing rows (before Kembali):

```go
		m.Row(
			m.Data("🔗 Klikcepat Settings", cbSettingsKlikcepat),
		),
```

- [ ] **Step 2: Create `bot/settings_klikcepat.go`**

```go
package bot

import (
	"fmt"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleSettingsKlikcepat(c tele.Context) error {
	cred := h.creds.Get()
	baseURL := cred.KlikcepatBaseURL
	apiKey := cred.KlikcepatAPIKey

	statusURL := "❌ belum di-set"
	if baseURL != "" {
		statusURL = "✅ `" + baseURL + "`"
	}
	statusKey := "❌ belum di-set"
	if apiKey != "" {
		statusKey = "✅ `" + store.MaskAPIKey(apiKey) + "`"
	}

	text := fmt.Sprintf(
		"🔗 *Klikcepat Settings*\n\n"+
			"🌐 *Base URL:* %s\n"+
			"🔑 *API Key:* %s\n\n"+
			"_Setup: enable API di plan klikcepat → normal admin generate API key → paste di sini._",
		statusURL, statusKey,
	)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🌐 Set Base URL", cbSettingsKlikcepatSetURL),
			m.Data("🔑 Set API Key", cbSettingsKlikcepatSetKey),
		),
		m.Row(
			m.Data("✅ Test Koneksi", cbSettingsKlikcepatTest),
		),
		m.Row(
			m.Data("🗑 Hapus Credentials", cbSettingsKlikcepatClear),
		),
		m.Row(m.Data("🔙 Kembali", cbSettings)),
	)
	return c.Edit(text, m, tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatSetURL(c tele.Context) error {
	h.cancelPriorPrompt(c, StepSettingsKlikcepatURL)
	prompt := "🌐 *Set Klikcepat Base URL*\n\n" +
		"Ketik URL klikcepat kamu (tanpa trailing slash).\n\n" +
		"*Contoh:* `https://klikcepat.com`"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsKlikcepatURL,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardSettingsKlikcepatURL(c tele.Context, sess *Session) error {
	h.showTyping(c)
	url := strings.TrimSpace(c.Text())
	if url == "" || (!strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://")) {
		return h.reply(c, "❌ URL invalid — harus mulai dengan http:// atau https://", cancelMenu())
	}
	h.creds.SetKlikcepatBaseURL(url)
	h.applyKlikcepatCreds()
	h.sessions.Delete(c.Sender().ID)
	return h.reply(c,
		fmt.Sprintf("✅ Base URL tersimpan: `%s`", escapeMD(url)),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatSetKey(c tele.Context) error {
	h.cancelPriorPrompt(c, StepSettingsKlikcepatKey)
	prompt := "🔑 *Set Klikcepat API Key*\n\n" +
		"Ketik API key dari klikcepat (normal admin account).\n\n" +
		"📍 *Cara ambil:*\n" +
		"1. Login klikcepat → Account → API\n" +
		"2. Klik *Generate API Key*\n" +
		"3. Copy → paste di sini\n\n" +
		"🔒 _Pesan kamu akan otomatis dihapus setelah disimpan._"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsKlikcepatKey,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardSettingsKlikcepatKey(c tele.Context, sess *Session) error {
	h.showTyping(c)
	key := strings.TrimSpace(c.Text())
	if len(key) < 20 {
		return h.reply(c, "⚠️ API key terlalu pendek. Coba lagi:", cancelMenu())
	}
	h.creds.SetKlikcepatAPIKey(key)
	h.applyKlikcepatCreds()
	h.sessions.Delete(c.Sender().ID)
	_ = h.bot.Delete(c.Message())
	return h.reply(c,
		fmt.Sprintf("✅ API Key tersimpan: `%s`\n\n_Pesan API key kamu sudah dihapus._",
			escapeMD(store.MaskAPIKey(key))),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatTest(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit("⚠️ Credentials belum lengkap. Set Base URL & API Key dulu.",
			backToSettings(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Testing koneksi ke Klikcepat...", backToSettings())
	if err := h.klikcepat.Ping(); err != nil {
		return c.Edit(
			fmt.Sprintf("❌ *Test GAGAL*\n\n```\n%s\n```\n\nCek Base URL & API Key.", escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown)
	}
	return c.Edit(
		"✅ *Test BERHASIL*\n\nCredentials valid — Klikcepat API responding.",
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatClear(c tele.Context) error {
	h.creds.ClearKlikcepat()
	h.applyKlikcepatCreds()
	return c.Edit("✅ *Klikcepat credentials dihapus.*", backToSettings(), tele.ModeMarkdown)
}

// applyKlikcepatCreds re-syncs the client with latest stored credentials.
func (h *Handler) applyKlikcepatCreds() {
	cred := h.creds.Get()
	if cred.KlikcepatBaseURL != "" || cred.KlikcepatAPIKey != "" {
		h.klikcepat.SetCredentials(cred.KlikcepatBaseURL, cred.KlikcepatAPIKey)
	}
}
```

Add `import "strings"` at top.

- [ ] **Step 3: Route callbacks in handleCallback + text wizards**

In `bot/bot.go` switch `unique`, add (after Klikcepat root):

```go
		case cbSettingsKlikcepat:
			return h.handleSettingsKlikcepat(c)
		case cbSettingsKlikcepatSetURL:
			return h.handleSettingsKlikcepatSetURL(c)
		case cbSettingsKlikcepatSetKey:
			return h.handleSettingsKlikcepatSetKey(c)
		case cbSettingsKlikcepatTest:
			return h.handleSettingsKlikcepatTest(c)
		case cbSettingsKlikcepatClear:
			return h.handleSettingsKlikcepatClear(c)
```

In `handleText`, add (after existing settings cases):

```go
		case StepSettingsKlikcepatURL:
			return h.wizardSettingsKlikcepatURL(c, sess)
		case StepSettingsKlikcepatKey:
			return h.wizardSettingsKlikcepatKey(c, sess)
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add bot/settings_klikcepat.go bot/settings.go bot/bot.go bot/menu.go
git commit -m "feat(bot): klikcepat settings — set URL/API key + test koneksi"
```

---

## Phase 5: Klikcepat CRUD — Links

### Task 10: Add Link wizard

**Files:**
- Modify: `bot/klikcepat.go`
- Modify: `bot/bot.go` (route callbacks + text wizards)

- [ ] **Step 1: Add wizard handlers to `bot/klikcepat.go`**

Append to `bot/klikcepat.go`:

```go
import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// ─── Add Link wizard ─────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatAdd(c tele.Context) error {
	h.cancelPriorPrompt(c, StepKlikcepatAddType)
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Setup Klikcepat credentials dulu", ShowAlert: true})
	}

	prompt := "➕ *Tambah Link Klikcepat — Step 1/5: Pilih Tipe*\n\n" +
		"_Tipe link menentukan behavior klikcepat:_\n" +
		"• *🔗 Shortlink* — URL pendek redirect ke 1 target\n" +
		"• *📄 Biolink* — landing page bio (kayak Linktree)\n" +
		"• Lainnya: VCard, Event, File, Static"

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🔗 Shortlink", cbKlikcepatAddType, "link"),
			m.Data("📄 Biolink", cbKlikcepatAddType, "biolink"),
		),
		m.Row(
			m.Data("📇 VCard", cbKlikcepatAddType, "vcard"),
			m.Data("📅 Event", cbKlikcepatAddType, "event"),
		),
		m.Row(
			m.Data("❌ Batal", cbCancel),
		),
	)
	msg, _ := h.bot.Edit(c.Message(), prompt, m, tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatAddType,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) handleKlikcepatAddType(c tele.Context) error {
	linkType := extractParam(c)
	if linkType == "" {
		return h.handleKlikcepatAdd(c)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatAddType {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["type"] = linkType
	sess.Step = StepKlikcepatAddTitle
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf("➕ *Step 2/5: Title*\n\nTipe: *%s* ✅\n\nKetik *title* untuk link ini:\n\n*Contoh:* `Promo Maha 2026`", linkType)
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatAddTitle(c tele.Context, sess *Session) error {
	h.showTyping(c)
	title := strings.TrimSpace(c.Text())
	if title == "" {
		return h.reply(c, "❌ Title kosong, coba lagi:", cancelMenu())
	}
	sess.Data["title"] = title
	sess.Step = StepKlikcepatAddSlug
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"➕ *Step 3/5: Slug*\n\nTitle: *%s* ✅\n\n"+
			"Ketik *slug* (path URL setelah klikcepat.com/) atau ketik `-` untuk auto-generate:\n\n"+
			"*Contoh:* `promo-maha` → klikcepat.com/promo-maha", escapeMD(title))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardKlikcepatAddSlug(c tele.Context, sess *Session) error {
	h.showTyping(c)
	slug := strings.TrimSpace(c.Text())
	if slug == "-" {
		slug = ""
	}
	sess.Data["slug"] = slug
	sess.Step = StepKlikcepatAddLocationURL
	h.sessions.Set(c.Sender().ID, sess)

	slugDisplay := slug
	if slug == "" {
		slugDisplay = "(auto)"
	}
	prompt := fmt.Sprintf(
		"➕ *Step 4/5: Location URL*\n\nSlug: *%s* ✅\n\n"+
			"Ketik *target URL* (kemana redirect tujunya):\n\n"+
			"*Contoh:* `https://maha-supreme.com/daftar`", escapeMD(slugDisplay))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardKlikcepatAddLocation(c tele.Context, sess *Session) error {
	h.showTyping(c)
	loc := strings.TrimSpace(c.Text())
	if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") {
		loc = "https://" + loc
	}
	sess.Data["location_url"] = loc
	sess.Step = StepKlikcepatAddProject
	h.sessions.Set(c.Sender().ID, sess)

	// Fetch projects for picker
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		// Skip project step if API fails
		return h.doKlikcepatAddCreate(c, sess, 0)
	}
	if len(projects) == 0 {
		// No projects, skip
		return h.doKlikcepatAddCreate(c, sess, 0)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	rows = append(rows, m.Row(m.Data("⏭ Skip (no project)", cbKlikcepatAddPickProject, "0")))
	for _, p := range projects {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📁 %s", p.Name),
			cbKlikcepatAddPickProject, strconv.Itoa(p.ID))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	prompt := fmt.Sprintf(
		"➕ *Step 5/5: Project*\n\nLocation: *%s* ✅\n\n"+
			"Pilih project (atau Skip):", escapeMD(loc))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: m})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) handleKlikcepatAddPickProject(c tele.Context) error {
	projectIDStr := extractParam(c)
	projectID, _ := strconv.Atoi(projectIDStr)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatAddProject {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	return h.doKlikcepatAddCreate(c, sess, projectID)
}

func (h *Handler) doKlikcepatAddCreate(c tele.Context, sess *Session, projectID int) error {
	h.sessions.Delete(c.Sender().ID)
	linkType := sess.Data["type"]
	title := sess.Data["title"]
	slug := sess.Data["slug"]
	locURL := sess.Data["location_url"]

	loadingText := fmt.Sprintf("⏳ Membuat link `%s`...", escapeMD(title))
	loadingMsg, _ := h.bot.Send(c.Chat(), loadingText, tele.ModeMarkdown)

	link, err := h.klikcepat.CreateLink(linkType, title, slug, locURL, projectID)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal create link*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepat(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepat(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf(
		"✅ *Link dibuat!*\n\n"+
			"📛 Title: *%s*\n"+
			"🔗 Slug: `%s`\n"+
			"🎯 Target: `%s`\n"+
			"📌 Type: *%s*\n"+
			"🆔 ID: `%d`",
		escapeMD(link.Title), escapeMD(link.URL), escapeMD(link.LocationURL),
		link.Type, link.ID)

	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepat(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepat(), tele.ModeMarkdown)
}
```

- [ ] **Step 2: Route callbacks + text wizards in bot.go**

In `handleCallback` switch:

```go
		case cbKlikcepatAdd:
			return h.handleKlikcepatAdd(c)
		case cbKlikcepatAddType:
			return h.handleKlikcepatAddType(c)
		case cbKlikcepatAddPickProject:
			return h.handleKlikcepatAddPickProject(c)
```

In `handleText` switch:

```go
		case StepKlikcepatAddTitle:
			return h.wizardKlikcepatAddTitle(c, sess)
		case StepKlikcepatAddSlug:
			return h.wizardKlikcepatAddSlug(c, sess)
		case StepKlikcepatAddLocationURL:
			return h.wizardKlikcepatAddLocation(c, sess)
		// StepKlikcepatAddType & StepKlikcepatAddProject handled via callback button
		case StepKlikcepatAddType, StepKlikcepatAddProject:
			return nil
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 4: Manual test**

Run bot locally or deploy → DM bot → 🔗 KLIKCEPAT → ➕ Tambah Link.
Walk through wizard with test data → verify link muncul di klikcepat.com dashboard.

- [ ] **Step 5: Commit**

```bash
git add bot/klikcepat.go bot/bot.go
git commit -m "feat(klikcepat): Add Link wizard (5-step type/title/slug/location/project)"
```

---

### Task 11: List Link (paginated, with project filter)

**Files:**
- Modify: `bot/klikcepat.go`
- Modify: `bot/bot.go`

- [ ] **Step 1: Add List handler**

Append to `bot/klikcepat.go`:

```go
const klikcepatLinksPerPage = 10

func (h *Handler) handleKlikcepatList(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Setup credentials dulu", ShowAlert: true})
	}
	pageStr := extractParam(c)
	if pageStr == "" {
		pageStr = "0"
	}
	page, _ := strconv.Atoi(pageStr)

	c.Edit("⏳ Loading links dari klikcepat...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(
			fmt.Sprintf("❌ Gagal fetch links: %s", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	if len(links) == 0 {
		return c.Edit(
			"📭 Belum ada link di klikcepat. Klik *➕ Tambah Link* untuk mulai.",
			backToKlikcepat(), tele.ModeMarkdown)
	}

	total := len(links)
	totalPages := (total + klikcepatLinksPerPage - 1) / klikcepatLinksPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * klikcepatLinksPerPage
	end := start + klikcepatLinksPerPage
	if end > total {
		end = total
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *List Link Klikcepat* — page %d/%d (total %d)\n═══════════════════════════\n\n", page+1, totalPages, total))
	for i := start; i < end; i++ {
		l := links[i]
		typeIcon := "🔗"
		switch l.Type {
		case "biolink":
			typeIcon = "📄"
		case "vcard":
			typeIcon = "📇"
		case "event":
			typeIcon = "📅"
		case "file":
			typeIcon = "📁"
		case "static":
			typeIcon = "📌"
		}
		enabled := "✅"
		if l.IsEnabled == 0 {
			enabled = "⛔"
		}
		sb.WriteString(fmt.Sprintf("%s *%s* (`/%s`) %s\n", typeIcon, escapeMD(l.Title), escapeMD(l.URL), enabled))
		if l.LocationURL != "" {
			sb.WriteString(fmt.Sprintf("   🎯 `%s`\n", escapeMD(l.LocationURL)))
		}
		sb.WriteString(fmt.Sprintf("   🆔 `%d`\n\n", l.ID))
	}

	m := &tele.ReplyMarkup{}
	var navRow tele.Row
	if page > 0 {
		navRow = append(navRow, m.Data("⬅️ Prev", cbKlikcepatList, strconv.Itoa(page-1)))
	}
	navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
	if page < totalPages-1 {
		navRow = append(navRow, m.Data("Next ➡️", cbKlikcepatList, strconv.Itoa(page+1)))
	}
	rows := []tele.Row{navRow, m.Row(m.Data("🔙 Kembali", cbKlikcepat))}
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}
```

- [ ] **Step 2: Route callback**

In `handleCallback` switch:

```go
		case cbKlikcepatList:
			return h.handleKlikcepatList(c)
```

- [ ] **Step 3: Verify build + commit**

```bash
go build ./...
git add bot/klikcepat.go bot/bot.go
git commit -m "feat(klikcepat): List Link paginated view"
```

---

### Task 12: Edit Link wizard

**Files:**
- Modify: `bot/klikcepat.go`
- Modify: `bot/bot.go`

- [ ] **Step 1: Add Edit wizard handlers**

Append to `bot/klikcepat.go`:

```go
// ─── Edit Link wizard ────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatEdit(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Setup credentials dulu", ShowAlert: true})
	}
	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	if len(links) == 0 {
		return c.Edit("📭 Belum ada link.", backToKlikcepat(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	maxPerRow := 1
	for _, l := range links {
		if len(rows) >= 30 {
			break // pagination optional later
		}
		btn := m.Data(
			fmt.Sprintf("✏️ %s (/%s)", truncate(l.Title, 30), l.URL),
			cbKlikcepatEditPick, strconv.Itoa(l.ID))
		row := tele.Row{btn}
		_ = maxPerRow
		rows = append(rows, row)
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepat)))
	m.Inline(rows...)

	return c.Edit("✏️ *Edit Link — Pilih link yang mau di-edit:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatEditPick(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID == 0 {
		return h.handleKlikcepatEdit(c)
	}
	link, err := h.klikcepat.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch link: %s", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatEditPickField,
		Data:      map[string]string{"link_id": linkIDStr},
		PromptMsg: c.Message(),
	})

	prompt := fmt.Sprintf(
		"✏️ *Edit Link*\n\n"+
			"📛 Title: *%s*\n"+
			"🔗 Slug: `%s`\n"+
			"🎯 Target: `%s`\n"+
			"📌 Type: *%s*\n\n"+
			"Pilih field yang mau di-edit:",
		escapeMD(link.Title), escapeMD(link.URL), escapeMD(link.LocationURL), link.Type)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📛 Title", cbKlikcepatEditField, "title"),
			m.Data("🔗 Slug", cbKlikcepatEditField, "url"),
		),
		m.Row(
			m.Data("🎯 Location URL", cbKlikcepatEditField, "location_url"),
		),
		m.Row(m.Data("❌ Batal", cbCancel)),
	)
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatEditField(c tele.Context) error {
	field := extractParam(c)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatEditPickField {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["field"] = field
	sess.Step = StepKlikcepatEditValue
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf("✏️ Ketik nilai baru untuk *%s*:", field)
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatEditValue(c tele.Context, sess *Session) error {
	h.showTyping(c)
	val := strings.TrimSpace(c.Text())
	if val == "" {
		return h.reply(c, "❌ Nilai kosong, coba lagi:", cancelMenu())
	}
	linkID, _ := strconv.Atoi(sess.Data["link_id"])
	field := sess.Data["field"]
	h.sessions.Delete(c.Sender().ID)

	if field == "location_url" {
		if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
			val = "https://" + val
		}
	}

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Updating link...", tele.ModeMarkdown)

	_, err := h.klikcepat.UpdateLink(linkID, map[string]string{field: val})
	if err != nil {
		errText := fmt.Sprintf("❌ *Update gagal*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepat(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepat(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf("✅ Link ID `%d` updated!\nField *%s* = `%s`",
		linkID, field, escapeMD(val))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepat(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepat(), tele.ModeMarkdown)
}
```

- [ ] **Step 2: Route callbacks + text wizard**

In `handleCallback`:

```go
		case cbKlikcepatEdit:
			return h.handleKlikcepatEdit(c)
		case cbKlikcepatEditPick:
			return h.handleKlikcepatEditPick(c)
		case cbKlikcepatEditField:
			return h.handleKlikcepatEditField(c)
```

In `handleText`:

```go
		case StepKlikcepatEditValue:
			return h.wizardKlikcepatEditValue(c, sess)
		case StepKlikcepatEditPickField:
			return nil // callback-only
```

- [ ] **Step 3: Verify build + commit**

```bash
go build ./...
git add bot/klikcepat.go bot/bot.go
git commit -m "feat(klikcepat): Edit Link wizard (pick link → pick field → input value)"
```

---

### Task 13: Delete Link with confirmation

**Files:**
- Modify: `bot/klikcepat.go`
- Modify: `bot/bot.go`

- [ ] **Step 1: Add Delete handlers**

Append to `bot/klikcepat.go`:

```go
func (h *Handler) handleKlikcepatDelete(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Setup credentials dulu", ShowAlert: true})
	}
	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	if len(links) == 0 {
		return c.Edit("📭 Belum ada link.", backToKlikcepat(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, l := range links {
		if len(rows) >= 30 {
			break
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 %s (/%s)", truncate(l.Title, 30), l.URL),
			cbKlikcepatDeletePick, strconv.Itoa(l.ID))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepat)))
	m.Inline(rows...)

	return c.Edit("🗑 *Hapus Link — Pilih link yang mau dihapus:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatDeletePick(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID == 0 {
		return h.handleKlikcepatDelete(c)
	}
	link, err := h.klikcepat.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}

	prompt := fmt.Sprintf(
		"⚠️ *Konfirmasi Hapus*\n\n"+
			"📛 Title: *%s*\n"+
			"🔗 Slug: `%s`\n"+
			"🎯 Target: `%s`\n\n"+
			"Yakin mau hapus? Action ini *tidak bisa di-undo*.",
		escapeMD(link.Title), escapeMD(link.URL), escapeMD(link.LocationURL))

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🗑 Ya, Hapus", cbKlikcepatDeleteConfirm, linkIDStr),
			m.Data("❌ Batal", cbKlikcepat),
		),
	)
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatDeleteConfirm(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID == 0 {
		return c.Edit("❌ Invalid link ID", backToKlikcepat(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Deleting...", tele.ModeMarkdown)
	if err := h.klikcepat.DeleteLink(linkID); err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal hapus: %s", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	return c.Edit(fmt.Sprintf("✅ Link ID `%d` berhasil dihapus.", linkID),
		backToKlikcepat(), tele.ModeMarkdown)
}
```

- [ ] **Step 2: Route callbacks**

In `handleCallback`:

```go
		case cbKlikcepatDelete:
			return h.handleKlikcepatDelete(c)
		case cbKlikcepatDeletePick:
			return h.handleKlikcepatDeletePick(c)
		case cbKlikcepatDeleteConfirm:
			return h.handleKlikcepatDeleteConfirm(c)
```

- [ ] **Step 3: Verify build + commit**

```bash
go build ./...
git add bot/klikcepat.go bot/bot.go
git commit -m "feat(klikcepat): Delete Link with confirmation"
```

---

## Phase 6: Klikcepat CRUD — Projects

### Task 14: Project CRUD (Add/List/Edit/Delete)

**Files:**
- Create: `bot/klikcepat_projects.go`
- Modify: `bot/bot.go`

- [ ] **Step 1: Create `bot/klikcepat_projects.go`**

```go
package bot

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleKlikcepatProjects(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Setup credentials dulu", ShowAlert: true})
	}
	return c.Edit(textKlikcepatProjects, klikcepatProjectsMenu(), tele.ModeMarkdown)
}

// ─── Add Project ─────────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectAdd(c tele.Context) error {
	h.cancelPriorPrompt(c, StepKlikcepatProjectAddName)
	prompt := "📁 *Tambah Project — Step 1/2: Name*\n\nKetik nama project:\n\n*Contoh:* `Promo Mahaslot`"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatProjectAddName,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardKlikcepatProjectAddName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	name := strings.TrimSpace(c.Text())
	if name == "" {
		return h.reply(c, "❌ Name kosong, coba lagi:", cancelMenu())
	}
	sess.Data["name"] = name
	sess.Step = StepKlikcepatProjectAddColor
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"📁 *Step 2/2: Color*\n\nName: *%s* ✅\n\n"+
			"Ketik *hex color* untuk project (atau `-` untuk default `#000000`):\n\n"+
			"*Contoh:* `#FF5733`", escapeMD(name))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardKlikcepatProjectAddColor(c tele.Context, sess *Session) error {
	h.showTyping(c)
	color := strings.TrimSpace(c.Text())
	if color == "-" {
		color = ""
	}
	name := sess.Data["name"]
	h.sessions.Delete(c.Sender().ID)

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Creating project...", tele.ModeMarkdown)

	proj, err := h.klikcepat.CreateProject(name, color)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal create project*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf(
		"✅ *Project dibuat!*\n\n"+
			"📁 Name: *%s*\n"+
			"🎨 Color: `%s`\n"+
			"🆔 ID: `%d`",
		escapeMD(proj.Name), proj.Color, proj.ID)
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
}

// ─── List Projects ───────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectList(c tele.Context) error {
	c.Edit("⏳ Loading projects...", tele.ModeMarkdown)
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *Klikcepat Projects* (total %d)\n═══════════════════════════\n\n", len(projects)))
	for _, p := range projects {
		sb.WriteString(fmt.Sprintf("📁 *%s* (🎨 `%s`) — 🆔 `%d`\n", escapeMD(p.Name), p.Color, p.ID))
	}

	return c.Edit(sb.String(), backToKlikcepatProjects(), tele.ModeMarkdown)
}

// ─── Edit Project ────────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectEdit(c tele.Context) error {
	c.Edit("⏳ Loading...", tele.ModeMarkdown)
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("✏️ %s", truncate(p.Name, 40)),
			cbKlikcepatProjectEditPick, strconv.Itoa(p.ID))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepatProjects)))
	m.Inline(rows...)

	return c.Edit("✏️ *Pilih project yang mau di-edit:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatProjectEditPick(c tele.Context) error {
	projIDStr := extractParam(c)
	projID, _ := strconv.Atoi(projIDStr)
	if projID == 0 {
		return h.handleKlikcepatProjectEdit(c)
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatProjectEditName,
		Data:      map[string]string{"project_id": projIDStr},
		PromptMsg: c.Message(),
	})
	prompt := "✏️ Ketik nilai baru, format `name|color` (color opsional):\n\n" +
		"*Contoh:* `Promo Baru|#FF0000`\n_(ketik `name|` aja kalau cuma rename, color preserved)_"
	h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatProjectEditName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	raw := strings.TrimSpace(c.Text())
	if raw == "" {
		return h.reply(c, "❌ Kosong, coba lagi:", cancelMenu())
	}
	parts := strings.SplitN(raw, "|", 2)
	name := strings.TrimSpace(parts[0])
	color := ""
	if len(parts) > 1 {
		color = strings.TrimSpace(parts[1])
	}
	projID, _ := strconv.Atoi(sess.Data["project_id"])
	h.sessions.Delete(c.Sender().ID)

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Updating project...", tele.ModeMarkdown)
	_, err := h.klikcepat.UpdateProject(projID, name, color)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal update*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	successText := fmt.Sprintf("✅ Project ID `%d` updated!\nName: *%s*", projID, escapeMD(name))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
}

// ─── Delete Project ──────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectDelete(c tele.Context) error {
	c.Edit("⏳ Loading...", tele.ModeMarkdown)
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 %s", truncate(p.Name, 40)),
			cbKlikcepatProjectDeletePick, strconv.Itoa(p.ID))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepatProjects)))
	m.Inline(rows...)

	return c.Edit("🗑 *Pilih project yang mau dihapus:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatProjectDeletePick(c tele.Context) error {
	projIDStr := extractParam(c)
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🗑 Ya, Hapus", cbKlikcepatProjectDeleteConfirm, projIDStr),
			m.Data("❌ Batal", cbKlikcepatProjects),
		),
	)
	return c.Edit(fmt.Sprintf("⚠️ Yakin hapus project ID `%s`?", projIDStr), m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatProjectDeleteConfirm(c tele.Context) error {
	projIDStr := extractParam(c)
	projID, _ := strconv.Atoi(projIDStr)
	c.Edit("⏳ Deleting...", tele.ModeMarkdown)
	if err := h.klikcepat.DeleteProject(projID); err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal: %s", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	return c.Edit(fmt.Sprintf("✅ Project ID `%d` dihapus.", projID),
		backToKlikcepatProjects(), tele.ModeMarkdown)
}
```

- [ ] **Step 2: Route callbacks + text wizards**

In `handleCallback`:

```go
		case cbKlikcepatProjects:
			return h.handleKlikcepatProjects(c)
		case cbKlikcepatProjectAdd:
			return h.handleKlikcepatProjectAdd(c)
		case cbKlikcepatProjectList:
			return h.handleKlikcepatProjectList(c)
		case cbKlikcepatProjectEdit:
			return h.handleKlikcepatProjectEdit(c)
		case cbKlikcepatProjectEditPick:
			return h.handleKlikcepatProjectEditPick(c)
		case cbKlikcepatProjectDelete:
			return h.handleKlikcepatProjectDelete(c)
		case cbKlikcepatProjectDeletePick:
			return h.handleKlikcepatProjectDeletePick(c)
		case cbKlikcepatProjectDeleteConfirm:
			return h.handleKlikcepatProjectDeleteConfirm(c)
```

In `handleText`:

```go
		case StepKlikcepatProjectAddName:
			return h.wizardKlikcepatProjectAddName(c, sess)
		case StepKlikcepatProjectAddColor:
			return h.wizardKlikcepatProjectAddColor(c, sess)
		case StepKlikcepatProjectEditName:
			return h.wizardKlikcepatProjectEditName(c, sess)
```

- [ ] **Step 3: Verify build + commit**

```bash
go build ./...
git add bot/klikcepat_projects.go bot/bot.go
git commit -m "feat(klikcepat): Project CRUD (Add/List/Edit/Delete)"
```

---

## Phase 7: Unified Auto Rotator

### Task 15: Refactor Auto Rotator entry to pick type first

**Files:**
- Modify: `bot/autorotate.go`
- Create: `bot/klikcepat_rotator.go`
- Modify: `bot/bot.go`

- [ ] **Step 1: Refactor `handleRotatorAdd` to show type picker first**

Edit `bot/autorotate.go`. Find `handleRotatorAdd` function. Replace BODY with:

```go
func (h *Handler) handleRotatorAdd(c tele.Context) error {
	// NEW: Pick type first (CF vs Klikcepat)
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("⚙️ CF Redirect", cbRotatorAddTypeCF),
			m.Data("🔗 KLIKCEPAT", cbRotatorAddTypeKlikcepat),
		),
		m.Row(m.Data("❌ Batal", cbCancel)),
	)
	return c.Edit(
		"🔄 *Setup Rotator — Pilih Tipe*\n\n"+
			"Pilih platform mana yang mau di-setup auto-swap-nya:\n\n"+
			"• *⚙️ CF Redirect* — auto-swap target URL Cloudflare rule\n"+
			"• *🔗 KLIKCEPAT* — auto-swap location_url link klikcepat",
		m, tele.ModeMarkdown)
}

// handleRotatorAddTypeCF — existing flow (was handleRotatorAdd before refactor)
func (h *Handler) handleRotatorAddTypeCF(c tele.Context) error {
	// Copy original logic from previous handleRotatorAdd here:
	// (the part that lists CF rules dengan filter belum punya rotator)
	allRules := h.cfrules.GetAll()
	if len(allRules) == 0 {
		return c.Edit(
			"⚠️ *Belum ada CF Rule terdaftar*\n\n"+
				"Auto Rotator butuh CF Rule. Tambah dulu via *⚙️ CF Redirect → ➕ Add Rule*.",
			backToRotator(), tele.ModeMarkdown)
	}
	hasRotator := make(map[string]bool)
	for _, rot := range h.rotators.GetAll() {
		hasRotator[rot.CFRuleID] = true
	}
	var rules []store.CFRule
	for _, r := range allRules {
		if !hasRotator[r.ID] {
			rules = append(rules, r)
		}
	}
	if len(rules) == 0 {
		return c.Edit(
			fmt.Sprintf("✅ Semua CF Rule udah punya Rotator (%d total).\n\nHapus rotator lama dulu via *📋 List Rotator*.", len(allRules)),
			backToRotator(), tele.ModeMarkdown)
	}
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, r := range rules {
		btn := "⚙️ " + r.Label
		if r.Domain != "" {
			btn = fmt.Sprintf("⚙️ %s (%s)", r.Label, r.Domain)
		}
		rows = append(rows, m.Row(m.Data(btn, cbRotatorCFSel, r.ID)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	hiddenCount := len(allRules) - len(rules)
	hiddenNote := ""
	if hiddenCount > 0 {
		hiddenNote = fmt.Sprintf("\n_(%d rule udah punya rotator — disembunyikan.)_", hiddenCount)
	}
	return c.Edit(
		fmt.Sprintf("🔄 *Setup CF Rotator — Pilih CF Rule:*%s", hiddenNote),
		m, tele.ModeMarkdown)
}
```

> Note: Move the CF logic that was previously in `handleRotatorAdd` into `handleRotatorAddTypeCF`. The new `handleRotatorAdd` only shows type picker.

- [ ] **Step 2: Create `bot/klikcepat_rotator.go`**

```go
package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// handleRotatorAddTypeKlikcepat — Klikcepat rotator wizard entry
func (h *Handler) handleRotatorAddTypeKlikcepat(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Loading links dari klikcepat...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch: %s", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	// Filter: skip links yang udah punya rotator
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatRotators.GetAll() {
		hasRotator[rot.LinkID] = true
	}
	var picks []klikcepatLinkPick
	for _, l := range links {
		if hasRotator[l.ID] {
			continue
		}
		// Only link & biolink types make sense for auto-swap
		if l.Type != "link" && l.Type != "biolink" {
			continue
		}
		picks = append(picks, klikcepatLinkPick{ID: l.ID, URL: l.URL, Type: l.Type, Title: l.Title})
	}
	if len(picks) == 0 {
		return c.Edit(
			"✅ Semua link klikcepat udah punya rotator (atau gak ada link tipe link/biolink).\n\n"+
				"Hapus rotator lama via *📋 List Rotator* atau create link baru via *🔗 KLIKCEPAT → ➕ Tambah Link*.",
			backToRotator(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range picks {
		if len(rows) >= 30 {
			break
		}
		typeIcon := "🔗"
		if p.Type == "biolink" {
			typeIcon = "📄"
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("%s %s (/%s)", typeIcon, truncate(p.Title, 30), p.URL),
			cbKlikcepatRotPickLink, strconv.Itoa(p.ID))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit(
		"🔄 *Setup Klikcepat Rotator — Step 1/3: Pick Link*",
		m, tele.ModeMarkdown)
}

type klikcepatLinkPick struct {
	ID    int
	URL   string
	Type  string
	Title string
}

func (h *Handler) handleKlikcepatRotPickLink(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID == 0 {
		return h.handleRotatorAddTypeKlikcepat(c)
	}
	link, err := h.klikcepat.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch link: %s", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	// Pick pool label
	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit(
			"⚠️ Belum ada pool di Monitor. Add domain dulu via *📡 Monitor → ➕ Add Domain*.",
			backToRotator(), tele.ModeMarkdown)
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlikcepatRotatorPickPool,
		Data: map[string]string{
			"link_id":   linkIDStr,
			"link_url":  link.URL,
			"link_type": link.Type,
		},
		PromptMsg: c.Message(),
	})

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, lbl := range labels {
		domains := h.domains.GetByLabel(lbl)
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📂 %s (%d domain)", lbl, len(domains)),
			cbKlikcepatRotPickPool, lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	prompt := fmt.Sprintf(
		"🔄 *Setup Klikcepat Rotator — Step 2/3: Pick Pool*\n\n"+
			"🔗 Link: `/%s` (%s)\n"+
			"🎯 Current target: `%s`\n\n"+
			"Pilih pool domain (dari Monitor):",
		escapeMD(link.URL), link.Type, escapeMD(link.LocationURL))
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatRotPickPool(c tele.Context) error {
	pool := extractParam(c)
	if pool == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Pool kosong", ShowAlert: true})
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotatorPickPool {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["pool"] = pool
	sess.Step = StepKlikcepatRotatorAddLabel
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"🔄 *Step 3/3: Label Rotator*\n\n"+
			"🔗 Link: `/%s`\n"+
			"📂 Pool: *%s*\n\n"+
			"Ketik label untuk rotator ini (bebas, untuk identifikasi):\n\n"+
			"*Contoh:* `PROMO-MAHA-ROT`",
		escapeMD(sess.Data["link_url"]), escapeMD(pool))
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatRotatorAddLabel(c tele.Context, sess *Session) error {
	h.showTyping(c)
	label := strings.ToUpper(strings.TrimSpace(c.Text()))
	if label == "" {
		return h.reply(c, "❌ Label kosong, coba lagi:", cancelMenu())
	}
	linkID, _ := strconv.Atoi(sess.Data["link_id"])
	pool := sess.Data["pool"]
	linkURL := sess.Data["link_url"]
	linkType := sess.Data["link_type"]
	h.sessions.Delete(c.Sender().ID)

	rot := store.KlikcepatRotator{
		Label:     label,
		LinkID:    linkID,
		LinkURL:   linkURL,
		LinkType:  linkType,
		PoolLabel: pool,
	}
	if err := h.klikcepatRotators.Add(rot); err != nil {
		return h.reply(c, fmt.Sprintf("❌ Gagal save rotator: %s", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	return h.reply(c,
		fmt.Sprintf(
			"✅ *Klikcepat Rotator dibuat!*\n\n"+
				"📛 Label: *%s*\n"+
				"🔗 Link: `/%s`\n"+
				"📂 Pool: *%s*\n"+
				"🟢 Active",
			label, escapeMD(linkURL), escapeMD(pool)),
		backToRotator(), tele.ModeMarkdown)
}
```

- [ ] **Step 3: Route callbacks + text wizard**

In `handleCallback`:

```go
		case cbRotatorAddTypeCF:
			return h.handleRotatorAddTypeCF(c)
		case cbRotatorAddTypeKlikcepat:
			return h.handleRotatorAddTypeKlikcepat(c)
		case cbKlikcepatRotPickLink:
			return h.handleKlikcepatRotPickLink(c)
		case cbKlikcepatRotPickPool:
			return h.handleKlikcepatRotPickPool(c)
```

In `handleText`:

```go
		case StepKlikcepatRotatorAddLabel:
			return h.wizardKlikcepatRotatorAddLabel(c, sess)
		case StepKlikcepatRotatorPickLink, StepKlikcepatRotatorPickPool:
			return nil // callback-only
```

- [ ] **Step 4: Verify build + commit**

```bash
go build ./...
git add bot/autorotate.go bot/klikcepat_rotator.go bot/bot.go
git commit -m "feat(rotator): unified entry — pick CF or Klikcepat type before setup"
```

---

### Task 16: Update List Rotator to show grouped CF + Klikcepat

**Files:**
- Modify: `bot/autorotate.go`

- [ ] **Step 1: Update handleRotatorList**

Find `handleRotatorList` in `bot/autorotate.go`. Replace BODY:

```go
func (h *Handler) handleRotatorList(c tele.Context) error {
	cfRotators := h.rotators.GetAll()
	klcRotators := h.klikcepatRotators.GetAll()

	if len(cfRotators) == 0 && len(klcRotators) == 0 {
		return c.Edit(
			"📭 *Belum ada rotator config*\n\nKlik *➕ Setup Rotator* untuk mulai.",
			backToRotator(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Rotator List*\n═══════════════════════════\n")

	if len(cfRotators) > 0 {
		sb.WriteString("\n═══ ⚙️ CF Redirect ═══\n")
		activeCF := 0
		for _, r := range cfRotators {
			status := "⛔"
			if r.Active {
				status = "✅"
				activeCF++
			}
			sb.WriteString(fmt.Sprintf("%s *%s*\n   🔧 Rule: `%s` → 📂 Pool: `%s`\n",
				status, escapeMD(r.Label), escapeMD(r.CFRuleID), escapeMD(r.PoolLabel)))
		}
		sb.WriteString(fmt.Sprintf("_%d CF rotator (%d aktif)_\n", len(cfRotators), activeCF))
	}

	if len(klcRotators) > 0 {
		sb.WriteString("\n═══ 🔗 KLIKCEPAT ═══\n")
		activeKLC := 0
		for _, r := range klcRotators {
			status := "⛔"
			if r.Active {
				status = "✅"
				activeKLC++
			}
			typeIcon := "🔗"
			if r.LinkType == "biolink" {
				typeIcon = "📄"
			}
			sb.WriteString(fmt.Sprintf("%s *%s*\n   %s Link: `/%s` → 📂 Pool: `%s`\n",
				status, escapeMD(r.Label), typeIcon, escapeMD(r.LinkURL), escapeMD(r.PoolLabel)))
		}
		sb.WriteString(fmt.Sprintf("_%d Klikcepat rotator (%d aktif)_\n", len(klcRotators), activeKLC))
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("*Total:* %d CF + %d Klikcepat = %d rotator",
		len(cfRotators), len(klcRotators), len(cfRotators)+len(klcRotators)))

	// Action buttons for management — keep simple: per-rotator toggle/delete via separate menu
	// For now, just back button. Future: add inline action buttons per rotator.
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("🔙 Kembali", cbRotator)),
	)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}
```

- [ ] **Step 2: Verify build + commit**

```bash
go build ./...
git add bot/autorotate.go
git commit -m "feat(rotator): unified List Rotator view — CF + Klikcepat grouped"
```

---

## Phase 8: Auto-Swap Integration

### Task 17: Extend MonitorScanner with Klikcepat auto-swap

**Files:**
- Modify: `rotator/monitor_scanner.go`
- Modify: `main.go`

- [ ] **Step 1: Add fields to MonitorScanner**

Edit `rotator/monitor_scanner.go`. Find `type MonitorScanner struct`. Add fields:

```go
type MonitorScanner struct {
	cf                cfUpdater
	domains           *store.DomainStore
	cfrules           *store.CFRuleStore
	rotators          *store.RotatorStore
	notify            Notifier
	chk               *checker.Checker
	history           *store.HistoryStore
	klikcepat         KlikcepatUpdater                 // NEW
	klikcepatRotators *store.KlikcepatRotatorStore     // NEW

	// existing fields...
	mu      sync.Mutex
	blocked map[string]*blockCycle
	interval time.Duration
	cursor       int
	lastChunkNum int
	lastChunkOf  int
}
```

Update imports at top of `monitor_scanner.go` to include klikcepat:

```go
import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"bongbot/checker"
	"bongbot/klikcepat"
	"bongbot/store"
)
```

Add interface near `cfUpdater` (avoid hard dependency for future testability):

```go
// KlikcepatUpdater is the minimal interface MonitorScanner needs from klikcepat client.
// *klikcepat.Client satisfies this automatically.
type KlikcepatUpdater interface {
	HasCredentials() bool
	GetLink(id int) (*klikcepat.Link, error)
	UpdateLinkLocation(id int, locationURL string) error
}
```

Update `NewMonitorScanner` constructor:

```go
func NewMonitorScanner(
	cf cfUpdater,
	domains *store.DomainStore,
	cfrules *store.CFRuleStore,
	rotators *store.RotatorStore,
	notify Notifier,
	interval time.Duration,
	history *store.HistoryStore,
	klc KlikcepatUpdater,
	klcRotators *store.KlikcepatRotatorStore,
) *MonitorScanner {
	return &MonitorScanner{
		cf:                cf,
		domains:           domains,
		cfrules:           cfrules,
		rotators:          rotators,
		notify:            notify,
		chk:               checker.Default(),
		history:           history,
		blocked:           make(map[string]*blockCycle),
		interval:          interval,
		klikcepat:         klc,
		klikcepatRotators: klcRotators,
	}
}
```

- [ ] **Step 2: Extend triggerAutoSwap to scan klikcepat**

Find `triggerAutoSwap` function. At the END (after CF auto-swap logic), add:

```go
	// NEW: scan klikcepat rotators
	if ms.klikcepat != nil && ms.klikcepat.HasCredentials() && ms.klikcepatRotators != nil {
		ms.triggerKlikcepatAutoSwap(blockedDomain, blockedLabel)
	}
}

// triggerKlikcepatAutoSwap — scan klikcepat rotators, swap location_url if matches blocked.
func (ms *MonitorScanner) triggerKlikcepatAutoSwap(blockedDomain, blockedLabel string) {
	rotators := ms.klikcepatRotators.GetAll()
	for _, rot := range rotators {
		if !rot.Active {
			continue
		}
		link, err := ms.klikcepat.GetLink(rot.LinkID)
		if err != nil {
			log.Printf("[KLIKCEPAT-SWAP] gagal fetch link %d (%s): %v", rot.LinkID, rot.LinkURL, err)
			continue
		}
		currentHost := extractHost(link.LocationURL)
		if !strings.EqualFold(currentHost, blockedDomain) {
			continue
		}
		pool := ms.domains.GetByLabel(rot.PoolLabel)
		nextDomain := ms.pickNextSafe(pool, blockedDomain)
		if nextDomain == "" {
			ms.notify.Notify(fmt.Sprintf(
				"🚨 *Klikcepat swap GAGAL — pool kosong*\n"+
					"🔗 Link: `/%s`\n"+
					"📂 Pool: `%s` (%d domain — semua blocked)\n"+
					"🚫 Blocked: `%s`",
				rot.LinkURL, rot.PoolLabel, len(pool), blockedDomain))
			continue
		}
		newLocationURL := buildSwapURL(link.LocationURL, nextDomain)
		if err := ms.klikcepat.UpdateLinkLocation(rot.LinkID, newLocationURL); err != nil {
			if ms.history != nil {
				ms.history.LogSwap("klikcepat-scan", rot.Label, rot.LinkURL, link.LocationURL, newLocationURL, false, err.Error())
			}
			ms.notify.Notify(fmt.Sprintf(
				"❌ *Klikcepat AUTO-SWAP GAGAL*\n"+
					"🔗 Link: `/%s`\n"+
					"⚠️ Error: %v",
				rot.LinkURL, err))
			continue
		}
		if ms.history != nil {
			ms.history.LogSwap("klikcepat-scan", rot.Label, rot.LinkURL, link.LocationURL, newLocationURL, true, "")
		}
		ms.notify.Notify(fmt.Sprintf(
			"⚡ *KLIKCEPAT AUTO-SWAP*\n"+
				"🔗 Link: `/%s` (%s)\n"+
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
			time.Now().Format("02/01/2006 15:04:05")))
		log.Printf("[KLIKCEPAT-SWAP] rotator=%s link=%s pool=%s: %s → %s",
			rot.Label, rot.LinkURL, rot.PoolLabel, blockedDomain, nextDomain)
	}
}
```

- [ ] **Step 3: Update main.go to pass klikcepat to scanner**

In `main.go`, find `monScanner := rotator.NewMonitorScanner(...)` call. Update:

```go
	monScanner := rotator.NewMonitorScanner(cf, domains, cfrules, rotators, notify, cfg.CheckInterval, history, klc, klcRotators)
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add rotator/monitor_scanner.go main.go
git commit -m "feat(scanner): auto-swap klikcepat location_url when domain blocked"
```

---

## Phase 9: Polish & Deploy

### Task 18: Final integration test + README update

**Files:**
- Modify: `README.md` (if exists, otherwise skip)
- Modify: `deploy/DEPLOY.md`

- [ ] **Step 1: Update DEPLOY.md with klikcepat setup**

Append to `deploy/DEPLOY.md`:

```markdown

## 🔗 KLIKCEPAT Integration (Optional)

Bot bisa integrate dengan klikcepat.com (klikcepat) untuk auto-swap link.

### Setup

1. **Enable API di klikcepat** (sebagai master admin):
   - Admin Panel → Settings → Enable users API system ✅
   - Admin Panel → Plans → pilih plan normal admin → API access ✅

2. **Normal admin generate API key**:
   - Login klikcepat.com → Account → API → Generate API Key

3. **Set di bot** (2 cara):

   **Via .env** (restart bot setelah edit):
   ```env
   KLIKCEPAT_BASE_URL=https://klikcepat.com
   KLIKCEPAT_API_KEY=<api-key>
   ```

   **Via bot menu** (tanpa restart):
   - DM bot → 🔧 Settings → 🔗 Klikcepat Settings
   - Set Base URL + Set API Key → Test Koneksi

### Auto-swap setup

- DM bot → 🔄 Auto Rotator → ➕ Setup Rotator → 🔗 KLIKCEPAT
- Pick link → pick pool label → kasih label → save
- Done! Bot bakal auto-swap location_url tiap domain target blocked.
```

- [ ] **Step 2: Manual integration test checklist**

Walk through these manually:

```
□ Setup klikcepat credentials via bot menu
□ Test Koneksi → ✅ Success
□ Tambah Link wizard (5 step) → verify muncul di klikcepat.com
□ List Link → liat link yg baru dibuat (paginated)
□ Edit Link → ubah title → verify update di klikcepat
□ Hapus Link → confirm dialog → delete
□ Tambah Project → verify muncul di klikcepat
□ List Project → liat
□ Edit Project → rename
□ Hapus Project → delete
□ Auto Rotator → Setup Rotator → Pick "KLIKCEPAT" → pick link → pool → label → save
□ List Rotator → liat grouped view (CF + Klikcepat)
□ Force Block domain di Monitor (lewat 🔨 Force Block)
□ Tunggu 45s → verify klikcepat link location_url ke-update otomatis
□ Verify notif "⚡ KLIKCEPAT AUTO-SWAP" muncul di group
□ Verify di klikcepat.com dashboard target berubah
```

- [ ] **Step 3: Commit + deploy**

```bash
git add deploy/DEPLOY.md
git commit -m "docs: add KLIKCEPAT setup section to DEPLOY.md"
git push origin main

# Deploy to VPS:
# sudo bash /opt/bongbot/deploy/install.sh
```

---

## Spec Coverage Check

| Spec Requirement | Task |
|---|---|
| `klikcepat/` package — API client | Task 1, 2, 3 |
| Klikcepat types (Link, Project) | Task 1 |
| Bearer token auth | Task 1 |
| Link CRUD methods | Task 2 |
| Project CRUD methods | Task 3 |
| Unit tests | Task 4 |
| `store/klikcepat.go` — KlikcepatRotatorStore | Task 6 |
| Config + .env extensions | Task 5 |
| Credentials store extension | Task 5 |
| 5-button main menu | Task 7 |
| Klikcepat sub-menu | Task 7 |
| Settings → Klikcepat section | Task 9 |
| Settings: Set URL/Key/Test | Task 9 |
| Add Link wizard (5 steps) | Task 10 |
| List Link paginated | Task 11 |
| Edit Link wizard | Task 12 |
| Delete Link with confirm | Task 13 |
| Project Add/List/Edit/Delete | Task 14 |
| Auto Rotator unified entry (pick CF/Klikcepat) | Task 15 |
| Klikcepat rotator wizard | Task 15 |
| List Rotator grouped view | Task 16 |
| MonitorScanner klikcepat integration | Task 17 |
| Auto-swap via API | Task 17 |
| History logging | Task 17 |
| Notif success/fail | Task 17 |
| Documentation update | Task 18 |
| Manual integration test | Task 18 |

All spec requirements covered ✅

---

## Execution Notes

- **Each task self-contained** — bisa di-implement + test + commit independent.
- **TDD limited:** Klikcepat client unit tests in Task 4. Bot handler tests skipped (manual integration via Telegram).
- **Build verification mandatory** after each task (`go build ./...`).
- **Commit per task** untuk rollback-safe.
- **Manual deploy** ke VPS dengan `sudo bash /opt/bongbot/deploy/install.sh` setelah implementation complete.
