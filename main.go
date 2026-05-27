package main

import (
	"log"
	"os"
	"time"

	"bongbot/bot"
	"bongbot/checker"
	"bongbot/cloudflare"
	"bongbot/config"
	"bongbot/klikcepat"
	"bongbot/rotator"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

func main() {
	// Pastikan folder data ada
	os.MkdirAll("data", 0755)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Inisialisasi stores (shared data)
	domains := store.NewDomainStore()
	cfrules := store.NewCFRuleStore()
	rotators := store.NewRotatorStore()
	creds := store.NewCredentialStore()
	history := store.NewHistoryStore()

	// CF Client — credential prioritas: data/credentials.json > .env
	cfEmail := cfg.CFEmail
	cfKey := cfg.CFAPIKey
	if c := creds.Get(); c.CFEmail != "" || c.CFAPIKey != "" {
		if c.CFEmail != "" {
			cfEmail = c.CFEmail
		}
		if c.CFAPIKey != "" {
			cfKey = c.CFAPIKey
		}
		log.Printf("✅ CF credentials loaded dari data/credentials.json")
	} else if cfEmail == "email@example.com" || cfKey == "global_api_key_cloudflare" || cfEmail == "" || cfKey == "" {
		log.Printf("⚠️  CF credentials belum di-set — pakai menu ⚙️ Settings di bot")
		cfEmail = ""
		cfKey = ""
	}
	cf := cloudflare.New(cfEmail, cfKey)

	// Klikcepat (66biolinks) integration — credentials prioritas: credentials.json > .env
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

	// Optional API keys untuk checker (Source 2 & 3)
	if cfg.TrustPositifKey != "" {
		checker.SetAPIKey(cfg.TrustPositifKey)
		log.Printf("✅ TrustPositif API key loaded (Source 2 premium tier)")
	}
	if cfg.NawalaCheckKey != "" {
		checker.SetNawalaCheckKey(cfg.NawalaCheckKey)
		log.Printf("✅ NawalaCheck API key loaded (Source 3 aktif — triple-source mode)")
	}

	// Bot
	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Bot error: %v", err)
	}

	// Auto-resolve BotUsername kalau belum di-set di env (buat deep-link Setup di DM)
	if cfg.BotUsername == "" && b.Me != nil {
		cfg.BotUsername = b.Me.Username
		log.Printf("✅ BotUsername auto-detected: @%s", cfg.BotUsername)
	}

	// Notifier (kirim notif ke chat)
	notify := &telegramNotifier{b: b, chatID: cfg.AllowedChatID}

	// Rotator service (per-CF-rule check)
	rotSvc := rotator.New(cf, domains, cfrules, rotators, notify, cfg.CheckInterval, history)

	// Monitor scanner (scan SEMUA domain di monitor + reactive auto-swap + spam alert).
	// Pool untuk swap diambil dari Rotator config (CFRule → PoolLabel).
	monScanner := rotator.NewMonitorScanner(cf, domains, cfrules, rotators, notify, cfg.CheckInterval, history, klc, klcRotators)

	// Handler bot
	h := bot.New(b, cfg, domains, cfrules, rotators, creds, cf, rotSvc, monScanner, history, klc, klcRotators)
	h.Register()

	// Backfill: rule lama yg field Domain-nya kosong → fetch zone name dari CF.
	// Jalan async biar gak block startup. Skip kalau credentials belum di-set.
	if cf.HasCredentials() {
		go backfillCFRuleDomain(cf, cfrules)
	}

	// Start services
	rotSvc.Start()
	monScanner.Start()

	log.Printf("✅ BongBot started | interval=%v | admins=%d | sticky=%d",
		cfg.CheckInterval, len(cfg.AdminIDs), len(checker.Default().GetStickyList()))

	b.Start()
}

type telegramNotifier struct {
	b      *tele.Bot
	chatID int64
}

// backfillCFRuleDomain — one-time scan: untuk setiap CF rule yang field Domain-nya
// kosong tapi ZoneID-nya ada, fetch zone name dari CF API → update store.
// Best-effort: error logged tapi gak fatal, biar service tetap jalan.
func backfillCFRuleDomain(cf *cloudflare.Client, cfrules *store.CFRuleStore) {
	time.Sleep(2 * time.Second) // tunggu bot ready
	rules := cfrules.GetAll()
	patched := 0
	skipped := 0
	for _, r := range rules {
		if r.Domain != "" {
			continue
		}
		if r.ZoneID == "" {
			skipped++
			continue
		}
		name, err := cf.GetZoneName(r.ZoneID)
		if err != nil {
			log.Printf("[BACKFILL] rule=%s zone=%s gagal fetch: %v", r.Label, r.ZoneID, err)
			skipped++
			continue
		}
		if cfrules.UpdateDomain(r.ID, name) {
			log.Printf("[BACKFILL] rule=%s → Domain=%s", r.Label, name)
			patched++
		}
	}
	if patched > 0 {
		log.Printf("✅ Backfill done: %d rule di-update, %d skipped", patched, skipped)
	}
}

func (n *telegramNotifier) Notify(msg string) {
	if n.chatID == 0 {
		log.Printf("[NOTIFY] %s", msg)
		return
	}
	n.b.Send(&tele.Chat{ID: n.chatID}, msg, tele.ModeMarkdown)
}

// NotifyBlockedAlert: kirim alert blocked + button "🗑 Hapus dari Monitor"
// supaya admin bisa quick-action di group tanpa harus DM bot.
func (n *telegramNotifier) NotifyBlockedAlert(msg, domain string) {
	if n.chatID == 0 {
		log.Printf("[NOTIFY-ALERT] domain=%s | %s", domain, msg)
		return
	}
	mkup := &tele.ReplyMarkup{}
	// Callback "alert_remove|<domain>" — admin-only check di handler
	mkup.Inline(
		mkup.Row(mkup.Data("🗑 Hapus dari Monitor", "alert_remove", domain)),
	)
	if _, err := n.b.Send(&tele.Chat{ID: n.chatID}, msg, mkup, tele.ModeMarkdown); err != nil {
		log.Printf("[NOTIFY-ALERT ERROR] %v", err)
	}
}
