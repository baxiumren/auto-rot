package rotator

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
	// MonitorSpamInterval: jarak antar alert untuk domain BLOCKED.
	// Spam continuous (gak ada cooldown) sampai user hapus dari Monitor.
	MonitorSpamInterval = 25 * time.Second

	// Concurrency tuning — confirmed via browser test:
	//   - Parallel 5+ → Kominfo balikin HTTP 404 anti-spam (~25% requests gagal)
	//   - Parallel 2-3 → stable, 0 error
	MonitorMaxConcurrent = 3 // max paralel saat scan (turun dari 10)

	// Delay antar request per worker (anti rate-limit Kominfo)
	PerCheckDelay = 200 * time.Millisecond

	// ChunkSize: jumlah domain maksimum yang di-cek per cycle/tick.
	// Kalau total domain > ChunkSize → auto rotating batch:
	//   tick 1 cek domain 1-100, tick 2 cek 101-200, dst, lalu ulang.
	// Sticky/force-blocked di-skip dari pool chunkable (udah dikenal blocked,
	// gak perlu cek API lagi — spam loop terus jalan via ms.blocked).
	ChunkSize = 100
)

type blockCycle struct {
	domain        string
	label         string
	firstDetected time.Time // kapan pertama kali terdeteksi blocked
	lastAlertSent time.Time // kapan alert terakhir kekirim
	alertCount    int       // berapa kali alert udah dikirim
	swapped       bool      // udah kena auto-swap?
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

	klikcepat         KlikcepatUpdater
	klikcepatRotators *store.KlikcepatRotatorStore

	mu      sync.Mutex
	blocked map[string]*blockCycle

	interval time.Duration

	// Rotating batch state (chunking untuk total > ChunkSize)
	cursor       int // index awal chunk berikutnya di sorted active list
	lastChunkNum int // chunk yang baru aja kelar (1-based, untuk display)
	lastChunkOf  int // total chunk di cycle terakhir (1-based)
}

// cfUpdater adalah interface kecil — biar gampang inject mock di test.
type cfUpdater interface {
	GetCurrentURL(rule store.CFRule) (string, error)
	UpdateURL(rule store.CFRule, newURL string) error
}

// KlikcepatUpdater is the minimal interface MonitorScanner needs from klikcepat client.
// *klikcepat.Client satisfies this automatically.
type KlikcepatUpdater interface {
	HasCredentials() bool
	GetLink(id int) (*klikcepat.Link, error)
	UpdateLinkLocation(id int, locationURL string) error
}

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
		klikcepat:         klc,
		klikcepatRotators: klcRotators,
		blocked:           make(map[string]*blockCycle),
		interval:          interval,
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
	iter := 0
	for {
		iter++
		ms.mu.Lock()
		interval := ms.interval
		ms.mu.Unlock()

		startTime := time.Now()

		// Hitung dulu jumlah domain di monitor
		all := ms.domains.GetAll()
		totalDomains := 0
		for _, doms := range all {
			totalDomains += len(doms)
		}

		if totalDomains == 0 {
			log.Printf("[MONITOR-SCAN] cycle #%d SKIP — gak ada domain di Monitor. Sleep %v...", iter, interval)
		} else {
			log.Printf("[MONITOR-SCAN] cycle #%d START — total %d domain dalam %d label", iter, totalDomains, len(all))
			ms.scanOnce()

			// Snapshot status setelah scan
			ms.mu.Lock()
			blockedCount := len(ms.blocked)
			chunkNum, chunkOf := ms.lastChunkNum, ms.lastChunkOf
			ms.mu.Unlock()

			elapsed := time.Since(startTime)
			chunkInfo := ""
			if chunkOf > 1 {
				chunkInfo = fmt.Sprintf(" [chunk %d/%d]", chunkNum, chunkOf)
			}
			if blockedCount > 0 {
				log.Printf("[MONITOR-SCAN] cycle #%d DONE%s in %v — %d blocked, %d safe. Sleep %v...",
					iter, chunkInfo, elapsed, blockedCount, totalDomains-blockedCount, interval)
			} else {
				log.Printf("[MONITOR-SCAN] cycle #%d DONE%s in %v — semua %d domain SAFE ✅. Sleep %v...",
					iter, chunkInfo, elapsed, totalDomains, interval)
			}
		}

		time.Sleep(interval)
	}
}

type scanEntry struct {
	domain string
	label  string
}

// pickChunk: pilih chunk dari sorted active list, advance cursor untuk tick berikutnya.
// Return: chunk yang harus di-cek di tick ini, chunkNum (1-based), totalChunks.
// Sticky+force-blocked HARUS udah ke-filter sebelum dipanggil (mereka skip dari API call).
func (ms *MonitorScanner) pickChunk(active []scanEntry) ([]scanEntry, int, int) {
	if len(active) == 0 {
		ms.mu.Lock()
		ms.cursor = 0
		ms.lastChunkNum, ms.lastChunkOf = 0, 0
		ms.mu.Unlock()
		return nil, 0, 0
	}

	// Sort deterministic biar cursor konsisten antar tick (kalau domain ada
	// yang ditambah/dihapus di tengah, cursor masih reasonably stable).
	sort.Slice(active, func(i, j int) bool { return active[i].domain < active[j].domain })

	// Kalau total ≤ ChunkSize → full scan tiap tick (no rotation)
	if len(active) <= ChunkSize {
		ms.mu.Lock()
		ms.cursor = 0
		ms.lastChunkNum, ms.lastChunkOf = 1, 1
		ms.mu.Unlock()
		return active, 1, 1
	}

	// Rotating batch
	ms.mu.Lock()
	start := ms.cursor
	if start >= len(active) || start < 0 {
		start = 0
	}
	end := start + ChunkSize
	var chunk []scanEntry
	if end > len(active) {
		chunk = append(chunk, active[start:]...)
		chunk = append(chunk, active[:end-len(active)]...)
	} else {
		chunk = active[start:end]
	}
	totalChunks := (len(active) + ChunkSize - 1) / ChunkSize
	chunkNum := (start / ChunkSize) + 1
	if chunkNum > totalChunks {
		chunkNum = totalChunks
	}
	// Advance cursor untuk tick berikutnya
	next := end
	if next >= len(active) {
		next = 0
	}
	ms.cursor = next
	ms.lastChunkNum, ms.lastChunkOf = chunkNum, totalChunks
	ms.mu.Unlock()
	return chunk, chunkNum, totalChunks
}

// GetChunkInfo: untuk display di Status menu.
// Return (chunkNum, totalChunks, cursor, chunkSize).
func (ms *MonitorScanner) GetChunkInfo() (int, int, int, int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.lastChunkNum, ms.lastChunkOf, ms.cursor, ChunkSize
}

func (ms *MonitorScanner) scanOnce() {
	all := ms.domains.GetAll()
	if len(all) == 0 {
		return
	}

	// Build a unique list of (domain, label)
	var entries []scanEntry
	seen := make(map[string]bool)
	for label, doms := range all {
		for _, d := range doms {
			if seen[d] {
				continue
			}
			seen[d] = true
			entries = append(entries, scanEntry{domain: d, label: label})
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

	// Filter sticky+force-blocked dari pool chunkable.
	// Domain udah dikenal blocked → gak perlu hit API lagi, spam loop tetap jalan
	// via ms.blocked map. Hemat kuota dan kasih ruang chunk buat domain SAFE.
	var active []scanEntry
	skippedSticky := 0
	for _, e := range entries {
		if blocked, _ := ms.chk.IsSticky(e.domain); blocked {
			skippedSticky++
			// Pastikan ms.blocked tracks domain ini (kalau belum, register sekarang)
			ms.ensureBlockedTracked(e.domain, e.label)
			// Retry klikcepat swap tiap cycle kalau location_url masih match
			// (idempotent — skip otomatis kalau host udah beda)
			if ms.klikcepat != nil && ms.klikcepat.HasCredentials() && ms.klikcepatRotators != nil {
				ms.triggerKlikcepatAutoSwap(e.domain, e.label)
			}
			continue
		}
		if ms.chk.IsForceBlocked(e.domain) {
			skippedSticky++
			ms.ensureBlockedTracked(e.domain, e.label)
			if ms.klikcepat != nil && ms.klikcepat.HasCredentials() && ms.klikcepatRotators != nil {
				ms.triggerKlikcepatAutoSwap(e.domain, e.label)
			}
			continue
		}
		active = append(active, e)
	}

	// Pick chunk untuk tick ini (rotating kalau >ChunkSize)
	chunk, chunkNum, totalChunks := ms.pickChunk(active)
	if totalChunks > 1 {
		log.Printf("[MONITOR-SCAN] chunk %d/%d — cek %d/%d domain aktif (sticky-blocked skip: %d)",
			chunkNum, totalChunks, len(chunk), len(active), skippedSticky)
	} else if skippedSticky > 0 {
		log.Printf("[MONITOR-SCAN] full scan — %d domain aktif (sticky-blocked skip: %d)",
			len(chunk), skippedSticky)
	}

	// Scan paralel dengan semaphore + delay antar request per worker.
	sem := make(chan struct{}, MonitorMaxConcurrent)
	var wg sync.WaitGroup
	for _, e := range chunk {
		wg.Add(1)
		go func(e scanEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ms.checkOne(e.domain, e.label)
			time.Sleep(PerCheckDelay)
		}(e)
	}
	wg.Wait()
}

// ensureBlockedTracked — kalau domain udah sticky/force-blocked tapi belum
// ada di ms.blocked (misal: sticky di-load dari file saat startup), register
// supaya spam loop tetap jalanin alert.
func (ms *MonitorScanner) ensureBlockedTracked(domain, label string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if _, exists := ms.blocked[domain]; exists {
		return
	}
	now := time.Now()
	ms.blocked[domain] = &blockCycle{
		domain:        domain,
		label:         label,
		firstDetected: now,
		lastAlertSent: now, // delay first alert sampai SpamInterval — gak spam mendadak saat restart
		alertCount:    0,
		swapped:       false,
	}
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
				alertCount:    1,
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

		// Klikcepat auto-swap: dipanggil SETIAP cycle BLOCKED (idempotent — skip kalau host udah beda).
		// Decoupled dari CF — biar kalau CF gak ada rotator, klikcepat tetep jalan.
		if ms.klikcepat != nil && ms.klikcepat.HasCredentials() && ms.klikcepatRotators != nil {
			ms.triggerKlikcepatAutoSwap(domain, label)
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
// Spam CONTINUOUS tiap MonitorSpamInterval, gak ada cooldown.
// Berhenti cuma kalau domain di-hapus dari Monitor (akan auto-cleared di scanOnce).
func (ms *MonitorScanner) evaluateSpam(cycle *blockCycle, now time.Time) {
	if now.Sub(cycle.lastAlertSent) >= MonitorSpamInterval {
		cycle.alertCount++
		cycle.lastAlertSent = now
		go ms.sendBlockAlert(cycle, false)
	}
}

// sendBlockAlert kirim notif blocked. firstTime=true untuk first detection.
func (ms *MonitorScanner) sendBlockAlert(cycle *blockCycle, firstTime bool) {
	var prefix, modeText string

	if firstTime {
		prefix = "🚨 *DOMAIN KENA NAWALA!*"
		modeText = "🆕 Baru pertama kali terdeteksi"
	} else {
		prefix = fmt.Sprintf("🛑 *MASIH BLOCKED!* [Alert #%d]", cycle.alertCount)
		modeText = "🔄 Spam continuous — gak akan berhenti sampai kamu hapus domain"
	}

	swapNote := ""
	if cycle.swapped {
		swapNote = "\n✅ _CF Rule udah ke-swap ke domain backup. Domain ini tetap dipantau._"
	}

	// Hitung berapa lama domain udah blocked
	elapsed := time.Since(cycle.firstDetected)
	var durasi string
	switch {
	case elapsed < time.Minute:
		durasi = fmt.Sprintf("%d detik", int(elapsed.Seconds()))
	case elapsed < time.Hour:
		durasi = fmt.Sprintf("%d menit", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		durasi = fmt.Sprintf("%.1f jam", elapsed.Hours())
	default:
		durasi = fmt.Sprintf("%.1f hari", elapsed.Hours()/24)
	}

	msg := fmt.Sprintf(
		"%s\n"+
			"📛 Label: `%s`\n"+
			"🌐 Domain: `%s`\n"+
			"📅 Pertama detect: %s\n"+
			"⏱ Sudah blocked: %s\n"+
			"%s%s\n\n"+
			"_💡 Klik tombol di bawah buat langsung hapus, atau biarin spam terus sampai dihapus._\n"+
			"_Kominfo udah gak di-cek lagi — sticky cache aktif (hemat API)._",
		prefix, cycle.label, cycle.domain,
		cycle.firstDetected.Format("02/01 15:04:05"),
		durasi,
		modeText, swapNote,
	)
	ms.notify.NotifyBlockedAlert(msg, cycle.domain)
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
	// NOTE: triggerKlikcepatAutoSwap dipanggil terpisah dari BLOCKED detector,
	// gak nested di sini biar gak ke-skip kalau CF rules kosong.
}

// triggerKlikcepatAutoSwap — scan klikcepat rotators, swap location_url if matches blocked domain.
func (ms *MonitorScanner) triggerKlikcepatAutoSwap(blockedDomain, blockedLabel string) {
	rotators := ms.klikcepatRotators.GetAll()
	activeCount := 0
	for _, r := range rotators {
		if r.Active {
			activeCount++
		}
	}
	log.Printf("[KLIKCEPAT-SWAP] check blocked=%s vs %d rotator (%d aktif)",
		blockedDomain, len(rotators), activeCount)

	matched := 0
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
		log.Printf("[KLIKCEPAT-SWAP] rotator=%s link=/%s location_host=%q vs blocked=%q",
			rot.Label, rot.LinkURL, currentHost, blockedDomain)
		if !strings.EqualFold(currentHost, blockedDomain) {
			continue
		}
		matched++
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
		log.Printf("[KLIKCEPAT-SWAP] attempt link=%d (/%s): %q → %q",
			rot.LinkID, rot.LinkURL, link.LocationURL, newLocationURL)
		if err := ms.klikcepat.UpdateLinkLocation(rot.LinkID, newLocationURL); err != nil {
			if ms.history != nil {
				ms.history.LogSwap("klikcepat-scan", rot.Label, rot.LinkURL, link.LocationURL, newLocationURL, false, err.Error())
			}
			log.Printf("[KLIKCEPAT-SWAP] FAIL link=%d url=%q err=%v",
				rot.LinkID, newLocationURL, err)
			ms.notify.Notify(fmt.Sprintf(
				"❌ *Klikcepat AUTO-SWAP GAGAL*\n"+
					"🔗 Link: `/%s`\n"+
					"📤 Dari: `%s`\n"+
					"📥 Ke: `%s`\n"+
					"⚠️ Error: %v",
				rot.LinkURL, link.LocationURL, newLocationURL, err))
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
	if matched == 0 && activeCount > 0 {
		log.Printf("[KLIKCEPAT-SWAP] no rotator matched blocked=%s (cek apakah location_url benar2 pake domain itu)", blockedDomain)
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
