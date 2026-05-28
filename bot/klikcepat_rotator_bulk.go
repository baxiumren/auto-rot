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
const klikcepatBulkPageKey = "klc_page"    // session.Data key (current page)
const klikcepatBulkPerPage = 10            // links per page

// klikcepatBulkPick is a row in the bulk picker (subset of klikcepat.Link).
type klikcepatBulkPick struct {
	ID    int
	URL   string
	Type  string
	Title string
}

// handleRotatorBulkTypeKlikcepat — entry from Bulk Setup pick-type screen.
// Show subtype: BIOLINK BLOCK vs SHORTLINK
func (h *Handler) handleRotatorBulkTypeKlikcepat(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📄 BIOLINK", cbRotatorBulkTypeKlcBiolink),
			m.Data("🔗 SHORTLINK", cbRotatorBulkTypeKlcShortlink),
		),
		m.Row(m.Data("❌ Batal", cbRotator)),
	)
	return c.Edit(
		"📦 *Bulk Setup Klikcepat — Pilih Tipe*\n\n"+
			"• *📄 BIOLINK* — pick 1 biolink → multi-select blocks → 1 pool\n"+
			"• *🔗 SHORTLINK* — multi-select shortlinks → 1 pool",
		m, tele.ModeMarkdown)
}

// handleRotatorBulkTypeKlcShortlink — bulk shortlink picker (existing logic, renamed).
func (h *Handler) handleRotatorBulkTypeKlcShortlink(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Loading shortlinks dari klikcepat...", tele.ModeMarkdown)

	picks, err := h.fetchKlikcepatBulkPicks()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	if len(picks) == 0 {
		return c.Edit(
			"✅ *Semua shortlink klikcepat udah punya rotator*\n\nHapus rotator lama via *📋 List Rotator* atau create shortlink baru via *🔗 KLIKCEPAT → ➕ Tambah Link*.",
			backToRotator(), tele.ModeMarkdown)
	}

	linkIDsStr := make([]string, len(picks))
	for i, p := range picks {
		linkIDsStr[i] = strconv.Itoa(int(p.ID))
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlikcepatRotBulkPick,
		Data: map[string]string{
			klikcepatBulkSelKey:  "",
			klikcepatBulkPageKey: "0",
			"available":          strings.Join(linkIDsStr, ","),
		},
	})

	return h.renderKlikcepatBulkPicker(c, picks, map[int]bool{}, 0)
}

// fetchKlikcepatBulkPicks fetches SHORTLINK only (biolink punya bulk-flow sendiri).
// Filter:
//   - sudah punya rotator → skip
//   - type != "link" → skip (biolink di-handle terpisah)
func (h *Handler) fetchKlikcepatBulkPicks() ([]klikcepatBulkPick, error) {
	links, err := h.klikcepat.ListLinks("link")
	if err != nil {
		return nil, err
	}
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatRotators.GetAll() {
		hasRotator[rot.LinkID] = true
	}
	var picks []klikcepatBulkPick
	for _, l := range links {
		if l.Type != "link" {
			continue
		}
		if hasRotator[int(l.ID)] {
			continue
		}
		picks = append(picks, klikcepatBulkPick{
			ID:    int(l.ID),
			URL:   l.URL,
			Type:  l.Type,
			Title: l.Title,
		})
	}
	return picks, nil
}

// renderKlikcepatBulkPicker renders the checkbox picker view with pagination.
func (h *Handler) renderKlikcepatBulkPicker(c tele.Context, picks []klikcepatBulkPick, selected map[int]bool, page int) error {
	total := len(picks)
	totalPages := (total + klikcepatBulkPerPage - 1) / klikcepatBulkPerPage
	if totalPages == 0 {
		totalPages = 1
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * klikcepatBulkPerPage
	end := start + klikcepatBulkPerPage
	if end > total {
		end = total
	}

	var sb strings.Builder
	sb.WriteString("📦 *Bulk Setup Klikcepat Rotator — Pilih Links*\n═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("📊 *Dipilih:* %d / %d link • Page %d/%d\n\n", len(selected), total, page+1, totalPages))
	if len(selected) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n*Link yang dipilih:*\n")
		shown := 0
		for _, p := range picks {
			if selected[p.ID] {
				typeIcon := "🔗"
				if p.Type == "biolink" {
					typeIcon = "📄"
				}
				sb.WriteString(fmt.Sprintf("✅ %s `/%s`\n", typeIcon, escapeMD(strings.ToUpper(p.URL))))
				shown++
				if shown >= 15 {
					sb.WriteString(fmt.Sprintf("_...dan %d lainnya_\n", len(selected)-shown))
					break
				}
			}
		}
		sb.WriteString("\n💡 _Klik *Lanjut* untuk pilih pool._")
	} else {
		sb.WriteString("_(Belum ada yang dipilih — klik tombol link di bawah.)_")
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := start; i < end; i++ {
		p := picks[i]
		check := "☐"
		if selected[p.ID] {
			check = "☑"
		}
		typeIcon := "🔗"
		if p.Type == "biolink" {
			typeIcon = "📄"
		}
		label := strings.ToUpper(p.URL)
		if label == "" {
			label = "(no slug)"
		}
		btnText := fmt.Sprintf("%s %s %s", check, typeIcon, truncate(label, 40))
		rows = append(rows, m.Row(m.Data(btnText, cbKlikcepatRotBulkToggle, strconv.Itoa(p.ID))))
	}

	// Pagination row
	if totalPages > 1 {
		var navRow tele.Row
		if page > 0 {
			navRow = append(navRow, m.Data("⬅️ Prev", cbKlikcepatRotBulkPage, strconv.Itoa(page-1)))
		}
		navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
		if page < totalPages-1 {
			navRow = append(navRow, m.Data("Next ➡️", cbKlikcepatRotBulkPage, strconv.Itoa(page+1)))
		}
		rows = append(rows, navRow)
	}

	allSelected := len(selected) == total && total > 0
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
	page, _ := strconv.Atoi(sess.Data[klikcepatBulkPageKey])
	return h.renderKlikcepatBulkPicker(c, picks, selected, page)
}

// handleKlikcepatRotBulkPage — navigasi halaman Prev/Next di picker
func (h *Handler) handleKlikcepatRotBulkPage(c tele.Context) error {
	pageStr := extractParam(c)
	page, _ := strconv.Atoi(pageStr)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotBulkPick {
		return h.handleRotatorBulkTypeKlikcepat(c)
	}
	sess.Data[klikcepatBulkPageKey] = strconv.Itoa(page)
	h.sessions.Set(c.Sender().ID, sess)
	picks, err := h.fetchKlikcepatBulkPicks()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	selected := parseSelectedInts(sess.Data[klikcepatBulkSelKey])
	return h.renderKlikcepatBulkPicker(c, picks, selected, page)
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
	page, _ := strconv.Atoi(sess.Data[klikcepatBulkPageKey])
	return h.renderKlikcepatBulkPicker(c, picks, selected, page)
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
		if !selected[int(l.ID)] {
			continue
		}
		// Auto-generate label: TYPE-SLUG (uppercased). Truncate if too long.
		label := strings.ToUpper(l.Type) + "-" + strings.ToUpper(l.URL)
		if len(label) > 40 {
			label = label[:40]
		}
		rot := store.KlikcepatRotator{
			Label:     label,
			LinkID:    int(l.ID),
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
