package rotator

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"bongbot/checker"
	"bongbot/store"
)

// ─── Monitor Scanner ──────────────────────────────────────────────────────────
//
// Scan SEMUA domain di Monitor list secara periodik (bukan cuma yang lagi
// jadi target di CF Rule). Saat detect domain blocked:
//
// 1. Track ke blockedCycle (untuk spam interval)
// 2. Spam notif berulang tiap SpamInterval, sampai domain dihapus dari Monitor
// 3. *Reaktif auto-swap*: cek SEMUA CF Rule, kalau ada yang current URL
//    domain-nya match dengan domain yang blocked → swap detik itu juga.
//
// Berbeda dengan rotator.checkAndRotate() yang per-rule (check current URL
// dari satu CF rule). Monitor scanner lebih proaktif: catch blocked domain
// SEBELUM dia jadi target di CF rule.

const (
	MonitorAlertWindow  = 2 * time.Minute  // berapa lama alert mode
	MonitorCooldown     = 10 * time.Minute // diam sebentar setelah alert window
	MonitorSpamInterval = 25 * time.Second // jarak antar alert spam
	MonitorMaxConcurrent = 10              // max paralel saat scan
)

type blockCycle struct {
	domain        string
	label         string
	firstDetected time.Time
	lastAlertSent time.Time
	cycleStart    time.Time
	alertCount    int
	inCooldown    bool
	cycleNumber   int
	swapped       bool // udah kena auto-swap di cycle ini?
}

// MonitorScanner adalah background task yang scan semua domain di Monitor list.
type MonitorScanner struct {
	cf       cfUpdater
	domains  *store.DomainStore
	cfrules  *store.CFRuleStore
	rotators *store.RotatorStore // sumber pool untuk auto-swap (CF rule → PoolLabel)
	notify   Notifier
	chk      *checker.Checker
	history  *store.HistoryStore

	mu      sync.Mutex
	blocked map[string]*blockCycle

	interval time.Duration
}

// cfUpdater adalah interface kecil — biar gampang inject mock di test.
type cfUpdater interface {
	GetCurrentURL(rule store.CFRule) (string, error)
	UpdateURL(rule store.CFRule, newURL string) error
}

func NewMonitorScanner(
	cf cfUpdater,
	domains *store.DomainStore,
	cfrules *store.CFRuleStore,
	rotators *store.RotatorStore,
	notify Notifier,
	interval time.Duration,
	history *store.HistoryStore,
) *MonitorScanner {
	return &MonitorScanner{
		cf:       cf,
		domains:  domains,
		cfrules:  cfrules,
		rotators: rotators,
		notify:   notify,
		chk:      checker.Default(),
		history:  history,
		blocked:  make(map[string]*blockCycle),
		interval: interval,
	}
}

func (ms *MonitorScanner) SetInterval(d time.Duration) {
	ms.mu.Lock()
	ms.interval = d
	ms.mu.Unlock()
}

func (ms *MonitorScanner) GetInterval() time.Duration {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.interval
}

// GetBlockedSnapshot returns copy of currently-blocked domains for display.
func (ms *MonitorScanner) GetBlockedSnapshot() map[string]time.Time {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	out := make(map[string]time.Time, len(ms.blocked))
	for d, c := range ms.blocked {
		out[d] = c.firstDetected
	}
	return out
}

// Start launches the scanner + spam loops.
func (ms *MonitorScanner) Start() {
	log.Printf("[MONITOR-SCAN] service dimulai")
	go ms.scanLoop()
	go ms.spamLoop()
}

// ─── Scan Loop — periodically check all monitor domains ──────────────────────

func (ms *MonitorScanner) scanLoop() {
	time.Sleep(3 * time.Second) // initial delay
	for {
		ms.mu.Lock()
		interval := ms.interval
		ms.mu.Unlock()

		ms.scanOnce()
		time.Sleep(interval)
	}
}

func (ms *MonitorScanner) scanOnce() {
	all := ms.domains.GetAll()
	if len(all) == 0 {
		return
	}

	// Build a unique list of (domain, label)
	type entry struct {
		domain string
		label  string
	}
	var entries []entry
	seen := make(map[string]bool)
	for label, doms := range all {
		for _, d := range doms {
			if seen[d] {
				continue
			}
			seen[d] = true
			entries = append(entries, entry{domain: d, label: label})
		}
	}

	// Build set of valid domains (still in monitor) — untuk auto-cleanup
	validDomains := make(map[string]bool, len(entries))
	for _, e := range entries {
		validDomains[e.domain] = true
	}

	// Cleanup blocked entries yang udah ga ada di monitor
	ms.mu.Lock()
	for d := range ms.blocked {
		if !validDomains[d] {
			delete(ms.blocked, d)
			log.Printf("[MONITOR-SCAN] %s removed from blocked (no longer in monitor)", d)
		}
	}
	ms.mu.Unlock()

	// Auto-cleanup orphan sticky+force entries (domain yang gak ada di monitor)
	// Jaga supaya file data/sticky_blocked.json gak meledak.
	if sCleared, fCleared := ms.chk.CleanOrphans(validDomains); sCleared > 0 || fCleared > 0 {
		log.Printf("[MONITOR-SCAN] orphan cleanup: %d sticky, %d force cleared", sCleared, fCleared)
	}

	// Scan paralel dengan semaphore
	sem := make(chan struct{}, MonitorMaxConcurrent)
	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func(e entry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ms.checkOne(e.domain, e.label)
		}(e)
	}
	wg.Wait()
}

func (ms *MonitorScanner) checkOne(domain, label string) {
	status := ms.chk.CheckFast(domain)

	// Skip ERROR — JANGAN treat sebagai SAFE atau BLOCKED, biar gak salah swap
	if status == "ERROR" {
		log.Printf("[MONITOR-SCAN] %s: ERROR (API timeout/unreachable) — skip cycle", domain)
		return
	}

	ms.mu.Lock()
	cycle, exists := ms.blocked[domain]
	ms.mu.Unlock()

	switch status {
	case "BLOCKED":
		if !exists {
			// First detection
			cycle = &blockCycle{
				domain:        domain,
				label:         label,
				firstDetected: time.Now(),
				lastAlertSent: time.Now(),
				cycleStart:    time.Now(),
				alertCount:    1,
				inCooldown:    false,
				cycleNumber:   1,
				swapped:       false,
			}
			ms.mu.Lock()
			ms.blocked[domain] = cycle
			ms.mu.Unlock()

			// Send FIRST alert + trigger auto-swap immediately
			ms.sendBlockAlert(cycle, true)
			ms.triggerAutoSwap(domain, label)
		} else {
			// Sudah ada — update label kalau berubah
			ms.mu.Lock()
			cycle.label = label
			ms.mu.Unlock()
		}

	case "SAFE":
		// Sticky-block yang nyangkut: kalau gak sticky lagi, hapus dari blocked
		if exists && !ms.chk.IsForceBlocked(domain) {
			if blocked, _ := ms.chk.IsSticky(domain); !blocked {
				ms.mu.Lock()
				delete(ms.blocked, domain)
				ms.mu.Unlock()
				ms.notify.Notify(fmt.Sprintf(
					"🟢 *DOMAIN PULIH*\n📛 Label: `%s`\n🌐 `%s`\n_Status: gak terdeteksi blocked lagi._",
					label, domain,
				))
			}
		}
	}
}

// ─── Spam Loop — repeat alerts in cycle ───────────────────────────────────────

func (ms *MonitorScanner) spamLoop() {
	time.Sleep(5 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ms.mu.Lock()
		now := time.Now()
		for _, cycle := range ms.blocked {
			ms.evaluateSpam(cycle, now)
		}
		ms.mu.Unlock()
	}
}

// evaluateSpam dijalankan dengan ms.mu locked.
func (ms *MonitorScanner) evaluateSpam(cycle *blockCycle, now time.Time) {
	elapsed := now.Sub(cycle.cycleStart)

	if cycle.inCooldown {
		// Cek apakah cooldown selesai → restart cycle
		if elapsed >= MonitorCooldown {
			cycle.inCooldown = false
			cycle.cycleStart = now
			cycle.alertCount = 1
			cycle.cycleNumber++
			cycle.lastAlertSent = now
			cycle.swapped = false // reset, kalau masih blocked nanti swap lagi
			// Kirim cycle restart message
			go ms.sendBlockAlert(cycle, false)
			// Re-trigger auto-swap kalau emang masih kena
			go ms.triggerAutoSwap(cycle.domain, cycle.label)
		}
		return
	}

	// Active alert mode
	if elapsed > MonitorAlertWindow {
		// Pindah ke cooldown
		cycle.inCooldown = true
		cycle.cycleStart = now
		return
	}

	// Cek waktu untuk spam berikutnya
	if now.Sub(cycle.lastAlertSent) >= MonitorSpamInterval {
		cycle.alertCount++
		cycle.lastAlertSent = now
		go ms.sendBlockAlert(cycle, false)
	}
}

// sendBlockAlert kirim notif blocked. firstTime=true untuk first detection.
func (ms *MonitorScanner) sendBlockAlert(cycle *blockCycle, firstTime bool) {
	var prefix, modeText string

	switch {
	case firstTime:
		prefix = "🚨 *DOMAIN KENA NAWALA!*"
		modeText = "🆕 Baru pertama kali terdeteksi"
	case cycle.cycleNumber > 1 && cycle.alertCount == 1:
		prefix = fmt.Sprintf("🔔 *MASIH BLOCKED!* [Cycle #%d restart]", cycle.cycleNumber)
		modeText = "⏰ Cycle baru setelah cooldown"
	default:
		prefix = fmt.Sprintf("🛑 *MASIH BLOCKED!* [Alert #%d]", cycle.alertCount)
		modeText = "🔄 Spam mode aktif"
	}

	swapNote := ""
	if cycle.swapped {
		swapNote = "\n✅ _CF Rule udah ke-swap. Domain ini tetap dipantau._"
	}

	msg := fmt.Sprintf(
		"%s\n"+
			"📛 Label: `%s`\n"+
			"🌐 Domain: `%s`\n"+
			"📅 Pertama detect: %s\n"+
			"%s%s\n\n"+
			"_Spam akan terus sampai kamu hapus domain ini dari Monitor._",
		prefix, cycle.label, cycle.domain,
		cycle.firstDetected.Format("02/01 15:04:05"),
		modeText, swapNote,
	)
	ms.notify.Notify(msg)
}

// ─── Auto-Swap: scan CF rules, swap any matching current URL ─────────────────
//
// Pool untuk swap diambil dari *Rotator config* (RotatorRule.PoolLabel),
// BUKAN dari label monitor domain yang blocked.
//
// Contoh:
//   CF Rule maha66.id, current target=domain1.com (label: MONEYSITE)
//   Rotator config: CFRuleID=maha66 → PoolLabel=STOCK-MS
//   domain2-4 ada di label STOCK-MS
//
//   domain1.com BLOCKED → scanner cari rotator yg punya CFRuleID=maha66
//   → ambil pool dari STOCK-MS → swap maha66.id ke domain2/3/4.

func (ms *MonitorScanner) triggerAutoSwap(blockedDomain, blockedLabel string) {
	rules := ms.cfrules.GetAll()
	if len(rules) == 0 {
		return
	}
	allRotators := ms.rotators.GetAll()

	for _, rule := range rules {
		currentURL, err := ms.cf.GetCurrentURL(rule)
		if err != nil {
			log.Printf("[MONITOR-SCAN] gagal fetch URL rule %s: %v", rule.Label, err)
			continue
		}
		currentHost := extractHost(currentURL)
		if !strings.EqualFold(currentHost, blockedDomain) {
			continue // bukan rule yang ini
		}

		// Match! Cari rotator config untuk CF rule ini
		// (bisa lebih dari 1 rotator pakai CF rule yang sama — gak masalah, swap satu kali aja)
		var rot *store.RotatorRule
		for i := range allRotators {
			if allRotators[i].CFRuleID == rule.ID && allRotators[i].Active {
				rot = &allRotators[i]
				break
			}
		}

		if rot == nil {
			// CF rule ke-match tapi gak ada Rotator config → kasih warning, gak swap
			ms.notify.Notify(fmt.Sprintf(
				"⚠️ *Auto-swap di-skip — belum ada Rotator config*\n"+
					"🌐 Domain blocked: `%s`\n"+
					"🔧 CF Rule yang match: `%s`\n\n"+
					"📍 *Solusi:* Setup Auto Rotator untuk CF Rule ini via menu *🔄 Auto Rotator → ➕ Setup Rotator*.\n"+
					"_Bot perlu tau pool mana yg dipakai untuk swap._",
				blockedDomain, rule.Label,
			))
			continue
		}

		// Ambil pool dari Rotator config (BUKAN dari label monitor domain blocked!)
		pool := ms.domains.GetByLabel(rot.PoolLabel)
		if len(pool) == 0 {
			ms.notify.Notify(fmt.Sprintf(
				"⚠️ *Auto-swap gagal — pool kosong*\n"+
					"🔧 Rule: `%s`\n"+
					"🌐 Domain blocked: `%s`\n"+
					"📂 Pool: `%s` (kosong)\n\n"+
					"_Tambah domain ke label `%s` via Monitor → Add Domain._",
				rule.Label, blockedDomain, rot.PoolLabel, rot.PoolLabel,
			))
			continue
		}

		// Pilih next domain SAFE
		nextDomain := ms.pickNextSafe(pool, blockedDomain)
		if nextDomain == "" {
			ms.notify.Notify(fmt.Sprintf(
				"🚨 *Semua pool BLOCKED!*\n"+
					"🔧 Rule: `%s`\n"+
					"📂 Pool: `%s` (%d domain — semua blocked)\n"+
					"🚫 Domain blocked: `%s`\n\n"+
					"_Tambah domain baru ke pool `%s` segera!_",
				rule.Label, rot.PoolLabel, len(pool), blockedDomain, rot.PoolLabel,
			))
			continue
		}

		// Preserve path & query dari URL lama (cuma host yang diganti)
		// Contoh: https://domain1.com/daftar?ref=x → https://domain2.com/daftar?ref=x
		nextURL := buildSwapURL(currentURL, nextDomain)
		if err := ms.cf.UpdateURL(rule, nextURL); err != nil {
			if ms.history != nil {
				ms.history.LogSwap("monitor-scan", rule.Label, rule.Domain, currentURL, nextURL, false, err.Error())
			}
			ms.notify.Notify(fmt.Sprintf(
				"❌ *Auto-swap GAGAL*\n"+
					"🔧 Rule: `%s`\n"+
					"⚠️ Error: %v",
				rule.Label, err,
			))
			continue
		}
		if ms.history != nil {
			ms.history.LogSwap("monitor-scan", rule.Label, rule.Domain, currentURL, nextURL, true, "")
		}

		// Mark cycle as swapped
		ms.mu.Lock()
		if c, ok := ms.blocked[blockedDomain]; ok {
			c.swapped = true
		}
		ms.mu.Unlock()

		ms.notify.Notify(fmt.Sprintf(
			"⚡ *AUTO-SWAP via MONITOR*\n"+
				"🔧 Rule: `%s` (%s)\n"+
				"🚫 Sebelum: `%s` *(BLOCKED — label: %s)*\n"+
				"   URL: `%s`\n"+
				"✅ Sekarang: `%s`\n"+
				"   URL: `%s`\n"+
				"📂 Pool dipakai: `%s`\n"+
				"🕐 %s",
			rule.Label, rule.Domain,
			blockedDomain, blockedLabel, currentURL,
			nextDomain, nextURL,
			rot.PoolLabel,
			time.Now().Format("02/01/2006 15:04:05"),
		))
		log.Printf("[MONITOR-SCAN] auto-swap rule=%s pool=%s: %s → %s | URL: %s → %s",
			rule.Label, rot.PoolLabel, blockedDomain, nextDomain, currentURL, nextURL)
	}
}

// pickNextSafe pilih domain berikutnya dari pool yang gak BLOCKED.
func (ms *MonitorScanner) pickNextSafe(pool []string, currentHost string) string {
	// Cari index current di pool
	currentIdx := -1
	for i, d := range pool {
		if strings.EqualFold(d, currentHost) {
			currentIdx = i
			break
		}
	}

	// Loop pool mulai dari setelah current
	for attempt := 1; attempt <= len(pool); attempt++ {
		nextIdx := (currentIdx + attempt) % len(pool)
		next := pool[nextIdx]
		if strings.EqualFold(next, currentHost) {
			continue // skip dirinya sendiri
		}
		status := ms.chk.CheckFast(next)
		if status == "BLOCKED" {
			continue
		}
		return next
	}
	return ""
}

// (extractHost di-share dari rotator.go)
