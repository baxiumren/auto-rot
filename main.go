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

	// CF Client
	cf := cloudflare.New(cfg.CFEmail, cfg.CFAPIKey)

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

	// Rotator service
	rotSvc := rotator.New(cf, domains, cfrules, rotators, notify, cfg.CheckInterval)

	// Handler bot
	h := bot.New(b, cfg, domains, cfrules, rotators, cf, rotSvc)
	h.Register()

	// Start auto rotator di background
	rotSvc.Start()

	log.Printf("✅ BongBot started | interval=%v | admins=%d",
		cfg.CheckInterval, len(cfg.AdminIDs))

	// Keep alive — checker package imported to ensure it's compiled
	_ = checker.Clean

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
