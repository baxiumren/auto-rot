package rotator

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"bongbot/checker"
	"bongbot/cloudflare"
	"bongbot/store"
)

type Notifier interface {
	Notify(msg string)
}

type Service struct {
	cf       *cloudflare.Client
	domains  *store.DomainStore
	cfrules  *store.CFRuleStore
	rotators *store.RotatorStore
	notify   Notifier
	history  *store.HistoryStore

	mu       sync.RWMutex
	interval time.Duration
	blocked  map[string]time.Time // domain → waktu pertama blocked
}

func New(
	cf *cloudflare.Client,
	domains *store.DomainStore,
	cfrules *store.CFRuleStore,
	rotators *store.RotatorStore,
	notify Notifier,
	interval time.Duration,
	history *store.HistoryStore,
) *Service {
	return &Service{
		cf:       cf,
		domains:  domains,
		cfrules:  cfrules,
		rotators: rotators,
		notify:   notify,
		history:  history,
		interval: interval,
		blocked:  make(map[string]time.Time),
	}
}

func (s *Service) GetInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.interval
}

func (s *Service) SetInterval(d time.Duration) {
	s.mu.Lock()
	s.interval = d
	s.mu.Unlock()
	log.Printf("[ROTATOR] Interval diubah ke %v", d)
}

func (s *Service) GetBlockedDomains() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]time.Time)
	for k, v := range s.blocked {
		cp[k] = v
	}
	return cp
}

func (s *Service) Start() {
	log.Printf("[ROTATOR] Service dimulai")
	go s.loop()
}

func (s *Service) loop() {
	for {
		s.mu.RLock()
		interval := s.interval
		s.mu.RUnlock()

		rotators := s.rotators.GetAll()
		for _, rot := range rotators {
			if !rot.Active {
				continue
			}
			go s.checkAndRotate(rot)
		}
		time.Sleep(interval)
	}
}

func (s *Service) checkAndRotate(rot store.RotatorRule) {
	cfRule, ok := s.cfrules.GetByID(rot.CFRuleID)
	if !ok {
		log.Printf("[ROTATOR] CF rule %s tidak ditemukan untuk rotator %s", rot.CFRuleID, rot.Label)
		return
	}

	currentURL, err := s.cf.GetCurrentURL(cfRule)
	if err != nil {
		log.Printf("[ROTATOR] Gagal ambil URL rule %s: %v", rot.Label, err)
		return
	}

	currentHost := extractHost(currentURL)
	log.Printf("[ROTATOR] Rule=%s current=%s", rot.Label, currentHost)

	status := checker.CheckDomain(currentHost)
	log.Printf("[ROTATOR] Rule=%s host=%s status=%s", rot.Label, currentHost, status)

	if status == "SAFE" {
		// Clear dari blocked list jika sudah aman
		s.mu.Lock()
		delete(s.blocked, currentHost)
		s.mu.Unlock()
		return
	}

	if status == "BLOCKED" {
		s.mu.Lock()
		if _, exists := s.blocked[currentHost]; !exists {
			s.blocked[currentHost] = time.Now()
		}
		s.mu.Unlock()
	}

	if status != "BLOCKED" {
		return
	}

	// Ambil pool dari domain store
	pool := s.domains.GetByLabel(rot.PoolLabel)
	if len(pool) == 0 {
		log.Printf("[ROTATOR] Pool label %s kosong untuk rotator %s", rot.PoolLabel, rot.Label)
		return
	}

	// Cari index current di pool
	currentIdx := -1
	for i, d := range pool {
		if strings.EqualFold(d, currentHost) || strings.EqualFold("https://"+d, normURL(currentURL)) {
			currentIdx = i
			break
		}
	}

	// Cari next domain yang SAFE
	for attempt := 1; attempt < len(pool); attempt++ {
		nextIdx := (currentIdx + attempt) % len(pool)
		nextDomain := pool[nextIdx]
		nextStatus := checker.CheckDomain(nextDomain)

		if nextStatus == "BLOCKED" {
			log.Printf("[ROTATOR] Pool[%d] %s juga BLOCKED", nextIdx, nextDomain)
			continue
		}

		// Preserve path & query dari URL lama, cuma host yang diganti
		nextURL := buildSwapURL(currentURL, nextDomain)
		if err := s.cf.UpdateURL(cfRule, nextURL); err != nil {
			log.Printf("[ROTATOR] Gagal update CF rule %s: %v", rot.Label, err)
			if s.history != nil {
				s.history.LogSwap("rotator", cfRule.Label, cfRule.Domain, currentURL, nextURL, false, err.Error())
			}
			s.notify.Notify(fmt.Sprintf(
				"❌ *AUTO ROTATE GAGAL*\n📛 Rotator: `%s`\n⚠️ Error: %v",
				rot.Label, err,
			))
			return
		}
		if s.history != nil {
			s.history.LogSwap("rotator", cfRule.Label, cfRule.Domain, currentURL, nextURL, true, "")
		}

		// Update blocked map
		s.mu.Lock()
		delete(s.blocked, currentHost)
		s.blocked[nextDomain] = time.Now() // track yang baru juga jika nanti blocked
		delete(s.blocked, nextDomain)      // clear — ini domain baru, belum blocked
		s.mu.Unlock()

		log.Printf("[ROTATOR] ROTATE %s: %s → %s", rot.Label, currentHost, nextDomain)
		s.notify.Notify(fmt.Sprintf(
			"🔄 *AUTO ROTATE*\n"+
				"📛 Rotator: `%s`\n"+
				"🚫 Domain lama: `%s` *(BLOCKED)*\n"+
				"✅ Domain baru: `%s`\n"+
				"🕐 %s",
			rot.Label, currentHost, nextDomain,
			time.Now().Format("02/01/2006 15:04:05"),
		))
		return
	}

	// Semua pool blocked
	s.notify.Notify(fmt.Sprintf(
		"🚨 *SEMUA DOMAIN BLOCKED!*\n"+
			"📛 Rotator: `%s`\n"+
			"⚠️ Semua %d domain di pool *%s* kena nawala!\n"+
			"🕐 %s",
		rot.Label, len(pool), rot.PoolLabel,
		time.Now().Format("02/01/2006 15:04:05"),
	))
}

func (s *Service) ForceRotate(rot store.RotatorRule) error {
	cfRule, ok := s.cfrules.GetByID(rot.CFRuleID)
	if !ok {
		return fmt.Errorf("CF rule %s tidak ditemukan", rot.CFRuleID)
	}

	currentURL, err := s.cf.GetCurrentURL(cfRule)
	if err != nil {
		return fmt.Errorf("gagal ambil URL: %w", err)
	}

	pool := s.domains.GetByLabel(rot.PoolLabel)
	if len(pool) == 0 {
		return fmt.Errorf("pool %s kosong", rot.PoolLabel)
	}

	currentHost := extractHost(currentURL)
	currentIdx := -1
	for i, d := range pool {
		if strings.EqualFold(d, currentHost) {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(pool)
	nextURL := buildSwapURL(currentURL, pool[nextIdx])

	if err := s.cf.UpdateURL(cfRule, nextURL); err != nil {
		if s.history != nil {
			s.history.LogSwap("force", cfRule.Label, cfRule.Domain, currentURL, nextURL, false, err.Error())
		}
		return fmt.Errorf("gagal update CF: %w", err)
	}
	if s.history != nil {
		s.history.LogSwap("force", cfRule.Label, cfRule.Domain, currentURL, nextURL, true, "")
	}

	s.notify.Notify(fmt.Sprintf(
		"🔀 *FORCE ROTATE*\n📛 Rotator: `%s`\n🔗 `%s` → `%s`",
		rot.Label, currentHost, pool[nextIdx],
	))
	return nil
}

func extractHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Hostname()
}

func normURL(u string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(u), "/"))
}

// buildSwapURL: bikin URL baru dengan host diganti newDomain, sisanya (path,
// query, fragment, scheme) PRESERVED dari originalURL.
//
// Contoh:
//   buildSwapURL("https://domain1.com/daftar?ref=mahaslot", "domain2.com")
//   → "https://domain2.com/daftar?ref=mahaslot"
//
//   buildSwapURL("https://domain1.com/", "domain2.com")
//   → "https://domain2.com/"
//
//   buildSwapURL("https://domain1.com", "domain2.com")
//   → "https://domain2.com"
func buildSwapURL(originalURL, newDomain string) string {
	newDomain = strings.TrimSpace(newDomain)
	// Bersihkan kalau user kasih dengan prefix
	newDomain = strings.TrimPrefix(newDomain, "https://")
	newDomain = strings.TrimPrefix(newDomain, "http://")
	newDomain = strings.TrimPrefix(newDomain, "www.")
	if idx := strings.Index(newDomain, "/"); idx != -1 {
		newDomain = newDomain[:idx]
	}

	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return "https://" + newDomain
	}

	// Pastikan ada scheme biar url.Parse-nya bener
	hasScheme := strings.Contains(originalURL, "://")
	parseTarget := originalURL
	if !hasScheme {
		parseTarget = "https://" + originalURL
	}

	u, err := url.Parse(parseTarget)
	if err != nil || u.Host == "" {
		return "https://" + newDomain
	}

	// Replace host saja, sisanya tetap
	u.Host = newDomain
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return u.String()
}
