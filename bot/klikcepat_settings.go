package bot

import (
	"fmt"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Settings ──────────────────────────────────────────────────────
//
// Settings → 🔗 Klikcepat section. User dapat:
//   - Set Base URL (default: https://klikcepat.com)
//   - Set API Key (Bearer token dari normal admin account)
//   - Test Koneksi (call /api/user untuk verify)
//   - Hapus credentials

func (h *Handler) handleSettingsKlikcepat(c tele.Context) error {
	cred := h.creds.Get()
	baseURL := cred.KlikcepatBaseURL
	apiKey := cred.KlikcepatAPIKey
	displayDomain := cred.KlikcepatDisplayDomain

	statusURL := "❌ belum di-set"
	if baseURL != "" {
		statusURL = "✅ `" + escapeMD(baseURL) + "`"
	}
	statusKey := "❌ belum di-set"
	if apiKey != "" {
		statusKey = "✅ `" + escapeMD(store.MaskAPIKey(apiKey)) + "`"
	}
	statusDomain := "klikcepat.com (default)"
	if displayDomain != "" {
		statusDomain = "✅ `" + escapeMD(displayDomain) + "`"
	}

	text := fmt.Sprintf(
		"🔗 *Klikcepat Settings*\n\n"+
			"🌐 *Base URL:* %s\n"+
			"🔑 *API Key:* %s\n"+
			"🏷 *Display Domain:* %s\n\n"+
			"_Display Domain: untuk custom domain yang di-share dari master account (sub-user API gak see domain)._",
		statusURL, statusKey, statusDomain,
	)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🌐 Set Base URL", cbSettingsKlikcepatSetURL),
			m.Data("🔑 Set API Key", cbSettingsKlikcepatSetKey),
		),
		m.Row(
			m.Data("🏷 Set Display Domain", cbSettingsKlikcepatSetDomain),
		),
		m.Row(
			m.Data("✅ Test Koneksi", cbSettingsKlikcepatTest),
		),
		m.Row(
			m.Data("🗑 Hapus Credentials", cbSettingsKlikcepatClear),
		),
		m.Row(m.Data("🔙 Kembali", cbSettings)),
	)
	return c.Edit(text, m, tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatSetURL(c tele.Context) error {
	h.cancelPriorPrompt(c, StepSettingsKlikcepatURL)
	prompt := "🌐 *Set Klikcepat Base URL*\n\n" +
		"Ketik URL klikcepat kamu (tanpa trailing slash).\n\n" +
		"*Contoh:* `https://klikcepat.com`"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsKlikcepatURL,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardSettingsKlikcepatURL(c tele.Context, sess *Session) error {
	h.showTyping(c)
	url := strings.TrimSpace(c.Text())
	if url == "" || (!strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://")) {
		return h.reply(c, "❌ URL invalid — harus mulai dengan http:// atau https://", cancelMenu())
	}
	h.creds.SetKlikcepatBaseURL(url)
	h.applyKlikcepatCreds()
	h.sessions.Delete(c.Sender().ID)
	return h.reply(c,
		fmt.Sprintf("✅ Base URL tersimpan: `%s`", escapeMD(url)),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatSetKey(c tele.Context) error {
	h.cancelPriorPrompt(c, StepSettingsKlikcepatKey)
	prompt := "🔑 *Set Klikcepat API Key*\n\n" +
		"Ketik API key dari klikcepat (normal admin account).\n\n" +
		"📍 *Cara ambil:*\n" +
		"1. Login klikcepat → Account → API\n" +
		"2. Klik *Generate API Key*\n" +
		"3. Copy → paste di sini\n\n" +
		"🔒 _Pesan kamu akan otomatis dihapus setelah disimpan._"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsKlikcepatKey,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardSettingsKlikcepatKey(c tele.Context, sess *Session) error {
	h.showTyping(c)
	key := strings.TrimSpace(c.Text())
	if len(key) < 20 {
		return h.reply(c, "⚠️ API key terlalu pendek. Coba lagi:", cancelMenu())
	}
	h.creds.SetKlikcepatAPIKey(key)
	h.applyKlikcepatCreds()
	h.sessions.Delete(c.Sender().ID)
	_ = h.bot.Delete(c.Message())
	return h.reply(c,
		fmt.Sprintf("✅ API Key tersimpan: `%s`\n\n_Pesan API key kamu sudah dihapus._",
			escapeMD(store.MaskAPIKey(key))),
		backToSettings(), tele.ModeMarkdown)
}

// handleSettingsKlikcepatSetDomain — set custom display domain (untuk kasus
// custom domain di-share dari master account, sub-user API gak see domain list).
func (h *Handler) handleSettingsKlikcepatSetDomain(c tele.Context) error {
	h.cancelPriorPrompt(c, StepSettingsKlikcepatDomain)
	prompt := "🌐 *Set Custom Display Domain*\n\n" +
		"Ketik nama domain yang dipake untuk display short URL klikcepat kamu.\n\n" +
		"*Contoh:*\n" +
		"• `thymeband.com` (tanpa https://)\n" +
		"• `klikcepat.vip`\n" +
		"• `links.maha-domain.com`\n\n" +
		"_Atau ketik `-` untuk hapus (back to klikcepat.com default)._\n\n" +
		"💡 _Setting ini cuma untuk display di bot — gak ngubah klikcepat-side data._"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsKlikcepatDomain,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardSettingsKlikcepatDomain(c tele.Context, sess *Session) error {
	h.showTyping(c)
	domain := strings.TrimSpace(c.Text())
	if domain == "-" {
		domain = ""
	}
	h.creds.SetKlikcepatDisplayDomain(domain)
	h.sessions.Delete(c.Sender().ID)
	if domain == "" {
		return h.reply(c,
			"✅ Custom display domain dihapus — bot kembali pakai `klikcepat.com` default.",
			backToSettings(), tele.ModeMarkdown)
	}
	return h.reply(c,
		fmt.Sprintf("✅ Display domain tersimpan: `%s`\n\nSemua link sekarang akan ditampilkan dengan domain ini di bot.", escapeMD(domain)),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatTest(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit("⚠️ Credentials belum lengkap. Set Base URL & API Key dulu.",
			backToSettings(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Testing koneksi ke Klikcepat...", backToSettings())
	if err := h.klikcepat.Ping(); err != nil {
		return c.Edit(
			fmt.Sprintf("❌ *Test GAGAL*\n\n```\n%s\n```\n\nCek Base URL & API Key.", escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown)
	}
	return c.Edit(
		"✅ *Test BERHASIL*\n\nCredentials valid — Klikcepat API responding.",
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatClear(c tele.Context) error {
	h.creds.ClearKlikcepat()
	h.applyKlikcepatCreds()
	return c.Edit("✅ *Klikcepat credentials dihapus.*", backToSettings(), tele.ModeMarkdown)
}

// applyKlikcepatCreds re-syncs the client with latest stored credentials.
func (h *Handler) applyKlikcepatCreds() {
	cred := h.creds.Get()
	h.klikcepat.SetCredentials(cred.KlikcepatBaseURL, cred.KlikcepatAPIKey)
}
