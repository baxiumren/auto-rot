package bot

import (
	"fmt"
	"net/url"
	"sort"
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
	prompt := "📝 *Tambah Domain ke Monitor*\n\n" +
		"_Langkah 1 dari 2_\n\n" +
		"Ketik nama domain yang mau dipantau:\n\n" +
		"*Contoh format yang benar:*\n" +
		"• `example.com`\n" +
		"• `sub.example.com`\n" +
		"• `https://example.com` (bot otomatis bersihkan)\n\n" +
		"_Setelah ini kamu pilih *label/kategori* (pengelompokan)._"
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
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return c.Send("❌ Domain tidak valid, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	sess.Data["domain"] = domain
	sess.Step = StepMonitorAddLabel

	// Tampilkan existing labels sebagai pilihan + input manual
	labels := h.domains.Labels()
	prompt := fmt.Sprintf(
		"✅ Domain: `%s`\n\n"+
			"📂 *Langkah 2 dari 2 — Pilih Label/Kategori*\n\n"+
			"Label = kelompok domain serupa. Kalau salah satu domain di label ini kena nawala, bot bakal swap ke domain lain di *label yang sama*.\n\n"+
			"💡 *Contoh label:* KWAI, MONEYSITE, STOCK-MS, PROMO, dll.\n\n"+
			"Ketik nama label atau klik tombol di bawah:",
		domain)
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
	newMsg, _ := h.bot.Send(c.Chat(), prompt, m, tele.ModeMarkdown)
	if newMsg != nil {
		sess.PromptMsg = newMsg
	}
	h.sessions.Set(c.Sender().ID, sess)
	return nil
}

func (h *Handler) wizardMonitorAddLabel(c tele.Context, sess *Session) error {
	label := strings.ToUpper(strings.TrimSpace(c.Text()))
	if label == "" {
		return c.Send("❌ Label tidak boleh kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	return h.doAddDomain(c, sess, label)
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
	h.sessions.Delete(c.Sender().ID)
	domain := sess.Data["domain"]

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
	prompt := "🗑 *Hapus Domain dari Monitor*\n\n" +
		"Ketik nama domain yang mau dihapus dari list pemantauan:\n\n" +
		"_Contoh:_ `example.com`\n\n" +
		"⚠️ Domain cuma dihapus dari bot — *gak mempengaruhi setting Cloudflare* kamu."
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
	h.sessions.Delete(c.Sender().ID)
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return c.Send("❌ Domain tidak valid", backToMonitor(), tele.ModeMarkdown)
	}
	label, found := h.domains.Remove(domain)
	if !found {
		return c.Send(
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
	return c.Send(msg, backToMonitor(), tele.ModeMarkdown)
}

// ─── Check Domain ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorCheck(c tele.Context) error {
	prompt := "🔍 *Cek Status Domain Manual*\n\n" +
		"Bot akan cek apakah domain ini kena nawala (terblokir Kominfo) atau aman.\n\n" +
		"Ketik domain yang mau dicek:\n\n" +
		"_Contoh:_ `example.com`\n\n" +
		"_(Domain gak harus terdaftar di Monitor — kamu bisa cek domain apapun.)_"
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

func (h *Handler) wizardMonitorCheck(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return c.Send("❌ Domain tidak valid", backToMonitor(), tele.ModeMarkdown)
	}
	loadingMsg, _ := h.bot.Send(c.Chat(),
		fmt.Sprintf("⏳ *Cek domain `%s`...*\n\n_Bot lagi cek 3 ronde ke TrustPositif untuk akurasi maksimal._", domain),
		tele.ModeMarkdown)

	go func() {
		// 3-ronde manual check untuk akurasi lebih tinggi
		status, blockedCount, total := checker.CheckDomainManual(domain)
		label := h.domains.FindLabel(domain)
		inList := label != ""

		var msg string
		switch status {
		case "BLOCKED":
			// Sumber status: force-block / sticky / fresh API check
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
				saran = "Sebagian ronde blocked — recheck dulu sebelum ganti"
			}

			msg = fmt.Sprintf(
				"🛑 *DIBLOKIR KOMINFO*\n"+
					"🌐 Domain: `%s`\n\n"+
					"⚠️ *Status:* TERBLOKIR\n"+
					"🔍 *API Check:* %d/%d blocked%s%s\n"+
					"💡 *Saran:* %s",
				domain, blockedCount, total, extraInfo, kategoriInfo, saran)

		case "SAFE":
			kategoriInfo := ""
			if inList {
				kategoriInfo = fmt.Sprintf("\n📂 *Kategori:* `%s`", label)
			}
			msg = fmt.Sprintf(
				"🟢 *AMAN*\n"+
					"🌐 Domain: `%s`\n\n"+
					"✅ Tidak terdaftar dalam Daftar Blokir KOMINFO\n"+
					"🔍 *API Check:* 0/%d blocked%s",
				domain, total, kategoriInfo)

		default:
			msg = fmt.Sprintf(
				"⚠️ *Gagal Cek Domain*\n"+
					"🌐 `%s`\n\n"+
					"❌ *Status:* ERROR\n"+
					"💡 *Saran:* TrustPositif gak respon. Coba lagi 1-2 menit lagi.",
				domain)
		}
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, msg, backToMonitor(), tele.ModeMarkdown)
		} else {
			h.bot.Send(c.Chat(), msg, backToMonitor(), tele.ModeMarkdown)
		}
	}()
	return nil
}

// ─── List Domain ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorList(c tele.Context) error {
	all := h.domains.GetAll()
	if len(all) == 0 {
		return c.Edit(
			"📭 *Belum ada domain terdaftar*\n\n"+
				"Tambah domain pakai *➕ Add Domain* di menu Monitor.\n\n"+
				"_Bot butuh minimal 2 domain di satu label biar Auto Rotator ada pilihan saat swap._",
			backToMonitor(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Domain List per Kategori*\n═══════════════════════════\n\n")

	labels := make([]string, 0, len(all))
	for l := range all {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	total := 0
	for _, label := range labels {
		domains := all[label]
		sb.WriteString(fmt.Sprintf("📂 *[%s]* — %d domain\n", label, len(domains)))
		for _, d := range domains {
			sb.WriteString(fmt.Sprintf("  • `%s`\n", d))
		}
		sb.WriteString("\n")
		total += len(domains)
	}
	sb.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━\n📊 *Total:* %d domain dalam %d kategori\n\n", total, len(labels)))
	sb.WriteString("💡 _Label ini bisa dipakai sebagai *pool* di Auto Rotator._")

	text := sb.String()
	if len(text) > 3800 {
		text = text[:3800] + "\n\n_(dipotong — terlalu panjang)_"
	}
	return c.Edit(text, backToMonitor(), tele.ModeMarkdown)
}

// ─── Status Blocked ───────────────────────────────────────────────────────────

func (h *Handler) handleMonitorStatus(c tele.Context) error {
	blocked := h.monScanner.GetBlockedSnapshot()
	if len(blocked) == 0 {
		return c.Edit(
			"✅ *Semua aman!*\n\n"+
				"Saat ini gak ada domain yang terdeteksi kena nawala.\n\n"+
				"_Bot bakal otomatis update list ini kalau ada domain yang kena blokir._",
			backToMonitor(), tele.ModeMarkdown)
	}
	var sb strings.Builder
	sb.WriteString("🚨 *Domain yang Sedang Terblokir Kominfo*\n═══════════════════════════\n\n")
	for domain, since := range blocked {
		sb.WriteString(fmt.Sprintf("🔴 `%s`\n   📅 Terdeteksi sejak: %s\n\n", domain, since.Format("02/01 15:04")))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━\n💡 _Kalau Auto Rotator-mu udah aktif, domain ini bakal otomatis di-swap dengan domain lain di pool yang sama._")
	return c.Edit(sb.String(), backToMonitor(), tele.ModeMarkdown)
}

// ─── Set Interval ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorInterval(c tele.Context) error {
	current := h.rotSvc.GetInterval()
	prompt := fmt.Sprintf(
		"⏱ *Set Interval Cek Otomatis*\n\n"+
			"Interval = seberapa sering bot cek domain ke TrustPositif (situs nawala Kominfo).\n\n"+
			"⏲ *Interval saat ini:* `%v`\n\n"+
			"Ketik interval baru — *minimal 10 detik*:\n\n"+
			"*Format yang diterima:*\n"+
			"• `30s` → 30 detik\n"+
			"• `1m` → 1 menit\n"+
			"• `2m30s` → 2 menit 30 detik\n"+
			"• `5m` → 5 menit\n\n"+
			"💡 *Rekomendasi:* `45s` cukup gesit, gak bikin CF rate limit.",
		current,
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
		return c.Send(
			"❌ Interval tidak valid. Minimal 10s\n_(contoh: 30s, 1m, 2m30s)_",
			cancelMenu(), tele.ModeMarkdown)
	}
	h.sessions.Delete(c.Sender().ID)
	h.rotSvc.SetInterval(d)
	h.monScanner.SetInterval(d)
	return c.Send(
		fmt.Sprintf("✅ Interval cek diubah ke *%v*\n\n_Auto Rotator & Monitor Scanner sync sama interval ini._", d),
		backToMonitor(), tele.ModeMarkdown)
}
