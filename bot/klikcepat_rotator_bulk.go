package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Bulk Setup Rotator ────────────────────────────────────────────
//
// Flow:
// 1. User klik 📦 Bulk Setup → pick type 🔗 KLIKCEPAT
// 2. Bot fetch klikcepat links → tampilkan checkbox picker
// 3. User toggle ☑/☐ banyak link (atau Pilih Semua)
// 4. Klik Lanjut → tampilkan pool picker
// 5. User pilih 1 pool → save rotator config untuk SEMUA selected link

const klikcepatBulkSelKey = "klc_selected" // session.Data key

// klikcepatBulkPick is a row in the bulk picker (subset of klikcepat.Link).
type klikcepatBulkPick struct {
	ID    int
	URL   string
	Type  string
	Title string
}

// handleRotatorBulkTypeKlikcepat — entry from Bulk Setup pick-type screen
func (h *Handler) handleRotatorBulkTypeKlikcepat(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Loading links dari klikcepat...", tele.ModeMarkdown)

	picks, err := h.fetchKlikcepatBulkPicks()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	if len(picks) == 0 {
		return c.Edit(
			"✅ *Semua link klikcepat udah punya rotator*\n\nGak ada link tipe link/biolink yang belum ke-setup.\n\nHapus rotator lama via *📋 List Rotator* atau create link baru via *🔗 KLIKCEPAT → ➕ Tambah Link*.",
			backToRotator(), tele.ModeMarkdown)
	}

	// Cache available link IDs in session — biar Pilih Semua / fallback gampang
	linkIDsStr := make([]string, len(picks))
	for i, p := range picks {
		linkIDsStr[i] = strconv.Itoa(p.ID)
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlikcepatRotBulkPick,
		Data: map[string]string{
			klikcepatBulkSelKey: "",
			"available":         strings.Join(linkIDsStr, ","),
		},
	})

	return h.renderKlikcepatBulkPicker(c, picks, map[int]bool{})
}

// fetchKlikcepatBulkPicks fetches links and filters out those that:
//   - already have a klikcepat rotator
//   - have a non-rotatable type (only "link" / "biolink" are valid swap targets)
func (h *Handler) fetchKlikcepatBulkPicks() ([]klikcepatBulkPick, error) {
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return nil, err
	}
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatRotators.GetAll() {
		hasRotator[rot.LinkID] = true
	}
	var picks []klikcepatBulkPick
	for _, l := range links {
		if hasRotator[l.ID] {
			continue
		}
		if l.Type != "link" && l.Type != "biolink" {
			continue
		}
		picks = append(picks, klikcepatBulkPick{
			ID:    l.ID,
			URL:   l.URL,
			Type:  l.Type,
			Title: l.Title,
		})
	}
	return picks, nil
}

// renderKlikcepatBulkPicker renders the checkbox picker view.
func (h *Handler) renderKlikcepatBulkPicker(c tele.Context, picks []klikcepatBulkPick, selected map[int]bool) error {
	var sb strings.Builder
	sb.WriteString("📦 *Bulk Setup Klikcepat Rotator — Pilih Links*\n═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("📊 *Dipilih:* %d / %d link\n\n", len(selected), len(picks)))
	if len(selected) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n*Link yang dipilih:*\n")
		for _, p := range picks {
			if selected[p.ID] {
				typeIcon := "🔗"
				if p.Type == "biolink" {
					typeIcon = "📄"
				}
				sb.WriteString(fmt.Sprintf("✅ %s *%s* (`/%s`)\n", typeIcon, escapeMD(p.Title), escapeMD(p.URL)))
			}
		}
		sb.WriteString("\n💡 _Klik *Lanjut* untuk pilih pool._")
	} else {
		sb.WriteString("_(Belum ada yang dipilih — klik tombol link di bawah.)_")
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range picks {
		check := "☐"
		if selected[p.ID] {
			check = "☑"
		}
		typeIcon := "🔗"
		if p.Type == "biolink" {
			typeIcon = "📄"
		}
		btnText := fmt.Sprintf("%s %s %s (/%s)", check, typeIcon, truncate(p.Title, 20), p.URL)
		rows = append(rows, m.Row(m.Data(truncate(btnText, 60), cbKlikcepatRotBulkToggle, strconv.Itoa(p.ID))))
		if len(rows) >= 30 {
			break
		}
	}

	allSelected := len(selected) == len(picks) && len(picks) > 0
	if allSelected {
		rows = append(rows, m.Row(m.Data("☐ Hapus Semua", cbKlikcepatRotBulkSelNone)))
	} else {
		rows = append(rows, m.Row(m.Data("☑ Pilih Semua", cbKlikcepatRotBulkSelAll)))
	}
	if len(selected) > 0 {
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("✅ Lanjut (%d)", len(selected)), cbKlikcepatRotBulkProceed),
			m.Data("❌ Batal", cbRotator),
		))
	} else {
		rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotator)))
	}
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatRotBulkToggle(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID == 0 {
		return nil
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotBulkPick {
		return h.handleRotatorBulkTypeKlikcepat(c)
	}

	selected := parseSelectedInts(sess.Data[klikcepatBulkSelKey])
	if selected[linkID] {
		delete(selected, linkID)
	} else {
		selected[linkID] = true
	}
	sess.Data[klikcepatBulkSelKey] = serializeSelectedInts(selected)
	h.sessions.Set(c.Sender().ID, sess)

	picks, err := h.fetchKlikcepatBulkPicks()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	return h.renderKlikcepatBulkPicker(c, picks, selected)
}

func (h *Handler) handleKlikcepatRotBulkSelectAll(c tele.Context, selectAll bool) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotBulkPick {
		return h.handleRotatorBulkTypeKlikcepat(c)
	}
	picks, err := h.fetchKlikcepatBulkPicks()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	selected := map[int]bool{}
	if selectAll {
		for _, p := range picks {
			selected[p.ID] = true
		}
	}
	sess.Data[klikcepatBulkSelKey] = serializeSelectedInts(selected)
	h.sessions.Set(c.Sender().ID, sess)
	return h.renderKlikcepatBulkPicker(c, picks, selected)
}

func (h *Handler) handleKlikcepatRotBulkProceed(c tele.Context) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotBulkPick {
		return h.handleRotatorBulkTypeKlikcepat(c)
	}
	selected := parseSelectedInts(sess.Data[klikcepatBulkSelKey])
	if len(selected) == 0 {
		return c.Respond(&tele.CallbackResponse{
			Text: "⚠️ Belum pilih link!", ShowAlert: true,
		})
	}

	sess.Step = StepKlikcepatRotBulkPool
	h.sessions.Set(c.Sender().ID, sess)

	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit(
			"⚠️ Belum ada pool di Monitor. Add domain dulu via *📡 Monitor → ➕ Add Domain*.",
			backToRotator(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, lbl := range labels {
		domains := h.domains.GetByLabel(lbl)
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📂 %s (%d domain)", lbl, len(domains)),
			cbKlikcepatRotBulkPickPool, lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit(
		fmt.Sprintf(
			"📦 *Bulk Setup — Pick Pool*\n\n"+
				"✅ %d link dipilih\n\n"+
				"Pilih pool yang akan di-assign ke SEMUA link tersebut:",
			len(selected)),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatRotBulkPickPool(c tele.Context) error {
	pool := extractParam(c)
	if pool == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Pool kosong", ShowAlert: true})
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotBulkPool {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}

	selected := parseSelectedInts(sess.Data[klikcepatBulkSelKey])
	h.sessions.Delete(c.Sender().ID)

	if len(selected) == 0 {
		return c.Edit("⚠️ No selection", backToRotator(), tele.ModeMarkdown)
	}

	c.Edit(fmt.Sprintf("⏳ Saving %d rotator config(s)...", len(selected)), tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	created := 0
	skipped := 0
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 *Bulk Setup — Result*\n\nPool: *%s*\n\n", escapeMD(pool)))

	for _, l := range links {
		if !selected[l.ID] {
			continue
		}
		// Auto-generate label: TYPE-SLUG (uppercased). Truncate if too long.
		label := strings.ToUpper(l.Type) + "-" + strings.ToUpper(l.URL)
		if len(label) > 40 {
			label = label[:40]
		}
		rot := store.KlikcepatRotator{
			Label:     label,
			LinkID:    l.ID,
			LinkURL:   l.URL,
			LinkType:  l.Type,
			PoolLabel: pool,
		}
		if err := h.klikcepatRotators.Add(rot); err != nil {
			sb.WriteString(fmt.Sprintf("⚠️ `%s` skipped: %s\n", escapeMD(l.URL), escapeMD(err.Error())))
			skipped++
			continue
		}
		sb.WriteString(fmt.Sprintf("✅ `%s` → label `%s`\n", escapeMD(l.URL), escapeMD(label)))
		created++
	}

	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n*Total:* %d created, %d skipped", created, skipped))

	return c.Edit(sb.String(), backToRotator(), tele.ModeMarkdown)
}

// ─── Helpers (selected set serialization for klikcepat bulk) ─────────────────

func parseSelectedInts(s string) map[int]bool {
	out := make(map[int]bool)
	if s == "" {
		return out
	}
	for _, p := range strings.Split(s, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil && id > 0 {
			out[id] = true
		}
	}
	return out
}

func serializeSelectedInts(set map[int]bool) string {
	if len(set) == 0 {
		return ""
	}
	parts := make([]string, 0, len(set))
	for id := range set {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}
