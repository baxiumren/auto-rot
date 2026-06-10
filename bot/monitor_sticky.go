package bot

import (
	"fmt"
	"sort"
	"strings"

	"bongbot/checker"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Sticky Block List ────────────────────────────────────────────────────────
//
// Sticky-blocked = domain yang udah pernah ke-detect blocked oleh TrustPositif.
// Bot skip API call untuk domain ini → balikin BLOCKED langsung.
// Persisted ke data/sticky_blocked.json.

func (h *Handler) handleMonitorSticky(c tele.Context) error {
	sticky := checker.Default().GetStickyList()
	validDomains := h.validMonitorDomains()
	stickyOrphan, _ := checker.Default().CountOrphans(validDomains)

	if len(sticky) == 0 {
		return c.Edit(
			"💎 *S T I C K Y   B L O C K E D* 💎\n"+
				"|\n"+
				"📌 *STATUS*\n"+
				"└ ✅ List kosong (semua domain SAFE)\n"+
				"|\n"+
				"💡 *FUNGSI STICKY*\n"+
				"└ Cache domain BLOCKED\n"+
				"└ Bot skip API call (hemat request)\n"+
				"└ Rotasi swap lebih cepat\n"+
				"|\n"+
				"🔄 *AUTO-CLEAR*\n"+
				"└ Pas lo hapus domain dari Monitor\n"+
				"└ Manual via tombol 🔓 Unblock",
			backToMonitor(), tele.ModeMarkdown)
	}

	// Sort + tandai orphan
	type entry struct {
		domain   string
		ts       string
		isOrphan bool
	}
	var rows []entry
	for d, t := range sticky {
		rows = append(rows, entry{
			domain:   d,
			ts:       t.Format("02/01 15:04"),
			isOrphan: !validDomains[d],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].domain < rows[j].domain })

	var sb strings.Builder
	sb.WriteString("💎 *S T I C K Y   B L O C K E D* 💎\n")
	sb.WriteString("|\n")
	sb.WriteString("📊 *STATISTIK*\n")
	sb.WriteString(fmt.Sprintf("└ Total domain : %d\n", len(rows)))
	if stickyOrphan > 0 {
		sb.WriteString(fmt.Sprintf("└ ⚠️ Orphan     : %d (gak ada di Monitor)\n", stickyOrphan))
	}
	sb.WriteString("|\n")
	sb.WriteString("🔴 *DOMAIN BLOCKED*\n")

	m := &tele.ReplyMarkup{}
	var btns []tele.Row
	for _, e := range rows {
		marker := ""
		if e.isOrphan {
			marker = " ⚠️"
		}
		sb.WriteString(fmt.Sprintf("└ `%s`%s\n", e.domain, marker))
		sb.WriteString(fmt.Sprintf("   └ Sejak : %s\n", e.ts))
		btns = append(btns, m.Row(m.Data("🔓 Unblock "+e.domain, cbMonitorStickyDel, e.domain)))
	}
	sb.WriteString("|\n")
	sb.WriteString("💡 *INFO*\n")
	sb.WriteString("└ Cache BLOCKED — bot skip API call\n")
	sb.WriteString("└ Klik 🔓 Unblock untuk paksa cek ulang")
	if stickyOrphan > 0 {
		sb.WriteString("\n|\n⚠️ *ORPHAN INFO*\n")
		sb.WriteString("└ Domain di sticky tapi gak ada di Monitor\n")
		sb.WriteString("└ Biasanya dari Cek Domain Manual\n")
		sb.WriteString("└ Bisa dibersihin sekaligus 👇")
		btns = append(btns, m.Row(m.Data(fmt.Sprintf("🧹 Bersihkan %d Orphan", stickyOrphan), cbMonitorStickyClean)))
	}
	btns = append(btns, m.Row(m.Data("🔙 Kembali", cbMonitor)))
	m.Inline(btns...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

func (h *Handler) handleMonitorStickyClean(c tele.Context) error {
	valid := h.validMonitorDomains()
	stickyCleared, _ := checker.Default().CleanOrphans(valid)
	if stickyCleared == 0 {
		c.Respond(&tele.CallbackResponse{Text: "Gak ada orphan untuk dibersihkan"})
	} else {
		c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("✅ %d orphan sticky dibersihkan", stickyCleared),
			ShowAlert: true,
		})
	}
	return h.handleMonitorSticky(c)
}

// validMonitorDomains return set domain (lowercase) yang ada di Monitor list.
func (h *Handler) validMonitorDomains() map[string]bool {
	all := h.domains.GetAll()
	valid := make(map[string]bool)
	for _, doms := range all {
		for _, d := range doms {
			valid[d] = true
		}
	}
	return valid
}

func (h *Handler) handleMonitorStickyDel(c tele.Context) error {
	domain := extractParam(c)
	if domain == "" {
		return h.handleMonitorSticky(c)
	}
	if checker.Default().RemoveSticky(domain) {
		c.Respond(&tele.CallbackResponse{Text: "✅ " + domain + " di-unblock"})
	} else {
		c.Respond(&tele.CallbackResponse{Text: "⚠️ Tidak ditemukan"})
	}
	// Refresh list
	return h.handleMonitorSticky(c)
}

// ─── Force Block ─────────────────────────────────────────────────────────────
//
// Force-block = manual override. Tandai domain sebagai BLOCKED *tanpa* hit API.
// Berguna buat:
// • Testing rotasi tanpa nunggu domain beneran kena nawala
// • Emergency block kalau kamu udah tau domain bermasalah

func (h *Handler) handleMonitorForce(c tele.Context) error {
	force := checker.Default().GetForceList()
	validDomains := h.validMonitorDomains()
	_, forceOrphan := checker.Default().CountOrphans(validDomains)

	var sb strings.Builder
	sb.WriteString("🔨 *Force Block — Manual Override*\n")
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString("_Force-block = paksa tandai domain sebagai BLOCKED tanpa cek TrustPositif._\n\n")
	sb.WriteString("*Berguna untuk:*\n")
	sb.WriteString("• 🧪 *Testing rotasi* — paksa rotasi tanpa nunggu domain beneran kena nawala\n")
	sb.WriteString("• ⚡ *Emergency* — kamu udah tahu domain bermasalah, gak perlu nunggu konfirmasi API\n\n")

	if len(force) == 0 {
		sb.WriteString("_Belum ada domain di-force block._\n")
	} else {
		sb.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━\n*Force-blocked saat ini (%d):*\n", len(force)))
		if forceOrphan > 0 {
			sb.WriteString(fmt.Sprintf("⚠️ _Orphan: %d (gak ada di Monitor list)_\n", forceOrphan))
		}
		// Sort
		var sortedDomains []string
		for d := range force {
			sortedDomains = append(sortedDomains, d)
		}
		sort.Strings(sortedDomains)
		for _, d := range sortedDomains {
			label := force[d]
			extra := ""
			if label != "" && label != "manual" {
				extra = " — " + label
			}
			marker := ""
			if !validDomains[d] {
				marker = " ⚠️"
			}
			sb.WriteString(fmt.Sprintf("🔨 `%s`%s%s\n", d, marker, extra))
		}
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	rows = append(rows, m.Row(m.Data("➕ Tambah Force Block", cbMonitorForceAdd)))

	if len(force) > 0 {
		var sortedDomains []string
		for d := range force {
			sortedDomains = append(sortedDomains, d)
		}
		sort.Strings(sortedDomains)
		for _, d := range sortedDomains {
			rows = append(rows, m.Row(m.Data("🗑 Hapus "+d, cbMonitorForceDel, d)))
		}
	}
	if forceOrphan > 0 {
		rows = append(rows, m.Row(m.Data(fmt.Sprintf("🧹 Bersihkan %d Orphan", forceOrphan), cbMonitorForceClean)))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbMonitor)))
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

func (h *Handler) handleMonitorForceClean(c tele.Context) error {
	valid := h.validMonitorDomains()
	_, forceCleared := checker.Default().CleanOrphans(valid)
	if forceCleared == 0 {
		c.Respond(&tele.CallbackResponse{Text: "Gak ada orphan untuk dibersihkan"})
	} else {
		c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("✅ %d orphan force dibersihkan", forceCleared),
			ShowAlert: true,
		})
	}
	return h.handleMonitorForce(c)
}

func (h *Handler) handleMonitorForceAdd(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepMonitorForceAdd,
		Data: map[string]string{},
	})
	return c.Edit(
		"🔨 *Tambah Force Block*\n\n"+
			"Ketik domain yang mau di-force block:\n\n"+
			"*Contoh:*\n"+
			"• `example.com`\n"+
			"• `mysite.net`\n\n"+
			"⚠️ Domain akan ditandai BLOCKED **tanpa** cek API. Berguna buat testing rotasi.",
		cancelMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) wizardMonitorForceAdd(c tele.Context, sess *Session) error {
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		return c.Send("❌ Domain tidak valid, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	h.sessions.Delete(c.Sender().ID)

	checker.Default().AddForceBlock(domain, "manual")

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("🔨 Buka Force Block", cbMonitorForce)),
		m.Row(m.Data("🔙 Monitor", cbMonitor)),
	)
	return c.Send(
		fmt.Sprintf("🔨 *Force-block aktif untuk `%s`*\n\n"+
			"Mulai sekarang, bot bakal balikin status BLOCKED untuk domain ini *tanpa cek API*.\n\n"+
			"_Auto Rotator akan langsung rotasi domain ini di cek berikutnya._\n\n"+
			"Hapus force-block kapan aja via menu *🔨 Force Block*.", domain),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleMonitorForceDel(c tele.Context) error {
	domain := extractParam(c)
	if domain == "" {
		return h.handleMonitorForce(c)
	}
	if checker.Default().RemoveForceBlock(domain) {
		c.Respond(&tele.CallbackResponse{Text: "✅ " + domain + " unforce"})
	} else {
		c.Respond(&tele.CallbackResponse{Text: "⚠️ Tidak ditemukan"})
	}
	return h.handleMonitorForce(c)
}
