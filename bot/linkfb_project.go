package bot

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// ─── LinkFB Projects CRUD ─────────────────────────────────────────────────────
//
// Same engine as Klikcepat. Project = grouping for shortlinks.

func (h *Handler) handleLinkfbProjects(c tele.Context) error {
	if !h.linkfb.HasCredentials() {
		return c.Respond(&tele.CallbackResponse{
			Text: "⚠️ Setup credentials dulu", ShowAlert: true,
		})
	}
	text := "💎 *L I N K F B   P R O J E C T S* 💎\n" +
		"|\n" +
		"📁 *FUNGSI*\n" +
		"└ Group shortlink by project\n" +
		"└ Contoh: KONTAK, PROMO, RTP\n" +
		"|\n" +
		"🎯 *YG BISA DILAKUIN*\n" +
		"└ ➕ Tambah Project (name + color)\n" +
		"└ 📋 List Projects (semua project)\n" +
		"└ ✏️ Edit Project (rename + recolor)\n" +
		"└ 🗑 Hapus Project (with confirm)"
	return c.Edit(text, linkfbProjectsMenu(), tele.ModeMarkdown)
}

// ─── Add Project (2-step: name → color) ──────────────────────────────────────

func (h *Handler) handleLinkfbProjectAdd(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepLinkfbProjectAddName,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *T A M B A H   P R O J E C T* 💎\n"+
			"|\n"+
			"📝 *STEP 1/2 — NAME*\n"+
			"└ Ketik nama project\n"+
			"└ Contoh: `Promo Mahaslot`",
		cancelMenu(), tele.ModeMarkdown)
}

func (h *Handler) wizardLinkfbProjectAddName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	name := strings.TrimSpace(c.Text())
	if name == "" {
		return h.reply(c, "❌ Name kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	sess.Data["name"] = name
	sess.Step = StepLinkfbProjectAddColor
	h.sessions.Set(c.Sender().ID, sess)

	prompt := fmt.Sprintf(
		"💎 *T A M B A H   P R O J E C T* 💎\n"+
			"|\n"+
			"✅ Name : *%s*\n"+
			"|\n"+
			"🎨 *STEP 2/2 — COLOR*\n"+
			"└ Ketik hex color (atau `-` buat default)\n"+
			"└ Contoh: `#FF5733`\n"+
			"└ Default: `#000000`",
		escapeMD(name))
	newMsg, _ := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" "+prompt,
		&tele.SendOptions{ReplyTo: c.Message(), ParseMode: tele.ModeMarkdown, ReplyMarkup: cancelMenu()})
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

func (h *Handler) wizardLinkfbProjectAddColor(c tele.Context, sess *Session) error {
	h.showTyping(c)
	color := strings.TrimSpace(c.Text())
	if color == "-" {
		color = ""
	}
	name := sess.Data["name"]
	h.sessions.Delete(c.Sender().ID)

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Creating project...", tele.ModeMarkdown)

	proj, err := h.linkfb.CreateProject(name, color)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal create project*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToLinkfbProjects(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToLinkfbProjects(), tele.ModeMarkdown)
	}

	successText := fmt.Sprintf(
		"💎 *T A M B A H   P R O J E C T   ✅* 💎\n"+
			"|\n"+
			"✅ *SUCCESS*\n"+
			"└ 🆔 ID    : %d\n"+
			"└ 📁 Name  : *%s*\n"+
			"└ 🎨 Color : `%s`",
		proj.ID, escapeMD(proj.Name), proj.Color)
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToLinkfbProjects(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToLinkfbProjects(), tele.ModeMarkdown)
}

// ─── List Projects ───────────────────────────────────────────────────────────

func (h *Handler) handleLinkfbProjectList(c tele.Context) error {
	c.Edit("⏳ Loading projects...", tele.ModeMarkdown)
	projects, err := h.linkfb.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfbProjects(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("💎 *L I S T   P R O J E C T S* 💎\n")
	sb.WriteString("|\n")

	if len(projects) == 0 {
		sb.WriteString("📭 *EMPTY*\n└ Belum ada project")
	} else {
		sb.WriteString(fmt.Sprintf("📊 *TOTAL : %d project*\n", len(projects)))
		sb.WriteString("|\n")
		sb.WriteString("📁 *LIST*\n")
		for _, p := range projects {
			sb.WriteString(fmt.Sprintf("└ *%s*\n", escapeMD(p.Name)))
			sb.WriteString(fmt.Sprintf("   └ ID    : `%d`\n", p.ID))
			sb.WriteString(fmt.Sprintf("   └ Color : `%s`\n", p.Color))
		}
	}

	return c.Edit(sb.String(), backToLinkfbProjects(), tele.ModeMarkdown)
}

// ─── Edit Project ────────────────────────────────────────────────────────────

func (h *Handler) handleLinkfbProjectEdit(c tele.Context) error {
	c.Edit("⏳ Loading...", tele.ModeMarkdown)
	projects, err := h.linkfb.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfbProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToLinkfbProjects(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		if len(rows) >= 30 {
			break
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("✏️ %s", truncate(p.Name, 40)),
			cbLinkfbProjectEditPick, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbLinkfbProjects)))
	m.Inline(rows...)

	return c.Edit(
		"💎 *E D I T   P R O J E C T* 💎\n"+
			"|\n"+
			"✏️ Pilih project yg mau di-edit 👇",
		m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbProjectEditPick(c tele.Context) error {
	projIDStr := extractParam(c)
	projID, _ := strconv.Atoi(projIDStr)
	if projID <= 0 {
		return h.handleLinkfbProjectEdit(c)
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepLinkfbProjectEditName,
		Data:      map[string]string{"project_id": projIDStr},
		PromptMsg: c.Message(),
	})
	prompt := "💎 *E D I T   P R O J E C T* 💎\n" +
		"|\n" +
		"📝 *INPUT*\n" +
		"└ Format : `name|color`\n" +
		"└ Color optional\n" +
		"|\n" +
		"📝 *CONTOH*\n" +
		"└ `Promo Baru|#FF0000` — rename + recolor\n" +
		"└ `Promo Baru|` — rename aja (color preserved)"
	h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardLinkfbProjectEditName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	raw := strings.TrimSpace(c.Text())
	if raw == "" {
		return h.reply(c, "❌ Kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
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
	_, err := h.linkfb.UpdateProject(projID, name, color)
	if err != nil {
		errText := fmt.Sprintf("❌ *Gagal update*\n\n```\n%s\n```", escapeMD(err.Error()))
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToLinkfbProjects(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToLinkfbProjects(), tele.ModeMarkdown)
	}
	successText := fmt.Sprintf(
		"💎 *E D I T   P R O J E C T   ✅* 💎\n"+
			"|\n"+
			"✅ *SUCCESS*\n"+
			"└ 🆔 ID   : %d\n"+
			"└ 📁 Name : *%s*",
		projID, escapeMD(name))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToLinkfbProjects(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToLinkfbProjects(), tele.ModeMarkdown)
}

// ─── Delete Project (with confirm) ───────────────────────────────────────────

func (h *Handler) handleLinkfbProjectDelete(c tele.Context) error {
	c.Edit("⏳ Loading...", tele.ModeMarkdown)
	projects, err := h.linkfb.ListProjects()
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfbProjects(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return c.Edit("📭 Belum ada project.", backToLinkfbProjects(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		if len(rows) >= 30 {
			break
		}
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("🗑 %s", truncate(p.Name, 40)),
			cbLinkfbProjectDeletePick, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbLinkfbProjects)))
	m.Inline(rows...)

	return c.Edit(
		"💎 *H A P U S   P R O J E C T* 💎\n"+
			"|\n"+
			"🗑 Pilih project yg mau dihapus 👇",
		m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbProjectDeletePick(c tele.Context) error {
	projIDStr := extractParam(c)
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🗑 Ya, Hapus", cbLinkfbProjectDeleteConfirm, projIDStr),
			m.Data("❌ Batal", cbLinkfbProjects),
		),
	)
	return c.Edit(
		fmt.Sprintf(
			"💎 *H A P U S   P R O J E C T* 💎\n"+
				"|\n"+
				"⚠️ *KONFIRMASI*\n"+
				"└ Yakin hapus project ID `%s`?\n"+
				"|\n"+
				"💡 *NOTE*\n"+
				"└ Link assigned ke project ini akan jadi\n"+
				"   tanpa project (project_id=0)",
			projIDStr),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleLinkfbProjectDeleteConfirm(c tele.Context) error {
	projIDStr := extractParam(c)
	projID, _ := strconv.Atoi(projIDStr)
	if projID <= 0 {
		return c.Edit("❌ Invalid project ID", backToLinkfbProjects(), tele.ModeMarkdown)
	}
	c.Edit("⏳ Deleting...", tele.ModeMarkdown)
	if err := h.linkfb.DeleteProject(projID); err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal:\n```\n%s\n```", escapeMD(err.Error())),
			backToLinkfbProjects(), tele.ModeMarkdown)
	}
	return c.Edit(
		fmt.Sprintf(
			"💎 *H A P U S   P R O J E C T   ✅* 💎\n"+
				"|\n"+
				"✅ Project ID `%d` dihapus",
			projID),
		backToLinkfbProjects(), tele.ModeMarkdown)
}
