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

	// Klikcepat rotators
	klcSLRotators := h.klikcepatRotators.GetAll()
	klcSLActive, klcSLPause := 0, 0
	for _, r := range klcSLRotators {
		if r.Active {
			klcSLActive++
		} else {
			klcSLPause++
		}
	}
	klcBLRotators := h.klikcepatBlockRotators.GetAll()
	klcBLActive, klcBLPause := 0, 0
	for _, r := range klcBLRotators {
		if r.Active {
			klcBLActive++
		} else {
			klcBLPause++
		}
	}

	// Klikcepat status
	klcStatus := "❌ Belum di-set (Klikcepat auto-swap NONAKTIF)"
	klcPingErr := ""
	if h.klikcepat.HasCredentials() {
		if err := h.klikcepat.Ping(); err != nil {
			klcStatus = "⚠️ Credential terdaftar tapi gagal ping Klikcepat"
			klcPingErr = err.Error()
		} else {
			klcStatus = "✅ Aktif & terhubung"
		}
	}
	klcDomainMap := h.creds.GetKlikcepatDomainMap()

	// Blocked saat ini (dari monitor scanner)
	blocked := h.monScanner.GetBlockedSnapshot()
	stickyList := checker.Default().GetStickyList()
	forceList := checker.Default().GetForceList()

	// Build pesan
	var sb strings.Builder
	sb.WriteString("💎 *H E A L T H   D A S H B O A R D* 💎\n")
	sb.WriteString("|\n")

	// CF Status
	sb.WriteString("⚙️ *CLOUDFLARE*\n")
	sb.WriteString(fmt.Sprintf("└ Status : %s\n", cfStatus))
	if cfPingErr != "" {
		sb.WriteString(fmt.Sprintf("└ Error  : %s\n", escapeMD(truncate(cfPingErr, 60))))
	}
	sb.WriteString("|\n")

	// Monitor
	sb.WriteString("📡 *MONITOR*\n")
	sb.WriteString(fmt.Sprintf("└ Total domain  : %d\n", totalDomains))
	sb.WriteString(fmt.Sprintf("└ Total kategori: %d\n", len(labels)))
	sb.WriteString(fmt.Sprintf("└ Interval cek  : %v\n", h.monScanner.GetInterval()))
	if len(labels) > 0 {
		sb.WriteString("└ Per kategori:\n")
		for _, lbl := range labels {
			sb.WriteString(fmt.Sprintf("   └ 📂 %s : %d domain\n", escapeMD(lbl), len(allDomains[lbl])))
		}
	}
	sb.WriteString("|\n")

	// CF Rules
	sb.WriteString("⚙️ *CF REDIRECT*\n")
	sb.WriteString(fmt.Sprintf("└ Total rule : %d\n", len(cfRules)))
	if len(cfRules) > 0 {
		v1, v2 := 0, 0
		for _, r := range cfRules {
			if r.Type == "page_rules" {
				v1++
			} else {
				v2++
			}
		}
		sb.WriteString(fmt.Sprintf("└ V2 Redirect: %d\n", v2))
		sb.WriteString(fmt.Sprintf("└ V1 Page    : %d\n", v1))
	}
	sb.WriteString("|\n")

	// CF Rotators
	sb.WriteString("🔄 *CF AUTO ROTATOR*\n")
	sb.WriteString(fmt.Sprintf("└ Total config : %d\n", len(allRotators)))
	sb.WriteString(fmt.Sprintf("└ ▶️ Aktif      : %d\n", activeCount))
	if pauseCount > 0 {
		sb.WriteString(fmt.Sprintf("└ ⏸ Pause      : %d\n", pauseCount))
	}
	sb.WriteString("|\n")

	// Klikcepat Status + Rotators
	sb.WriteString("🔗 *KLIKCEPAT*\n")
	sb.WriteString(fmt.Sprintf("└ Status : %s\n", klcStatus))
	if klcPingErr != "" {
		sb.WriteString(fmt.Sprintf("└ Error  : %s\n", escapeMD(truncate(klcPingErr, 60))))
	}
	if len(klcDomainMap) > 0 {
		sb.WriteString(fmt.Sprintf("└ 🌐 Domain mapping : %d entry\n", len(klcDomainMap)))
	}
	sb.WriteString(fmt.Sprintf("└ 🔗 Shortlink rot. : %d (▶️ %d / ⏸ %d)\n",
		len(klcSLRotators), klcSLActive, klcSLPause))
	sb.WriteString(fmt.Sprintf("└ 📄 Block rotator  : %d (▶️ %d / ⏸ %d)\n",
		len(klcBLRotators), klcBLActive, klcBLPause))
	sb.WriteString("|\n")

	// Blocked
	sb.WriteString("🚨 *STATUS BLOCKED*\n")
	if len(blocked) == 0 {
		sb.WriteString("└ ✅ Gak ada domain blocked saat ini\n")
	} else {
		sb.WriteString(fmt.Sprintf("└ 🔴 %d domain sedang blocked:\n", len(blocked)))
		var bdom []string
		for d := range blocked {
			bdom = append(bdom, d)
		}
		sort.Strings(bdom)
		for i, d := range bdom {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("   └ ... dan %d lainnya\n", len(bdom)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("   └ `%s`\n", d))
		}
	}
	sb.WriteString(fmt.Sprintf("└ 📌 Sticky-blocked: %d domain\n", len(stickyList)))
	if len(forceList) > 0 {
		sb.WriteString(fmt.Sprintf("└ 🔨 Force-blocked : %d domain\n", len(forceList)))
	}
	sb.WriteString("|\n")

	// History
	sb.WriteString("📜 *SWAP HISTORY*\n")
	sb.WriteString(fmt.Sprintf("└ Total swap : %d\n", h.history.Count()))
	if recent := h.history.GetRecent(1); len(recent) > 0 {
		sb.WriteString(fmt.Sprintf("└ Terakhir   : %s (%s)\n",
			recent[0].Timestamp.Format("02/01 15:04:05"), recent[0].Source))
	}

	sb.WriteString("|\n")
	sb.WriteString("🔄 Update real-time tiap klik 🩺 STATUS")

	// Tombol bawah (Edit kalau dari callback, Send kalau dari text)
	mkup := backToMain()
	if c.Callback() != nil {
		return c.Edit(sb.String(), mkup, tele.ModeMarkdown)
	}
	return c.Send(sb.String(), mkup, tele.ModeMarkdown)
}
