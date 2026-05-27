package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Rotator wizard ────────────────────────────────────────────────
//
// Called from Auto Rotator → Setup Rotator → 🔗 KLIKCEPAT type.
// Flow: pick link → pick pool → input label → save to KlikcepatRotatorStore.

const klikcepatRotatorPerPage = 10

// handleRotatorAddTypeKlikcepat — entry from Auto Rotator menu after user picks "KLIKCEPAT".
// Param: page index (default 0).
func (h *Handler) handleRotatorAddTypeKlikcepat(c tele.Context) error {
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

	c.Edit("⏳ Loading links dari klikcepat...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToRotator(), tele.ModeMarkdown)
	}

	// Filter: skip links yang udah punya rotator + skip non-rotatable types
	hasRotator := make(map[int]bool)
	for _, rot := range h.klikcepatRotators.GetAll() {
		hasRotator[rot.LinkID] = true
	}
	type pick struct {
		ID    int
		URL   string
		Type  string
		Title string
	}
	var picks []pick
	for _, l := range links {
		if hasRotator[int(l.ID)] {
			continue
		}
		if l.Type != "link" && l.Type != "biolink" {
			continue
		}
		picks = append(picks, pick{int(l.ID), l.URL, l.Type, l.Title})
	}
	if len(picks) == 0 {
		return c.Edit(
			"✅ Semua link klikcepat udah punya rotator (atau gak ada link tipe link/biolink).\n\n"+
				"Hapus rotator lama via *📋 List Rotator* atau create link baru via *🔗 KLIKCEPAT → ➕ Tambah Link*.",
			backToRotator(), tele.ModeMarkdown)
	}

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
		typeIcon := "🔗"
		if p.Type == "biolink" {
			typeIcon = "📄"
		}
		// Label: TYPE_ICON + UPPERCASE_SLUG (same as List Link display)
		label := strings.ToUpper(p.URL)
		if label == "" {
			label = "(no slug)"
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("%s %s", typeIcon, truncate(label, 40)),
			cbKlikcepatRotPickLink, strconv.Itoa(p.ID))))
	}

	// Pagination row
	var navRow tele.Row
	if page > 0 {
		navRow = append(navRow, m.Data("⬅️ Prev", cbRotatorAddTypeKlikcepat, strconv.Itoa(page-1)))
	}
	navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
	if page < totalPages-1 {
		navRow = append(navRow, m.Data("Next ➡️", cbRotatorAddTypeKlikcepat, strconv.Itoa(page+1)))
	}
	rows = append(rows, navRow)
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	text := fmt.Sprintf(
		"🔄 *Setup Klikcepat Rotator — Step 1/3: Pick Link*\n\n"+
			"Page %d/%d • Total %d link belum rotator",
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

	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepKlikcepatRotatorPickPool,
		Data: map[string]string{
			"link_id":   linkIDStr,
			"link_url":  link.URL,
			"link_type": link.Type,
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
		"🔄 *Setup Klikcepat Rotator — Step 2/3: Pick Pool*\n\n"+
			"🔗 Link: `/%s` (%s)\n"+
			"🎯 Current target: `%s`\n\n"+
			"Pilih pool domain (dari Monitor):",
		escapeMD(link.URL), link.Type, escapeMD(link.LocationURL))
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

	prompt := fmt.Sprintf(
		"🔄 *Step 3/3: Label Rotator*\n\n"+
			"🔗 Link: `/%s`\n"+
			"📂 Pool: *%s*\n\n"+
			"Ketik label untuk rotator ini (bebas, untuk identifikasi):\n\n"+
			"*Contoh:* `PROMO-MAHA-ROT`",
		escapeMD(sess.Data["link_url"]), escapeMD(pool))
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
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
	return h.reply(c,
		fmt.Sprintf(
			"✅ *Klikcepat Rotator dibuat!*\n\n"+
				"📛 Label: *%s*\n"+
				"🔗 Link: `/%s`\n"+
				"📂 Pool: *%s*\n"+
				"🟢 Active",
			label, escapeMD(linkURL), escapeMD(pool)),
		backToRotator(), tele.ModeMarkdown)
}
