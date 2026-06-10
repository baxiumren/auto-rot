package bot

import (
	"fmt"
	"strings"

	"bongbot/klikcepat"
	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── LinkFB Settings ─────────────────────────────────────────────────────────
//
// LinkFB pakai engine Pixly yg sama dengan Klikcepat — instance terpisah.
// Default base URL: https://linkfb.io

func (h *Handler) handleSettingsLinkfb(c tele.Context) error {
	cred := h.creds.Get()
	baseURL := cred.LinkfbBaseURL
	apiKey := cred.LinkfbAPIKey

	statusURL := "❌ belum di-set"
	if baseURL != "" {
		statusURL = "✅ `" + baseURL + "`"
	} else {
		statusURL = "(default: linkfb.io)"
	}
	statusKey := "❌ belum di-set"
	if apiKey != "" {
		statusKey = "✅ `" + store.MaskAPIKey(apiKey) + "`"
	}

	text := fmt.Sprintf(
		"💎 *L I N K F B   S E T T I N G S* 💎\n"+
			"|\n"+
			"📋 *CREDENTIALS*\n"+
			"└ 🌐 Base URL : %s\n"+
			"└ 🔑 API Key  : %s\n"+
			"|\n"+
			"💡 *INFO*\n"+
			"└ Pakai engine Pixly yg sama dengan klikcepat\n"+
			"└ Default base URL: linkfb.io\n"+
			"└ Edit shortlink + project (no biolink edit)",
		statusURL, statusKey,
	)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🌐 Set Base URL", cbSettingsLinkfbSetURL),
			m.Data("🔑 Set API Key", cbSettingsLinkfbSetKey),
		),
		m.Row(m.Data("✅ Test Koneksi", cbSettingsLinkfbTest)),
		m.Row(m.Data("🗑 Hapus Credentials", cbSettingsLinkfbClear)),
		m.Row(m.Data("🔙 Kembali", cbSettings)),
	)
	return c.Edit(text, m, tele.ModeMarkdown)
}

func (h *Handler) handleSettingsLinkfbSetURL(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsLinkfbURL,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *S E T   B A S E   U R L* 💎\n"+
			"|\n"+
			"🌐 *INPUT*\n"+
			"└ Ketik URL LinkFB (tanpa trailing slash)\n"+
			"└ Default: `https://linkfb.io`\n"+
			"|\n"+
			"💡 *NOTE*\n"+
			"└ Kosongin atau ketik `-` buat pake default",
		cancelMenu(), tele.ModeMarkdown)
}

func (h *Handler) wizardSettingsLinkfbURL(c tele.Context, sess *Session) error {
	h.showTyping(c)
	url := strings.TrimSpace(c.Text())
	if url == "-" || url == "" {
		url = "https://linkfb.io"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	h.creds.SetLinkfbBaseURL(url)
	h.applyCredsToLinkfbClient()
	h.sessions.Delete(c.Sender().ID)

	return h.reply(c,
		fmt.Sprintf("✅ LinkFB Base URL tersimpan:\n`%s`", url),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsLinkfbSetKey(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsLinkfbKey,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *S E T   A P I   K E Y* 💎\n"+
			"|\n"+
			"🔑 *INPUT*\n"+
			"└ Paste API key LinkFB di chat\n"+
			"|\n"+
			"📍 *CARA AMBIL*\n"+
			"└ 1. Login linkfb.io → Account\n"+
			"└ 2. Tab API\n"+
			"└ 3. Generate API Key → copy\n"+
			"|\n"+
			"🔒 *KEAMANAN*\n"+
			"└ Pesan auto-deleted setelah disimpan",
		cancelMenu(), tele.ModeMarkdown)
}

func (h *Handler) wizardSettingsLinkfbKey(c tele.Context, sess *Session) error {
	h.showTyping(c)
	key := strings.TrimSpace(c.Text())
	if len(key) < 20 {
		return h.reply(c, "❌ API Key terlalu pendek. Coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	h.creds.SetLinkfbAPIKey(key)
	h.applyCredsToLinkfbClient()
	h.sessions.Delete(c.Sender().ID)

	// Delete user message (security)
	_ = h.bot.Delete(c.Message())

	return h.reply(c,
		fmt.Sprintf("✅ LinkFB API Key tersimpan: `%s`\n\n_(Pesan API key kamu udah dihapus.)_",
			store.MaskAPIKey(key)),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsLinkfbTest(c tele.Context) error {
	if !h.linkfb.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ Set Base URL + API Key dulu",
			ShowAlert: true,
		})
	}
	c.Edit("⏳ Test koneksi ke LinkFB...", tele.ModeMarkdown)
	if err := h.linkfb.Ping(); err != nil {
		return c.Edit(
			fmt.Sprintf("💎 *T E S T   K O N E K S I* 💎\n"+
				"|\n"+
				"❌ *GAGAL*\n"+
				"└ Error: `%s`\n"+
				"|\n"+
				"💡 *FIX*\n"+
				"└ Cek API key valid\n"+
				"└ Cek Base URL bener", err.Error()),
			h.backToLinkfbSettings(), tele.ModeMarkdown)
	}
	return c.Edit(
		"💎 *T E S T   K O N E K S I* 💎\n"+
			"|\n"+
			"✅ *SUCCESS*\n"+
			"└ LinkFB API responsive\n"+
			"└ Credentials valid",
		h.backToLinkfbSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsLinkfbClear(c tele.Context) error {
	h.creds.ClearLinkfb()
	h.applyCredsToLinkfbClient()
	return c.Edit(
		"💎 *C R E D E N T I A L S* 💎\n"+
			"|\n"+
			"🗑 *DIHAPUS*\n"+
			"└ Base URL + API Key cleared\n"+
			"└ LinkFB fitur disabled sampai di-set ulang",
		h.backToLinkfbSettings(), tele.ModeMarkdown)
}

func (h *Handler) backToLinkfbSettings() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbSettingsLinkfb)))
	return m
}

// applyCredsToLinkfbClient — push fresh creds dari store ke runtime client.
func (h *Handler) applyCredsToLinkfbClient() {
	cred := h.creds.Get()
	baseURL := cred.LinkfbBaseURL
	if baseURL == "" {
		baseURL = "https://linkfb.io"
	}
	h.linkfb.SetCredentials(baseURL, cred.LinkfbAPIKey)
}

// Keep import alive (klikcepat package = generic Pixly client)
var _ = klikcepat.New
