package bot

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Projects CRUD ─────────────────────────────────────────────────
//
// Project = grouping container for klikcepat links (e.g., KONTAK, PROMO, RTP).
// API endpoints: /api/projects (GET/POST), /api/projects/{id} (GET/POST/DELETE).

func (h *Handler) handleKlikcepatProjects(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text: "⚠️ Setup credentials dulu", ShowAlert: true,
		})
	}
	return c.Edit(textKlikcepatProjects, klikcepatProjectsMenu(), tele.ModeMarkdown)
}

// ─── Add Project (2-step: name → color) ──────────────────────────────────────

func (h *Handler) handleKlikcepatProjectAdd(c tele.Context) error {
	h.cancelPriorPrompt(c, StepKlikcepatProjectAddName)
	prompt := "📁 *Tambah Project — Step 1/2: Name*\n\n" +
		"Ketik nama project:\n\n" +
		"*Contoh:* `Promo Mahaslot`"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatProjectAddName,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardKlikcepatProjectAddName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	name := strings.TrimSpace(c.Text())
	if name == "" {
		return h.reply(c, "❌ Name kosong, coba lagi:", cancelMenu())
	}
	sess.Data["name"] = name
	sess.Step = StepKlikcepatProjectAddColor
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"📁 *Step 2/2: Color*\n\nName: *%s* ✅\n\n"+
			"Ketik *hex color* untuk project (atau `-` untuk default `#000000`):\n\n"+
			"*Contoh:* `#FF5733`", escapeMD(name))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardKlikcepatProjectAddColor(c tele.Context, sess *Session) error {
	h.showTyping(c)
	color := strings.TrimSpace(c.Text())
	if color == "-" {
		color = ""
	}
	name := sess.Data["name"]
	h.sessions.Delete(c.Sender().ID)

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Creating project...", tele.ModeMarkdown)

	proj, err := h.klikcepat.CreateProject(name, color)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal create project*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf(
		"✅ *Project dibuat!*\n\n"+
			"📁 Name: *%s*\n"+
			"🎨 Color: `%s`\n"+
			"🆔 ID: `%d`",
		escapeMD(proj.Name), proj.Color, proj.ID)
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
}

// ─── List Projects ───────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectList(c tele.Context) error {
	c.Edit("⏳ Loading projects...", tele.ModeMarkdown)
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *Klikcepat Projects* (total %d)\n═══════════════════════════\n\n", len(projects)))
	for _, p := range projects {
		sb.WriteString(fmt.Sprintf("📁 *%s* (🎨 `%s`) — 🆔 `%d`\n", escapeMD(p.Name), p.Color, p.ID))
	}

	return c.Edit(sb.String(), backToKlikcepatProjects(), tele.ModeMarkdown)
}

// ─── Edit Project ────────────────────────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectEdit(c tele.Context) error {
	c.Edit("⏳ Loading...", tele.ModeMarkdown)
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		if len(rows) >= 30 {
			break
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("✏️ %s", truncate(p.Name, 40)),
			cbKlikcepatProjectEditPick, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepatProjects)))
	m.Inline(rows...)

	return c.Edit("✏️ *Pilih project yang mau di-edit:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatProjectEditPick(c tele.Context) error {
	projIDStr := extractParam(c)
	projID, _ := strconv.Atoi(projIDStr)
	if projID <= 0 {
		return h.handleKlikcepatProjectEdit(c)
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepKlikcepatProjectEditName,
		Data:      map[string]string{"project_id": projIDStr},
		PromptMsg: c.Message(),
	})
	prompt := "✏️ Ketik nilai baru, format `name|color` (color opsional):\n\n" +
		"*Contoh:* `Promo Baru|#FF0000`\n_(ketik `name|` aja kalau cuma rename, color preserved)_"
	h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardKlikcepatProjectEditName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	raw := strings.TrimSpace(c.Text())
	if raw == "" {
		return h.reply(c, "❌ Kosong, coba lagi:", cancelMenu())
	}
	parts := strings.SplitN(raw, "|", 2)
	name := strings.TrimSpace(parts[0])
	color := ""
	if len(parts) > 1 {
		color = strings.TrimSpace(parts[1])
	}
	projID, _ := strconv.Atoi(sess.Data["project_id"])
	h.sessions.Delete(c.Sender().ID)

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Updating project...", tele.ModeMarkdown)
	_, err := h.klikcepat.UpdateProject(projID, name, color)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal update*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	successText := fmt.Sprintf("✅ Project ID `%d` updated!\nName: *%s*", projID, escapeMD(name))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToKlikcepatProjects(), tele.ModeMarkdown)
}

// ─── Delete Project (with confirm) ───────────────────────────────────────────

func (h *Handler) handleKlikcepatProjectDelete(c tele.Context) error {
	c.Edit("⏳ Loading...", tele.ModeMarkdown)
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToKlikcepatProjects(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		if len(rows) >= 30 {
			break
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 %s", truncate(p.Name, 40)),
			cbKlikcepatProjectDeletePick, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbKlikcepatProjects)))
	m.Inline(rows...)

	return c.Edit("🗑 *Pilih project yang mau dihapus:*", m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatProjectDeletePick(c tele.Context) error {
	projIDStr := extractParam(c)
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🗑 Ya, Hapus", cbKlikcepatProjectDeleteConfirm, projIDStr),
			m.Data("❌ Batal", cbKlikcepatProjects),
		),
	)
	return c.Edit(fmt.Sprintf("⚠️ Yakin hapus project ID `%s`?\n\n_Note: link yang assigned ke project ini akan jadi tanpa project (project_id=0)._", projIDStr),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleKlikcepatProjectDeleteConfirm(c tele.Context) error {
	projIDStr := extractParam(c)
	projID, _ := strconv.Atoi(projIDStr)
	if projID <= 0 {
		return c.Edit("❌ Invalid project ID", backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Deleting...", tele.ModeMarkdown)
	if err := h.klikcepat.DeleteProject(projID); err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal:\n```\n%s\n```", escapeMD(err.Error())),
			backToKlikcepatProjects(), tele.ModeMarkdown)
	}
	return c.Edit(fmt.Sprintf("✅ Project ID `%d` dihapus.", projID),
		backToKlikcepatProjects(), tele.ModeMarkdown)
}
