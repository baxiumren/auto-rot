package checker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	StickyFile = "data/sticky_blocked.json"
	APITimeout = 15 * time.Second

	// ─── Source 1: Rest_server (backend public web form — UNLIMITED) ───────
	RestServerEndpoint = "https://trustpositif.komdigi.go.id/Rest_server/getrecordsname_home"

	// ─── Source 2: API v1 (developer API — 100/day freemium) ──────────────
	APIv1Endpoint = "https://trustpositif.id/api/v1/check"

	// ─── Source 3: NawalaCheck.com (paid API — opsional, butuh X-API-Key) ─
	NawalaCheckEndpoint = "https://api.nawalacheck.com/api/"

	// Cooldown saat API hit 429 / quota habis — skip selama ini
	quotaResetCooldown = 1 * time.Hour

	// ─── Retry config untuk anti rate-limit ─────────────────────────────
	// Kominfo Rest_server kadang balikin 404 saat di-hit terlalu cepat
	// (deceptive anti-spam). Retry dengan exponential backoff solve ini.
	MaxRetryAttempts = 3                      // total attempt = 1 + 2 retry
	RetryBaseDelay   = 800 * time.Millisecond // delay attempt-1: 800ms, attempt-2: 1600ms
)

// MaxSources return total sumber yg aktif (2 atau 3 tergantung NawalaCheck key).
func MaxSources() int {
	if getNawalaCheckKey() != "" {
		return 3
	}
	return 2
}

// ─── HTTP Client ──────────────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: APITimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// Optional API key untuk premium tier TrustPositif API v1
var (
	apiKey   string
	apiKeyMu sync.RWMutex
)

func SetAPIKey(key string) {
	apiKeyMu.Lock()
	apiKey = strings.TrimSpace(key)
	apiKeyMu.Unlock()
}

func getAPIKey() string {
	apiKeyMu.RLock()
	defer apiKeyMu.RUnlock()
	return apiKey
}

// Optional API key untuk NawalaCheck.com (Source 3 — paid service)
var (
	nawalaCheckKey   string
	nawalaCheckKeyMu sync.RWMutex
)

func SetNawalaCheckKey(key string) {
	nawalaCheckKeyMu.Lock()
	nawalaCheckKey = strings.TrimSpace(key)
	nawalaCheckKeyMu.Unlock()
	if key != "" {
		log.Printf("[NAWALA] NawalaCheck API key di-set (Source 3 aktif)")
	}
}

func getNawalaCheckKey() string {
	nawalaCheckKeyMu.RLock()
	defer nawalaCheckKeyMu.RUnlock()
	return nawalaCheckKey
}

// HasNawalaCheckKey return true kalau NawalaCheck API key udah di-set.
// Buat bot UI: enable/disable tombol pilihan source.
func HasNawalaCheckKey() bool {
	return getNawalaCheckKey() != ""
}

// HasTrustPositifKey return true kalau TrustPositif API key udah di-set.
// Buat bot UI: tampilin tombol Trust Positif ID atau enggak.
func HasTrustPositifKey() bool {
	return getAPIKey() != ""
}

// NawalaCheck quota cooldown (kalau 403 Limit habis)
var (
	nawalaCheckCooldown time.Time
	nawalaCheckMu       sync.RWMutex
)

func setNawalaCheckCooldown() {
	nawalaCheckMu.Lock()
	nawalaCheckCooldown = time.Now().Add(quotaResetCooldown)
	nawalaCheckMu.Unlock()
	log.Printf("[NAWALA] NawalaCheck cooldown sampai %s (quota habis)", nawalaCheckCooldown.Format("15:04:05"))
}

func isNawalaCheckInCooldown() bool {
	nawalaCheckMu.RLock()
	defer nawalaCheckMu.RUnlock()
	return time.Now().Before(nawalaCheckCooldown)
}

// ─── API v1 Quota Management ─────────────────────────────────────────────────
// Saat API v1 hit 429, set `apiV1Cooldown` ke time.Now()+1jam.
// Selama cooldown aktif, skip API v1 call (cuma pakai Rest_server source).
// Ini bikin quota 100/hari LANGGENG karena cuma dipakai pas Rest_server fail.

var (
	apiV1Cooldown time.Time
	cooldownMu    sync.RWMutex
)

func setAPIv1Cooldown() {
	cooldownMu.Lock()
	apiV1Cooldown = time.Now().Add(quotaResetCooldown)
	cooldownMu.Unlock()
	log.Printf("[NAWALA] API v1 cooldown sampai %s (quota habis)", apiV1Cooldown.Format("15:04:05"))
}

func isAPIv1InCooldown() bool {
	cooldownMu.RLock()
	defer cooldownMu.RUnlock()
	return time.Now().Before(apiV1Cooldown)
}

// ─── Checker = stateful sticky+force store + dual-source caller ──────────────

type Checker struct {
	stickyMu sync.RWMutex
	sticky   map[string]time.Time

	forceMu sync.RWMutex
	force   map[string]string
}

var defaultChecker = NewChecker()

func NewChecker() *Checker {
	c := &Checker{
		sticky: make(map[string]time.Time),
		force:  make(map[string]string),
	}
	c.loadSticky()
	return c
}

// ─── Public Domain Cleaner ────────────────────────────────────────────────────

func Clean(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "www.")
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	return strings.TrimSuffix(domain, "/")
}

// ─── Sticky Block API ─────────────────────────────────────────────────────────

func (c *Checker) loadSticky() {
	c.stickyMu.Lock()
	defer c.stickyMu.Unlock()
	data, err := os.ReadFile(StickyFile)
	if err != nil {
		return
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("[STICKY] gagal parse %s: %v", StickyFile, err)
		return
	}
	for d, ts := range raw {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			c.sticky[d] = t
		}
	}
	log.Printf("[STICKY] load %d domain dari %s", len(c.sticky), StickyFile)
}

func (c *Checker) saveSticky() {
	c.stickyMu.RLock()
	raw := make(map[string]string, len(c.sticky))
	for d, t := range c.sticky {
		raw[d] = t.Format(time.RFC3339)
	}
	c.stickyMu.RUnlock()
	data, _ := json.MarshalIndent(raw, "", "  ")
	os.WriteFile(StickyFile, data, 0644)
}

func (c *Checker) IsSticky(domain string) (bool, time.Time) {
	c.stickyMu.RLock()
	defer c.stickyMu.RUnlock()
	t, ok := c.sticky[domain]
	return ok, t
}

func (c *Checker) AddSticky(domain string) {
	c.stickyMu.Lock()
	if _, exists := c.sticky[domain]; !exists {
		c.sticky[domain] = time.Now()
		c.stickyMu.Unlock()
		log.Printf("[STICKY] added: %s", domain)
		go c.saveSticky()
		return
	}
	c.stickyMu.Unlock()
}

func (c *Checker) RemoveSticky(domain string) bool {
	c.stickyMu.Lock()
	if _, exists := c.sticky[domain]; exists {
		delete(c.sticky, domain)
		c.stickyMu.Unlock()
		go c.saveSticky()
		return true
	}
	c.stickyMu.Unlock()
	return false
}

func (c *Checker) GetStickyList() map[string]time.Time {
	c.stickyMu.RLock()
	defer c.stickyMu.RUnlock()
	out := make(map[string]time.Time, len(c.sticky))
	for k, v := range c.sticky {
		out[k] = v
	}
	return out
}

func (c *Checker) CleanOrphans(validDomains map[string]bool) (stickyCleared, forceCleared int) {
	c.stickyMu.Lock()
	for d := range c.sticky {
		if !validDomains[d] {
			delete(c.sticky, d)
			stickyCleared++
		}
	}
	c.stickyMu.Unlock()

	c.forceMu.Lock()
	for d := range c.force {
		if !validDomains[d] {
			delete(c.force, d)
			forceCleared++
		}
	}
	c.forceMu.Unlock()

	if stickyCleared > 0 {
		go c.saveSticky()
	}
	return stickyCleared, forceCleared
}

func (c *Checker) CountOrphans(validDomains map[string]bool) (stickyOrphan, forceOrphan int) {
	c.stickyMu.RLock()
	for d := range c.sticky {
		if !validDomains[d] {
			stickyOrphan++
		}
	}
	c.stickyMu.RUnlock()

	c.forceMu.RLock()
	for d := range c.force {
		if !validDomains[d] {
			forceOrphan++
		}
	}
	c.forceMu.RUnlock()

	return stickyOrphan, forceOrphan
}

// ─── Force Block API ─────────────────────────────────────────────────────────

func (c *Checker) IsForceBlocked(domain string) bool {
	c.forceMu.RLock()
	defer c.forceMu.RUnlock()
	_, ok := c.force[domain]
	return ok
}

func (c *Checker) AddForceBlock(domain, label string) {
	c.forceMu.Lock()
	c.force[domain] = label
	c.forceMu.Unlock()
	log.Printf("[FORCE] added: %s (label=%s)", domain, label)
}

func (c *Checker) RemoveForceBlock(domain string) bool {
	c.forceMu.Lock()
	defer c.forceMu.Unlock()
	if _, ok := c.force[domain]; ok {
		delete(c.force, domain)
		return true
	}
	return false
}

func (c *Checker) GetForceList() map[string]string {
	c.forceMu.RLock()
	defer c.forceMu.RUnlock()
	out := make(map[string]string, len(c.force))
	for k, v := range c.force {
		out[k] = v
	}
	return out
}

// ─── Core Check Methods (DUAL SOURCE - Smart Quota) ──────────────────────────
//
// Tiap check ke 2 source secara PARALEL:
//   - Source 1: Rest_server (UNLIMITED — backend public web form)
//   - Source 2: API v1 (100/day limit, di-skip kalau cooldown aktif)
//
// Quota management:
//   - Kalau API v1 hit 429, set cooldown 1 jam.
//   - Selama cooldown, API v1 di-skip (cuma Rest_server jalan).
//   - Source Check displays "1/2" or "2/2" tergantung berapa source aktif.

type sourceResult struct {
	name    string
	status  string
	err     error
	elapsed time.Duration
	skipped bool // true kalau di-skip karena quota cooldown
}

func (c *Checker) checkAllSources(domain string) (status string, blockedSources, totalSources int) {
	// Tentuin source aktif (2 atau 3 tergantung NawalaCheck key)
	hasNawalaCheck := getNawalaCheckKey() != ""
	numSources := 2
	if hasNawalaCheck {
		numSources = 3
	}

	var wg sync.WaitGroup
	results := make([]sourceResult, numSources)
	wg.Add(numSources)

	// Source 1: Rest_server (selalu jalan, unlimited)
	go func() {
		defer wg.Done()
		start := time.Now()
		results[0] = sourceResult{name: "Rest"}
		s, err := checkViaRestServer(domain)
		results[0].status = s
		results[0].err = err
		results[0].elapsed = time.Since(start)
	}()

	// Source 2: API v1 (cek cooldown dulu)
	go func() {
		defer wg.Done()
		start := time.Now()
		results[1] = sourceResult{name: "APIv1"}
		if isAPIv1InCooldown() {
			results[1].skipped = true
			results[1].err = fmt.Errorf("quota cooldown active")
			return
		}
		s, err := checkViaAPIv1(domain)
		results[1].status = s
		results[1].err = err
		results[1].elapsed = time.Since(start)
		if err != nil && (strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Daily limit")) {
			setAPIv1Cooldown()
		}
	}()

	// Source 3: NawalaCheck (opsional — kalau API key ada)
	if hasNawalaCheck {
		go func() {
			defer wg.Done()
			start := time.Now()
			results[2] = sourceResult{name: "NwCheck"}
			if isNawalaCheckInCooldown() {
				results[2].skipped = true
				results[2].err = fmt.Errorf("quota cooldown active")
				return
			}
			s, err := checkViaNawalaCheck(domain)
			results[2].status = s
			results[2].err = err
			results[2].elapsed = time.Since(start)
			// Detect 403 (limit habis / plan inactive) → cooldown
			if err != nil && strings.Contains(err.Error(), "403") {
				setNawalaCheckCooldown()
			}
		}()
	}

	wg.Wait()

	for _, r := range results {
		if r.skipped {
			log.Printf("[NAWALA] %s %s → SKIP (cooldown)", r.name, domain)
			continue
		}
		if r.err != nil {
			log.Printf("[NAWALA] %s %s → ERROR in %v: %v", r.name, domain, r.elapsed, r.err)
			continue
		}
		log.Printf("[NAWALA] %s %s → %s in %v", r.name, domain, r.status, r.elapsed)
		totalSources++
		if r.status == "BLOCKED" {
			blockedSources++
		}
	}

	if blockedSources > 0 {
		return "BLOCKED", blockedSources, totalSources
	}
	if totalSources == 0 {
		return "ERROR", 0, 0
	}
	return "SAFE", 0, totalSources
}

// Backward-compat alias
func (c *Checker) checkBothSources(domain string) (status string, blockedSources, totalSources int) {
	return c.checkAllSources(domain)
}

// CheckFast: untuk AUTO-CHECK (Monitor Scanner & Rotator service).
// CUMA pakai Kominfo Rest_server — paling reliable & update 24/7.
// Hemat quota Source 2 (TrustPositif API v1) & gak butuh paid Source 3.
func (c *Checker) CheckFast(domain string) string {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED"
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED"
	}

	// AUTO-CHECK = KOMINFO ONLY (single source, with retry already built-in)
	status, err := checkViaRestServer(domain)
	if err != nil {
		log.Printf("[NAWALA-AUTO] Kominfo %s → ERROR: %v", domain, err)
		return "ERROR"
	}
	if status == "BLOCKED" {
		c.AddSticky(domain)
	}
	return status
}

// SourceMode untuk MANUAL CHECK (user pilih lewat tombol di Telegram).
// Tiap mode = SATU source.
type SourceMode int

const (
	// SourceKominfo: trustpositif.komdigi.go.id — official Kominfo, UNLIMITED
	SourceKominfo SourceMode = 1
	// SourceTrustPositif: trustpositif.id/api/v1 — third-party mirror, 100/day
	SourceTrustPositif SourceMode = 2
	// SourceNawalaCheck: nawalacheck.com — paid ISP-based, ~10/day di free tier
	SourceNawalaCheck SourceMode = 3

	// Backward-compat alias (deprecated, untuk gak break code lama)
	SourceKominfoPlusTP SourceMode = 1
)

// CheckManual: untuk user manual cek. Accept SourceMode untuk milih sumber.
// Auto-check JANGAN pakai ini — pakai CheckFast (Kominfo only) biar hemat resource.
func (c *Checker) CheckManual(domain string, mode SourceMode) (status string, blockedCount, totalSources int) {
	domain = Clean(domain)

	// Sticky/force shortcut — apapun mode-nya
	if c.IsForceBlocked(domain) {
		return "BLOCKED", 1, 1
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED", 1, 1
	}

	switch mode {
	case SourceKominfo:
		return c.checkKominfoOnly(domain)
	case SourceTrustPositif:
		return c.checkTrustPositifOnly(domain)
	case SourceNawalaCheck:
		return c.checkNawalaCheckOnly(domain)
	default:
		return "ERROR", 0, 0
	}
}

// checkKominfoOnly — cek single source Kominfo (trustpositif.komdigi.go.id)
func (c *Checker) checkKominfoOnly(domain string) (status string, blockedSources, totalSources int) {
	start := time.Now()
	s, err := checkViaRestServer(domain)
	if err != nil {
		log.Printf("[NAWALA-MANUAL] Kominfo %s → ERROR in %v: %v", domain, time.Since(start), err)
		return "ERROR", 0, 0
	}
	log.Printf("[NAWALA-MANUAL] Kominfo %s → %s in %v", domain, s, time.Since(start))
	if s == "BLOCKED" {
		c.AddSticky(domain)
		return "BLOCKED", 1, 1
	}
	return "SAFE", 0, 1
}

// checkTrustPositifOnly — cek single source TrustPositif ID (trustpositif.id/api/v1)
func (c *Checker) checkTrustPositifOnly(domain string) (status string, blockedSources, totalSources int) {
	if isAPIv1InCooldown() {
		log.Printf("[NAWALA-MANUAL] TrustPositif %s → ERROR: cooldown (quota habis sebelumnya)", domain)
		return "ERROR", 0, 0
	}
	start := time.Now()
	s, err := checkViaAPIv1(domain)
	if err != nil {
		log.Printf("[NAWALA-MANUAL] TrustPositif %s → ERROR in %v: %v", domain, time.Since(start), err)
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Daily limit") {
			setAPIv1Cooldown()
		}
		return "ERROR", 0, 0
	}
	log.Printf("[NAWALA-MANUAL] TrustPositif %s → %s in %v", domain, s, time.Since(start))
	if s == "BLOCKED" {
		c.AddSticky(domain)
		return "BLOCKED", 1, 1
	}
	return "SAFE", 0, 1
}

// checkKominfoPlusTP — cek paralel Kominfo + TrustPositif ID.
func (c *Checker) checkKominfoPlusTP(domain string) (status string, blockedSources, totalSources int) {
	var wg sync.WaitGroup
	results := make([]sourceResult, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		start := time.Now()
		results[0] = sourceResult{name: "Kominfo"}
		s, err := checkViaRestServer(domain)
		results[0].status = s
		results[0].err = err
		results[0].elapsed = time.Since(start)
	}()

	go func() {
		defer wg.Done()
		start := time.Now()
		results[1] = sourceResult{name: "TrustPositif"}
		if isAPIv1InCooldown() {
			results[1].skipped = true
			results[1].err = fmt.Errorf("quota cooldown")
			return
		}
		s, err := checkViaAPIv1(domain)
		results[1].status = s
		results[1].err = err
		results[1].elapsed = time.Since(start)
		if err != nil && (strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Daily limit")) {
			setAPIv1Cooldown()
		}
	}()

	wg.Wait()

	for _, r := range results {
		if r.skipped {
			log.Printf("[NAWALA-MANUAL] %s %s → SKIP (cooldown)", r.name, domain)
			continue
		}
		if r.err != nil {
			log.Printf("[NAWALA-MANUAL] %s %s → ERROR in %v: %v", r.name, domain, r.elapsed, r.err)
			continue
		}
		log.Printf("[NAWALA-MANUAL] %s %s → %s in %v", r.name, domain, r.status, r.elapsed)
		totalSources++
		if r.status == "BLOCKED" {
			blockedSources++
		}
	}

	if blockedSources > 0 {
		c.AddSticky(domain)
		return "BLOCKED", blockedSources, totalSources
	}
	if totalSources == 0 {
		return "ERROR", 0, 0
	}
	return "SAFE", 0, totalSources
}

// checkNawalaCheckOnly — cek NawalaCheck single source (paid API).
func (c *Checker) checkNawalaCheckOnly(domain string) (status string, blockedSources, totalSources int) {
	if getNawalaCheckKey() == "" {
		log.Printf("[NAWALA-MANUAL] NawalaCheck %s → ERROR: API key gak di-set", domain)
		return "ERROR", 0, 0
	}
	if isNawalaCheckInCooldown() {
		log.Printf("[NAWALA-MANUAL] NawalaCheck %s → ERROR: cooldown", domain)
		return "ERROR", 0, 0
	}

	start := time.Now()
	s, err := checkViaNawalaCheck(domain)
	if err != nil {
		log.Printf("[NAWALA-MANUAL] NawalaCheck %s → ERROR in %v: %v", domain, time.Since(start), err)
		if strings.Contains(err.Error(), "403") {
			setNawalaCheckCooldown()
		}
		return "ERROR", 0, 0
	}
	log.Printf("[NAWALA-MANUAL] NawalaCheck %s → %s in %v", domain, s, time.Since(start))

	if s == "BLOCKED" {
		c.AddSticky(domain)
		return "BLOCKED", 1, 1
	}
	return "SAFE", 0, 1
}

// ─── SOURCE 1: Rest_server (UNLIMITED) ───────────────────────────────────────
//
// Endpoint backend public web form. POST application/x-www-form-urlencoded,
// body "name=domain.com". Response JSON.
//
// Format response:
// {"values":[{"Domain":"...","Status":"Ada"}],"response":1}
//   Status="Ada" → BLOCKED, else → SAFE

type restServerResponse struct {
	Values []struct {
		Domain string `json:"Domain"`
		Status string `json:"Status"`
	} `json:"values"`
	Response int `json:"response"`
}

// checkViaRestServer wrapper dengan retry logic.
// Kominfo Rest_server kadang balikin 404 saat di-hit cepat (anti-spam).
// Strategy yang udah confirmed work (tested via browser console):
//   - Attempt 1 → kalau 404/5xx/timeout → wait 800ms
//   - Attempt 2 → kalau gagal → wait 1600ms
//   - Attempt 3 → final, return error kalau masih gagal
func checkViaRestServer(domain string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= MaxRetryAttempts; attempt++ {
		status, err := doRestServerCall(domain)
		if err == nil {
			if attempt > 1 {
				log.Printf("[NAWALA] Rest %s → OK after %d attempts", domain, attempt)
			}
			return status, nil
		}
		lastErr = err

		// Retry hanya untuk error yang RECOVERABLE (404, 5xx, timeout)
		// 4xx lain (401, 403) atau parse error → langsung return (gak guna retry)
		if !isRetryableError(err) {
			return "", err
		}

		if attempt < MaxRetryAttempts {
			delay := time.Duration(attempt) * RetryBaseDelay // 800ms, 1600ms
			time.Sleep(delay)
		}
	}
	return "", fmt.Errorf("setelah %d attempts: %w", MaxRetryAttempts, lastErr)
}

// isRetryableError: HTTP 404, 5xx, timeout, dan network error = boleh retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Sangat umum dari anti-spam Kominfo
	if strings.Contains(msg, "HTTP 404") || strings.Contains(msg, "HTTP 5") {
		return true
	}
	// Network errors
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset") || strings.Contains(msg, "EOF") {
		return true
	}
	return false
}

// doRestServerCall: actual HTTP call, single attempt.
func doRestServerCall(domain string) (string, error) {
	body := bytes.NewBufferString("name=" + domain)
	req, err := http.NewRequest("POST", RestServerEndpoint, body)
	if err != nil {
		return "", fmt.Errorf("req: %w", err)
	}

	// CRITICAL: User-Agent harus curl/8.5.0 (Mozilla kena 403)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "curl/8.5.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://trustpositif.komdigi.go.id")
	req.Header.Set("Referer", "https://trustpositif.komdigi.go.id/")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	var parsed restServerResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}

	for _, item := range parsed.Values {
		if strings.EqualFold(Clean(item.Domain), domain) {
			s := strings.ToLower(strings.TrimSpace(item.Status))
			if strings.Contains(s, "ada") && !strings.Contains(s, "tidak") {
				return "BLOCKED", nil
			}
			return "SAFE", nil
		}
	}
	// Domain gak ada di values = gak terdaftar = SAFE
	return "SAFE", nil
}

// ─── SOURCE 2: API v1 (100/day freemium) ─────────────────────────────────────

type apiV1Result struct {
	Domain  string `json:"Domain"`
	Blocked bool   `json:"Blocked"`
}

type apiV1Response struct {
	Success bool          `json:"success"`
	Results []apiV1Result `json:"results"`
	Count   int           `json:"count"`
	Message string        `json:"message,omitempty"`
	Code    int           `json:"code,omitempty"`
}

func checkViaAPIv1(domain string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"domains": domain})
	req, err := http.NewRequest("POST", APIv1Endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("req: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "BongBot/1.0")
	req.Header.Set("Accept", "application/json")
	if key := getAPIKey(); key != "" {
		req.Header.Set("X-API-Key", key)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	// Handle 429 dan limit exceeded specifically
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("HTTP 429 (Daily limit exceeded)")
	}
	if resp.StatusCode != 200 {
		// Coba parse error message
		var errResp apiV1Response
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Message)
		}
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var parsed apiV1Response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if !parsed.Success {
		return "", fmt.Errorf("API success=false: %s", parsed.Message)
	}

	for _, r := range parsed.Results {
		if strings.EqualFold(Clean(r.Domain), domain) {
			if r.Blocked {
				return "BLOCKED", nil
			}
			return "SAFE", nil
		}
	}
	return "SAFE", nil
}

// ─── SOURCE 3: NawalaCheck.com (opsional — butuh X-API-Key) ──────────────────
//
// Endpoint: GET https://api.nawalacheck.com/api/?domain=example.com
// Header:   X-API-Key: tp_xxx
// Response: {"example.com": {"blocked": true|false}}
// Errors:   401 (invalid key), 403 (IP/limit), 500
// Docs:     https://nawalacheck.com (Domain Checker API)

func checkViaNawalaCheck(domain string) (string, error) {
	key := getNawalaCheckKey()
	if key == "" {
		return "", fmt.Errorf("no API key configured")
	}

	endpoint := NawalaCheckEndpoint + "?domain=" + domain
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("req: %w", err)
	}
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "BongBot/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateLog(string(body), 100))
	}

	// Parse response: {"domain.com": {"blocked": true|false}}
	var parsed map[string]struct {
		Blocked bool `json:"blocked"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}

	for d, info := range parsed {
		if strings.EqualFold(Clean(d), domain) {
			if info.Blocked {
				return "BLOCKED", nil
			}
			return "SAFE", nil
		}
	}
	// Domain gak ada di response → asumsikan SAFE (gak terdaftar blocklist)
	return "SAFE", nil
}

func truncateLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ─── Backward-compat package functions ───────────────────────────────────────

// CheckDomain (package-level): legacy wrapper untuk auto-check via Kominfo only.
func CheckDomain(domain string) string {
	return defaultChecker.CheckFast(domain)
}

// CheckDomainManual (package-level): default ke SourceKominfo (paling reliable).
// Untuk source lain, panggil langsung defaultChecker.CheckManual(domain, ...).
func CheckDomainManual(domain string) (status string, blockedCount, total int) {
	return defaultChecker.CheckManual(domain, SourceKominfo)
}

func Default() *Checker {
	return defaultChecker
}
