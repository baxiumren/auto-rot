package checker

import (
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
	MaxRetries      = 2
	RoundsAuto      = 2 // ronde untuk auto-check (rotator)
	RoundsManual    = 3 // ronde untuk manual check (user)
	StickyFile      = "data/sticky_blocked.json"
	APITimeout      = 20 * time.Second
)

// ─── HTTP Client (shared, with sane timeout) ──────────────────────────────────

var httpClient = &http.Client{
	Timeout:       APITimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
}

// ─── Checker = stateful sticky+force store + API caller ──────────────────────

type Checker struct {
	stickyMu sync.RWMutex
	sticky   map[string]time.Time // domain → waktu pertama ke-detect blocked

	forceMu sync.RWMutex
	force   map[string]string // domain → label (alasan force)
}

// global default checker instance (untuk backward compat dengan rotator/bot lama)
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

// Clean strip prefix http(s):// dan www., trim trailing slash.
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

// CleanOrphans hapus semua entry sticky+force yang gak ada di set validDomains.
// Return: jumlah sticky cleared, force cleared.
func (c *Checker) CleanOrphans(validDomains map[string]bool) (stickyCleared, forceCleared int) {
	// Sticky
	c.stickyMu.Lock()
	for d := range c.sticky {
		if !validDomains[d] {
			delete(c.sticky, d)
			stickyCleared++
		}
	}
	c.stickyMu.Unlock()

	// Force
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

// CountOrphans hitung sticky+force entry yang gak ada di validDomains.
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

// ─── Force Block API (manual override) ────────────────────────────────────────

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

// ─── Core Check Methods ──────────────────────────────────────────────────────

// CheckFast: untuk rotator (auto-check). 2 ronde, hit sticky+force dulu.
// Return "BLOCKED" kalau salah satu ronde dapat BLOCKED, kalau gak "SAFE".
func (c *Checker) CheckFast(domain string) string {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED"
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED"
	}

	for round := 1; round <= RoundsAuto; round++ {
		result := c.doAPICheck(domain, round)
		if result == "BLOCKED" {
			c.AddSticky(domain)
			return "BLOCKED"
		}
		if round < RoundsAuto && result == "SAFE" {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return "SAFE"
}

// CheckManual: untuk user manual cek. 3 ronde, return status + count BLOCKED/total.
// Lebih akurat — kalau 0/3 BLOCKED → SAFE pasti; kalau 1+/3 → BLOCKED.
func (c *Checker) CheckManual(domain string) (status string, blockedCount, totalRounds int) {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED", RoundsManual, RoundsManual
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED", RoundsManual, RoundsManual
	}

	for round := 1; round <= RoundsManual; round++ {
		result := c.doAPICheck(domain, round)
		if result == "BLOCKED" {
			blockedCount++
		}
		if round < RoundsManual {
			time.Sleep(200 * time.Millisecond)
		}
	}

	if blockedCount > 0 {
		c.AddSticky(domain)
		return "BLOCKED", blockedCount, RoundsManual
	}
	return "SAFE", 0, RoundsManual
}

// doAPICheck — internal: hit TrustPositif API dengan retry.
func (c *Checker) doAPICheck(domain string, round int) string {
	for attempt := 1; attempt <= MaxRetries; attempt++ {
		status, err := checkTrustPositif(domain)
		if err != nil {
			log.Printf("[NAWALA] round=%d attempt=%d %s error: %v", round, attempt, domain, err)
			if attempt < MaxRetries {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return "ERROR"
		}
		result := tpToStatus(status)
		log.Printf("[NAWALA] round=%d %s → %s (raw=%s)", round, domain, result, status)
		return result
	}
	return "ERROR"
}

// ─── Backward-compat package functions (pakai default checker) ────────────────

// CheckDomain: legacy API, alias ke defaultChecker.CheckFast.
func CheckDomain(domain string) string {
	return defaultChecker.CheckFast(domain)
}

// CheckDomainManual: 3-round manual check via default checker.
func CheckDomainManual(domain string) (status string, blockedCount, total int) {
	return defaultChecker.CheckManual(domain)
}

// Default mengembalikan instance default — biar bot bisa akses sticky/force.
func Default() *Checker {
	return defaultChecker
}

// ─── HTTP-Level Scraping (TrustPositif) ──────────────────────────────────────

func checkTrustPositif(domain string) (string, error) {
	const baseURL = "https://trustpositif.komdigi.go.id/"

	req, _ := http.NewRequest("GET", baseURL, nil)
	setHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	csrf := extractCSRF(string(body))
	if csrf == "" {
		return "", fmt.Errorf("csrf_token tidak ketemu")
	}

	checkURL := fmt.Sprintf(
		"https://trustpositif.komdigi.go.id/welcome?csrf_token=%s&recaptcha_token=&domains=%s",
		url.QueryEscape(csrf), url.QueryEscape(domain),
	)
	req2, _ := http.NewRequest("GET", checkURL, nil)
	setHeaders(req2)
	req2.Header.Set("Referer", baseURL)

	resp2, err := httpClient.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)

	return parseHTML(string(body2), domain)
}

func setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
}

func extractCSRF(html string) string {
	if m := regexp.MustCompile(`csrf_token=([a-fA-F0-9]+)`).FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := regexp.MustCompile(`csrf_token["'\s:=]+([a-fA-F0-9]+)`).FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseHTML(html, domain string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	var found string
	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() >= 2 {
			if normalize(tds.Eq(0).Text()) == normalize(domain) {
				found = strings.TrimSpace(tds.Eq(1).Text())
			}
		}
	})
	if found == "" {
		text := strings.ToLower(doc.Text())
		if strings.Contains(text, "tidak ada") {
			return "Tidak Ada", nil
		}
		if strings.Contains(text, "ada") {
			return "Ada", nil
		}
		return "", fmt.Errorf("status tidak ketemu di HTML")
	}
	return found, nil
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "www.")
	return strings.TrimSuffix(s, "/")
}

func tpToStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	if strings.Contains(s, "tidak ada") {
		return "SAFE"
	}
	if strings.Contains(s, "ada") {
		return "BLOCKED"
	}
	return "ERROR"
}
