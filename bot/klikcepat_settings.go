package bot

import (
	"fmt"
	"sort"
	"strconv"
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
		statusURL = "✅ `" + baseURL + "`"
	}
	statusKey := "❌ belum di-set"
	if apiKey != "" {
		statusKey = "✅ `" + store.MaskAPIKey(apiKey) + "`"
	}
	statusDomain := "klikcepat.com (default)"
	if displayDomain != "" {
		statusDomain = "✅ `" + displayDomain + "`"
	}

	mapCount := len(h.creds.GetKlikcepatDomainMap())
	statusMappings := fmt.Sprintf("✅ %d mapping", mapCount)
	if mapCount == 0 {
		statusMappings = "❌ belum di-set"
	}

	text := fmt.Sprintf(
		"💎 *K L I K C E P A T   S E T T I N G S* 💎\n"+
			"|\n"+
			"📋 *CREDENTIALS*\n"+
			"└ 🌐 Base URL : %s\n"+
			"└ 🔑 API Key  : %s\n"+
			"|\n"+
			"🏷 *DISPLAY & MAPPING*\n"+
			"└ Display Domain  : %s\n"+
			"└ Domain Mappings : %s\n"+
			"|\n"+
			"💡 *INFO*\n"+
			"└ Display Domain buat custom shared domain\n"+
			"└ Domain Mapping buat resolve domain_id → host",
		statusURL, statusKey, statusDomain, statusMappings,
	)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🌐 Set Base URL", cbSettingsKlikcepatSetURL),
			m.Data("🔑 Set API Key", cbSettingsKlikcepatSetKey),
		),
		m.Row(
			m.Data("🏷 Manage Domain Mappings", cbSettingsKlikcepatDomMap),
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
	prompt := "💎 *S E T   B A S E   U R L* 💎\n" +
		"|\n" +
		"🌐 *INPUT*\n" +
		"└ Ketik URL klikcepat (tanpa trailing slash)\n" +
		"└ Contoh: `https://klikcepat.com`\n" +
		"|\n" +
		"💡 *NOTE*\n" +
		"└ Default: klikcepat.com\n" +
		"└ Bisa custom kalau lo deploy Pixly sendiri"
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
	prompt := "💎 *S E T   A P I   K E Y* 💎\n" +
		"|\n" +
		"🔑 *INPUT*\n" +
		"└ Paste API key klikcepat di chat\n" +
		"|\n" +
		"📍 *CARA AMBIL*\n" +
		"└ 1. Login klikcepat → Account\n" +
		"└ 2. Tab API\n" +
		"└ 3. Klik Generate API Key\n" +
		"└ 4. Copy → paste sini\n" +
		"|\n" +
		"🔒 *KEAMANAN*\n" +
		"└ Pesan auto-deleted setelah disimpan"
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

// ─── Domain Mappings Management ──────────────────────────────────────────────

func (h *Handler) handleSettingsKlikcepatDomMap(c tele.Context) error {
	mapping := h.creds.GetKlikcepatDomainMap()
	var sb strings.Builder
	sb.WriteString("🏷 *Domain Mappings*\n═══════════════════════════\n\n")
	sb.WriteString("Map `domain_id` (dari klikcepat) ke host URL.\n")
	sb.WriteString("Bot pake mapping ini buat display URL link akurat.\n\n")
	if len(mapping) == 0 {
		sb.WriteString("📭 Belum ada mapping.\nKlik *➕ Tambah* buat mulai.\n\n")
	} else {
		sb.WriteString("*Current mappings:*\n")
		ids := make([]int, 0, len(mapping))
		for id := range mapping {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			sb.WriteString(fmt.Sprintf("• ID `%d` → `%s`\n", id, escapeMD(mapping[id])))
		}
		sb.WriteString("\nID `0` = klikcepat.com default — gak perlu di-map.\n")
	}

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Tambah Mapping", cbSettingsKlikcepatDomMapAdd),
			m.Data("🗑 Hapus Mapping", cbSettingsKlikcepatDomMapDel),
		),
		m.Row(m.Data("🔙 Kembali", cbSettingsKlikcepat)),
	)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatDomMapAdd(c tele.Context) error {
	h.cancelPriorPrompt(c, StepSettingsKlikcepatDomMapID)
	prompt := "➕ *Tambah Domain Mapping — Step 1/2*\n\n" +
		"Ketik *domain ID* dari klikcepat (angka).\n\n" +
		"*Contoh:* `2` (untuk klikcepat.vip)\n\n" +
		"💡 Cara tau ID: liat di klikcepat admin panel → Domains → URL bar nampilin ID."
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepSettingsKlikcepatDomMapID,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardSettingsKlikcepatDomMapID(c tele.Context, sess *Session) error {
	h.showTyping(c)
	idStr := strings.TrimSpace(c.Text())
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return h.reply(c, "⚠️ ID harus angka positif (e.g., `2`). Coba lagi:", cancelMenu())
	}
	sess.Data["domain_id"] = idStr
	sess.Step = StepSettingsKlikcepatDomMapHost
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"➕ *Step 2/2 — Host*\n\nID: `%d` ✅\n\n"+
			"Ketik *host domain* (tanpa https://, tanpa trailing slash).\n\n"+
			"*Contoh:*\n• `klikcepat.vip`\n• `thymeband.com`\n• `links.maha-domain.com`",
		id)
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardSettingsKlikcepatDomMapHost(c tele.Context, sess *Session) error {
	h.showTyping(c)
	host := strings.TrimSpace(c.Text())
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	if host == "" {
		return h.reply(c, "⚠️ Host kosong. Coba lagi:", cancelMenu())
	}
	id, _ := strconv.Atoi(sess.Data["domain_id"])
	h.creds.SetKlikcepatDomainMapping(id, host)
	h.sessions.Delete(c.Sender().ID)
	return h.reply(c,
		fmt.Sprintf("✅ *Mapping tersimpan!*\n\n• ID `%d` → `%s`\n\nSemua link dengan `domain_id` = `%d` sekarang display dengan host ini.", id, escapeMD(host), id),
		backToSettings(), tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatDomMapDel(c tele.Context) error {
	mapping := h.creds.GetKlikcepatDomainMap()
	if len(mapping) == 0 {
		return c.Edit("📭 Belum ada mapping untuk dihapus.",
			backToSettings(), tele.ModeMarkdown)
	}
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	ids := make([]int, 0, len(mapping))
	for id := range mapping {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 ID %d → %s", id, mapping[id]),
			cbSettingsKlikcepatDomMapDelID, strconv.Itoa(id))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbSettingsKlikcepatDomMap)))
	m.Inline(rows...)
	return c.Edit("🗑 *Pilih mapping yang mau dihapus:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleSettingsKlikcepatDomMapDelID(c tele.Context) error {
	idStr := extractParam(c)
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		return h.handleSettingsKlikcepatDomMapDel(c)
	}
	if h.creds.RemoveKlikcepatDomainMapping(id) {
		return c.Edit(fmt.Sprintf("✅ Mapping ID `%d` dihapus.", id),
			backToSettings(), tele.ModeMarkdown)
	}
	return c.Edit("⚠️ Mapping gak ditemukan.", backToSettings(), tele.ModeMarkdown)
}
