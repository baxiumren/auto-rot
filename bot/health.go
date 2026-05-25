package bot

import (
	"fmt"
	"sort"
	"strings"

	"bongbot/checker"
	tele "gopkg.in/telebot.v3"
)

// ─── Health Dashboard ────────────────────────────────────────────────────────
//
// Ringkasan kondisi bot dalam 1 layar:
// • CF credentials status
// • Total domain di Monitor (+ per label)
// • Total CF rule
// • Total rotator (active/pause)
// • Domain blocked saat ini
// • Sticky & force count
// • Interval cek
// • Total swap history

func (h *Handler) handleHealth(c tele.Context) error {
	// CF status
	cfStatus := "❌ Belum di-set (CF Redirect & Auto-swap NONAKTIF)"
	cfPingErr := ""
	if h.cf.HasCredentials() {
		if err := h.cf.Ping(); err != nil {
			cfStatus = "⚠️ Credential terdaftar tapi gagal ping CF"
			cfPingErr = err.Error()
		} else {
			cfStatus = "✅ Aktif & terhubung"
		}
	}

	// Monitor stats
	allDomains := h.domains.GetAll()
	totalDomains := h.domains.TotalCount()
	labels := h.domains.Labels()
	sort.Strings(labels)

	// CF rules
	cfRules := h.cfrules.GetAll()

	// Rotators (count active vs pause)
	allRotators := h.rotators.GetAll()
	activeCount, pauseCount := 0, 0
	for _, r := range allRotators {
		if r.Active {
			activeCount++
		} else {
			pauseCount++
		}
	}

	// Blocked saat ini (dari monitor scanner)
	blocked := h.monScanner.GetBlockedSnapshot()
	stickyList := checker.Default().GetStickyList()
	forceList := checker.Default().GetForceList()

	// Build pesan
	var sb strings.Builder
	sb.WriteString("🩺 *HEALTH DASHBOARD*\n")
	sb.WriteString("═══════════════════════════\n\n")

	// CF Status
	sb.WriteString("*⚙️ Cloudflare:*\n")
	sb.WriteString(fmt.Sprintf("• %s\n", cfStatus))
	if cfPingErr != "" {
		sb.WriteString(fmt.Sprintf("  _error: %s_\n", escapeMD(truncate(cfPingErr, 80))))
	}
	sb.WriteString("\n")

	// Monitor
	sb.WriteString("*📡 Monitor:*\n")
	sb.WriteString(fmt.Sprintf("• Total domain: *%d*\n", totalDomains))
	sb.WriteString(fmt.Sprintf("• Total kategori: *%d*\n", len(labels)))
	if len(labels) > 0 {
		sb.WriteString("• Per kategori:\n")
		for _, lbl := range labels {
			sb.WriteString(fmt.Sprintf("   📂 *%s*: %d domain\n", lbl, len(allDomains[lbl])))
		}
	}
	sb.WriteString(fmt.Sprintf("• Interval cek: *%v*\n", h.monScanner.GetInterval()))
	sb.WriteString("\n")

	// CF Rules
	sb.WriteString("*⚙️ CF Redirect:*\n")
	sb.WriteString(fmt.Sprintf("• Total rule terdaftar: *%d*\n", len(cfRules)))
	if len(cfRules) > 0 {
		v1, v2 := 0, 0
		for _, r := range cfRules {
			if r.Type == "page_rules" {
				v1++
			} else {
				v2++
			}
		}
		sb.WriteString(fmt.Sprintf("   • V2 Redirect Rules: *%d*\n", v2))
		sb.WriteString(fmt.Sprintf("   • V1 Page Rules: *%d*\n", v1))
	}
	sb.WriteString("\n")

	// Rotators
	sb.WriteString("*🔄 Auto Rotator:*\n")
	sb.WriteString(fmt.Sprintf("• Total config: *%d*\n", len(allRotators)))
	sb.WriteString(fmt.Sprintf("   • ▶️ Aktif: *%d*\n", activeCount))
	if pauseCount > 0 {
		sb.WriteString(fmt.Sprintf("   • ⏸ Pause: *%d*\n", pauseCount))
	}
	sb.WriteString("\n")

	// Blocked
	sb.WriteString("*🚨 Status Blocked:*\n")
	if len(blocked) == 0 {
		sb.WriteString("• ✅ Gak ada domain blocked saat ini\n")
	} else {
		sb.WriteString(fmt.Sprintf("• 🔴 *%d domain* sedang blocked:\n", len(blocked)))
		// Sort by domain name
		var bdom []string
		for d := range blocked {
			bdom = append(bdom, d)
		}
		sort.Strings(bdom)
		for i, d := range bdom {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("   _... dan %d lainnya_\n", len(bdom)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("   • `%s`\n", d))
		}
	}
	sb.WriteString(fmt.Sprintf("• 📌 Sticky-blocked: *%d* domain\n", len(stickyList)))
	if len(forceList) > 0 {
		sb.WriteString(fmt.Sprintf("• 🔨 Force-blocked: *%d* domain\n", len(forceList)))
	}
	sb.WriteString("\n")

	// History
	sb.WriteString("*📜 Swap History:*\n")
	sb.WriteString(fmt.Sprintf("• Total swap tercatat: *%d*\n", h.history.Count()))
	if recent := h.history.GetRecent(1); len(recent) > 0 {
		sb.WriteString(fmt.Sprintf("• Swap terakhir: %s (%s)\n",
			recent[0].Timestamp.Format("02/01 15:04:05"), recent[0].Source))
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("_Update real-time tiap klik 🩺 STATUS._")

	// Tombol bawah (Edit kalau dari callback, Send kalau dari text)
	mkup := backToMain()
	if c.Callback() != nil {
		return c.Edit(sb.String(), mkup, tele.ModeMarkdown)
	}
	return c.Send(sb.String(), mkup, tele.ModeMarkdown)
}
