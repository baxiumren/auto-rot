package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"bongbot/klikcepat"
	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Link CRUD ─────────────────────────────────────────────────────
//
// Files split by feature:
//   - klikcepat_link.go       — Link CRUD (Add/List/Edit/Delete)
//   - klikcepat_project.go    — Project CRUD
//   - klikcepat_rotator.go    — Klikcepat rotator wizard
//   - klikcepat_settings.go   — Settings/credentials
//
// This file: Add Link wizard. List/Edit/Delete in subsequent tasks.

// ─── Add Link wizard ─────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatAdd(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "⚠️ Setup Klikcepat credentials dulu via 🔧 Settings",
			ShowAlert: true,
		})
	}
	h.cancelPriorPrompt(c, StepKlikcepatAddType)

	prompt := "➕ *Tambah Link Klikcepat — Step 1/5: Pilih Tipe*\n\n" +
		"_Tipe link menentukan behavior klikcepat:_\n" +
		"• *🔗 Shortlink* — URL pendek redirect ke 1 target\n" +
		"• *📄 Biolink* — landing page bio (kayak Linktree)\n" +
		"• *📇 VCard / 📅 Event* — kartu nama / event link"

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🔗 Shortlink", cbKlikcepatAddType, "link"),
			m.Data("📄 Biolink", cbKlikcepatAddType, "biolink"),
		),
		m.Row(
			m.Data("📇 VCard", cbKlikcepatAddType, "vcard"),
			m.Data("📅 Event", cbKlikcepatAddType, "event"),
		),
		m.Row(
			m.Data("❌ Batal", cbCancel),
		),
	)
	msg, _ := h.bot.Edit(c.Message(), prompt, m, tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatAddType,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) handleKlikcepatAddType(c tele.Context) error {
	linkType := extractParam(c)
	if linkType == "" {
		return h.handleKlikcepatAdd(c)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatAddType {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["type"] = linkType
	sess.Step = StepKlikcepatAddTitle
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"➕ *Step 2/5: Title*\n\nTipe: *%s* ✅\n\nKetik *title* untuk link ini:\n\n*Contoh:* `Promo Maha 2026`",
		linkType)
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatAddTitle(c tele.Context, sess *Session) error {
	h.showTyping(c)
	title := strings.TrimSpace(c.Text())
	if title == "" {
		return h.reply(c, "❌ Title kosong, coba lagi:", cancelMenu())
	}
	sess.Data["title"] = title
	sess.Step = StepKlikcepatAddSlug
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"➕ *Step 3/5: Slug*\n\nTitle: *%s* ✅\n\n"+
			"Ketik *slug* (path URL setelah klikcepat.com/) atau ketik `-` untuk auto-generate:\n\n"+
			"*Contoh:* `promo-maha` → klikcepat.com/promo-maha", escapeMD(title))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardKlikcepatAddSlug(c tele.Context, sess *Session) error {
	h.showTyping(c)
	slug := strings.TrimSpace(c.Text())
	if slug == "-" {
		slug = ""
	}
	sess.Data["slug"] = slug
	sess.Step = StepKlikcepatAddLocationURL
	h.sessions.Set(c.Sender().ID, sess)

	slugDisplay := slug
	if slug == "" {
		slugDisplay = "(auto)"
	}
	prompt := fmt.Sprintf(
		"➕ *Step 4/5: Location URL*\n\nSlug: *%s* ✅\n\n"+
			"Ketik *target URL* (kemana redirect tujunya):\n\n"+
			"*Contoh:* `https://maha-supreme.com/daftar`", escapeMD(slugDisplay))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardKlikcepatAddLocation(c tele.Context, sess *Session) error {
	h.showTyping(c)
	loc := strings.TrimSpace(c.Text())
	if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") {
		loc = "https://" + loc
	}
	sess.Data["location_url"] = loc
	sess.Step = StepKlikcepatAddProject
	h.sessions.Set(c.Sender().ID, sess)

	// Fetch projects for picker
	projects, err := h.klikcepat.ListProjects()
	if err != nil || len(projects) == 0 {
		// Skip project step — directly create
		return h.doKlikcepatAddCreate(c, sess, 0)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	rows = append(rows, m.Row(m.Data("⏭ Skip (no project)", cbKlikcepatAddPickProject, "0")))
	for _, p := range projects {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📁 %s", p.Name),
			cbKlikcepatAddPickProject, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	prompt := fmt.Sprintf(
		"➕ *Step 5/5: Project*\n\nLocation: *%s* ✅\n\n"+
			"Pilih project (atau Skip):", escapeMD(loc))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: m})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) handleKlikcepatAddPickProject(c tele.Context) error {
	projectIDStr := extractParam(c)
	projectID, _ := strconv.Atoi(projectIDStr)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatAddProject {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	return h.doKlikcepatAddCreate(c, sess, projectID)
}

// doKlikcepatAddCreate finalizes the wizard — calls klikcepat API to create the link.
func (h *Handler) doKlikcepatAddCreate(c tele.Context, sess *Session, projectID int) error {
	h.sessions.Delete(c.Sender().ID)
	linkType := sess.Data["type"]
	title := sess.Data["title"]
	slug := sess.Data["slug"]
	locURL := sess.Data["location_url"]

	loadingText := fmt.Sprintf("⏳ Membuat link `%s`...", escapeMD(title))
	loadingMsg, _ := h.bot.Send(c.Chat(), loadingText, tele.ModeMarkdown)

	link, err := h.klikcepat.CreateLink(linkType, title, slug, locURL, projectID)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal create link*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepat(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepat(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf(
		"✅ *Link dibuat!*\n\n"+
			"📛 Title: *%s*\n"+
			"🔗 Slug: `%s`\n"+
			"🎯 Target: `%s`\n"+
			"📌 Type: *%s*\n"+
			"🆔 ID: `%d`",
		escapeMD(link.Title), escapeMD(link.URL), escapeMD(link.LocationURL),
		link.Type, link.ID)

	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepat(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepat(), tele.ModeMarkdown)
}

// ─── List Link (paginated) ───────────────────────────────────────────────────

const klikcepatLinksPerPage = 10

func (h *Handler) handleKlikcepatList(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "⚠️ Setup credentials dulu via 🔧 Settings → 🔗 Klikcepat",
			ShowAlert: true,
		})
	}

	// Parse param: empty = show type picker; "biolink|0" / "link|2" = filtered list
	param := extractParam(c)
	if param == "" {
		// Show type picker
		return h.renderKlikcepatListTypePicker(c)
	}

	parts := strings.SplitN(param, "|", 2)
	linkType := parts[0]
	page := 0
	if len(parts) > 1 {
		page, _ = strconv.Atoi(parts[1])
	}
	if linkType != "biolink" && linkType != "link" {
		// Invalid type → back to type picker
		return h.renderKlikcepatListTypePicker(c)
	}
	return h.renderKlikcepatListByType(c, linkType, page)
}

// buildKlikcepatFullURL constructs the public-facing URL for a link.
// If link.DomainID > 0 and domain exists in map → use custom domain (scheme://host/slug).
// Otherwise → fallback to platform default (klikcepat.com/slug).
func buildKlikcepatFullURL(l klikcepat.Link, domainMap map[int]klikcepat.Domain) string {
	if d, ok := domainMap[int(l.DomainID)]; ok && d.Host != "" {
		scheme := d.Scheme
		if scheme == "" {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s/%s", scheme, d.Host, l.URL)
	}
	return fmt.Sprintf("https://klikcepat.com/%s", l.URL)
}

// renderKlikcepatListTypePicker — first screen: pick Biolink vs Shortlink.
func (h *Handler) renderKlikcepatListTypePicker(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📄 BIOLINK", cbKlikcepatList, "biolink|0"),
			m.Data("🔗 SHORTLINK", cbKlikcepatList, "link|0"),
		),
		m.Row(m.Data("❌ Batal", cbKlikcepat)),
	)
	return c.Edit(
		"📋 *List Link — Pilih Kategori*\n\n"+
			"Pilih tipe link yang mau dilihat:\n\n"+
			"• 📄 *BIOLINK* — landing page bio (kayak Linktree)\n"+
			"• 🔗 *SHORTLINK* — URL pendek redirect ke target",
		m, tele.ModeMarkdown)
}

// renderKlikcepatListByType — paginated list filtered by type (link / biolink).
func (h *Handler) renderKlikcepatListByType(c tele.Context, linkType string, page int) error {
	c.Edit("⏳ Loading links + domains...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks(linkType)
	if err != nil {
		return c.Edit(
			fmt.Sprintf("❌ Gagal fetch links:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	// Defensive: filter again in case API returns mixed
	var filtered []klikcepat.Link
	for _, l := range links {
		if l.Type == linkType {
			filtered = append(filtered, l)
		}
	}
	links = filtered

	// Fetch custom domains (best effort — gak fatal kalau gagal)
	// Build map[domain_id]Host for fast lookup saat render.
	domainMap := make(map[int]klikcepat.Domain)
	domains, derr := h.klikcepat.ListDomains()
	if derr != nil {
		log.Printf("[KLC-LIST] ListDomains ERROR (fallback ke default URL): %v", derr)
	} else {
		log.Printf("[KLC-LIST] ListDomains returned %d domains", len(domains))
		for _, d := range domains {
			domainMap[int(d.ID)] = d
			log.Printf("[KLC-LIST] Domain: ID=%d Scheme=%q Host=%q IsEnabled=%d",
				int(d.ID), d.Scheme, d.Host, int(d.IsEnabled))
		}
	}

	// Diagnostic: log first link's DomainID to verify matching
	if len(links) > 0 {
		first := links[0]
		matched := "NO-MATCH"
		if d, ok := domainMap[int(first.DomainID)]; ok {
			matched = fmt.Sprintf("MATCHED host=%s", d.Host)
		}
		log.Printf("[KLC-LIST] First link sample: ID=%d Slug=%q DomainID=%d → %s",
			int(first.ID), first.URL, int(first.DomainID), matched)
	}

	typeLabel := "SHORTLINK"
	typeIconHeader := "🔗"
	if linkType == "biolink" {
		typeLabel = "BIOLINK"
		typeIconHeader = "📄"
	}

	if len(links) == 0 {
		return c.Edit(
			fmt.Sprintf("📭 *Belum ada %s di klikcepat.*\n\nKlik *➕ Tambah Link* untuk mulai (pilih tipe %s saat wizard).", typeLabel, typeLabel),
			backToKlikcepat(), tele.ModeMarkdown)
	}

	total := len(links)
	totalPages := (total + klikcepatLinksPerPage - 1) / klikcepatLinksPerPage
	if page >= totalPages {
		page = totalPages - 1
	}
	if page < 0 {
		page = 0
	}
	start := page * klikcepatLinksPerPage
	end := start + klikcepatLinksPerPage
	if end > total {
		end = total
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s *List %s* — page %d/%d (total %d)\n═══════════════════════════\n\n",
		typeIconHeader, typeLabel, page+1, totalPages, total))
	for i := start; i < end; i++ {
		l := links[i]
		enabled := "✅"
		if l.IsEnabled == 0 {
			enabled = "⛔"
		}

		// Build full URL: scheme://host/slug (custom domain or klikcepat.com fallback)
		fullURL := buildKlikcepatFullURL(l, domainMap)

		// Title = uppercase slug (display-only, klikcepat side gak berubah)
		title := strings.ToUpper(l.URL)
		if title == "" {
			title = "(no slug)"
		}

		sb.WriteString(fmt.Sprintf("%s *%s* %s\n", typeIconHeader, escapeMD(title), enabled))
		sb.WriteString(fmt.Sprintf("   🌐 `%s`\n", escapeMD(fullURL)))
		if l.LocationURL != "" {
			sb.WriteString(fmt.Sprintf("   🎯 `%s`\n", escapeMD(l.LocationURL)))
		}
		sb.WriteString(fmt.Sprintf("   🆔 `%d`\n\n", int(l.ID)))
	}

	if linkType == "biolink" {
		sb.WriteString("_💡 Biolink page punya buttons di dalamnya (LOGIN/DAFTAR/dll)._\n")
		sb.WriteString("_Buttons (blocks) gak bisa di-CRUD via API standard — manage via dashboard._\n\n")
	}

	m := &tele.ReplyMarkup{}
	var navRow tele.Row
	if page > 0 {
		navRow = append(navRow, m.Data("⬅️ Prev", cbKlikcepatList, fmt.Sprintf("%s|%d", linkType, page-1)))
	}
	navRow = append(navRow, m.Data(fmt.Sprintf("%d/%d", page+1, totalPages), cbNoop))
	if page < totalPages-1 {
		navRow = append(navRow, m.Data("Next ➡️", cbKlikcepatList, fmt.Sprintf("%s|%d", linkType, page+1)))
	}
	rows := []tele.Row{
		navRow,
		m.Row(m.Data("🔄 Ganti Kategori", cbKlikcepatList)),
		m.Row(m.Data("🔙 Kembali", cbKlikcepat)),
	}
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// ─── Edit Link wizard ────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatEdit(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text:      "⚠️ Setup credentials dulu",
			ShowAlert: true,
		})
	}
	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	if len(links) == 0 {
		return c.Edit("📭 Belum ada link.", backToKlikcepat(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, l := range links {
		if len(rows) >= 30 {
			break // simple cap — pagination not yet implemented for edit picker
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("✏️ %s (/%s)", truncate(l.Title, 30), l.URL),
			cbKlikcepatEditPick, strconv.Itoa(int(l.ID)))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepat)))
	m.Inline(rows...)

	return c.Edit("✏️ *Edit Link — Pilih link yang mau di-edit:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatEditPick(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return h.handleKlikcepatEdit(c)
	}
	link, err := h.klikcepat.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch link:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}

	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatEditPickField,
		Data:      map[string]string{"link_id": linkIDStr},
		PromptMsg: c.Message(),
	})

	prompt := fmt.Sprintf(
		"✏️ *Edit Link*\n\n"+
			"📛 Title: *%s*\n"+
			"🔗 Slug: `%s`\n"+
			"🎯 Target: `%s`\n"+
			"📌 Type: *%s*\n\n"+
			"Pilih field yang mau di-edit:",
		escapeMD(link.Title), escapeMD(link.URL), escapeMD(link.LocationURL), link.Type)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📛 Title", cbKlikcepatEditField, "title"),
			m.Data("🔗 Slug", cbKlikcepatEditField, "url"),
		),
		m.Row(
			m.Data("🎯 Location URL", cbKlikcepatEditField, "location_url"),
		),
		m.Row(m.Data("❌ Batal", cbCancel)),
	)
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatEditField(c tele.Context) error {
	field := extractParam(c)
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepKlikcepatEditPickField {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}
	sess.Data["field"] = field
	sess.Step = StepKlikcepatEditValue
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf("✏️ Ketik nilai baru untuk *%s*:", field)
	h.bot.Edit(sess.PromptMsg, prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatEditValue(c tele.Context, sess *Session) error {
	h.showTyping(c)
	val := strings.TrimSpace(c.Text())
	if val == "" {
		return h.reply(c, "❌ Nilai kosong, coba lagi:", cancelMenu())
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

	_, err := h.klikcepat.UpdateLink(linkID, map[string]string{field: val})
	if err != nil {
		errText := fmt.Sprintf("❌ *Update gagal*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepat(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepat(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf("✅ Link ID `%d` updated!\nField *%s* = `%s`",
		linkID, field, escapeMD(val))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepat(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepat(), tele.ModeMarkdown)
}

// ─── Delete Link (with confirm) ──────────────────────────────────────────────

func (h *Handler) handleKlikcepatDelete(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text: "⚠️ Setup credentials dulu", ShowAlert: true,
		})
	}
	c.Edit("⏳ Loading links...", tele.ModeMarkdown)
	links, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	if len(links) == 0 {
		return c.Edit("📭 Belum ada link.", backToKlikcepat(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, l := range links {
		if len(rows) >= 30 {
			break
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 %s (/%s)", truncate(l.Title, 30), l.URL),
			cbKlikcepatDeletePick, strconv.Itoa(int(l.ID)))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepat)))
	m.Inline(rows...)

	return c.Edit("🗑 *Hapus Link — Pilih link yang mau dihapus:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatDeletePick(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return h.handleKlikcepatDelete(c)
	}
	link, err := h.klikcepat.GetLink(linkID)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}

	prompt := fmt.Sprintf(
		"⚠️ *Konfirmasi Hapus*\n\n"+
			"📛 Title: *%s*\n"+
			"🔗 Slug: `%s`\n"+
			"🎯 Target: `%s`\n\n"+
			"Yakin mau hapus? Action ini *tidak bisa di-undo*.",
		escapeMD(link.Title), escapeMD(link.URL), escapeMD(link.LocationURL))

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🗑 Ya, Hapus", cbKlikcepatDeleteConfirm, linkIDStr),
			m.Data("❌ Batal", cbKlikcepat),
		),
	)
	return c.Edit(prompt, m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatDeleteConfirm(c tele.Context) error {
	linkIDStr := extractParam(c)
	linkID, _ := strconv.Atoi(linkIDStr)
	if linkID <= 0 {
		return c.Edit("❌ Invalid link ID", backToKlikcepat(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Deleting...", tele.ModeMarkdown)
	if err := h.klikcepat.DeleteLink(linkID); err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal hapus:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepat(), tele.ModeMarkdown)
	}
	return c.Edit(fmt.Sprintf("✅ Link ID `%d` berhasil dihapus.", linkID),
		backToKlikcepat(), tele.ModeMarkdown)
}
