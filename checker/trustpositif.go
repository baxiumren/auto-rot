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
	MaxRetries   = 2
	RoundsAuto   = 2 // ronde untuk auto-check (rotator)
	RoundsManual = 3 // ronde untuk manual check (user)
	StickyFile   = "data/sticky_blocked.json"
	APITimeout   = 30 * time.Second
)

// Endpoint list — kalau satu down, coba yang lain (urut dari paling cepat)
var apiEndpoints = []string{
	"https://trustpositif.komdigi.go.id/Rest_server/getrecordsname_home",
	"https://trustpositif.app/Rest_server/getrecordsname_home",
	"https://trustpositif.kominfo.go.id/Rest_server/getrecordsname_home",
}

// ─── HTTP Client (shared, optimized for keep-alive) ──────────────────────────

var httpClient = &http.Client{
	Timeout: APITimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// ─── Checker = stateful sticky+force store + API caller ──────────────────────

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

// CleanOrphans hapus semua entry sticky+force yang gak ada di set validDomains.
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

// ─── Core Check Methods ──────────────────────────────────────────────────────

// CheckFast: untuk rotator. 2 ronde, hit sticky+force dulu.
// Return: "BLOCKED" | "SAFE" | "ERROR"
func (c *Checker) CheckFast(domain string) string {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED"
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED"
	}

	gotSafe := false
	for round := 1; round <= RoundsAuto; round++ {
		result := c.doAPICheck(domain, round)
		if result == "BLOCKED" {
			c.AddSticky(domain)
			return "BLOCKED"
		}
		if result == "SAFE" {
			gotSafe = true
			if round < RoundsAuto {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	if !gotSafe {
		log.Printf("[NAWALA] %s: SEMUA ronde gagal → return ERROR", domain)
		return "ERROR"
	}
	return "SAFE"
}

// CheckManual: untuk user manual cek. 3 ronde, return status + count BLOCKED/total.
func (c *Checker) CheckManual(domain string) (status string, blockedCount, totalRounds int) {
	domain = Clean(domain)
	if c.IsForceBlocked(domain) {
		return "BLOCKED", RoundsManual, RoundsManual
	}
	if blocked, _ := c.IsSticky(domain); blocked {
		return "BLOCKED", RoundsManual, RoundsManual
	}

	gotSafe := false
	errorCount := 0
	for round := 1; round <= RoundsManual; round++ {
		result := c.doAPICheck(domain, round)
		switch result {
		case "BLOCKED":
			blockedCount++
		case "SAFE":
			gotSafe = true
		case "ERROR":
			errorCount++
		}
		if round < RoundsManual {
			time.Sleep(200 * time.Millisecond)
		}
	}

	if blockedCount > 0 {
		c.AddSticky(domain)
		return "BLOCKED", blockedCount, RoundsManual
	}
	if !gotSafe {
		log.Printf("[NAWALA] %s: %d/%d ronde error → return ERROR", domain, errorCount, RoundsManual)
		return "ERROR", 0, RoundsManual
	}
	return "SAFE", 0, RoundsManual
}

// doAPICheck — coba SEMUA endpoint, kembalikan hasil pertama yang sukses.
func (c *Checker) doAPICheck(domain string, round int) string {
	var lastErr error
	for _, endpoint := range apiEndpoints {
		for attempt := 1; attempt <= MaxRetries; attempt++ {
			status, err := checkSingleEndpoint(domain, endpoint)
			if err != nil {
				lastErr = err
				log.Printf("[NAWALA] round=%d attempt=%d endpoint=%s %s → %v",
					round, attempt, shortEndpoint(endpoint), domain, err)
				if attempt < MaxRetries {
					time.Sleep(300 * time.Millisecond)
					continue
				}
				break // pindah ke endpoint berikutnya
			}
			log.Printf("[NAWALA] round=%d endpoint=%s %s → %s",
				round, shortEndpoint(endpoint), domain, status)
			return status
		}
	}
	log.Printf("[NAWALA] round=%d %s: SEMUA endpoint gagal, last err: %v", round, domain, lastErr)
	return "ERROR"
}

// shortEndpoint helper buat log lebih ringkas.
func shortEndpoint(url string) string {
	url = strings.TrimPrefix(url, "https://")
	if i := strings.Index(url, "/"); i > 0 {
		return url[:i]
	}
	return url
}

// ─── HTTP-Level API Call (POST + JSON parse) ─────────────────────────────────

// checkSingleEndpoint POST ke endpoint dengan body "name=domain" dan parse JSON.
func checkSingleEndpoint(domain, endpoint string) (string, error) {
	body := bytes.NewBufferString("name=" + domain)

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return "", fmt.Errorf("req create: %w", err)
	}

	// Headers PENTING — harus mimic browser Indonesia
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Origin", "https://trustpositif.komdigi.go.id")
	req.Header.Set("Referer", "https://trustpositif.komdigi.go.id/")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return parseResponse(domain, respBody)
}

// parseResponse parse multi-format JSON response dari TrustPositif.
// Format paling umum: {"values":[{"Domain":"...","Status":"Ada"}],"response":1}
// "Ada" = BLOCKED, "Tidak Ada" / anything else = SAFE
func parseResponse(domain string, body []byte) (string, error) {
	// Format 1 (paling umum): {"values":[{"Domain":"...","Status":"Ada"}]}
	var format1 struct {
		Values []struct {
			Domain string `json:"Domain"`
			Status string `json:"Status"`
		} `json:"values"`
		Response int `json:"response"`
	}
	if err := json.Unmarshal(body, &format1); err == nil && len(format1.Values) > 0 {
		for _, item := range format1.Values {
			if strings.EqualFold(item.Domain, domain) {
				if strings.EqualFold(strings.TrimSpace(item.Status), "Ada") {
					return "BLOCKED", nil
				}
				return "SAFE", nil
			}
		}
		// values ada tapi domain gak ke-match → asumsikan SAFE (gak terdaftar di blocklist)
		return "SAFE", nil
	}

	// Format 2: {"data":[{"domain":"...","status":"diblokir"}]}
	var format2 struct {
		Data []struct {
			Domain string `json:"domain"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &format2); err == nil && len(format2.Data) > 0 {
		for _, item := range format2.Data {
			if strings.EqualFold(item.Domain, domain) {
				s := strings.ToLower(item.Status)
				if strings.Contains(s, "diblokir") || strings.Contains(s, "blocked") || strings.Contains(s, "ada") {
					return "BLOCKED", nil
				}
				return "SAFE", nil
			}
		}
		return "SAFE", nil
	}

	// Format 3: empty values = SAFE (gak ada di blocklist)
	if strings.Contains(string(body), `"values":[]`) || strings.Contains(string(body), `"values": []`) {
		return "SAFE", nil
	}

	// Format 4: {"blocked":true/false}
	var format4 struct {
		Blocked bool `json:"blocked"`
	}
	if err := json.Unmarshal(body, &format4); err == nil {
		if format4.Blocked {
			return "BLOCKED", nil
		}
		return "SAFE", nil
	}

	// Last resort: fallback keyword scan
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, `"status":"ada"`) || strings.Contains(lower, `"status": "ada"`) {
		return "BLOCKED", nil
	}
	if strings.Contains(lower, `"status":"tidak ada"`) {
		return "SAFE", nil
	}

	return "", fmt.Errorf("unknown response format: %s", truncateForLog(string(body), 200))
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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
