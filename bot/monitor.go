package bot

import (
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"bongbot/checker"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// checkDomainInCFRules cek apakah domain ini lagi dipakai sebagai current target
// di CF rule manapun. Return string info untuk ditampilkan ke user (atau empty kalau gak).
// Hasil format: "\n🔧 Note: ...\n" atau "" kalau bukan target di CF.
func (h *Handler) checkDomainInCFRules(domain string) string {
	if !h.cf.HasCredentials() {
		return "" // skip kalau credential belum di-set
	}
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return ""
	}

	var matches []store.CFRule
	for _, r := range rules {
		curURL, err := h.cf.GetCurrentURL(r)
		if err != nil {
			continue
		}
		if extractHostFromURL(curURL) == strings.ToLower(domain) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return ""
	}

	// Cek rotator config untuk match
	allRotators := h.rotators.GetAll()
	var sb strings.Builder
	sb.WriteString("\n🔧 *Note CF:* domain ini lagi dipakai sebagai _current target_ di:\n")
	for _, r := range matches {
		sb.WriteString(fmt.Sprintf("  • CF Rule *%s* (`%s`)\n", r.Label, r.Domain))
		// Cek rotator
		for _, rot := range allRotators {
			if rot.CFRuleID == r.ID {
				sb.WriteString(fmt.Sprintf("    └ Rotator: pool = *%s*\n", rot.PoolLabel))
			}
		}
	}
	sb.WriteString("✅ _CF tidak berubah_ — domain tetap dipakai. Cuma label di Monitor yang pindah.")
	return sb.String()
}

// extractHostFromURL ambil hostname dari URL (lowercase, tanpa www. & path).
func extractHostFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToLower(rawURL)
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func (h *Handler) handleMonitor(c tele.Context) error {
	return c.Edit(textMonitor, monitorMenu(), tele.ModeMarkdown)
}

// ─── Add Domain ───────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorAdd(c tele.Context) error {
	h.cancelPriorPrompt(c, StepMonitorAddDomain)
	prompt := "💎 *T A M B A H   D O M A I N* 💎\n" +
		"|\n" +
		"📝 *STEP 1/2 — INPUT DOMAIN*\n" +
		"└ Ketik nama domain yg mau dipantau\n" +
		"|\n" +
		"✅ *FORMAT YG BENAR*\n" +
		"└ `example.com`\n" +
		"└ `sub.example.com`\n" +
		"└ `https://example.com` (auto-clean)\n" +
		"|\n" +
		"🎯 *NEXT*\n" +
		"└ Step 2: pilih label/kategori"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorAddDomain,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorAddDomain(c tele.Context, sess *Session) error {
	h.showTyping(c) // feedback langsung biar user tau bot lagi proses
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return h.reply(c, "❌ Domain tidak valid, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	sess.Data["domain"] = domain
	sess.Step = StepMonitorAddLabel

	// Tampilkan existing labels sebagai pilihan + input manual
	labels := h.domains.Labels()
	prompt := fmt.Sprintf(
		"%s ✅ Domain: `%s`\n\n"+
			"📂 *Langkah 2 dari 2 — Pilih Label/Kategori*\n\n"+
			"Label = kelompok domain serupa. Kalau salah satu domain di label ini kena nawala, bot bakal swap ke domain lain di *label yang sama*.\n\n"+
			"💡 *Contoh label:* KWAI, MONEYSITE, STOCK-MS, PROMO, dll.\n\n"+
			"Ketik nama label atau klik tombol di bawah:",
		userTag(c.Sender()), domain)
	if len(labels) > 0 {
		prompt += "\n\n*Label yang sudah pernah dipakai:*"
	}

	// Buat inline buttons untuk label yang sudah ada
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	// Buat baris per 3 label
	var row tele.Row
	for i, lbl := range labels {
		row = append(row, m.Data(lbl, cbMonitorAdd, lbl))
		if len(row) == 3 || i == len(labels)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	// Send pesan baru → simpan sebagai PromptMsg baru biar step berikutnya bisa edit kalau perlu
	// Pakai ReplyTo biar di group chat kelihatan prompt ini reply ke pesan domain user.
	newMsg, _ := h.bot.Send(c.Chat(), prompt, &tele.SendOptions{
		ReplyTo:     c.Message(),
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: m,
	})
	if newMsg != nil {
		sess.PromptMsg = newMsg
	}
	h.sessions.Set(c.Sender().ID, sess)
	return nil
}

func (h *Handler) wizardMonitorAddLabel(c tele.Context, sess *Session) error {
	h.showTyping(c)
	rawLabel := strings.TrimSpace(c.Text())
	label := strings.ToUpper(rawLabel)
	if label == "" {
		return h.reply(c, "❌ Label tidak boleh kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}

	// 🛡️ Validasi: label seharusnya nama kategori, bukan API key/token panjang aneh
	if len(label) > 40 {
		h.sessions.Delete(c.Sender().ID)
		return h.reply(c, 
			"⚠️ *Label kelihatannya bukan kategori biasa* (>40 karakter).\n\n"+
				"Mungkin kamu salah paste API key / token di sini? Wizard dibatalkan demi keamanan.\n\n"+
				"Coba lagi dengan label pendek seperti `KWAI`, `MONEYSITE`, `PROMO`.",
			backToMonitor(), tele.ModeMarkdown)
	}
	// Cek karakter mencurigakan (API key biasanya alphanumeric panjang tanpa spasi)
	if isLikelyAPIKey(rawLabel) {
		h.sessions.Delete(c.Sender().ID)
		return h.reply(c, 
			"🛡️ *Label kelihatan seperti API key/token!*\n\n"+
				"Demi keamanan, wizard dibatalkan. Kalau memang mau pakai itu sebagai label, "+
				"gunakan format yg lebih pendek atau pisah dengan dash/underscore.",
			backToMonitor(), tele.ModeMarkdown)
	}

	return h.doAddDomain(c, sess, label)
}

// isLikelyAPIKey: heuristic deteksi API key. Long string alphanumeric tanpa pemisah.
func isLikelyAPIKey(s string) bool {
	if len(s) < 30 {
		return false
	}
	// Hitung karakter alphanumeric
	alnum := 0
	hasSeparator := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			alnum++
		} else if r == ' ' || r == '-' || r == '_' || r == '.' {
			hasSeparator = true
		}
	}
	// Kalau hampir semua alphanumeric (>= 95%) DAN minim separator → kemungkinan token
	if !hasSeparator && float64(alnum)/float64(len(s)) > 0.95 {
		return true
	}
	return false
}

func (h *Handler) handleMonitorAddLabelSelect(c tele.Context) error {
	label := extractParam(c)
	if label == "" {
		return h.handleMonitorAdd(c)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepMonitorAddLabel {
		return c.Edit(textMonitor, monitorMenu(), tele.ModeMarkdown)
	}
	return h.doAddDomain(c, sess, label)
}

func (h *Handler) doAddDomain(c tele.Context, sess *Session, label string) error {
	h.showTyping(c) // feedback langsung — handler ini bakal lakukan check nawala (network IO)
	h.sessions.Delete(c.Sender().ID)
	domain := sess.Data["domain"]

	// Defensive: validate domain BELUM tersimpan kalau session corrupt / stale.
	// Skenario: user klik stale inline button (cbMonitorAdd|LABEL) dari pesan
	// lama, session bukan StepMonitorAddLabel atau domain kosong → jangan save.
	if domain == "" {
		log.Printf("[ADD_DOMAIN] REJECT — sess.Data[domain] kosong user=%d label=%s step=%s",
			c.Sender().ID, label, sess.Step)
		return h.reply(c,
			"⚠️ *Session corrupt atau expired*\n\nWizard Add Domain tidak ditemukan untuk kamu. Mulai ulang via *📡 Monitor → ➕ Add Domain*.",
			backToMonitor(), tele.ModeMarkdown)
	}
	if sess.Step != StepMonitorAddLabel {
		log.Printf("[ADD_DOMAIN] REJECT — sess.Step bukan StepMonitorAddLabel user=%d label=%s step=%s domain=%s",
			c.Sender().ID, label, sess.Step, domain)
		return h.reply(c,
			"⚠️ *Action ini bukan untuk kamu*\n\nKamu lagi di wizard lain, atau tombol ini dari sesi lama. Mulai ulang via *🏠 MENU*.",
			mainMenu(), tele.ModeMarkdown)
	}
	if !looksLikeDomain(domain) {
		log.Printf("[ADD_DOMAIN] REJECT — domain invalid user=%d label=%s domain=%q",
			c.Sender().ID, label, domain)
		return h.reply(c,
			fmt.Sprintf("⚠️ *Domain tidak valid:* `%s`\n\nCoba ulang via *📡 Monitor → ➕ Add Domain*.", escapeMD(domain)),
			backToMonitor(), tele.ModeMarkdown)
	}
	log.Printf("[ADD_DOMAIN] save user=%d domain=%s label=%s", c.Sender().ID, domain, label)

	isMove, oldLabel := h.domains.Add(domain, label)

	// Cek apakah domain ini lagi dipakai sebagai current target di CF rule manapun
	// (biar user tau efek pindah label terhadap CF & rotator config)
	cfInfo := h.checkDomainInCFRules(domain)

	var loadingMsg string
	if isMove {
		loadingMsg = fmt.Sprintf(
			"✏️ *Domain dipindahkan!*\n"+
				"🌐 `%s`\n"+
				"📂 *%s* → *%s*\n"+
				"%s\n"+
				"⏳ Cek status nawala...",
			domain, oldLabel, label, cfInfo)
	} else {
		loadingMsg = fmt.Sprintf(
			"✅ *Domain ditambahkan!*\n"+
				"🌐 `%s`\n"+
				"📂 Kategori: *%s*\n"+
				"%s\n"+
				"⏳ Cek status nawala...",
			domain, label, cfInfo)
	}

	// Kalau dari callback (klik tombol label) → Edit pesan menu di tempat.
	// Kalau dari text (ngetik label) → Send pesan baru di bawah chat user.
	var targetMsg *tele.Message
	if c.Callback() != nil {
		h.bot.Edit(sess.PromptMsg, loadingMsg, tele.ModeMarkdown)
		targetMsg = sess.PromptMsg
	} else {
		m, _ := h.bot.Send(c.Chat(), loadingMsg, tele.ModeMarkdown)
		targetMsg = m
	}

	go func() {
		status := checker.CheckDomain(domain)
		var statusLine string
		switch status {
		case "BLOCKED":
			statusLine = "🛑 Status: *DIBLOKIR KOMINFO*"
		case "SAFE":
			statusLine = "🟢 Status: *AMAN*"
		default:
			statusLine = "⚠️ *Status: GAGAL CEK* — TrustPositif API tidak respon. Coba cek manual nanti via *🔍 Cek Domain*."
		}
		finalMsg := strings.Replace(loadingMsg, "\n\n⏳ Cek status nawala...", "\n"+statusLine, 1)
		if targetMsg != nil {
			h.bot.Edit(targetMsg, finalMsg, backToMonitor(), tele.ModeMarkdown)
		}
	}()
	return nil
}

// ─── Remove Domain ────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorRemove(c tele.Context) error {
	h.cancelPriorPrompt(c, StepMonitorRemove)
	prompt := "💎 *H A P U S   D O M A I N* 💎\n" +
		"|\n" +
		"🗑 *FUNGSI*\n" +
		"└ Hapus domain dari list Monitor\n" +
		"|\n" +
		"📝 *INPUT*\n" +
		"└ Ketik domain yg mau dihapus\n" +
		"└ Contoh: `example.com`\n" +
		"|\n" +
		"⚠️ *INFO PENTING*\n" +
		"└ Cuma hapus dari bot tracking\n" +
		"└ Gak ngaruh setting Cloudflare lo"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorRemove,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorRemove(c tele.Context, sess *Session) error {
	h.showTyping(c)
	h.sessions.Delete(c.Sender().ID)
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return h.reply(c, "❌ Domain tidak valid", backToMonitor(), tele.ModeMarkdown)
	}
	label, found := h.domains.Remove(domain)
	if !found {
		return h.reply(c, 
			fmt.Sprintf("⚠️ Domain `%s` tidak ditemukan di list", domain),
			backToMonitor(), tele.ModeMarkdown)
	}
	// Auto-cleanup sticky & force-block entries
	stickyCleared := checker.Default().RemoveSticky(domain)
	forceCleared := checker.Default().RemoveForceBlock(domain)

	msg := fmt.Sprintf("🗑 *Domain dihapus!*\n🌐 `%s`\n📂 Kategori: *%s*", domain, label)
	if stickyCleared {
		msg += "\n📌 _Sticky-block ke-clear._"
	}
	if forceCleared {
		msg += "\n🔨 _Force-block ke-clear._"
	}
	return h.reply(c, msg, backToMonitor(), tele.ModeMarkdown)
}

// ─── Check Domain ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorCheck(c tele.Context) error {
	h.cancelPriorPrompt(c, StepMonitorCheck)
	prompt := "💎 *C E K   D O M A I N* 💎\n" +
		"|\n" +
		"🔍 *FUNGSI*\n" +
		"└ Cek manual apakah domain kena nawala\n" +
		"└ Source: Kominfo / TrustPositif / Nawala\n" +
		"|\n" +
		"📝 *INPUT*\n" +
		"└ Ketik domain yg mau dicek\n" +
		"└ Contoh: `example.com`\n" +
		"|\n" +
		"💡 *NOTE*\n" +
		"└ Gak harus terdaftar di Monitor\n" +
		"└ Bisa cek domain apapun"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorCheck,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

// wizardMonitorCheck: user ketik domain → bot tampilin source picker (BUKAN langsung cek)
func (h *Handler) wizardMonitorCheck(c tele.Context, sess *Session) error {
	h.showTyping(c)
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return h.reply(c, "❌ Domain tidak valid", backToMonitor(), tele.ModeMarkdown)
	}

	// Simpan domain ke session, pindah ke step pilih source
	sess.Data["check_domain"] = domain
	sess.Step = StepMonitorCheckSrc
	h.sessions.Set(c.Sender().ID, sess)

	// Build picker buttons — conditional render berdasarkan API key
	hasTP := checker.HasTrustPositifKey()
	hasNW := checker.HasNawalaCheckKey()

	m := &tele.ReplyMarkup{}
	rows := []tele.Row{
		// KOMINFO selalu muncul (gak butuh API key)
		m.Row(m.Data("🏛️ KOMINFO", cbMonitorCheckKominfo)),
	}
	if hasTP {
		rows = append(rows, m.Row(m.Data("📋 TRUST POSITIF ID", cbMonitorCheckTP)))
	}
	if hasNW {
		rows = append(rows, m.Row(m.Data("🌐 NAWALA CHECKER", cbMonitorCheckNawala)))
	}
	rows = append(rows, m.Row(m.Data("❌ Cancel", cbCancel)))
	m.Inline(rows...)

	// Build info text — cuma tampilin source yg aktif
	var infoBuilder strings.Builder
	infoBuilder.WriteString(fmt.Sprintf(
		"🔍 *Pilih Sumber Pengecekan*\n\n"+
			"🌐 Domain: `%s`\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"*🏛️ KOMINFO*\n"+
			"📍 Source: `trustpositif.komdigi.go.id`\n"+
			"📊 Limit: *Unlimited* per hari\n"+
			"💡 Official database Kominfo, paling reliable",
		domain))

	if hasTP {
		infoBuilder.WriteString(
			"\n\n*📋 TRUST POSITIF ID*\n" +
				"📍 Source: `trustpositif.id`\n" +
				"📊 Limit: *100* check per hari\n" +
				"💡 Mirror third-party, butuh API key")
	}
	if hasNW {
		infoBuilder.WriteString(
			"\n\n*🌐 NAWALA CHECKER*\n" +
				"📍 Source: `nawalacheck.com`\n" +
				"📊 Limit: *10* check per hari (free tier)\n" +
				"💡 ISP-based detection (real-world ISP block)")
	}

	infoBuilder.WriteString("\n━━━━━━━━━━━━━━━━━━\n")

	// Footer hint — sesuai source yg aktif
	// Pakai plain text (no italic wrap) karena underscore di dalem code block sering bikin Telegram bingung parse markdown.
	if !hasTP && !hasNW {
		infoBuilder.WriteString(
			"💡 Tambah `TRUSTPOSITIF_API_KEY` atau `NAWALACHECK_API_KEY` di `.env` untuk dapet source tambahan.")
	} else {
		infoBuilder.WriteString("💡 Tip: Kominfo paling reliable & gratis unlimited.")
	}

	return h.reply(c, infoBuilder.String(), m, tele.ModeMarkdown)
}

// handleMonitorCheckPickSource: user klik tombol source pilihan → jalanin manual check.
func (h *Handler) handleMonitorCheckPickSource(c tele.Context, mode checker.SourceMode) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepMonitorCheckSrc {
		return c.Edit(textMonitor, monitorMenu(), tele.ModeMarkdown)
	}
	domain := sess.Data["check_domain"]
	if domain == "" {
		h.sessions.Delete(c.Sender().ID)
		return c.Edit("❌ Session error — coba lagi via menu Cek Domain", backToMonitor(), tele.ModeMarkdown)
	}
	h.sessions.Delete(c.Sender().ID)
	return h.runManualCheck(c, domain, mode)
}

// runManualCheck: actual eksekusi check dengan source yg dipilih.
// Dipanggil dari handleMonitorCheckPickSource.
func (h *Handler) runManualCheck(c tele.Context, domain string, mode checker.SourceMode) error {
	var sourceName, sourceSite string
	switch mode {
	case checker.SourceKominfo:
		sourceName = "🏛️ KOMINFO"
		sourceSite = "trustpositif.komdigi.go.id"
	case checker.SourceTrustPositif:
		sourceName = "📋 TRUST POSITIF ID"
		sourceSite = "trustpositif.id"
	case checker.SourceNawalaCheck:
		sourceName = "🌐 NAWALA CHECKER"
		sourceSite = "nawalacheck.com"
	default:
		sourceName = "Unknown"
		sourceSite = "?"
	}
	_ = sourceSite // dipakai di display

	loadingMsg, _ := h.bot.Edit(c.Message(),
		fmt.Sprintf("⏳ *Cek domain `%s`...*\n\nSumber: %s", domain, sourceName),
		tele.ModeMarkdown)

	go func() {
		status, blockedCount, total := checker.Default().CheckManual(domain, mode)
		label := h.domains.FindLabel(domain)
		inList := label != ""

		var msg string
		switch status {
		case "BLOCKED":
			// Sumber status: force-block / sticky / fresh check
			extraInfo := ""
			if checker.Default().IsForceBlocked(domain) {
				extraInfo = "\n🔨 *Source:* Force Block (manual override)"
			} else if sticky, t := checker.Default().IsSticky(domain); sticky {
				extraInfo = fmt.Sprintf("\n📌 *Sticky:* Sejak %s", t.Format("02 Jan 2006 15:04"))
			}
			kategoriInfo := ""
			if inList {
				kategoriInfo = fmt.Sprintf("\n📂 *Kategori:* `%s`", label)
			}

			// Saran kontekstual
			saran := "Gunakan domain baru"
			if blockedCount < total {
				saran = "Sebagian source confirm — recheck dulu sebelum ganti"
			}

			// Confidence indicator dinamis
			confidenceLine := ""
			if blockedCount == total && total > 0 {
				confidenceLine = fmt.Sprintf(" ✅ *(%d/%d sources confirm)*", blockedCount, total)
			} else if blockedCount > 0 {
				confidenceLine = fmt.Sprintf(" ⚠️ *(cuma %d/%d source confirm)*", blockedCount, total)
			}

			msg = fmt.Sprintf(
				"🛑 *DIBLOKIR KOMINFO*\n"+
					"🌐 Domain: `%s`\n\n"+
					"⚠️ *Status:* TERBLOKIR\n"+
					"🔍 *Source:* %s\n"+
					"🌐 *Via:* `%s`\n"+
					"📊 *Hasil:* %d/%d blocked%s%s%s\n"+
					"💡 *Saran:* %s",
				domain, sourceName, sourceSite, blockedCount, total, confidenceLine, extraInfo, kategoriInfo, saran)

		case "SAFE":
			kategoriInfo := ""
			if inList {
				kategoriInfo = fmt.Sprintf("\n📂 *Kategori:* `%s`", label)
			}
			msg = fmt.Sprintf(
				"🟢 *AMAN*\n"+
					"🌐 Domain: `%s`\n\n"+
					"✅ Tidak terdaftar dalam Daftar Blokir\n"+
					"🔍 *Source:* %s\n"+
					"🌐 *Via:* `%s`\n"+
					"📊 *Hasil:* 0/%d blocked%s",
				domain, sourceName, sourceSite, total, kategoriInfo)

		default:
			msg = fmt.Sprintf(
				"⚠️ *Gagal Cek Domain*\n"+
					"🌐 Domain: `%s`\n\n"+
					"❌ *Status:* ERROR\n"+
					"🔍 *Source:* %s\n"+
					"🌐 *Via:* `%s`\n"+
					"💡 *Saran:* Source gak respon (kemungkinan limit habis / network issue). "+
					"Coba pilih source lain di menu Cek Domain.",
				domain, sourceName, sourceSite)
		}
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, msg, backToMonitor(), tele.ModeMarkdown)
		} else {
			h.bot.Send(c.Chat(), msg, backToMonitor(), tele.ModeMarkdown)
		}
	}()
	return nil
}

// ─── List Domain (with pagination + per-label filter) ───────────────────────

const (
	DomainsPerPage    = 100 // max domain per halaman list
	CategoriesPerPage = 6   // max button kategori per halaman (3 rows × 2 cols)
)

// handleMonitorList: entry — tampilkan kategori picker
func (h *Handler) handleMonitorList(c tele.Context) error {
	return h.renderListMenu(c, 0)
}

// handleMonitorListMenuPage: navigasi page kategori picker
func (h *Handler) handleMonitorListMenuPage(c tele.Context) error {
	page, _ := strconv.Atoi(extractParam(c))
	return h.renderListMenu(c, page)
}

// renderListMenu: tampilin kategori picker dengan pagination
func (h *Handler) renderListMenu(c tele.Context, page int) error {
	all := h.domains.GetAll()
	if len(all) == 0 {
		return c.Edit(
			"📭 *Belum ada domain terdaftar*\n\n"+
				"Tambah domain pakai *➕ Add Domain* di menu Monitor.",
			backToMonitor(), tele.ModeMarkdown)
	}

	// Sort labels alphabetically
	labels := make([]string, 0, len(all))
	for l := range all {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	totalDomains := 0
	for _, l := range labels {
		totalDomains += len(all[l])
	}

	totalCategories := len(labels)
	totalPages := (totalCategories + CategoriesPerPage - 1) / CategoriesPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * CategoriesPerPage
	end := start + CategoriesPerPage
	if end > totalCategories {
		end = totalCategories
	}

	var sb strings.Builder
	sb.WriteString("💎 *D O M A I N   M O N I T O R* 💎\n")
	sb.WriteString("|\n")
	sb.WriteString("📊 *STATISTIK*\n")
	sb.WriteString(fmt.Sprintf("└ Total domain   : %d\n", totalDomains))
	sb.WriteString(fmt.Sprintf("└ Total kategori : %d\n", totalCategories))
	sb.WriteString("|\n")
	sb.WriteString("💡 *NAVIGASI*\n")
	sb.WriteString("└ Klik kategori → filter per label\n")
	sb.WriteString("└ Klik SEMUA DOMAIN → liat semua")

	// Build markup
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Row 1: SEMUA DOMAIN button
	rows = append(rows, m.Row(
		m.Data(fmt.Sprintf("📋 SEMUA DOMAIN (%d)", totalDomains), cbMonitorListAll, "0"),
	))

	// Rows 2-4: kategori buttons (2 cols × max 3 rows)
	pageLabels := labels[start:end]
	for i := 0; i < len(pageLabels); i += 2 {
		var row tele.Row
		for j := 0; j < 2 && i+j < len(pageLabels); j++ {
			lbl := pageLabels[i+j]
			count := len(all[lbl])
			btnText := fmt.Sprintf("🔍 %s (%d)", lbl, count)
			row = append(row, m.Data(btnText, cbMonitorListLabel, lbl+"|0"))
		}
		rows = append(rows, row)
	}

	// Row 5: pagination (kalau total > 1 page)
	if totalPages > 1 {
		var pagRow tele.Row
		if page > 0 {
			pagRow = append(pagRow, m.Data("‹", cbMonitorListMenuPage, strconv.Itoa(page-1)))
		}
		pagRow = append(pagRow, m.Data(fmt.Sprintf("📄 %d/%d", page+1, totalPages), cbNoop))
		if page < totalPages-1 {
			pagRow = append(pagRow, m.Data("›", cbMonitorListMenuPage, strconv.Itoa(page+1)))
		}
		rows = append(rows, pagRow)
	}

	// Row 6: back
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbMonitor)))
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// handleMonitorListAll: tampilin semua domain paginated
func (h *Handler) handleMonitorListAll(c tele.Context) error {
	page, _ := strconv.Atoi(extractParam(c))
	return h.renderDomainList(c, "", page)
}

// handleMonitorListLabel: tampilin domain per label paginated
// Param format: "label|page"
func (h *Handler) handleMonitorListLabel(c tele.Context) error {
	param := extractParam(c)
	parts := strings.SplitN(param, "|", 2)
	label := parts[0]
	page := 0
	if len(parts) > 1 {
		page, _ = strconv.Atoi(parts[1])
	}
	return h.renderDomainList(c, label, page)
}

// renderDomainList: tampilin list domain paginated.
// filterLabel="" → semua. filterLabel="MONEYSITE" → cuma kategori itu.
func (h *Handler) renderDomainList(c tele.Context, filterLabel string, page int) error {
	all := h.domains.GetAll()

	// Build flat list, sorted by label then domain
	type entry struct{ label, domain string }
	var items []entry
	labels := make([]string, 0, len(all))
	for l := range all {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	for _, l := range labels {
		if filterLabel != "" && l != filterLabel {
			continue
		}
		for _, d := range all[l] {
			items = append(items, entry{l, d})
		}
	}

	if len(items) == 0 {
		return c.Edit(
			fmt.Sprintf("📭 Kategori *[%s]* kosong.", filterLabel),
			backToListMenu(), tele.ModeMarkdown)
	}

	totalPages := (len(items) + DomainsPerPage - 1) / DomainsPerPage
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * DomainsPerPage
	end := start + DomainsPerPage
	if end > len(items) {
		end = len(items)
	}

	title := "📋 *Daftar Semua Domain*"
	if filterLabel != "" {
		title = fmt.Sprintf("📋 *Daftar Domain [%s]*", filterLabel)
	}

	var sb strings.Builder
	sb.WriteString(title)
	if totalPages > 1 {
		sb.WriteString(fmt.Sprintf(" — Hal %d/%d", page+1, totalPages))
	}
	sb.WriteString("\n")
	sb.WriteString("═══════════════════════════\n\n")

	for i := start; i < end; i++ {
		sb.WriteString(fmt.Sprintf("%d. `[%s]` %s\n", i+1, items[i].label, items[i].domain))
	}

	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n"))
	if filterLabel != "" {
		sb.WriteString(fmt.Sprintf("📊 *%d* domain di kategori *[%s]*", len(items), filterLabel))
	} else {
		sb.WriteString(fmt.Sprintf("📊 *Total:* %d domain", len(items)))
	}

	// Build markup
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Pagination (kalau total > 1 page)
	if totalPages > 1 {
		var pagRow tele.Row
		cbName := cbMonitorListAll
		if filterLabel != "" {
			cbName = cbMonitorListLabel
		}
		prevData := strconv.Itoa(page - 1)
		nextData := strconv.Itoa(page + 1)
		if filterLabel != "" {
			prevData = filterLabel + "|" + strconv.Itoa(page-1)
			nextData = filterLabel + "|" + strconv.Itoa(page+1)
		}
		if page > 0 {
			pagRow = append(pagRow, m.Data("‹", cbName, prevData))
		}
		pagRow = append(pagRow, m.Data(fmt.Sprintf("📄 %d/%d", page+1, totalPages), cbNoop))
		if page < totalPages-1 {
			pagRow = append(pagRow, m.Data("›", cbName, nextData))
		}
		rows = append(rows, pagRow)
	}

	// Back ke kategori picker
	rows = append(rows, m.Row(m.Data("🔙 Pilih Kategori", cbMonitorList)))
	m.Inline(rows...)

	text := sb.String()
	// Telegram limit 4096 chars
	if len(text) > 3800 {
		text = text[:3800] + "\n\n_(dipotong — coba per kategori)_"
	}
	return c.Edit(text, m, tele.ModeMarkdown)
}

// backToListMenu: helper untuk balik ke kategori picker
func backToListMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Pilih Kategori", cbMonitorList)))
	return m
}

// ─── Status Blocked ───────────────────────────────────────────────────────────

func (h *Handler) handleMonitorStatus(c tele.Context) error {
	blocked := h.monScanner.GetBlockedSnapshot()
	totalDomains := h.domains.TotalCount()
	chunkNum, chunkOf, _, chunkSize := h.monScanner.GetChunkInfo()
	interval := h.monScanner.GetInterval()

	// Build scanner info header
	var scanInfo strings.Builder
	scanInfo.WriteString("💎 *S T A T U S   B L O C K E D* 💎\n")
	scanInfo.WriteString("|\n")
	scanInfo.WriteString("🔍 *SCANNER INFO*\n")
	scanInfo.WriteString(fmt.Sprintf("└ Domain di Monitor : %d\n", totalDomains))
	scanInfo.WriteString(fmt.Sprintf("└ Interval tick     : %v\n", interval))
	if chunkOf > 1 {
		fullCycle := time.Duration(chunkOf) * interval
		scanInfo.WriteString(fmt.Sprintf("└ Mode              : 🔄 Rotating Batch\n"))
		scanInfo.WriteString(fmt.Sprintf("└ Chunk             : %d/%d (%d domain/chunk)\n", chunkNum, chunkOf, chunkSize))
		scanInfo.WriteString(fmt.Sprintf("└ Siklus penuh      : %.1f menit\n", fullCycle.Minutes()))
	} else {
		scanInfo.WriteString("└ Mode              : 🟢 Full Scan\n")
	}
	scanInfo.WriteString("|\n")

	if len(blocked) == 0 {
		return c.Edit(
			scanInfo.String()+
				"✅ *SEMUA AMAN*\n"+
				"└ Gak ada domain blocked saat ini\n"+
				"|\n"+
				"🔄 Bot auto-update list kalau ada yg keblock",
			backToMonitor(), tele.ModeMarkdown)
	}
	var sb strings.Builder
	sb.WriteString(scanInfo.String())
	sb.WriteString(fmt.Sprintf("🚨 *DOMAIN TERBLOKIR* (%d)\n", len(blocked)))
	for domain, since := range blocked {
		sb.WriteString(fmt.Sprintf("└ 🔴 `%s`\n", domain))
		sb.WriteString(fmt.Sprintf("   └ Terdeteksi : %s\n", since.Format("02/01 15:04")))
	}
	sb.WriteString("|\n")
	sb.WriteString("💡 *INFO*\n")
	sb.WriteString("└ Kalau Auto Rotator aktif, domain ini\n")
	sb.WriteString("   bakal auto-swap ke pool yg sama")
	return c.Edit(sb.String(), backToMonitor(), tele.ModeMarkdown)
}

// ─── Set Interval ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorInterval(c tele.Context) error {
	h.cancelPriorPrompt(c, StepMonitorInterval)
	current := h.rotSvc.GetInterval()
	domainCount := h.domains.TotalCount()

	// Rotating batch math: chunk = 100 per tick, kalau total > 100 auto-split
	const chunkSize = 100
	totalChunks := 1
	if domainCount > chunkSize {
		totalChunks = (domainCount + chunkSize - 1) / chunkSize
	}

	// Estimasi 1 chunk: 100 domain × 1.2s / 3 worker ≈ 40 detik (aman dalam 45s tick)
	chunkLen := domainCount
	if chunkLen > chunkSize {
		chunkLen = chunkSize
	}
	estChunkSec := float64(chunkLen) * 1.2 / 3.0
	estChunkStr := fmt.Sprintf("%.0f detik", estChunkSec)

	// Full cycle time = totalChunks × interval
	fullCycle := time.Duration(totalChunks) * current
	fullCycleStr := fullCycle.String()
	if fullCycle >= time.Minute {
		fullCycleStr = fmt.Sprintf("%.1f menit", fullCycle.Minutes())
	}

	modeText := "🟢 Full Scan — semua domain tiap tick"
	if totalChunks > 1 {
		modeText = fmt.Sprintf("🔄 Rotating Batch (%d chunk × 100 domain)", totalChunks)
	}

	prompt := fmt.Sprintf(
		"💎 *S E T   I N T E R V A L* 💎\n"+
			"|\n"+
			"📊 *STATS BOT*\n"+
			"└ Domain di Monitor : %d\n"+
			"└ Interval saat ini : %v\n"+
			"└ Chunk per tick    : max %d (~%s)\n"+
			"└ Total chunk       : %d\n"+
			"└ Siklus penuh      : %s\n"+
			"└ Mode              : %s\n"+
			"|\n"+
			"📐 *CARA KERJA*\n"+
			"└ Bagi domain jadi chunk 100 per tick\n"+
			"└ Zero rate-limit risk walau 1000+ domain\n"+
			"└ Domain BLOCKED auto-skip (sticky cache)\n"+
			"|\n"+
			"⏱ *RE-CHECK TIME (interval 45s)*\n"+
			"└ 100 domain  : tiap 45 detik\n"+
			"└ 200 domain  : tiap 1.5 menit\n"+
			"└ 500 domain  : tiap 3.75 menit\n"+
			"└ 1000 domain : tiap 7.5 menit\n"+
			"|\n"+
			"📝 *FORMAT INPUT*\n"+
			"└ `30s` → 30 detik (min 10s)\n"+
			"└ `1m` → 1 menit\n"+
			"└ `2m30s` → 2 menit 30 detik\n"+
			"|\n"+
			"💡 *REKOMENDASI*\n"+
			"└ `45s` (default) — optimal semua skala\n"+
			"|\n"+
			"🎯 Ketik interval baru 👇",
		domainCount, current, chunkSize, estChunkStr, totalChunks, fullCycleStr, modeText,
	)
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorInterval,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorInterval(c tele.Context, sess *Session) error {
	d, err := time.ParseDuration(strings.TrimSpace(c.Text()))
	if err != nil || d < 10*time.Second {
		return h.reply(c, 
			"❌ Interval tidak valid. Minimal 10s\n_(contoh: 30s, 1m, 2m30s)_",
			cancelMenu(), tele.ModeMarkdown)
	}
	h.sessions.Delete(c.Sender().ID)
	h.rotSvc.SetInterval(d)
	h.monScanner.SetInterval(d)
	return h.reply(c, 
		fmt.Sprintf("✅ Interval cek diubah ke *%v*\n\n_Auto Rotator & Monitor Scanner sync sama interval ini._", d),
		backToMonitor(), tele.ModeMarkdown)
}
