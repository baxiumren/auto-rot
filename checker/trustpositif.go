package checker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	StickyFile = "data/sticky_blocked.json"
	APITimeout = 15 * time.Second
	TotalSources = 2 // ada 2 source: API v1 + HTML scrape lama

	// Source 1: Official API v1 (paling cepat & reliable)
	// Documented: https://trust-positif.gitbook.io/docs
	APIv1Endpoint = "https://trustpositif.id/api/v1/check"

	// Source 2: Legacy HTML scrape (backup kalau API v1 down)
	HTMLEndpoint = "https://trustpositif.komdigi.go.id/Rest_server/getrecordsname_home"
)

// ─── HTTP Client ──────────────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: APITimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// Optional API key untuk premium tier
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

// ─── Core Check Methods (DUAL SOURCE) ────────────────────────────────────────
//
// Tiap check ke 2 source secara PARALEL:
//   - Source 1: API v1 (trustpositif.id/api/v1/check) — official, cepat
//   - Source 2: HTML scrape (trustpositif.komdigi.go.id) — legacy backup
//
// Logic:
//   - ANY source say BLOCKED  → BLOCKED (fail-safe: lebih baik swap salah daripada lewat block beneran)
//   - ALL source error        → ERROR
//   - Otherwise (all SAFE)    → SAFE

type sourceResult struct {
	name    string // "API" atau "HTML"
	status  string // "BLOCKED" | "SAFE" | "ERROR"
	err     error
	elapsed time.Duration
}

// checkBothSources jalanin paralel ke 2 source, return status + count.
// Return:
//   - status: "BLOCKED" | "SAFE" | "ERROR"
//   - blockedSources: berapa source confirm blocked (0/1/2)
//   - totalSources: berapa source yang sukses respon (gak ERROR)
func (c *Checker) checkBothSources(domain string) (status string, blockedSources, totalSources int) {
	var wg sync.WaitGroup
	results := make([]sourceResult, 2)

	wg.Add(2)

	// Source 1: API v1
	go func() {
		defer wg.Done()
		start := time.Now()
		results[0] = sourceResult{name: "API", elapsed: 0}
		s, err := checkViaAPIv1(domain)
		results[0].status = s
		results[0].err = err
		results[0].elapsed = time.Since(start)
	}()

	// Source 2: HTML scrape
	go func() {
		defer wg.Done()
		start := time.Now()
		results[1] = sourceResult{name: "HTML", elapsed: 0}
		s, err := checkViaHTMLScrape(domain)
		results[1].status = s
		results[1].err = err
		results[1].elapsed = time.Since(start)
	}()

	wg.Wait()

	for _, r := range results {
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

// CheckFast: untuk rotator (auto-check). 1 round ke 2 source paralel.
// Cepat (~1.5s), efisien, redundan.
func (c *Checker) CheckFast(domain string) string {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED"
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED"
	}

	status, _, _ := c.checkBothSources(domain)
	if status == "BLOCKED" {
		c.AddSticky(domain)
	}
	return status
}

// CheckManual: untuk user manual cek. Return status + count "X/2 sources confirm blocked".
//   - X = blockedSources (berapa source confirm blocked)
//   - total = TotalSources (2)
func (c *Checker) CheckManual(domain string) (status string, blockedCount, totalRounds int) {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED", TotalSources, TotalSources
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED", TotalSources, TotalSources
	}

	status, blocked, _ := c.checkBothSources(domain)
	if status == "BLOCKED" {
		c.AddSticky(domain)
	}
	return status, blocked, TotalSources
}

// ─── SOURCE 1: API v1 (trustpositif.id) ──────────────────────────────────────

type apiV1Result struct {
	Domain  string `json:"Domain"`
	Blocked bool   `json:"Blocked"`
}

type apiV1Response struct {
	Success bool          `json:"success"`
	Results []apiV1Result `json:"results"`
	Count   int           `json:"count"`
	Message string        `json:"message,omitempty"`
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

	if resp.StatusCode != 200 {
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
	// Domain gak ada di response = gak terdaftar = SAFE
	return "SAFE", nil
}

// ─── SOURCE 2: HTML Scrape Legacy (trustpositif.komdigi.go.id) ───────────────

func checkViaHTMLScrape(domain string) (string, error) {
	const baseURL = "https://trustpositif.komdigi.go.id/"

	// Step 1: GET halaman home untuk dapat CSRF token
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return "", fmt.Errorf("req1: %w", err)
	}
	setHTMLHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http1: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	csrf := extractCSRFToken(string(bodyBytes))
	if csrf == "" {
		return "", fmt.Errorf("csrf token gak ketemu")
	}

	// Step 2: GET halaman result dengan CSRF
	checkURL := fmt.Sprintf(
		"https://trustpositif.komdigi.go.id/welcome?csrf_token=%s&recaptcha_token=&domains=%s",
		url.QueryEscape(csrf), url.QueryEscape(domain),
	)
	req2, _ := http.NewRequest("GET", checkURL, nil)
	setHTMLHeaders(req2)
	req2.Header.Set("Referer", baseURL)

	resp2, err := httpClient.Do(req2)
	if err != nil {
		return "", fmt.Errorf("http2: %w", err)
	}
	defer resp2.Body.Close()
	resultBody, _ := io.ReadAll(resp2.Body)

	return parseHTMLResult(string(resultBody), domain)
}

func setHTMLHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "curl/8.5.0") // CRITICAL — Mozilla bakal kena 403
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
}

func extractCSRFToken(html string) string {
	if m := regexp.MustCompile(`csrf_token=([a-fA-F0-9]+)`).FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := regexp.MustCompile(`csrf_token["'\s:=]+([a-fA-F0-9]+)`).FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseHTMLResult(html, domain string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}
	var found string
	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() >= 2 {
			col1 := strings.TrimSpace(tds.Eq(0).Text())
			col2 := strings.TrimSpace(tds.Eq(1).Text())
			if normalizeHTML(col1) == normalizeHTML(domain) {
				found = col2
			}
		}
	})
	if found == "" {
		text := strings.ToLower(doc.Text())
		if strings.Contains(text, "tidak ada") {
			return "SAFE", nil
		}
		if strings.Contains(text, "ada") {
			return "BLOCKED", nil
		}
		return "", fmt.Errorf("status gak ketemu di HTML")
	}
	s := strings.ToLower(strings.TrimSpace(found))
	if strings.Contains(s, "tidak ada") {
		return "SAFE", nil
	}
	if strings.Contains(s, "ada") {
		return "BLOCKED", nil
	}
	return "", fmt.Errorf("status unknown: %s", found)
}

func normalizeHTML(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "www.")
	return strings.TrimSuffix(s, "/")
}

// ─── Backward-compat package functions ───────────────────────────────────────

func CheckDomain(domain string) string {
	return defaultChecker.CheckFast(domain)
}

func CheckDomainManual(domain string) (status string, blockedCount, total int) {
	return defaultChecker.CheckManual(domain)
}

func Default() *Checker {
	return defaultChecker
}
