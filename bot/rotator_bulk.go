package bot

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Bulk Setup Rotator ───────────────────────────────────────────────────────
//
// Flow:
// 1. User klik 📦 Bulk Setup → tampilkan list CF rule dengan checkbox
// 2. User pilih beberapa rule yang mau di-set rotator (atau Pilih Semua)
// 3. Klik Lanjut → tampilkan list pool label dari Monitor
// 4. User pilih 1 pool → bot bikin Rotator config untuk SEMUA CF rule terpilih
//    (pool sama untuk semua) → summary

const rotatorBulkSelKey = "selected" // session.Data key, value = "ruleID1,ruleID2,..."

func (h *Handler) handleRotatorBulk(c tele.Context) error {
	allRules := h.cfrules.GetAll()
	if len(allRules) == 0 {
		return c.Edit(
			"📭 *Belum ada CF Rule*\n\nTambah CF Rule dulu via *⚙️ CF Redirect → ➕ Add Rule*.",
			backToRotator(), tele.ModeMarkdown)
	}

	// Filter: skip rule yang udah punya rotator config
	hasRotator := make(map[string]bool)
	for _, rot := range h.rotators.GetAll() {
		hasRotator[rot.CFRuleID] = true
	}
	var rules []store.CFRule
	for _, r := range allRules {
		if !hasRotator[r.ID] {
			rules = append(rules, r)
		}
	}

	if len(rules) == 0 {
		return c.Edit(
			fmt.Sprintf(
				"✅ *Semua CF Rule udah punya Rotator*\n\n"+
					"Total %d CF Rule, semuanya udah di-setup auto-rotate.\n\n"+
					"_Mau ganti pool? Hapus rotator lama dulu via *📋 List Rotator*._",
				len(allRules)),
			backToRotator(), tele.ModeMarkdown)
	}

	// Init session
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepRotatorBulkPick,
		Data: map[string]string{rotatorBulkSelKey: ""},
	})

	return c.Edit(
		buildRotatorBulkPickerText(rules, map[string]bool{}, len(allRules)),
		buildRotatorBulkPickerMarkup(rules, map[string]bool{}),
		tele.ModeMarkdown)
}

func (h *Handler) handleRotatorBulkToggle(c tele.Context) error {
	ruleID := extractParam(c)
	if ruleID == "" {
		return nil
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepRotatorBulkPick {
		return h.handleRotatorBulk(c)
	}

	selected := parseSelected(sess.Data[rotatorBulkSelKey])
	if selected[ruleID] {
		delete(selected, ruleID)
	} else {
		selected[ruleID] = true
	}
	sess.Data[rotatorBulkSelKey] = serializeSelected(selected)
	h.sessions.Set(c.Sender().ID, sess)

	rules, totalAll := h.filteredCFRules()
	return c.Edit(
		buildRotatorBulkPickerText(rules, selected, totalAll),
		buildRotatorBulkPickerMarkup(rules, selected),
		tele.ModeMarkdown)
}

func (h *Handler) handleRotatorBulkSelectAll(c tele.Context, selectAll bool) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepRotatorBulkPick {
		return h.handleRotatorBulk(c)
	}
	rules, totalAll := h.filteredCFRules()
	selected := map[string]bool{}
	if selectAll {
		for _, r := range rules {
			selected[r.ID] = true
		}
	}
	sess.Data[rotatorBulkSelKey] = serializeSelected(selected)
	h.sessions.Set(c.Sender().ID, sess)

	return c.Edit(
		buildRotatorBulkPickerText(rules, selected, totalAll),
		buildRotatorBulkPickerMarkup(rules, selected),
		tele.ModeMarkdown)
}

// filteredCFRules return CF Rules yang BELUM punya rotator config + total all rules.
// Helper buat bulk picker — biar gak repeat filter logic.
func (h *Handler) filteredCFRules() (filtered []store.CFRule, totalAll int) {
	all := h.cfrules.GetAll()
	totalAll = len(all)
	hasRotator := make(map[string]bool)
	for _, rot := range h.rotators.GetAll() {
		hasRotator[rot.CFRuleID] = true
	}
	for _, r := range all {
		if !hasRotator[r.ID] {
			filtered = append(filtered, r)
		}
	}
	return filtered, totalAll
}

func (h *Handler) handleRotatorBulkProceed(c tele.Context) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepRotatorBulkPick {
		return h.handleRotatorBulk(c)
	}
	selected := parseSelected(sess.Data[rotatorBulkSelKey])
	if len(selected) == 0 {
		return c.Respond(&tele.CallbackResponse{
			Text: "⚠️ Belum pilih CF rule!", ShowAlert: true,
		})
	}

	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit(
			"⚠️ *Belum ada label/pool di Monitor*\n\nTambah dulu beberapa domain di Monitor untuk membentuk pool.\n\n_Pool = kumpulan domain cadangan saat swap._",
			backToRotator(), tele.ModeMarkdown)
	}

	sess.Step = StepRotatorBulkPool
	h.sessions.Set(c.Sender().ID, sess)

	rules := h.cfrules.GetAll()
	var pickedRules []store.CFRule
	for _, r := range rules {
		if selected[r.ID] {
			pickedRules = append(pickedRules, r)
		}
	}

	// Build text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 *Bulk Setup Rotator — Pilih Pool*\n"))
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("✅ %d CF Rule akan di-setup:\n", len(pickedRules)))
	for _, r := range pickedRules {
		dom := r.Domain
		if dom == "" {
			dom = "(no domain)"
		}
		sb.WriteString(fmt.Sprintf("• *%s* (`%s`)\n", escapeMD(r.Label), escapeMD(dom)))
	}
	sb.WriteString("\n━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📂 *Pilih 1 pool* untuk semua rule di atas:\n")
	sb.WriteString("_(pool ini bakal jadi sumber domain pengganti saat ada yg blocked)_")

	// Build markup
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, lbl := range labels {
		count := len(h.domains.GetByLabel(lbl))
		btnText := fmt.Sprintf("📂 %s (%d domain)", lbl, count)
		rows = append(rows, m.Row(m.Data(btnText, cbRotatorBulkPickPool, lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

func (h *Handler) handleRotatorBulkPickPool(c tele.Context) error {
	poolLabel := extractParam(c)
	if poolLabel == "" {
		return h.handleRotatorBulk(c)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepRotatorBulkPool {
		return h.handleRotatorBulk(c)
	}

	selected := parseSelected(sess.Data[rotatorBulkSelKey])
	h.sessions.Delete(c.Sender().ID)

	rules := h.cfrules.GetAll()
	allRotators := h.rotators.GetAll()
	hasRotator := make(map[string]bool, len(allRotators))
	for _, rot := range allRotators {
		hasRotator[rot.CFRuleID] = true
	}

	// Buat rotator untuk masing-masing CF rule terpilih
	var created, skipped []string
	for _, r := range rules {
		if !selected[r.ID] {
			continue
		}
		if hasRotator[r.ID] {
			skipped = append(skipped, r.Label)
			continue
		}
		// Generate label: pakai "BULK-<CFlabel>-<poolLabel>" atau cuma CFlabel
		labelStr := r.Label
		newRot := store.RotatorRule{
			Label:     labelStr,
			CFRuleID:  r.ID,
			PoolLabel: poolLabel,
		}
		h.rotators.Add(newRot)
		created = append(created, r.Label)
		log.Printf("[BULK-ROT] created: cf=%s pool=%s", r.Label, poolLabel)
	}

	// Build summary
	var sb strings.Builder
	sb.WriteString("📦 *Bulk Setup Rotator — Selesai*\n")
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("📂 *Pool:* `%s` (%d domain)\n\n",
		escapeMD(poolLabel), len(h.domains.GetByLabel(poolLabel))))

	if len(created) > 0 {
		sb.WriteString(fmt.Sprintf("✅ *%d Rotator dibuat:*\n", len(created)))
		for _, l := range created {
			sb.WriteString(fmt.Sprintf("  • %s\n", escapeMD(l)))
		}
	}
	if len(skipped) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️ *%d di-skip* (sudah ada rotator):\n", len(skipped)))
		for _, l := range skipped {
			sb.WriteString(fmt.Sprintf("  • %s\n", escapeMD(l)))
		}
		sb.WriteString("\n_Hapus rotator lama dulu kalau mau ganti pool._")
	}
	if len(created) > 0 {
		sb.WriteString("\n━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("🟢 *Status: AKTIF* — Monitor Scanner akan langsung pakai config ini saat detect blocked.")
	}

	return c.Edit(sb.String(), backToRotator(), tele.ModeMarkdown)
}

// ─── UI Helpers ──────────────────────────────────────────────────────────────

func buildRotatorBulkPickerText(rules []store.CFRule, selected map[string]bool, totalAll int) string {
	var sb strings.Builder
	sb.WriteString("📦 *Bulk Setup Rotator — Pilih CF Rules*\n")
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString("Pilih *banyak CF Rule sekaligus*, nanti assign ke 1 pool yang sama.\n")
	sb.WriteString("_Cocok kalau banyak rule pakai pool yg sama._\n\n")
	sb.WriteString(fmt.Sprintf("📊 *Dipilih:* %d / %d rule (belum punya rotator)\n", len(selected), len(rules)))
	if hidden := totalAll - len(rules); hidden > 0 {
		sb.WriteString(fmt.Sprintf("_(%d rule udah punya rotator — disembunyikan.)_\n", hidden))
	}
	sb.WriteString("\n")

	if len(selected) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n*Rule yang dipilih:*\n")
		for _, r := range rules {
			if selected[r.ID] {
				dom := r.Domain
				if dom == "" {
					dom = "(no domain)"
				}
				sb.WriteString(fmt.Sprintf("✅ *%s* — `%s`\n", escapeMD(r.Label), escapeMD(dom)))
			}
		}
		sb.WriteString("\n💡 _Klik *Lanjut* untuk pilih pool._")
	} else {
		sb.WriteString("_(Belum ada yang dipilih — klik tombol rule di bawah.)_")
	}
	return sb.String()
}

func buildRotatorBulkPickerMarkup(rules []store.CFRule, selected map[string]bool) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Sort rules by label for consistent order
	sortedRules := append([]store.CFRule{}, rules...)
	sort.Slice(sortedRules, func(i, j int) bool { return sortedRules[i].Label < sortedRules[j].Label })

	// Tombol per rule (checkbox)
	for _, r := range sortedRules {
		check := "☐"
		if selected[r.ID] {
			check = "☑"
		}
		dom := r.Domain
		if dom == "" {
			dom = "?"
		}
		btnText := fmt.Sprintf("%s %s (%s)", check, r.Label, dom)
		rows = append(rows, m.Row(m.Data(truncate(btnText, 60), cbRotatorBulkToggle, r.ID)))
	}

	// Action row
	allSelected := len(selected) == len(rules) && len(rules) > 0
	if allSelected {
		rows = append(rows, m.Row(m.Data("☐ Hapus Semua", cbRotatorBulkSelNone)))
	} else {
		rows = append(rows, m.Row(m.Data("☑ Pilih Semua", cbRotatorBulkSelAll)))
	}

	// Lanjut / Batal
	if len(selected) > 0 {
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("✅ Lanjut (%d)", len(selected)), cbRotatorBulkProceed),
			m.Data("❌ Batal", cbRotator),
		))
	} else {
		rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotator)))
	}

	m.Inline(rows...)
	return m
}
