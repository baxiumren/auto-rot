package bot

import (
	"fmt"
	"strings"

	"bongbot/checker"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Global Domain Search ────────────────────────────────────────────────────
//
// User klik 🔍 CARI di reply keyboard → bot prompt "ketik domain"
// → user ketik → bot cari di:
//   • Monitor list (label apa)
//   • CF Rule (sebagai current target di URL apa)
//   • Rotator config (jadi pool apa)
//   • Sticky blocked
//   • Force blocked

func (h *Handler) handleSearchPrompt(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepGlobalSearch,
		Data: map[string]string{},
	})
	return h.reply(c, 
		"🔍 *Pencarian Global Domain*\n\n"+
			"Ketik nama domain yang mau dicari:\n\n"+
			"*Contoh:* `example.com` atau `tokoku.id`\n\n"+
			"_Bot akan cek di Monitor, CF Rule, Rotator, Sticky, & Force list — semua tempat sekaligus._",
		cancelMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) wizardGlobalSearch(c tele.Context, sess *Session) error {
	query := store.CleanDomain(c.Text())
	h.sessions.Delete(c.Sender().ID)
	if query == "" {
		return h.reply(c, "❌ Domain gak valid. Coba lagi via 🔍 CARI.", backToMain(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 *Hasil Pencarian: `%s`*\n", escapeMD(query)))
	sb.WriteString("═══════════════════════════\n\n")

	found := false

	// 1. Cek di Monitor list
	monLabel := h.domains.FindLabel(query)
	if monLabel != "" {
		found = true
		sb.WriteString("📡 *Monitor:*\n")
		sb.WriteString(fmt.Sprintf("✅ Terdaftar di label *%s*\n\n", monLabel))
	}

	// 2. Cek apakah jadi current target di CF rule
	cfRules := h.cfrules.GetAll()
	var matchedCFTargets []store.CFRule
	if h.cf.HasCredentials() {
		for _, r := range cfRules {
			curURL, err := h.cf.GetCurrentURL(r)
			if err != nil {
				continue
			}
			if extractHostFromURL(curURL) == strings.ToLower(query) {
				matchedCFTargets = append(matchedCFTargets, r)
			}
		}
	}
	if len(matchedCFTargets) > 0 {
		found = true
		sb.WriteString("🎯 *CF Rule (sebagai current target redirect):*\n")
		for _, r := range matchedCFTargets {
			cur, _ := h.cf.GetCurrentURL(r)
			dom := r.Domain
			if dom == "" {
				dom = "?"
			}
			sb.WriteString(fmt.Sprintf("✅ Rule *%s* (`%s`)\n   🔗 `%s`\n",
				escapeMD(r.Label), escapeMD(dom), escapeMD(cur)))
		}
		sb.WriteString("\n")
	}

	// 3. Cek apakah ini DOMAIN sumber CF rule (rule.Domain match)
	var matchedCFDomain []store.CFRule
	for _, r := range cfRules {
		if strings.EqualFold(r.Domain, query) {
			matchedCFDomain = append(matchedCFDomain, r)
		}
	}
	if len(matchedCFDomain) > 0 {
		found = true
		sb.WriteString("⚙️ *CF Rule (sebagai domain sumber CF):*\n")
		for _, r := range matchedCFDomain {
			sb.WriteString(fmt.Sprintf("✅ Rule *%s* — Zone: `%s`\n", escapeMD(r.Label), r.ZoneID))
		}
		sb.WriteString("\n")
	}

	// 4. Cek apakah ini PoolLabel di rotator (kalau query == nama label)
	// Skip — query udah di-CleanDomain jadi gak masuk akal sbg label.
	// Tapi cek apakah dia ada di pool manapun (dari domain store).
	allRotators := h.rotators.GetAll()
	var inPools []string
	if monLabel != "" {
		for _, rot := range allRotators {
			if rot.PoolLabel == monLabel {
				inPools = append(inPools, rot.Label)
			}
		}
	}
	if len(inPools) > 0 {
		found = true
		sb.WriteString("🔄 *Auto Rotator (dipakai di pool):*\n")
		for _, rl := range inPools {
			sb.WriteString(fmt.Sprintf("✅ Rotator *%s* (pool: *%s*)\n", escapeMD(rl), monLabel))
		}
		sb.WriteString("\n")
	}

	// 5. Cek sticky blocked
	if blocked, ts := checker.Default().IsSticky(query); blocked {
		found = true
		sb.WriteString("📌 *Sticky Block:*\n")
		sb.WriteString(fmt.Sprintf("🔴 Aktif sejak %s\n\n", ts.Format("02/01 15:04")))
	}

	// 6. Cek force blocked
	if checker.Default().IsForceBlocked(query) {
		found = true
		sb.WriteString("🔨 *Force Block:*\n")
		sb.WriteString("🔴 Aktif (manual)\n\n")
	}

	// 7. Cek di monitor scanner blocked list (sedang dipantau spam)
	scannerBlocked := h.monScanner.GetBlockedSnapshot()
	if ts, ok := scannerBlocked[query]; ok {
		found = true
		sb.WriteString("🚨 *Monitor Scanner:*\n")
		sb.WriteString(fmt.Sprintf("🔴 Sedang dipantau (blocked sejak %s)\n\n", ts.Format("02/01 15:04")))
	}

	if !found {
		sb.WriteString(fmt.Sprintf("❌ *`%s` tidak ditemukan di mana-mana.*\n\n", escapeMD(query)))
		sb.WriteString("Tempat yang di-cek:\n")
		sb.WriteString("• 📡 Monitor list (semua label)\n")
		sb.WriteString("• ⚙️ CF Rule (sebagai source domain atau current target)\n")
		sb.WriteString("• 🔄 Rotator config (pool)\n")
		sb.WriteString("• 📌 Sticky-blocked list\n")
		sb.WriteString("• 🔨 Force-block list\n")
		sb.WriteString("• 🚨 Monitor scanner active alerts\n\n")
		sb.WriteString("_Coba ulangi pencarian via 🔍 CARI._")
	} else {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("_Klik 🔍 CARI lagi untuk pencarian baru._")
	}

	return h.reply(c, sb.String(), backToMain(), tele.ModeMarkdown)
}
