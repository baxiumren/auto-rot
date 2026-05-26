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

	err := c.Send(final, sendOpts)
	// Kalau pesan original udah ke-delete (misal: di security wizard kayak
	// settings API key), Telegram return 400 "message to be replied not found".
	// Retry tanpa ReplyTo — kirim message tetap nyampe walau gak attach reply.
	if err != nil && strings.Contains(err.Error(), "message to be replied not found") {
		sendOpts.ReplyTo = nil
		return c.Send(final, sendOpts)
	}
	return err
}

// showTyping — kirim "typing..." action ke chat biar user tau bot lagi proses.
// Best practice: panggil ini di awal handler yang butuh waktu (network IO,
// CF API call, dll) sebelum hasil ke-kirim. Error diabaikan (best-effort).
func (h *Handler) showTyping(c tele.Context) {
	_ = c.Notify(tele.Typing)
}

// quickToast — kirim popup notif kecil di top Telegram (untuk feedback callback click).
// Beda dengan c.Respond() default yang silent, ini nampilin text ke user.
// Pakai untuk acknowledge slow operation: "⏳ Memproses...", "✅ Done", dll.
func (h *Handler) quickToast(c tele.Context, text string) {
	_ = c.Respond(&tele.CallbackResponse{Text: text})
}

// cancelPriorPrompt — kalau user punya session aktif yang beda dari step baru,
// edit prompt message lama jadi "❌ Dibatalkan (mulai aksi baru)" biar gak nyangkut
// nyasar di group chat. Dipanggil di setiap entry-point handler sebelum set session baru.
func (h *Handler) cancelPriorPrompt(c tele.Context, newStep Step) {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess == nil {
		return
	}
	if sess.Step == newStep {
		return // sama, biarin
	}
	if sess.PromptMsg == nil {
		return
	}
	// Best-effort edit; abaikan error (mungkin message udah ke-edit / di-delete).
	h.bot.Edit(sess.PromptMsg,
		"❌ _Wizard sebelumnya dibatalkan — kamu mulai aksi baru._",
		tele.ModeMarkdown)
}

// promptUser — kirim wizard prompt baru sebagai pesan di chat dengan user-tag + ReplyTo.
// Dipakai di tengah wizard (Step 2+) supaya di group chat user tau prompt ini untuk dia.
// Return message yang baru di-send (buat di-simpan ke sess.PromptMsg).
func (h *Handler) promptUser(c tele.Context, text string, markup *tele.ReplyMarkup) *tele.Message {
	final := userTag(c.Sender()) + " " + text
	sendOpts := &tele.SendOptions{
		ReplyTo:     c.Message(),
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: markup,
	}
	msg, err := h.bot.Send(c.Chat(), final, sendOpts)
	if err != nil {
		return nil
	}
	return msg
}
