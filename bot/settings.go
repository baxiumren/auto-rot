package bot

import (
	"fmt"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Settings Hub ────────────────────────────────────────────────────────────
// handleSettings = top-level Settings entry. Shows hub picker (CF / Klikcepat).
// CF specifics di handleSettingsCF. Klikcepat di handleSettingsKlikcepat.

func (h *Handler) handleSettings(c tele.Context) error {
	cfStatus := "❌ belum di-set"
	if h.cf.HasCredentials() {
		cfStatus = "✅ aktif"
	}
	klcStatus := "❌ belum di-set"
	if h.klikcepat.HasCredentials() {
		klcStatus = "✅ aktif"
	}

	// Plain text only — no italic/underscore to avoid markdown parse errors
	// from emoji + special chars combinations.
	text := fmt.Sprintf(
		"🔧 *Settings — Pilih Akun*\n\n"+
			"Bot ini connect ke 2 platform external:\n\n"+
			"⚙️ *Cloudflare*: %s\n"+
			"   Untuk auto-rotate redirect rule Cloudflare.\n\n"+
			"🔗 *Klikcepat*: %s\n"+
			"   Untuk auto-rotate target URL biolink / shortlink.\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"Pilih platform yang mau di-configure:",
		cfStatus, klcStatus,
	)
	return c.Edit(text, settingsMenu(), tele.ModeMarkdown)
}

// handleSettingsCF — CF-specific settings (was the OLD handleSettings body).
func (h *Handler) handleSettingsCF(c tele.Context) error {
	creds := h.creds.Get()

	emailDisplay := creds.CFEmail
	if emailDisplay == "" {
		emailDisplay = "_(belum di-set)_"
	}
	keyDisplay := store.MaskAPIKey(creds.CFAPIKey)

	statusIcon := "❌"
	statusText := "Belum lengkap — set dulu email & API key di bawah"
	if h.cf.HasCredentials() {
		statusIcon = "✅"
		statusText = "Aktif & terhubung"
	}

	text := fmt.Sprintf(
		"⚙️ *Settings — Akun Cloudflare*\n\n"+
			"Di sini kamu *connect-in akun Cloudflare* ke bot. Bot butuh ini buat bisa baca & update redirect rule kamu otomatis.\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"📧 *Email Cloudflare:*\n`%s`\n\n"+
			"🔑 *Global API Key:*\n`%s`\n\n"+
			"%s *Status:* %s\n"+
			"━━━━━━━━━━━━━━━━━━\n\n"+
			"💡 *Tips pertama kali setup:*\n"+
			"Klik *🔄 Set Keduanya* untuk isi email + API key sekaligus (2 langkah wizard).\n\n"+
			"🔒 _Credentials disimpan aman di file lokal (`data/credentials.json`, permission 0600)._",
		escapeMD(emailDisplay), escapeMD(keyDisplay), statusIcon, statusText,
	)

	return c.Edit(text, settingsCFMenu(), tele.ModeMarkdown)
}

// ─── Set Email Only ───────────────────────────────────────────────────────────

func (h *Handler) handleSettingsSetEmail(c tele.Context) error {
	// Defensive: clear any lingering session sebelum mulai wizard sensitif
	h.sessions.Delete(c.Sender().ID)
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepSettingsEmail,
		Data: map[string]string{},
	})
	return c.Edit(
		"📧 *Set Email Cloudflare*\n\n"+
			"Ketik email yang kamu pakai untuk login ke Cloudflare.\n\n"+
			"📍 *Cara cek email kamu:*\n"+
			"1. Buka cloudflare.com → login\n"+
			"2. Klik foto profil kanan atas\n"+
			"3. Lihat email di sana\n\n"+
			"*Contoh:* `nama-kamu@gmail.com`",
		cancelMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) wizardSettingsEmail(c tele.Context, sess *Session) error {
	email := strings.TrimSpace(c.Text())
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return h.reply(c, "⚠️ Format email tidak valid. Coba lagi:", cancelMenu())
	}
	h.creds.SetEmail(email)
	h.applyCredsToCFClient()
	h.sessions.Delete(c.Sender().ID)

	return h.reply(c, 
		fmt.Sprintf("✅ CF Email berhasil di-set:\n`%s`", escapeMD(email)),
		backToSettings(), tele.ModeMarkdown,
	)
}

// ─── Set API Key Only ─────────────────────────────────────────────────────────

func (h *Handler) handleSettingsSetKey(c tele.Context) error {
	// Defensive: clear any lingering session sebelum wizard sensitif
	h.sessions.Delete(c.Sender().ID)
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepSettingsKey,
		Data: map[string]string{},
	})
	return c.Edit(
		"🔑 *Set Global API Key Cloudflare*\n\n"+
			"📍 *Cara ambil Global API Key (5 langkah):*\n"+
			"1. Buka cloudflare.com → login\n"+
			"2. Klik foto profil kanan atas → *My Profile*\n"+
			"3. Pilih tab *API Tokens*\n"+
			"4. Scroll ke bagian *Global API Key* → klik *View*\n"+
			"5. Masukin password CF → copy kode yang muncul\n\n"+
			"Lalu paste kode tersebut di chat ini.\n\n"+
			"⚠️ *Aman:* pesan kamu akan otomatis dihapus dari chat begitu disimpan, jadi gak terlihat orang lain di grup.",
		cancelMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) wizardSettingsKey(c tele.Context, sess *Session) error {
	key := strings.TrimSpace(c.Text())
	if len(key) < 20 {
		return h.reply(c, "⚠️ API Key terlalu pendek. Pastikan kamu copy Global API Key yang benar:", cancelMenu())
	}
	h.creds.SetAPIKey(key)
	h.applyCredsToCFClient()
	h.sessions.Delete(c.Sender().ID)

	// Hapus pesan API key dari chat (security)
	_ = h.bot.Delete(c.Message())

	return h.reply(c, 
		fmt.Sprintf("✅ CF API Key berhasil di-set:\n`%s`\n\n_Pesan API key kamu sudah dihapus dari chat._",
			escapeMD(store.MaskAPIKey(key))),
		backToSettings(), tele.ModeMarkdown,
	)
}

// ─── Set Both (Email + API Key) ───────────────────────────────────────────────

func (h *Handler) handleSettingsSetBoth(c tele.Context) error {
	// Defensive: clear any lingering session sebelum wizard sensitif
	h.sessions.Delete(c.Sender().ID)
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepSettingsBothEmail,
		Data: map[string]string{},
	})
	return c.Edit(
		"🔄 *Setup Email + API Key (Wizard 2 Langkah)*\n\n"+
			"_Langkah 1 dari 2_\n\n"+
			"📧 *Ketik email Cloudflare kamu:*\n\n"+
			"📍 _Cara cek: cloudflare.com → klik foto profil kanan atas → lihat email_\n\n"+
			"*Contoh:* `nama-kamu@gmail.com`",
		cancelMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) wizardSettingsBothEmail(c tele.Context, sess *Session) error {
	email := strings.TrimSpace(c.Text())
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return h.reply(c, "⚠️ Format email tidak valid. Coba lagi:", cancelMenu())
	}
	sess.Data["email"] = email
	sess.Step = StepSettingsBothKey
	h.sessions.Set(c.Sender().ID, sess)

	return h.reply(c, 
		fmt.Sprintf(
			"✅ Email tersimpan: `%s`\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"_Langkah 2 dari 2_\n\n"+
				"🔑 *Ketik Global API Key Cloudflare kamu:*\n\n"+
				"📍 *Cara ambil:*\n"+
				"1. cloudflare.com → klik foto profil → *My Profile*\n"+
				"2. Tab *API Tokens* → bagian *Global API Key* → klik *View*\n"+
				"3. Masukin password CF → copy kode → paste di sini\n\n"+
				"🔒 _Pesan kamu akan otomatis dihapus dari chat setelah disimpan._",
			escapeMD(email)),
		cancelMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) wizardSettingsBothKey(c tele.Context, sess *Session) error {
	key := strings.TrimSpace(c.Text())
	if len(key) < 20 {
		return h.reply(c, "⚠️ API Key terlalu pendek. Coba lagi:", cancelMenu())
	}
	email := sess.Data["email"]
	h.creds.Set(email, key)
	h.applyCredsToCFClient()
	h.sessions.Delete(c.Sender().ID)

	// Hapus pesan API key dari chat
	_ = h.bot.Delete(c.Message())

	return h.reply(c, 
		fmt.Sprintf(
			"✅ *Credentials tersimpan!*\n\n📧 Email: `%s`\n🔑 API Key: `%s`\n\n"+
				"_Pesan API key kamu sudah dihapus._\n\nKlik *Test Koneksi* buat verifikasi.",
			escapeMD(email), escapeMD(store.MaskAPIKey(key)),
		),
		backToSettings(), tele.ModeMarkdown,
	)
}

// ─── Test Connection ──────────────────────────────────────────────────────────

func (h *Handler) handleSettingsTest(c tele.Context) error {
	if !h.cf.HasCredentials() {
		return c.Edit(
			"⚠️ *Credentials belum lengkap*\n\nSet dulu email & API key sebelum test.",
			backToSettings(), tele.ModeMarkdown,
		)
	}

	c.Edit("⏳ Testing koneksi ke Cloudflare...", backToSettings())

	if err := h.cf.Ping(); err != nil {
		return c.Edit(
			fmt.Sprintf("❌ *Test GAGAL*\n\n```\n%s\n```\n\nCek email & API key kamu.", escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown,
		)
	}

	return c.Edit(
		"✅ *Test BERHASIL*\n\nCredentials valid — Cloudflare API responding.",
		backToSettings(), tele.ModeMarkdown,
	)
}

// ─── Clear Credentials ────────────────────────────────────────────────────────

func (h *Handler) handleSettingsClearConfirm(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("✅ Ya, Hapus", cbSettingsClearYes),
			m.Data("❌ Batal", cbSettings),
		),
	)
	return c.Edit(
		"🗑 *Hapus Credentials?*\n\nIni akan menghapus CF Email & API Key dari `data/credentials.json`.\n\n"+
			"⚠️ Setelah dihapus, fitur CF Redirect & Auto Rotator gak bisa dipakai sampai di-set ulang.",
		m, tele.ModeMarkdown,
	)
}

func (h *Handler) handleSettingsClearDo(c tele.Context) error {
	h.creds.Clear()
	h.cf.SetCredentials("", "")
	return c.Edit(
		"✅ Credentials dihapus. Set ulang via menu Settings.",
		backToSettings(), tele.ModeMarkdown,
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// applyCredsToCFClient sinkronkan store ke CF client (hot-reload).
func (h *Handler) applyCredsToCFClient() {
	c := h.creds.Get()
	h.cf.SetCredentials(c.CFEmail, c.CFAPIKey)
}

// escapeMD escapes karakter Markdown supaya gak break parsing.
func escapeMD(s string) string {
	r := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return r.Replace(s)
}
