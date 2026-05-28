package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/klikcepat"
	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Rotator wizard ────────────────────────────────────────────────
//
// Called from Auto Rotator → Setup Rotator → 🔗 KLIKCEPAT type.
// Flow: pick link → pick pool → input label → save to KlikcepatRotatorStore.

const klikcepatRotatorPerPage = 10

// handleRotatorAddTypeKlikcepat — after KLIKCEPAT picked, show subtype picker:
// BIOLINK (block rotator) vs SHORTLINK (link location_url rotator).
func (h *Handler) handleRotatorAddTypeKlikcepat(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📄 BIOLINK", cbRotatorAddTypeKlcBiolink),
			m.Data("🔗 SHORTLINK", cbRotatorAddTypeKlcShortlink),
		),
		m.Row(m.Data("❌ Batal", cbRotator)),
	)
	return c.Edit(
		"🔄 *Setup Klikcepat Rotator — Pilih Tipe*\n\n"+
			"• 📄 *BIOLINK* — rotate destination button di dalem biolink (LOGIN, DAFTAR, dll)\n"+
			"• 🔗 *SHORTLINK* — rotate destination dari shortlink (klikcepat.com/slug → external)",
		m, tele.ModeMarkdown)
}

// handleRotatorAddTypeKlcShortlink — pick shortlink (existing flow, dengan full URL display).
func (h *Handler) handleRotatorAddTypeKlcShortlink(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat*.",
			backToRotator(), tele.ModeMarkdown)
	}

	// Parse page param
	pageStr := extractParam(c)
	page := 0
	if pageStr != "" {
		page, _ = strconv.Atoi(pageStr)
	}

	c.Edit("⏳ Loading shortlinks dari klikcepat...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("link")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	// Filter: skip links yang udah punya rotator + cuma type="link" (sudah di-filter di API call)
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatRotators.GetAll() {
		hasRotator[rot.LinkID] = true
	}
	var picks []klikcepat.Link
	for _, l := range links {
		if hasRotator[int(l.ID)] {
			continue
		}
		picks = append(picks, l)
	}
	if len(picks) == 0 {
		return c.Edit(
			"✅ Semua shortlink klikcepat udah punya rotator.\n\n"+
				"Hapus rotator lama via *📋 List Rotator* atau create shortlink baru via *🔗 KLIKCEPAT → ➕ Tambah Link*.",
			backToRotator(), tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()

	// Pagination
	total := len(picks)
	totalPages := (total + klikcepatRotatorPerPage - 1) / klikcepatRotatorPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * klikcepatRotatorPerPage
	end := start + klikcepatRotatorPerPage
	if end > total {
		end = total
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := start; i < end; i++ {
		p := picks[i]
		fullURL := klikcepat.BuildShortlinkURL(p, userMap, nil)
		display := strings.TrimPrefix(fullURL, "https://")
		display = strings.TrimPrefix(display, "http://")
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🔗 %s", truncate(display, 45)),
			cbKlikcepatRotPickLink, strconv.Itoa(int(p.ID)))))
	}

	// Pagination row
	var navRow tele.Row
	if page > 0 {
		navRow = append(navRow, m.Data("⬅️ Prev", cbRotatorAddTypeKlcShortlink, strconv.Itoa(page-1)))
	}
	navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
	if page < totalPages-1 {
		navRow = append(navRow, m.Data("Next ➡️", cbRotatorAddTypeKlcShortlink, strconv.Itoa(page+1)))
	}
	rows = append(rows, navRow)
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	text := fmt.Sprintf(
		"🔗 *Setup Shortlink Rotator — Step 1/3: Pick Shortlink*\n\n"+
			"Page %d/%d • Total %d shortlink belum rotator",
		page+1, totalPages, total)
	return c.Edit(text, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatRotPickLink(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return h.handleRotatorAddTypeKlikcepat(c)
	}
	link, err := h.klikcepat.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch link:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	// Pick pool label
	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit(
			"⚠️ Belum ada pool di Monitor. Add domain dulu via *📡 Monitor → ➕ Add Domain*.",
			backToRotator(), tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()
	fullURL := klikcepat.BuildShortlinkURL(*link, userMap, nil)

	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlikcepatRotatorPickPool,
		Data: map[string]string{
			"link_id":   linkIDStr,
			"link_url":  link.URL,
			"link_type": link.Type,
			"link_full": fullURL,
		},
		PromptMsg: c.Message(),
	})

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, lbl := range labels {
		domains := h.domains.GetByLabel(lbl)
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📂 %s (%d domain)", lbl, len(domains)),
			cbKlikcepatRotPickPool, lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	prompt := fmt.Sprintf(
		"🔗 *Setup Shortlink Rotator — Step 2/3: Pick Pool*\n\n"+
			"🔗 Link: `%s`\n"+
			"🎯 Current target: `%s`\n\n"+
			"Pilih pool domain (dari Monitor):",
		escapeMD(fullURL), escapeMD(link.LocationURL))
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatRotPickPool(c tele.Context) error {
	pool := extractParam(c)
	if pool == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Pool kosong", ShowAlert: true})
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatRotatorPickPool {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["pool"] = pool
	sess.Step = StepKlikcepatRotatorAddLabel
	h.sessions.Set(c.Sender().ID, sess)

	displayURL := sess.Data["link_full"]
	if displayURL == "" {
		displayURL = "/" + sess.Data["link_url"]
	}
	prompt := fmt.Sprintf(
		"🔗 *Step 3/3: Label Rotator*\n\n"+
			"🔗 Link: `%s`\n"+
			"📂 Pool: *%s*\n\n"+
			"Ketik label untuk rotator ini (bebas, untuk identifikasi):\n\n"+
			"*Contoh:* `PROMO-MAHA-ROT`",
		escapeMD(displayURL), escapeMD(pool))
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

// handleKlikcepatRotToggle — pause/resume klikcepat rotator from List Rotator.
func (h *Handler) handleKlikcepatRotToggle(c tele.Context) error {
	rotID := extractParam(c)
	active, found := h.klikcepatRotators.Toggle(rotID)
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

// handleKlikcepatRotDelete — hapus klikcepat rotator from List Rotator.
func (h *Handler) handleKlikcepatRotDelete(c tele.Context) error {
	rotID := extractParam(c)
	rot, ok := h.klikcepatRotators.GetByID(rotID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}
	h.klikcepatRotators.Delete(rotID)
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("🗑 %s dihapus", rot.Label)})
	return h.handleRotatorList(c)
}

func (h *Handler) wizardKlikcepatRotatorAddLabel(c tele.Context, sess *Session) error {
	h.showTyping(c)
	label := strings.ToUpper(strings.TrimSpace(c.Text()))
	if label == "" {
		return h.reply(c, "❌ Label kosong, coba lagi:", cancelMenu())
	}
	linkID, _ := strconv.Atoi(sess.Data["link_id"])
	pool := sess.Data["pool"]
	linkURL := sess.Data["link_url"]
	linkType := sess.Data["link_type"]
	linkFull := sess.Data["link_full"]
	h.sessions.Delete(c.Sender().ID)

	rot := store.KlikcepatRotator{
		Label:     label,
		LinkID:    linkID,
		LinkURL:   linkURL,
		LinkType:  linkType,
		PoolLabel: pool,
	}
	if err := h.klikcepatRotators.Add(rot); err != nil {
		return h.reply(c, fmt.Sprintf("❌ Gagal save rotator: %s", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}
	displayURL := linkFull
	if displayURL == "" {
		displayURL = "/" + linkURL
	}
	return h.reply(c,
		fmt.Sprintf(
			"✅ *Shortlink Rotator dibuat!*\n\n"+
				"📛 Label: *%s*\n"+
				"🔗 Link: `%s`\n"+
				"📂 Pool: *%s*\n"+
				"🟢 Active",
			label, escapeMD(displayURL), escapeMD(pool)),
		backToRotator(), tele.ModeMarkdown)
}
