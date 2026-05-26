package bot

import (
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// userTag mengembalikan markdown mention user — pakai @username kalau ada,
// fallback ke inline mention by ID (works tanpa username).
// Pakai ini di awal pesan bot biar user dapet notification & tau pesan ini untuk dia.
func userTag(u *tele.User) string {
	if u == nil {
		return ""
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	name := u.FirstName
	if name == "" {
		name = "User"
	}
	// Escape markdown-special chars di nama biar gak break formatting
	name = strings.NewReplacer(
		"[", "(",
		"]", ")",
		"_", " ",
		"*", " ",
		"`", " ",
	).Replace(name)
	return fmt.Sprintf("[%s](tg://user?id=%d)", name, u.ID)
}

// reply adalah pengganti c.Send untuk wizard text responses.
// Tag user di awal pesan + reply ke message user → di group chat,
// jelas pesan ini untuk siapa & user dapet notification.
//
// Usage:
//
//	h.reply(c, "✅ Domain ditambahkan", backToMonitor())
//	h.reply(c, "❌ Error", cancelMenu())
//
// Default parseMode = Markdown. Bisa di-override dengan tele.ModeMarkdownV2.
func (h *Handler) reply(c tele.Context, text string, opts ...interface{}) error {
	final := userTag(c.Sender()) + " " + text

	sendOpts := &tele.SendOptions{
		ReplyTo:   c.Message(),
		ParseMode: tele.ModeMarkdown,
	}

	for _, opt := range opts {
		switch v := opt.(type) {
		case *tele.ReplyMarkup:
			sendOpts.ReplyMarkup = v
		case tele.ParseMode:
			sendOpts.ParseMode = v
		}
	}

	return c.Send(final, sendOpts)
}
