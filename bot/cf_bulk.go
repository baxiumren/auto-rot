package bot

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Bulk Change URL ──────────────────────────────────────────────────────────
//
// Flow:
// 1. User klik 📦 Bulk Change → tampilkan list rule dengan checkbox
// 2. User toggle rule yang mau diubah (☐ / ☑)
// 3. User klik ✅ Lanjut → bot tanya URL baru
// 4. User ketik URL → bot update semua rule paralel → tampilkan summary

const bulkSelectedKey = "selected" // session.Data key, value = "ruleID1,ruleID2,..."

func (h *Handler) handleCFBulkStart(c tele.Context) error {
	if !h.cf.HasCredentials() {
		return c.Edit(
			"⚠️ *CF Credentials belum di-set*\n\nSet email & API key dulu via menu *🔧 Settings*.",
			backToCF(), tele.ModeMarkdown,
		)
	}

	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit("📭 Belum ada CF rule. Tambah dulu via *Add Rule*.", backToCF(), tele.ModeMarkdown)
	}

	// Init session dengan selected kosong
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepCFBulkPick,
		Data: map[string]string{
			bulkSelectedKey: "",
		},
	})

	return c.Edit(buildBulkPickerText(rules, map[string]bool{}),
		buildBulkPickerMarkup(rules, map[string]bool{}), tele.ModeMarkdown)
}

func (h *Handler) handleCFBulkToggle(c tele.Context) error {
	ruleID := extractParam(c)
	if ruleID == "" {
		return nil
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFBulkPick {
		return h.handleCFBulkStart(c)
	}

	selected := parseSelected(sess.Data[bulkSelectedKey])
	if selected[ruleID] {
		delete(selected, ruleID)
	} else {
		selected[ruleID] = true
	}
	sess.Data[bulkSelectedKey] = serializeSelected(selected)
	h.sessions.Set(c.Sender().ID, sess)

	rules := h.cfrules.GetAll()
	return c.Edit(buildBulkPickerText(rules, selected),
		buildBulkPickerMarkup(rules, selected), tele.ModeMarkdown)
}

func (h *Handler) handleCFBulkSelectAll(c tele.Context, selectAll bool) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFBulkPick {
		return h.handleCFBulkStart(c)
	}

	rules := h.cfrules.GetAll()
	selected := map[string]bool{}
	if selectAll {
		for _, r := range rules {
			selected[r.ID] = true
		}
	}
	sess.Data[bulkSelectedKey] = serializeSelected(selected)
	h.sessions.Set(c.Sender().ID, sess)

	return c.Edit(buildBulkPickerText(rules, selected),
		buildBulkPickerMarkup(rules, selected), tele.ModeMarkdown)
}

func (h *Handler) handleCFBulkApply(c tele.Context) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFBulkPick {
		return h.handleCFBulkStart(c)
	}

	selected := parseSelected(sess.Data[bulkSelectedKey])
	if len(selected) == 0 {
		return c.Respond(&tele.CallbackResponse{
			Text:      "⚠️ Belum ada rule yang dipilih!",
			ShowAlert: true,
		})
	}

	// Build summary rule yang dipilih + URL terkini (paralel fetch)
	rules := h.cfrules.GetAll()
	var picked []store.CFRule
	for _, r := range rules {
		if selected[r.ID] {
			picked = append(picked, r)
		}
	}

	type fetchRes struct {
		idx int
		url string
		err error
	}
	results := make([]fetchRes, len(picked))
	var wg sync.WaitGroup
	for i, r := range picked {
		wg.Add(1)
		go func(idx int, rule store.CFRule) {
			defer wg.Done()
			url, err := h.cf.GetCurrentURL(rule)
			results[idx] = fetchRes{idx: idx, url: url, err: err}
		}(i, r)
	}
	wg.Wait()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 *Bulk Change — %d rule dipilih*\n", len(picked)))
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString("Yang akan diubah:\n")
	for i, r := range picked {
		curURL := "_(?)_"
		if results[i].err == nil && results[i].url != "" {
			curURL = "`" + escapeMD(truncate(results[i].url, 50)) + "`"
		}
		dom := r.Domain
		if dom == "" {
			dom = "_(no domain)_"
		}
		sb.WriteString(fmt.Sprintf("• *%s* (`%s`)\n  └ current: %s\n", escapeMD(r.Label), escapeMD(dom), curURL))
	}
	sb.WriteString("\n🔗 Ketik *URL baru* untuk semua rule di atas:\n_(contoh: https://newdomain.com)_")

	sess.Step = StepCFBulkURL
	h.sessions.Set(c.Sender().ID, sess)

	return c.Edit(sb.String(), cancelMenu(), tele.ModeMarkdown)
}

func (h *Handler) wizardCFBulkURL(c tele.Context, sess *Session) error {
	newURL := strings.TrimSpace(c.Text())
	if newURL == "" {
		return h.reply(c, "❌ URL tidak boleh kosong. Coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	if !strings.HasPrefix(newURL, "http://") && !strings.HasPrefix(newURL, "https://") {
		newURL = "https://" + newURL
	}

	selected := parseSelected(sess.Data[bulkSelectedKey])
	h.sessions.Delete(c.Sender().ID)

	rules := h.cfrules.GetAll()
	var picked []store.CFRule
	for _, r := range rules {
		if selected[r.ID] {
			picked = append(picked, r)
		}
	}
	if len(picked) == 0 {
		return h.reply(c, "❌ Rule yang dipilih sudah tidak ada. Coba ulangi.", backToCF(), tele.ModeMarkdown)
	}

	loadingMsg, _ := h.bot.Send(c.Chat(),
		fmt.Sprintf("⏳ Mengupdate %d rule ke URL:\n%s", len(picked), newURL))

	// Update paralel
	type updateRes struct {
		idx     int
		err     error
		prevURL string
	}
	results := make([]updateRes, len(picked))
	var wg sync.WaitGroup
	for i, r := range picked {
		wg.Add(1)
		go func(idx int, rule store.CFRule) {
			defer wg.Done()
			prevURL, _ := h.cf.GetCurrentURL(rule)
			err := h.cf.UpdateURL(rule, newURL)
			results[idx] = updateRes{idx: idx, err: err, prevURL: prevURL}
			if err != nil {
				log.Printf("[BULK] update failed for %s: %v", rule.Label, err)
				h.history.LogSwap("bulk", rule.Label, rule.Domain, prevURL, newURL, false, err.Error())
			} else {
				h.history.LogSwap("bulk", rule.Label, rule.Domain, prevURL, newURL, true, "")
			}
		}(i, r)
	}
	wg.Wait()

	// Build summary
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 *Bulk Change — Selesai*\n"))
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("🔗 Target URL baru:\n`%s`\n\n", escapeMD(newURL)))

	successCount := 0
	for i, r := range picked {
		if results[i].err == nil {
			successCount++
			sb.WriteString(fmt.Sprintf("✅ *%s* — sukses\n", escapeMD(r.Label)))
		} else {
			sb.WriteString(fmt.Sprintf("❌ *%s* — _%s_\n", escapeMD(r.Label),
				escapeMD(truncate(results[i].err.Error(), 60))))
		}
	}
	sb.WriteString(fmt.Sprintf("\n📊 *Berhasil:* %d/%d", successCount, len(picked)))

	if loadingMsg != nil {
		if _, err := h.bot.Edit(loadingMsg, sb.String(), backToCF(), tele.ModeMarkdown); err != nil {
			log.Printf("[BULK] Edit loadingMsg failed: %v — fallback Send", err)
			h.bot.Send(c.Chat(), sb.String(), backToCF(), tele.ModeMarkdown)
		}
		return nil
	}
	return h.reply(c, sb.String(), backToCF(), tele.ModeMarkdown)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseSelected(s string) map[string]bool {
	m := map[string]bool{}
	if s == "" {
		return m
	}
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			m[id] = true
		}
	}
	return m
}

func serializeSelected(m map[string]bool) string {
	var ids []string
	for id, sel := range m {
		if sel {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func buildBulkPickerText(rules []store.CFRule, selected map[string]bool) string {
	var sb strings.Builder
	sb.WriteString("📦 *Bulk Change URL — Ganti Banyak Sekaligus*\n")
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString("Pilih *beberapa rule* yang URL tujuannya mau diganti dengan URL yang sama.\n")
	sb.WriteString("_Klik tombol rule untuk centang ✅ / hapus centang ☐._\n\n")
	sb.WriteString(fmt.Sprintf("📊 *Dipilih:* %d / %d rule\n\n", len(selected), len(rules)))

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
		sb.WriteString("\n💡 _Klik *Lanjut* untuk ketik URL baru._")
	} else {
		sb.WriteString("_(Belum ada yang dipilih — klik tombol rule di bawah.)_")
	}
	return sb.String()
}

func buildBulkPickerMarkup(rules []store.CFRule, selected map[string]bool) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Tombol per rule (checkbox)
	for _, r := range rules {
		check := "☐"
		if selected[r.ID] {
			check = "☑"
		}
		label := r.Label
		if r.Domain != "" {
			label = fmt.Sprintf("%s (%s)", r.Label, r.Domain)
		}
		btnText := fmt.Sprintf("%s %s", check, truncate(label, 40))
		rows = append(rows, m.Row(m.Data(btnText, cbCFBulkToggle, r.ID)))
	}

	// Action row
	allSelected := len(selected) == len(rules) && len(rules) > 0
	if allSelected {
		rows = append(rows, m.Row(
			m.Data("☐ Hapus Semua", cbCFBulkSelNone),
		))
	} else {
		rows = append(rows, m.Row(
			m.Data("☑ Pilih Semua", cbCFBulkSelAll),
		))
	}

	// Lanjut + Batal
	if len(selected) > 0 {
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("✅ Lanjut (%d)", len(selected)), cbCFBulkApply),
			m.Data("❌ Batal", cbCF),
		))
	} else {
		rows = append(rows, m.Row(m.Data("🔙 Kembali", cbCF)))
	}

	m.Inline(rows...)
	return m
}
