package main

import (
	"log"
	"os"
	"time"

	"bongbot/bot"
	"bongbot/checker"
	"bongbot/cloudflare"
	"bongbot/config"
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

	// Bot
	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Bot error: %v", err)
	}

	// Notifier (kirim notif ke chat)
	notify := &telegramNotifier{b: b, chatID: cfg.AllowedChatID}

	// Rotator service (per-CF-rule check)
	rotSvc := rotator.New(cf, domains, cfrules, rotators, notify, cfg.CheckInterval, history)

	// Monitor scanner (scan SEMUA domain di monitor + reactive auto-swap + spam alert).
	// Pool untuk swap diambil dari Rotator config (CFRule → PoolLabel).
	monScanner := rotator.NewMonitorScanner(cf, domains, cfrules, rotators, notify, cfg.CheckInterval, history)

	// Handler bot
	h := bot.New(b, cfg, domains, cfrules, rotators, creds, cf, rotSvc, monScanner, history)
	h.Register()

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

func (n *telegramNotifier) Notify(msg string) {
	if n.chatID == 0 {
		log.Printf("[NOTIFY] %s", msg)
		return
	}
	n.b.Send(&tele.Chat{ID: n.chatID}, msg, tele.ModeMarkdown)
}
