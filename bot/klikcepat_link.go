package bot

import (
	"fmt"
	"strconv"
	"strings"

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
			cbKlikcepatAddPickProject, strconv.Itoa(p.ID))))
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
