package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/klikcepat"
	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Bulk Biolink Block Rotator ──────────────────────────────────
//
// Flow:
//   1. Bulk Setup → KLIKCEPAT → BIOLINK
//   2. Pick 1 biolink (paginated)
//   3. Multi-select blocks via checkbox
//   4. Pick pool
//   5. Save N rotators (1 per selected block) — auto-label BLK-{slug}-{block_name}

const klcBlockBulkSelKey = "klc_blk_selected"

type klcBlockBulkPick struct {
	ID   int
	Name string
	URL  string
}

// handleRotatorBulkTypeKlcBiolink — entry: list biolinks (paginated, full URL).
func (h *Handler) handleRotatorBulkTypeKlcBiolink(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}

	pageStr := extractParam(c)
	page := 0
	if pageStr != "" {
		page, _ = strconv.Atoi(pageStr)
	}

	c.Edit("⏳ Loading biolinks...", tele.ModeMarkdown)
	allLinks, err := h.klikcepat.ListLinks("biolink")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	// Client-side strict filter
	var links []klikcepat.Link
	for _, l := range allLinks {
		if l.Type == "biolink" {
			links = append(links, l)
		}
	}
	if len(links) == 0 {
		return c.Edit(
			"📭 *Belum ada biolink di klikcepat lo.*",
			backToRotator(), tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()

	const perPage = 10
	total := len(links)
	totalPages := (total + perPage - 1) / perPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * perPage
	end := start + perPage
	if end > total {
		end = total
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := start; i < end; i++ {
		l := links[i]
		fullURL := klikcepat.BuildShortlinkURL(l, userMap, nil)
		display := strings.TrimPrefix(fullURL, "https://")
		display = strings.TrimPrefix(display, "http://")
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📄 %s", truncate(display, 45)),
			cbKlcBlockBulkPickBiolink, strconv.Itoa(int(l.ID)))))
	}

	// Nav
	var navRow tele.Row
	if page > 0 {
		navRow = append(navRow, m.Data("⬅️ Prev", cbRotatorBulkTypeKlcBiolink, strconv.Itoa(page-1)))
	}
	navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
	if page < totalPages-1 {
		navRow = append(navRow, m.Data("Next ➡️", cbRotatorBulkTypeKlcBiolink, strconv.Itoa(page+1)))
	}
	rows = append(rows, navRow)
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit(fmt.Sprintf(
		"📦 *Bulk Block Rotator — Pick Biolink*\n\n"+
			"Page %d/%d • Total %d biolink\n\n"+
			"_Pilih 1 biolink dulu, terus multi-select block-nya._",
		page+1, totalPages, total),
		m, tele.ModeMarkdown)
}

// handleKlcBlockBulkPickBiolink — user picked biolink, show blocks checkbox picker.
func (h *Handler) handleKlcBlockBulkPickBiolink(c tele.Context) error {
	biolinkIDStr := extractParam(c)
	biolinkID, _ := strconv.Atoi(biolinkIDStr)
	if biolinkID <= 0 {
		return h.handleRotatorBulkTypeKlcBiolink(c)
	}

	c.Edit("⏳ Loading blocks...", tele.ModeMarkdown)
	biolink, err := h.klikcepat.GetLink(biolinkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch biolink:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	blocks, err := h.klikcepat.ListBiolinkBlocks(biolinkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch blocks:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatBlockRotators.GetAll() {
		hasRotator[rot.BlockID] = true
	}

	var picks []klcBlockBulkPick
	for _, b := range blocks {
		if b.Type != "link" || b.LocationURL == "" || hasRotator[int(b.ID)] {
			continue
		}
		picks = append(picks, klcBlockBulkPick{
			ID:   int(b.ID),
			Name: b.BlockName(),
			URL:  b.LocationURL,
		})
	}

	userMap := h.creds.GetKlikcepatDomainMap()
	biolinkFullURL := klikcepat.BuildShortlinkURL(*biolink, userMap, nil)

	if len(picks) == 0 {
		return c.Edit(
			fmt.Sprintf(
				"✅ *Semua block di biolink ini udah punya rotator.*\n\n"+
					"📄 Biolink: `%s`",
				escapeMD(biolinkFullURL)),
			backToRotator(), tele.ModeMarkdown)
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlcBlockBulkPick,
		Data: map[string]string{
			klcBlockBulkSelKey: "",
			"biolink_id":       biolinkIDStr,
			"biolink_slug":     biolink.URL,
			"biolink_domain":   strconv.Itoa(int(biolink.DomainID)),
			"biolink_url":      biolinkFullURL,
		},
	})

	return h.renderKlcBlockBulkPicker(c, picks, map[int]bool{})
}

func (h *Handler) renderKlcBlockBulkPicker(c tele.Context, picks []klcBlockBulkPick, selected map[int]bool) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok {
		return c.Edit("⚠️ Session expired", backToRotator(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📦 *Bulk Block Rotator — Pick Blocks*\n═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("📄 Biolink: `%s`\n", escapeMD(sess.Data["biolink_url"])))
	sb.WriteString(fmt.Sprintf("📊 *Dipilih:* %d / %d block\n\n", len(selected), len(picks)))

	if len(selected) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n*Block yang dipilih:*\n")
		for _, p := range picks {
			if selected[p.ID] {
				name := p.Name
				if name == "" {
					name = "(no name)"
				}
				sb.WriteString(fmt.Sprintf("✅ 🔘 *%s*\n", escapeMD(name)))
			}
		}
		sb.WriteString("\n💡 _Klik *Lanjut* untuk pilih pool._")
	} else {
		sb.WriteString("_(Belum ada yang dipilih — klik tombol di bawah.)_")
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range picks {
		check := "☐"
		if selected[p.ID] {
			check = "☑"
		}
		name := p.Name
		if name == "" {
			name = "(no name)"
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("%s 🔘 %s", check, truncate(name, 40)),
			cbKlcBlockBulkToggle, strconv.Itoa(p.ID))))
	}

	allSelected := len(selected) == len(picks) && len(picks) > 0
	if allSelected {
		rows = append(rows, m.Row(m.Data("☐ Hapus Semua", cbKlcBlockBulkSelNone)))
	} else {
		rows = append(rows, m.Row(m.Data("☑ Pilih Semua", cbKlcBlockBulkSelAll)))
	}
	if len(selected) > 0 {
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("✅ Lanjut (%d)", len(selected)), cbKlcBlockBulkProceed),
			m.Data("❌ Batal", cbRotator),
		))
	} else {
		rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotatorBulkTypeKlcBiolink)))
	}
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// loadKlcBlockBulkPicks reload blocks (used by toggle/select-all/proceed)
func (h *Handler) loadKlcBlockBulkPicks(biolinkID int) ([]klcBlockBulkPick, error) {
	blocks, err := h.klikcepat.ListBiolinkBlocks(biolinkID)
	if err != nil {
		return nil, err
	}
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatBlockRotators.GetAll() {
		hasRotator[rot.BlockID] = true
	}
	var picks []klcBlockBulkPick
	for _, b := range blocks {
		if b.Type != "link" || b.LocationURL == "" || hasRotator[int(b.ID)] {
			continue
		}
		picks = append(picks, klcBlockBulkPick{
			ID:   int(b.ID),
			Name: b.BlockName(),
			URL:  b.LocationURL,
		})
	}
	return picks, nil
}

func (h *Handler) handleKlcBlockBulkToggle(c tele.Context) error {
	blockIDStr := extractParam(c)
	blockID, _ := strconv.Atoi(blockIDStr)
	if blockID == 0 {
		return nil
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlcBlockBulkPick {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	selected := parseSelectedInts(sess.Data[klcBlockBulkSelKey])
	if selected[blockID] {
		delete(selected, blockID)
	} else {
		selected[blockID] = true
	}
	sess.Data[klcBlockBulkSelKey] = serializeSelectedInts(selected)
	h.sessions.Set(c.Sender().ID, sess)

	biolinkID, _ := strconv.Atoi(sess.Data["biolink_id"])
	picks, err := h.loadKlcBlockBulkPicks(biolinkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	return h.renderKlcBlockBulkPicker(c, picks, selected)
}

func (h *Handler) handleKlcBlockBulkSelectAll(c tele.Context, selectAll bool) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlcBlockBulkPick {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	biolinkID, _ := strconv.Atoi(sess.Data["biolink_id"])
	picks, err := h.loadKlcBlockBulkPicks(biolinkID)
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
	sess.Data[klcBlockBulkSelKey] = serializeSelectedInts(selected)
	h.sessions.Set(c.Sender().ID, sess)
	return h.renderKlcBlockBulkPicker(c, picks, selected)
}

func (h *Handler) handleKlcBlockBulkProceed(c tele.Context) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlcBlockBulkPick {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	selected := parseSelectedInts(sess.Data[klcBlockBulkSelKey])
	if len(selected) == 0 {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Belum pilih block!", ShowAlert: true})
	}

	sess.Step = StepKlcBlockBulkPool
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
			cbKlcBlockBulkPickPool, lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit(
		fmt.Sprintf(
			"📦 *Bulk Block — Pick Pool*\n\n"+
				"📄 Biolink: `%s`\n"+
				"✅ %d block dipilih\n\n"+
				"Pilih pool yang akan di-assign ke SEMUA block tersebut:",
			escapeMD(sess.Data["biolink_url"]), len(selected)),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleKlcBlockBulkPickPool(c tele.Context) error {
	pool := extractParam(c)
	if pool == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Pool kosong", ShowAlert: true})
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlcBlockBulkPool {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	selected := parseSelectedInts(sess.Data[klcBlockBulkSelKey])
	if len(selected) == 0 {
		return c.Edit("⚠️ No selection", backToRotator(), tele.ModeMarkdown)
	}

	// Switch to label-prompt step
	sess.Step = StepKlcBlockBulkLabel
	sess.Data["pool"] = pool
	sess.PromptMsg = c.Message()
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"📦 *Bulk Block — Step Akhir: Label Prefix*\n\n"+
			"📄 Biolink: `%s`\n"+
			"📂 Pool: *%s*\n"+
			"✅ %d block dipilih\n\n"+
			"Ketik *prefix label* buat tandain semua rotator ini.\n"+
			"Label final: `{PREFIX}-{BLOCK_NAME}`\n\n"+
			"*Contoh:* `MAHA` → jadi `MAHA-LOGIN`, `MAHA-DAFTAR`",
		escapeMD(sess.Data["biolink_url"]), escapeMD(pool), len(selected))
	return c.Edit(prompt, cancelMenu(), tele.ModeMarkdown)
}

// wizardKlcBlockBulkLabel — user typed label prefix, save N rotators.
func (h *Handler) wizardKlcBlockBulkLabel(c tele.Context, sess *Session) error {
	h.showTyping(c)
	prefix := strings.ToUpper(strings.TrimSpace(c.Text()))
	if prefix == "" {
		return h.reply(c, "❌ Prefix kosong, coba lagi:", cancelMenu())
	}

	selected := parseSelectedInts(sess.Data[klcBlockBulkSelKey])
	if len(selected) == 0 {
		h.sessions.Delete(c.Sender().ID)
		return h.reply(c, "⚠️ No selection", backToRotator(), tele.ModeMarkdown)
	}

	pool := sess.Data["pool"]
	biolinkID, _ := strconv.Atoi(sess.Data["biolink_id"])
	biolinkSlug := sess.Data["biolink_slug"]
	biolinkDomain, _ := strconv.Atoi(sess.Data["biolink_domain"])
	biolinkURL := sess.Data["biolink_url"]
	h.sessions.Delete(c.Sender().ID)

	blocks, err := h.klikcepat.ListBiolinkBlocks(biolinkID)
	if err != nil {
		return h.reply(c, fmt.Sprintf("❌ Gagal fetch blocks: %s", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	created := 0
	skipped := 0
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 *Bulk Block Rotator — Result*\n\n📄 Biolink: `%s`\n📂 Pool: *%s*\n\n",
		escapeMD(biolinkURL), escapeMD(pool)))

	for _, b := range blocks {
		if !selected[int(b.ID)] {
			continue
		}
		blockName := b.BlockName()
		if blockName == "" {
			blockName = fmt.Sprintf("BLK-%d", b.ID)
		}
		// Label: {PREFIX}-{BLOCK_NAME}
		safeName := strings.ToUpper(strings.ReplaceAll(blockName, " ", ""))
		label := prefix + "-" + safeName
		if len(label) > 40 {
			label = label[:40]
		}
		rot := store.KlikcepatBlockRotator{
			Label:         label,
			BlockID:       int(b.ID),
			BiolinkID:     biolinkID,
			BiolinkSlug:   biolinkSlug,
			BiolinkDomain: biolinkDomain,
			BlockName:     blockName,
			PoolLabel:     pool,
		}
		if err := h.klikcepatBlockRotators.Add(rot); err != nil {
			sb.WriteString(fmt.Sprintf("⚠️ `%s` skipped: %s\n", escapeMD(blockName), escapeMD(err.Error())))
			skipped++
			continue
		}
		sb.WriteString(fmt.Sprintf("✅ 🔘 `%s` → label `%s`\n", escapeMD(blockName), escapeMD(label)))
		created++
	}
	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n*Total:* %d created, %d skipped", created, skipped))
	return h.reply(c, sb.String(), backToRotator(), tele.ModeMarkdown)
}
