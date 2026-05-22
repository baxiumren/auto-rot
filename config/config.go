package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	AllowedChatID int64
	AdminIDs      map[int64]bool
	CFEmail       string
	CFAPIKey      string
	CheckInterval time.Duration
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

	return &Config{
		BotToken:      token,
		AllowedChatID: chatID,
		AdminIDs:      adminIDs,
		CFEmail:       os.Getenv("CF_EMAIL"),
		CFAPIKey:      os.Getenv("CF_API_KEY"),
		CheckInterval: interval,
	}, nil
}

func (c *Config) IsAdmin(userID int64) bool {
	return c.AdminIDs[userID]
}
