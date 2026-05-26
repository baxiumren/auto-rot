package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const dataDir = "data"

// ─── Domain List ─────────────────────────────────────────────────────────────

type DomainStore struct {
	mu   sync.RWMutex
	data map[string][]string // label → []domain
}

func NewDomainStore() *DomainStore {
	ds := &DomainStore{data: make(map[string][]string)}
	ds.load()
	return ds
}

func (ds *DomainStore) load() {
	b, err := os.ReadFile(dataDir + "/domains.json")
	if err != nil {
		return
	}
	json.Unmarshal(b, &ds.data)
}

func (ds *DomainStore) save() {
	b, _ := json.MarshalIndent(ds.data, "", "  ")
	os.WriteFile(dataDir+"/domains.json", b, 0644)
}

func (ds *DomainStore) GetAll() map[string][]string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	copy := make(map[string][]string)
	for k, v := range ds.data {
		cp := make([]string, len(v))
		copy[k] = append(cp[:0], v...)
	}
	return copy
}

func (ds *DomainStore) GetByLabel(label string) []string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return append([]string{}, ds.data[strings.ToUpper(label)]...)
}

func (ds *DomainStore) Labels() []string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	var labels []string
	for k := range ds.data {
		labels = append(labels, k)
	}
	sort.Strings(labels)
	return labels
}

func (ds *DomainStore) Add(domain, label string) (isMove bool, oldLabel string) {
	domain = CleanDomain(domain)
	label = strings.ToUpper(strings.TrimSpace(label))

	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Cek apakah sudah ada di label lain
	for l, domains := range ds.data {
		for i, d := range domains {
			if d == domain {
				if l == label {
					return false, ""
				}
				oldLabel = l
				isMove = true
				ds.data[l] = append(domains[:i], domains[i+1:]...)
				if len(ds.data[l]) == 0 {
					delete(ds.data, l)
				}
				break
			}
		}
		if isMove {
			break
		}
	}

	ds.data[label] = append(ds.data[label], domain)
	sort.Strings(ds.data[label])
	go ds.save()
	return isMove, oldLabel
}

func (ds *DomainStore) Remove(domain string) (label string, found bool) {
	domain = CleanDomain(domain)
	ds.mu.Lock()
	defer ds.mu.Unlock()

	for l, domains := range ds.data {
		for i, d := range domains {
			if d == domain {
				ds.data[l] = append(domains[:i], domains[i+1:]...)
				if len(ds.data[l]) == 0 {
					delete(ds.data, l)
				}
				go ds.save()
				return l, true
			}
		}
	}
	return "", false
}

func (ds *DomainStore) FindLabel(domain string) string {
	domain = CleanDomain(domain)
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	for l, domains := range ds.data {
		for _, d := range domains {
			if d == domain {
				return l
			}
		}
	}
	return ""
}

func (ds *DomainStore) TotalCount() int {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	n := 0
	for _, v := range ds.data {
		n += len(v)
	}
	return n
}

// ─── CF Rules ─────────────────────────────────────────────────────────────────

type CFRule struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Domain    string `json:"domain,omitempty"` // nama domain di CF (untuk display)
	ZoneID    string `json:"zone_id"`
	Type      string `json:"type"` // redirect_rules | page_rules
	RulesetID string `json:"ruleset_id,omitempty"`
	RuleID    string `json:"rule_id"`
}

type CFRuleStore struct {
	mu   sync.RWMutex
	data []CFRule
}

func NewCFRuleStore() *CFRuleStore {
	s := &CFRuleStore{}
	s.load()
	return s
}

func (s *CFRuleStore) load() {
	b, err := os.ReadFile(dataDir + "/cf_rules.json")
	if err != nil {
		return
	}
	json.Unmarshal(b, &s.data)
}

func (s *CFRuleStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "  ")
	os.WriteFile(dataDir+"/cf_rules.json", b, 0644)
}

func (s *CFRuleStore) GetAll() []CFRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]CFRule{}, s.data...)
}

func (s *CFRuleStore) GetByID(id string) (CFRule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.data {
		if r.ID == id {
			return r, true
		}
	}
	return CFRule{}, false
}

func (s *CFRuleStore) Add(r CFRule) {
	r.ID = slugify(r.Label)
	// Hindari duplicate ID
	s.mu.Lock()
	for _, existing := range s.data {
		if existing.ID == r.ID {
			r.ID = r.ID + fmt.Sprintf("_%d", time.Now().Unix())
			break
		}
	}
	s.data = append(s.data, r)
	s.mu.Unlock()
	go s.save()
}

// UpdateDomain backfill field Domain untuk rule lama yang Domain-nya kosong.
// Return true kalau ke-update, false kalau rule tidak ditemukan.
func (s *CFRuleStore) UpdateDomain(id, domain string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data {
		if r.ID == id {
			s.data[i].Domain = domain
			go s.save()
			return true
		}
	}
	return false
}

func (s *CFRuleStore) Delete(id string) bool {
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

// ─── Rotator Rules ───────────────────────────────────────────────────────────

type RotatorRule struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	CFRuleID   string `json:"cf_rule_id"`  // references CFRule.ID
	PoolLabel  string `json:"pool_label"`  // references DomainStore label
	Active     bool   `json:"active"`
}

type RotatorStore struct {
	mu   sync.RWMutex
	data []RotatorRule
}

func NewRotatorStore() *RotatorStore {
	s := &RotatorStore{}
	s.load()
	return s
}

func (s *RotatorStore) load() {
	b, err := os.ReadFile(dataDir + "/rotator_rules.json")
	if err != nil {
		return
	}
	json.Unmarshal(b, &s.data)
}

func (s *RotatorStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "  ")
	os.WriteFile(dataDir+"/rotator_rules.json", b, 0644)
}

func (s *RotatorStore) GetAll() []RotatorRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]RotatorRule{}, s.data...)
}

func (s *RotatorStore) GetByID(id string) (RotatorRule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.data {
		if r.ID == id {
			return r, true
		}
	}
	return RotatorRule{}, false
}

func (s *RotatorStore) Add(r RotatorRule) {
	r.ID = slugify(r.Label)
	r.Active = true
	s.mu.Lock()
	for _, existing := range s.data {
		if existing.ID == r.ID {
			r.ID = r.ID + fmt.Sprintf("_%d", time.Now().Unix())
			break
		}
	}
	s.data = append(s.data, r)
	s.mu.Unlock()
	go s.save()
}

func (s *RotatorStore) Toggle(id string) (active bool, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data {
		if r.ID == id {
			s.data[i].Active = !r.Active
			go s.save()
			return s.data[i].Active, true
		}
	}
	return false, false
}

func (s *RotatorStore) Delete(id string) bool {
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

// ─── Swap History ────────────────────────────────────────────────────────────

const maxHistoryEntries = 200 // simpan max 200 swap terakhir

// SwapEntry = 1 record swap (auto atau manual).
type SwapEntry struct {
	Timestamp  time.Time `json:"ts"`
	Source     string    `json:"source"`           // "monitor-scan", "rotator", "manual", "bulk", "force"
	RuleLabel  string    `json:"rule_label"`
	RuleDomain string    `json:"rule_domain,omitempty"`
	FromURL    string    `json:"from_url"`
	ToURL      string    `json:"to_url"`
	Success    bool      `json:"success"`
	ErrorMsg   string    `json:"error,omitempty"`
}

type HistoryStore struct {
	mu      sync.RWMutex
	entries []SwapEntry // newest first
}

func NewHistoryStore() *HistoryStore {
	hs := &HistoryStore{}
	hs.load()
	return hs
}

func (hs *HistoryStore) load() {
	b, err := os.ReadFile(dataDir + "/swap_history.json")
	if err != nil {
		return
	}
	json.Unmarshal(b, &hs.entries)
}

func (hs *HistoryStore) save() {
	b, _ := json.MarshalIndent(hs.entries, "", "  ")
	os.WriteFile(dataDir+"/swap_history.json", b, 0644)
}

// LogSwap menambah entry baru ke history (di depan = newest first).
// Otomatis trim ke maxHistoryEntries supaya gak meledak.
func (hs *HistoryStore) LogSwap(source, ruleLabel, ruleDomain, fromURL, toURL string, success bool, errMsg string) {
	hs.mu.Lock()
	entry := SwapEntry{
		Timestamp:  time.Now(),
		Source:     source,
		RuleLabel:  ruleLabel,
		RuleDomain: ruleDomain,
		FromURL:    fromURL,
		ToURL:      toURL,
		Success:    success,
		ErrorMsg:   errMsg,
	}
	hs.entries = append([]SwapEntry{entry}, hs.entries...)
	if len(hs.entries) > maxHistoryEntries {
		hs.entries = hs.entries[:maxHistoryEntries]
	}
	hs.mu.Unlock()
	go hs.save()
}

// GetRecent return N entry terbaru.
func (hs *HistoryStore) GetRecent(n int) []SwapEntry {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	if n > len(hs.entries) {
		n = len(hs.entries)
	}
	out := make([]SwapEntry, n)
	copy(out, hs.entries[:n])
	return out
}

func (hs *HistoryStore) Count() int {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return len(hs.entries)
}

func (hs *HistoryStore) Clear() {
	hs.mu.Lock()
	hs.entries = nil
	hs.mu.Unlock()
	go hs.save()
}

// ─── Credentials (CF Email & API Key) ────────────────────────────────────────

type Credentials struct {
	CFEmail  string `json:"cf_email"`
	CFAPIKey string `json:"cf_api_key"`
}

type CredentialStore struct {
	mu   sync.RWMutex
	data Credentials
}

func NewCredentialStore() *CredentialStore {
	s := &CredentialStore{}
	s.load()
	return s
}

func (s *CredentialStore) load() {
	b, err := os.ReadFile(dataDir + "/credentials.json")
	if err != nil {
		return
	}
	json.Unmarshal(b, &s.data)
}

func (s *CredentialStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "  ")
	os.WriteFile(dataDir+"/credentials.json", b, 0600) // 0600: hanya owner yang bisa baca
}

func (s *CredentialStore) Get() Credentials {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *CredentialStore) SetEmail(email string) {
	s.mu.Lock()
	s.data.CFEmail = strings.TrimSpace(email)
	s.mu.Unlock()
	go s.save()
}

func (s *CredentialStore) SetAPIKey(apiKey string) {
	s.mu.Lock()
	s.data.CFAPIKey = strings.TrimSpace(apiKey)
	s.mu.Unlock()
	go s.save()
}

func (s *CredentialStore) Set(email, apiKey string) {
	s.mu.Lock()
	s.data.CFEmail = strings.TrimSpace(email)
	s.data.CFAPIKey = strings.TrimSpace(apiKey)
	s.mu.Unlock()
	go s.save()
}

func (s *CredentialStore) Clear() {
	s.mu.Lock()
	s.data = Credentials{}
	s.mu.Unlock()
	go s.save()
}

// MaskAPIKey returns "abcd1234...wxyz" untuk display.
func MaskAPIKey(key string) string {
	if key == "" {
		return "(belum di-set)"
	}
	if len(key) <= 12 {
		return "****"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func CleanDomain(input string) string {
	d := strings.ToLower(strings.TrimSpace(input))
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "www.")
	if idx := strings.Index(d, "/"); idx != -1 {
		d = d[:idx]
	}
	return strings.TrimSuffix(d, "/")
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("rule-%d", time.Now().Unix())
	}
	return b.String()
}
