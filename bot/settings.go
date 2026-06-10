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

	text := fmt.Sprintf(
		"💎 *S E T T I N G S* 💎\n"+
			"|\n"+
			"🔌 *PLATFORM CONNECTED*\n"+
			"└ ⚙️ Cloudflare : %s\n"+
			"└ 🔗 Klikcepat  : %s\n"+
			"|\n"+
			"📐 *FUNGSI PLATFORM*\n"+
			"└ ⚙️ Cloudflare — auto-rotate CF redirect rule\n"+
			"└ 🔗 Klikcepat — auto-rotate biolink/shortlink\n"+
			"└ 💬 Group Cmd — slash commands buat member\n"+
			"|\n"+
			"🎯 Pilih platform yg mau di-configure 👇",
		cfStatus, klcStatus,
	)
	return c.Edit(text, settingsMenu(), tele.ModeMarkdown)
}

// handleSettingsCF — CF-specific settings (was the OLD handleSettings body).
func (h *Handler) handleSettingsCF(c tele.Context) error {
	creds := h.creds.Get()

	emailDisplay := creds.CFEmail
	if emailDisplay == "" {
		emailDisplay = "(belum di-set)"
	}
	keyDisplay := store.MaskAPIKey(creds.CFAPIKey)

	statusIcon := "❌"
	statusText := "Belum lengkap — set dulu email & API key di bawah"
	if h.cf.HasCredentials() {
		statusIcon = "✅"
		statusText = "Aktif & terhubung"
	}

	text := fmt.Sprintf(
		"💎 *C L O U D F L A R E* 💎\n"+
			"|\n"+
			"📋 *CREDENTIALS*\n"+
			"└ 📧 Email   : `%s`\n"+
			"└ 🔑 API Key : `%s`\n"+
			"└ %s Status  : %s\n"+
			"|\n"+
			"💡 *TIPS SETUP PERTAMA*\n"+
			"└ Klik 🔄 Set Keduanya\n"+
			"└ Wizard 2 langkah — email + API key\n"+
			"|\n"+
			"🔒 *KEAMANAN*\n"+
			"└ Disimpan di `data/credentials.json`\n"+
			"└ File permission 0600 (owner only)",
		emailDisplay, keyDisplay, statusIcon, statusText,
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
		"💎 *S E T   E M A I L* 💎\n"+
			"|\n"+
			"📧 *INPUT*\n"+
			"└ Ketik email akun Cloudflare\n"+
			"└ Contoh: `nama-kamu@gmail.com`\n"+
			"|\n"+
			"📍 *CARA CEK EMAIL*\n"+
			"└ 1. Buka cloudflare.com → login\n"+
			"└ 2. Klik foto profil kanan atas\n"+
			"└ 3. Email muncul di situ",
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
		"💎 *S E T   A P I   K E Y* 💎\n"+
			"|\n"+
			"📍 *CARA AMBIL GLOBAL API KEY*\n"+
			"└ 1. Buka cloudflare.com → login\n"+
			"└ 2. Foto profil kanan atas → My Profile\n"+
			"└ 3. Tab API Tokens\n"+
			"└ 4. Global API Key → klik View\n"+
			"└ 5. Input password CF → copy kode\n"+
			"|\n"+
			"📝 *INPUT*\n"+
			"└ Paste kode tersebut di chat ini\n"+
			"|\n"+
			"⚠️ *KEAMANAN*\n"+
			"└ Pesan auto-deleted setelah disimpan\n"+
			"└ Aman dari pengintaian di grup",
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
		"💎 *S E T U P   C F* 💎\n"+
			"|\n"+
			"📧 *STEP 1/2 — EMAIL*\n"+
			"└ Ketik email akun Cloudflare\n"+
			"└ Contoh: `nama-kamu@gmail.com`\n"+
			"|\n"+
			"📍 *CARA CEK EMAIL*\n"+
			"└ Buka cloudflare.com → foto profil kanan atas",
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
			"💎 *S E T U P   C F* 💎\n"+
				"|\n"+
				"✅ Email : `%s`\n"+
				"|\n"+
				"🔑 *STEP 2/2 — API KEY*\n"+
				"└ Ketik Global API Key Cloudflare\n"+
				"|\n"+
				"📍 *CARA AMBIL*\n"+
				"└ 1. cloudflare.com → foto profil → My Profile\n"+
				"└ 2. Tab API Tokens → Global API Key → View\n"+
				"└ 3. Input password → copy kode → paste sini\n"+
				"|\n"+
				"🔒 *KEAMANAN*\n"+
				"└ Pesan auto-deleted setelah disimpan",
			email),
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
