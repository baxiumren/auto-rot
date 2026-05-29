package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Version — release versioning untuk bot.
// Scheme: v1.0 → v1.1 → v1.2 → ... → v1.9 → v2.0 (naik major pas mau "double digit").
// Update tiap release (lihat CHANGELOG.md untuk history).
const Version = "1.5"

type Config struct {
	BotToken        string
	AllowedChatID   int64
	AdminIDs        map[int64]bool
	CFEmail         string
	CFAPIKey        string
	CheckInterval   time.Duration
	TrustPositifKey string // optional: API key untuk trustpositif.id/api/v1
	NawalaCheckKey  string // optional: API key untuk api.nawalacheck.com (Source 3)
	// Klikcepat integration — optional
	KlikcepatBaseURL       string // dari KLIKCEPAT_BASE_URL (untuk API calls, default klikcepat.com)
	KlikcepatAPIKey        string // dari KLIKCEPAT_API_KEY
	KlikcepatDisplayDomain string // dari KLIKCEPAT_DISPLAY_DOMAIN (untuk display URL, contoh: thymeband.com)
	// ContactUsername: handle Telegram (tanpa @) yang ditampilin ke non-admin
	// pas mereka coba DM bot. Default: "hokisetahun".
	ContactUsername string
	// BotUsername: handle bot sendiri (tanpa @) untuk deep-link "Setup di DM" dari group.
	// Default kosong → tombol Setup DM gak muncul (fallback ke instruksi text).
	BotUsername string
}

func Load() (*Config, error) {
	godotenv.Load() // ignore error — .env opsional

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN tidak di-set")
	}

	chatIDStr := os.Getenv("ALLOWED_CHAT_ID")
	chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

	adminIDs := make(map[int64]bool)
	for _, s := range strings.Split(os.Getenv("ADMIN_IDS"), ",") {
		s = strings.TrimSpace(s)
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			adminIDs[id] = true
		}
	}

	interval := 45 * time.Second
	if s := os.Getenv("CHECK_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			interval = d
		}
	}

	contactUser := strings.TrimPrefix(strings.TrimSpace(os.Getenv("CONTACT_USERNAME")), "@")
	if contactUser == "" {
		contactUser = "hokisetahun"
	}
	botUser := strings.TrimPrefix(strings.TrimSpace(os.Getenv("BOT_USERNAME")), "@")

	klikcepatBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("KLIKCEPAT_BASE_URL")), "/")
	klikcepatAPIKey := strings.TrimSpace(os.Getenv("KLIKCEPAT_API_KEY"))
	klikcepatDisplayDomain := strings.TrimSpace(os.Getenv("KLIKCEPAT_DISPLAY_DOMAIN"))
	// Normalize: strip http(s):// + trailing slash → just "thymeband.com"
	klikcepatDisplayDomain = strings.TrimPrefix(klikcepatDisplayDomain, "https://")
	klikcepatDisplayDomain = strings.TrimPrefix(klikcepatDisplayDomain, "http://")
	klikcepatDisplayDomain = strings.TrimSuffix(klikcepatDisplayDomain, "/")

	return &Config{
		BotToken:        token,
		AllowedChatID:   chatID,
		AdminIDs:        adminIDs,
		CFEmail:         os.Getenv("CF_EMAIL"),
		CFAPIKey:        os.Getenv("CF_API_KEY"),
		CheckInterval:   interval,
		TrustPositifKey: os.Getenv("TRUSTPOSITIF_API_KEY"),
		NawalaCheckKey:  os.Getenv("NAWALACHECK_API_KEY"),
		ContactUsername: contactUser,
		BotUsername:     botUser,
		KlikcepatBaseURL: klikcepatBaseURL,
		KlikcepatAPIKey:        klikcepatAPIKey,
		KlikcepatDisplayDomain: klikcepatDisplayDomain,
	}, nil
}

func (c *Config) IsAdmin(userID int64) bool {
	return c.AdminIDs[userID]
}
