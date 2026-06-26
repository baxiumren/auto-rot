package bot

import (
	"fmt"
	"strconv"
	"strings"

	"bongbot/klikcepat" // Pixly client (generic)

	tele "gopkg.in/telebot.v3"
)

// ─── LinkFB Link CRUD ─────────────────────────────────────────────────────────
//
// Scope: shortlink only (Type="link"). No biolink edit.
// Pixly engine = sama dengan klikcepat, instance terpisah.

const linkfbPerPage = 10

// buildLinkfbFullURL — same logic as klikcepat but uses linkfb domain map.
func (h *Handler) buildLinkfbFullURL(l klikcepat.Link) string {
	userMap := h.creds.GetLinkfbDomainMap()
	if host, ok := userMap[int(l.DomainID)]; ok && host != "" {
		return fmt.Sprintf("https://%s/%s", host, l.URL)
	}
	return fmt.Sprintf("https://linkfb.io/%s", l.URL)
}

// ─── List Link ───────────────────────────────────────────────────────────────

func (h *Handler) handleLinkfbList(c tele.Context) error {
	// Parse "page" param (default 0)
	param := extractParam(c)
	page := 0
	if param != "" {
		// Format: "page" (just page int, since linkfb is shortlink-only)
		page, _ = strconv.Atoi(param)
	}

	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.linkfb.ListLinks("link")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfb(), tele.ModeMarkdown)
	}

	// Filter client-side
	var shortlinks []klikcepat.Link
	for _, l := range links {
		if l.Type == "link" {
			shortlinks = append(shortlinks, l)
		}
	}

	if len(shortlinks) == 0 {
		return c.Edit(
			"💎 *L I N K F B   S H O R T L I N K* 💎\n"+
				"|\n"+
				"📭 *EMPTY*\n"+
				"└ Belum ada shortlink\n"+
				"|\n"+
				"🎯 *ACTION*\n"+
				"└ Klik ➕ Tambah Link buat bikin",
			backToLinkfb(), tele.ModeMarkdown)
	}

	total := len(shortlinks)
	totalPages := (total + linkfbPerPage - 1) / linkfbPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * linkfbPerPage
	end := start + linkfbPerPage
	if end > total {
		end = total
	}

	var sb strings.Builder
	sb.WriteString("💎 *L I N K F B   S H O R T L I N K* 💎\n")
	sb.WriteString("|\n")
	sb.WriteString(fmt.Sprintf("📊 *STATISTIK*\n└ Total : %d shortlink\n└ Page  : %d/%d\n",
		total, page+1, totalPages))
	sb.WriteString("|\n")

	for i := start; i < end; i++ {
		l := shortlinks[i]
		fullURL := h.buildLinkfbFullURL(l)
		title := l.Title
		if title == "" {
			title = "(no title)"
		}
		sb.WriteString(fmt.Sprintf("🔗 *%s*\n", escapeMD(title)))
		sb.WriteString(fmt.Sprintf("└ URL    : `%s`\n", fullURL))
		sb.WriteString(fmt.Sprintf("└ Target : `%s`\n", l.LocationURL))
		sb.WriteString("|\n")
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Pagination
	if totalPages > 1 {
		var navRow tele.Row
		if page > 0 {
			navRow = append(navRow, m.Data("⬅️ Prev", cbLinkfbList, strconv.Itoa(page-1)))
		}
		navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
		if page < totalPages-1 {
			navRow = append(navRow, m.Data("Next ➡️", cbLinkfbList, strconv.Itoa(page+1)))
		}
		rows = append(rows, navRow)
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbLinkfb)))
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// ─── Edit Link ───────────────────────────────────────────────────────────────

func (h *Handler) handleLinkfbEdit(c tele.Context) error {
	pageStr := extractParam(c)
	page, _ := strconv.Atoi(pageStr)

	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.linkfb.ListLinks("link")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfb(), tele.ModeMarkdown)
	}

	var shortlinks []klikcepat.Link
	for _, l := range links {
		if l.Type == "link" {
			shortlinks = append(shortlinks, l)
		}
	}

	if len(shortlinks) == 0 {
		return c.Edit(
			"💎 *E D I T   L I N K* 💎\n"+
				"|\n"+
				"📭 Belum ada shortlink buat di-edit",
			backToLinkfb(), tele.ModeMarkdown)
	}

	total := len(shortlinks)
	totalPages := (total + linkfbPerPage - 1) / linkfbPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * linkfbPerPage
	end := start + linkfbPerPage
	if end > total {
		end = total
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := start; i < end; i++ {
		l := shortlinks[i]
		fullURL := h.buildLinkfbFullURL(l)
		display := strings.TrimPrefix(fullURL, "https://")
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🔗 %s", truncate(display, 45)),
			cbLinkfbEditPick, strconv.Itoa(int(l.ID)))))
	}

	// Pagination
	if totalPages > 1 {
		var navRow tele.Row
		if page > 0 {
			navRow = append(navRow, m.Data("⬅️ Prev", cbLinkfbEdit, strconv.Itoa(page-1)))
		}
		navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
		if page < totalPages-1 {
			navRow = append(navRow, m.Data("Next ➡️", cbLinkfbEdit, strconv.Itoa(page+1)))
		}
		rows = append(rows, navRow)
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbLinkfb)))
	m.Inline(rows...)

	return c.Edit(
		fmt.Sprintf("💎 *E D I T   L I N K* 💎\n"+
			"|\n"+
			"📊 Page %d/%d • Total %d link\n"+
			"|\n"+
			"✏️ Pilih shortlink yg mau di-edit 👇",
			page+1, totalPages, total),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbEditPick(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return h.handleLinkfbEdit(c)
	}
	link, err := h.linkfb.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfb(), tele.ModeMarkdown)
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepLinkfbEditPickField,
		Data:      map[string]string{"link_id": linkIDStr},
		PromptMsg: c.Message(),
	})

	prompt := fmt.Sprintf(
		"💎 *E D I T   L I N K* 💎\n"+
			"|\n"+
			"📋 *INFO LINK*\n"+
			"└ 📛 Title  : *%s*\n"+
			"└ 🔗 Slug   : `%s`\n"+
			"└ 🎯 Target : `%s`\n"+
			"|\n"+
			"🎯 *PILIH FIELD YG MAU DI-EDIT*\n"+
			"└ 📛 Title — nama display\n"+
			"└ 🔗 Slug — bagian URL\n"+
			"└ 🎯 Location URL — destination redirect",
		escapeMD(link.Title), link.URL, link.LocationURL)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📛 Title", cbLinkfbEditField, "title"),
			m.Data("🔗 Slug", cbLinkfbEditField, "url"),
		),
		m.Row(m.Data("🎯 Location URL", cbLinkfbEditField, "location_url")),
		m.Row(m.Data("❌ Batal", cbCancel)),
	)
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbEditField(c tele.Context) error {
	field := extractParam(c)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepLinkfbEditPickField {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["field"] = field
	sess.Step = StepLinkfbEditValue
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"💎 *E D I T   L I N K* 💎\n"+
			"|\n"+
			"📝 *INPUT VALUE BARU*\n"+
			"└ Field : *%s*\n"+
			"└ Ketik nilai baru",
		field)
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardLinkfbEditValue(c tele.Context, sess *Session) error {
	h.showTyping(c)
	val := strings.TrimSpace(c.Text())
	if val == "" {
		return h.reply(c, "❌ Nilai kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	linkID, _ := strconv.Atoi(sess.Data["link_id"])
	field := sess.Data["field"]
	h.sessions.Delete(c.Sender().ID)

	if field == "location_url" {
		if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
			val = "https://" + val
		}
	}

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Updating link...", tele.ModeMarkdown)

	_, err := h.linkfb.UpdateLink(linkID, map[string]string{field: val})
	if err != nil {
		errText := fmt.Sprintf("❌ *Update gagal*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToLinkfb(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToLinkfb(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf(
		"💎 *E D I T   L I N K   ✅* 💎\n"+
			"|\n"+
			"✅ *SUCCESS*\n"+
			"└ Link ID : %d\n"+
			"└ Field   : *%s*\n"+
			"└ Value   : `%s`",
		linkID, field, val)
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToLinkfb(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToLinkfb(), tele.ModeMarkdown)
}

// ─── Delete Link ─────────────────────────────────────────────────────────────

func (h *Handler) handleLinkfbDelete(c tele.Context) error {
	pageStr := extractParam(c)
	page, _ := strconv.Atoi(pageStr)

	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.linkfb.ListLinks("link")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfb(), tele.ModeMarkdown)
	}

	var shortlinks []klikcepat.Link
	for _, l := range links {
		if l.Type == "link" {
			shortlinks = append(shortlinks, l)
		}
	}

	if len(shortlinks) == 0 {
		return c.Edit("📭 Belum ada link buat dihapus.", backToLinkfb(), tele.ModeMarkdown)
	}

	total := len(shortlinks)
	totalPages := (total + linkfbPerPage - 1) / linkfbPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * linkfbPerPage
	end := start + linkfbPerPage
	if end > total {
		end = total
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := start; i < end; i++ {
		l := shortlinks[i]
		fullURL := h.buildLinkfbFullURL(l)
		display := strings.TrimPrefix(fullURL, "https://")
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 %s", truncate(display, 45)),
			cbLinkfbDeletePick, strconv.Itoa(int(l.ID)))))
	}

	if totalPages > 1 {
		var navRow tele.Row
		if page > 0 {
			navRow = append(navRow, m.Data("⬅️ Prev", cbLinkfbDelete, strconv.Itoa(page-1)))
		}
		navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
		if page < totalPages-1 {
			navRow = append(navRow, m.Data("Next ➡️", cbLinkfbDelete, strconv.Itoa(page+1)))
		}
		rows = append(rows, navRow)
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbLinkfb)))
	m.Inline(rows...)

	return c.Edit(
		fmt.Sprintf("💎 *H A P U S   L I N K* 💎\n"+
			"|\n"+
			"📊 Page %d/%d • Total %d link\n"+
			"|\n"+
			"🗑 Pilih link yg mau dihapus 👇\n"+
			"|\n"+
			"⚠️ Action ini PERMANENT",
			page+1, totalPages, total),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbDeletePick(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return h.handleLinkfbDelete(c)
	}
	link, err := h.linkfb.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfb(), tele.ModeMarkdown)
	}

	prompt := fmt.Sprintf(
		"💎 *H A P U S   L I N K* 💎\n"+
			"|\n"+
			"📋 *INFO LINK*\n"+
			"└ 📛 Title  : *%s*\n"+
			"└ 🔗 Slug   : `%s`\n"+
			"└ 🎯 Target : `%s`\n"+
			"|\n"+
			"⚠️ *KONFIRMASI*\n"+
			"└ Yakin mau hapus?\n"+
			"└ Action ini *PERMANENT* — gak bisa di-undo",
		escapeMD(link.Title), link.URL, link.LocationURL)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🗑 Ya, Hapus", cbLinkfbDeleteConfirm, linkIDStr),
			m.Data("❌ Batal", cbLinkfb),
		),
	)
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbDeleteConfirm(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return h.handleLinkfbDelete(c)
	}
	if err := h.linkfb.DeleteLink(linkID); err != nil {
		return c.Edit(fmt.Sprintf("❌ *Delete gagal*\n\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfb(), tele.ModeMarkdown)
	}
	return c.Edit(
		fmt.Sprintf("💎 *H A P U S   L I N K   ✅* 💎\n"+
			"|\n"+
			"✅ Link ID %d dihapus permanent", linkID),
		backToLinkfb(), tele.ModeMarkdown)
}

// ─── Add Link (5-step wizard) ────────────────────────────────────────────────

func (h *Handler) handleLinkfbAdd(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepLinkfbAddTitle, // skip type picker — hardcoded "link"
		Data:      map[string]string{"type": "link"},
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *T A M B A H   L I N K* 💎\n"+
			"|\n"+
			"📝 *STEP 1/4 — TITLE*\n"+
			"└ Ketik title buat link ini\n"+
			"└ Contoh: `Promo Maha 2026`",
		cancelMenu(), tele.ModeMarkdown)
}

func (h *Handler) wizardLinkfbAddTitle(c tele.Context, sess *Session) error {
	h.showTyping(c)
	title := strings.TrimSpace(c.Text())
	if title == "" {
		return h.reply(c, "❌ Title kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	sess.Data["title"] = title
	sess.Step = StepLinkfbAddSlug
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"💎 *T A M B A H   L I N K* 💎\n"+
			"|\n"+
			"✅ Title : *%s*\n"+
			"|\n"+
			"📝 *STEP 2/4 — SLUG*\n"+
			"└ Path URL setelah linkfb.io/\n"+
			"└ Contoh: `promo-maha`\n"+
			"   → linkfb.io/promo-maha\n"+
			"|\n"+
			"💡 Ketik `-` buat auto-generate slug",
		escapeMD(title))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardLinkfbAddSlug(c tele.Context, sess *Session) error {
	h.showTyping(c)
	slug := strings.TrimSpace(c.Text())
	if slug == "-" {
		slug = ""
	}
	sess.Data["slug"] = slug
	sess.Step = StepLinkfbAddLocationURL
	h.sessions.Set(c.Sender().ID, sess)

	slugDisplay := slug
	if slug == "" {
		slugDisplay = "(auto)"
	}
	prompt := fmt.Sprintf(
		"💎 *T A M B A H   L I N K* 💎\n"+
			"|\n"+
			"✅ Slug : *%s*\n"+
			"|\n"+
			"🎯 *STEP 3/4 — LOCATION URL*\n"+
			"└ Ketik target URL (destination)\n"+
			"└ Contoh: `https://maha-supreme.com/daftar`",
		escapeMD(slugDisplay))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardLinkfbAddLocation(c tele.Context, sess *Session) error {
	h.showTyping(c)
	loc := strings.TrimSpace(c.Text())
	if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") {
		loc = "https://" + loc
	}
	sess.Data["location_url"] = loc
	sess.Step = StepLinkfbAddProject
	h.sessions.Set(c.Sender().ID, sess)

	projects, err := h.linkfb.ListProjects()
	if err != nil || len(projects) == 0 {
		return h.doLinkfbAddCreate(c, sess, 0)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	rows = append(rows, m.Row(m.Data("⏭ Skip (no project)", cbLinkfbAddPickProj, "0")))
	for _, p := range projects {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📁 %s", p.Name),
			cbLinkfbAddPickProj, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	prompt := fmt.Sprintf(
		"💎 *T A M B A H   L I N K* 💎\n"+
			"|\n"+
			"✅ Location : `%s`\n"+
			"|\n"+
			"📁 *STEP 4/4 — PROJECT*\n"+
			"└ Pilih project (atau Skip)",
		loc)
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: m})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) handleLinkfbAddPickProject(c tele.Context) error {
	pidStr := extractParam(c)
	pid, _ := strconv.Atoi(pidStr)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepLinkfbAddProject {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	return h.doLinkfbAddCreate(c, sess, pid)
}

func (h *Handler) doLinkfbAddCreate(c tele.Context, sess *Session, projectID int) error {
	defer h.sessions.Delete(c.Sender().ID)

	link, err := h.linkfb.CreateLink(
		sess.Data["type"],
		sess.Data["title"],
		sess.Data["slug"],
		sess.Data["location_url"],
		projectID,
	)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal create link*\n\n```\n%s\n```", escapeMD(err.Error()))
		return h.reply(c, errText, backToLinkfb(), tele.ModeMarkdown)
	}

	fullURL := h.buildLinkfbFullURL(*link)
	successText := fmt.Sprintf(
		"💎 *T A M B A H   L I N K   ✅* 💎\n"+
			"|\n"+
			"✅ *SUCCESS*\n"+
			"└ ID     : %d\n"+
			"└ 📛 Title  : *%s*\n"+
			"└ 🔗 URL    : `%s`\n"+
			"└ 🎯 Target : `%s`",
		link.ID, escapeMD(link.Title), fullURL, link.LocationURL)
	return h.reply(c, successText, backToLinkfb(), tele.ModeMarkdown)
}

// Projects CRUD moved to bot/linkfb_project.go
