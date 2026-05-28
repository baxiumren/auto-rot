package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/klikcepat"
	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Biolink Block Rotator wizard ──────────────────────────────────
//
// Flow:
//   1. Auto Rotator → Setup Rotator → 🔗 KLIKCEPAT → 📄 BIOLINK
//   2. Pick biolink (paginated, label = full URL klikcepat.lat/slug)
//   3. Pick block (LOGIN/DAFTAR/dll — hanya block type=link yg punya location_url)
//   4. Pick pool
//   5. Input label → save

const klcBlockRotatorPerPage = 10

// handleRotatorAddTypeKlcBiolink — list biolinks (paginated) untuk dipilih.
func (h *Handler) handleRotatorAddTypeKlcBiolink(c tele.Context) error {
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
	// Client-side filter — Pixly API `?type=` filter sering di-ignore
	var links []klikcepat.Link
	for _, l := range allLinks {
		if l.Type == "biolink" {
			links = append(links, l)
		}
	}
	if len(links) == 0 {
		return c.Edit(
			"📭 *Belum ada biolink di klikcepat lo.*\n\nBuat dulu lewat klikcepat web UI.",
			backToRotator(), tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()

	// Pagination
	total := len(links)
	totalPages := (total + klcBlockRotatorPerPage - 1) / klcBlockRotatorPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * klcBlockRotatorPerPage
	end := start + klcBlockRotatorPerPage
	if end > total {
		end = total
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := start; i < end; i++ {
		l := links[i]
		fullURL := klikcepat.BuildShortlinkURL(l, userMap, nil)
		// Strip "https://" untuk hemat space tombol
		display := strings.TrimPrefix(fullURL, "https://")
		display = strings.TrimPrefix(display, "http://")
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📄 %s", truncate(display, 45)),
			cbKlcBlockRotPickBiolink, strconv.Itoa(int(l.ID)))))
	}

	// Nav row
	var navRow tele.Row
	if page > 0 {
		navRow = append(navRow, m.Data("⬅️ Prev", cbRotatorAddTypeKlcBiolink, strconv.Itoa(page-1)))
	}
	navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
	if page < totalPages-1 {
		navRow = append(navRow, m.Data("Next ➡️", cbRotatorAddTypeKlcBiolink, strconv.Itoa(page+1)))
	}
	rows = append(rows, navRow)
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	text := fmt.Sprintf(
		"📄 *Setup Biolink Block Rotator — Step 1/4: Pick Biolink*\n\n"+
			"Page %d/%d • Total %d biolink",
		page+1, totalPages, total)
	return c.Edit(text, m, tele.ModeMarkdown)
}

// handleKlcBlockRotPickBiolink — user picked biolink, show blocks within it.
func (h *Handler) handleKlcBlockRotPickBiolink(c tele.Context) error {
	biolinkIDStr := extractParam(c)
	biolinkID, _ := strconv.Atoi(biolinkIDStr)
	if biolinkID <= 0 {
		return h.handleRotatorAddTypeKlcBiolink(c)
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

	// Filter: hanya block type=link yg punya location_url + belum punya rotator
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatBlockRotators.GetAll() {
		hasRotator[rot.BlockID] = true
	}
	type pick struct {
		ID   int
		Name string
		URL  string
	}
	var picks []pick
	for _, b := range blocks {
		if b.Type != "link" {
			continue
		}
		if b.LocationURL == "" {
			continue
		}
		if hasRotator[int(b.ID)] {
			continue
		}
		picks = append(picks, pick{int(b.ID), b.BlockName(), b.LocationURL})
	}

	userMap := h.creds.GetKlikcepatDomainMap()
	biolinkFullURL := klikcepat.BuildShortlinkURL(*biolink, userMap, nil)

	if len(picks) == 0 {
		return c.Edit(
			fmt.Sprintf(
				"✅ *Semua block di biolink ini udah punya rotator.*\n\n"+
					"📄 Biolink: `%s`\n\n"+
					"Hapus rotator lama via *📋 List Rotator* atau pilih biolink lain.",
				escapeMD(biolinkFullURL)),
			backToRotator(), tele.ModeMarkdown)
	}

	// Store biolink context di session
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlcBlockRotPickBlock,
		Data: map[string]string{
			"biolink_id":     biolinkIDStr,
			"biolink_slug":   biolink.URL,
			"biolink_domain": strconv.Itoa(int(biolink.DomainID)),
			"biolink_url":    biolinkFullURL,
		},
		PromptMsg: c.Message(),
	})

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range picks {
		name := p.Name
		if name == "" {
			name = "(no name)"
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🔘 %s", truncate(name, 40)),
			cbKlcBlockRotPickBlock, strconv.Itoa(p.ID))))
	}
	rows = append(rows, m.Row(m.Data("⬅️ Back", cbRotatorAddTypeKlcBiolink)))
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	text := fmt.Sprintf(
		"📄 *Step 2/4: Pick Block*\n\n"+
			"📄 Biolink: `%s`\n\n"+
			"Pilih block (tombol) yg mau di-auto-swap:\n"+
			"_Hanya block type \"link\" yg muncul (yg punya destination URL)._",
		escapeMD(biolinkFullURL))
	return c.Edit(text, m, tele.ModeMarkdown)
}

// handleKlcBlockRotPickBlock — user picked block, show pool picker.
func (h *Handler) handleKlcBlockRotPickBlock(c tele.Context) error {
	blockIDStr := extractParam(c)
	blockID, _ := strconv.Atoi(blockIDStr)
	if blockID <= 0 {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Invalid block", ShowAlert: true})
	}

	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlcBlockRotPickBlock {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}

	// Fetch block details
	block, err := h.klikcepat.GetBiolinkBlock(blockID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch block:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit(
			"⚠️ Belum ada pool di Monitor. Add domain dulu via *📡 Monitor → ➕ Add Domain*.",
			backToRotator(), tele.ModeMarkdown)
	}

	sess.Data["block_id"] = blockIDStr
	sess.Data["block_name"] = block.BlockName()
	sess.Data["block_url"] = block.LocationURL
	sess.Step = StepKlcBlockRotPickPool
	h.sessions.Set(c.Sender().ID, sess)

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, lbl := range labels {
		domains := h.domains.GetByLabel(lbl)
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📂 %s (%d domain)", lbl, len(domains)),
			cbKlcBlockRotPickPool, lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	blockName := block.BlockName()
	if blockName == "" {
		blockName = "(no name)"
	}
	text := fmt.Sprintf(
		"📄 *Step 3/4: Pick Pool*\n\n"+
			"📄 Biolink: `%s`\n"+
			"🔘 Block: *%s*\n"+
			"🎯 Current target: `%s`\n\n"+
			"Pilih pool domain (untuk swap kalau target keblock):",
		escapeMD(sess.Data["biolink_url"]), escapeMD(blockName), escapeMD(block.LocationURL))
	return c.Edit(text, m, tele.ModeMarkdown)
}

// handleKlcBlockRotPickPool — user picked pool, ask for label.
func (h *Handler) handleKlcBlockRotPickPool(c tele.Context) error {
	pool := extractParam(c)
	if pool == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Pool kosong", ShowAlert: true})
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlcBlockRotPickPool {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["pool"] = pool
	sess.Step = StepKlcBlockRotLabel
	h.sessions.Set(c.Sender().ID, sess)

	blockName := sess.Data["block_name"]
	if blockName == "" {
		blockName = "(no name)"
	}
	prompt := fmt.Sprintf(
		"📄 *Step 4/4: Label Rotator*\n\n"+
			"📄 Biolink: `%s`\n"+
			"🔘 Block: *%s*\n"+
			"📂 Pool: *%s*\n\n"+
			"Ketik label untuk rotator ini:\n\n"+
			"*Contoh:* `BLK-MAHA-LOGIN`",
		escapeMD(sess.Data["biolink_url"]), escapeMD(blockName), escapeMD(pool))
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

// wizardKlcBlockRotLabel — user typed label, save to store.
func (h *Handler) wizardKlcBlockRotLabel(c tele.Context, sess *Session) error {
	h.showTyping(c)
	label := strings.ToUpper(strings.TrimSpace(c.Text()))
	if label == "" {
		return h.reply(c, "❌ Label kosong, coba lagi:", cancelMenu())
	}
	blockID, _ := strconv.Atoi(sess.Data["block_id"])
	biolinkID, _ := strconv.Atoi(sess.Data["biolink_id"])
	biolinkDomain, _ := strconv.Atoi(sess.Data["biolink_domain"])
	pool := sess.Data["pool"]
	biolinkSlug := sess.Data["biolink_slug"]
	blockName := sess.Data["block_name"]
	biolinkURL := sess.Data["biolink_url"]
	h.sessions.Delete(c.Sender().ID)

	rot := store.KlikcepatBlockRotator{
		Label:         label,
		BlockID:       blockID,
		BiolinkID:     biolinkID,
		BiolinkSlug:   biolinkSlug,
		BiolinkDomain: biolinkDomain,
		BlockName:     blockName,
		PoolLabel:     pool,
	}
	if err := h.klikcepatBlockRotators.Add(rot); err != nil {
		return h.reply(c, fmt.Sprintf("❌ Gagal save rotator: %s", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	displayBlockName := blockName
	if displayBlockName == "" {
		displayBlockName = "(no name)"
	}
	return h.reply(c,
		fmt.Sprintf(
			"✅ *Biolink Block Rotator dibuat!*\n\n"+
				"📛 Label: *%s*\n"+
				"📄 Biolink: `%s`\n"+
				"🔘 Block: *%s*\n"+
				"📂 Pool: *%s*\n"+
				"🟢 Active",
			label, escapeMD(biolinkURL), escapeMD(displayBlockName), escapeMD(pool)),
		backToRotator(), tele.ModeMarkdown)
}

// handleKlcBlockRotToggle — pause/resume block rotator.
func (h *Handler) handleKlcBlockRotToggle(c tele.Context) error {
	rotID := extractParam(c)
	active, found := h.klikcepatBlockRotators.Toggle(rotID)
	if !found {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}
	state := "▶️ AKTIF"
	if !active {
		state = "⏸ PAUSE"
	}
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Rotator → %s", state)})
	return h.handleRotatorList(c)
}

// handleKlcBlockRotDelete — delete block rotator.
func (h *Handler) handleKlcBlockRotDelete(c tele.Context) error {
	rotID := extractParam(c)
	rot, ok := h.klikcepatBlockRotators.GetByID(rotID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}
	h.klikcepatBlockRotators.Delete(rotID)
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("🗑 %s dihapus", rot.Label)})
	return h.handleRotatorList(c)
}
