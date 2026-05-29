package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bongbot/checker"
	"bongbot/klikcepat"
	"bongbot/store"

	tele "gopkg.in/telebot.v3"
)

// ─── Group Commands wizard ───────────────────────────────────────────────────
//
// Use case: member di group ketik /rtp → bot reply list link RTP yg available.
//
// Flow CRUD:
//   1. Settings → Group Commands → daftar semua command yg udah di-set
//   2. ➕ Add Command → wizard: input nama (`rtp`) → pick project klikcepat → input deskripsi
//   3. Tiap Add/Delete → bot auto-call SetMyCommands(AllGroupChats) → Telegram autocomplete update
//
// Flow Group Reply:
//   1. Member ketik /cmd di group
//   2. Bot listen text (di handleText) → cek prefix "/"
//   3. Lookup di GroupCommandStore — kalau match, fetch klikcepat link by project_id
//   4. Reply dengan inline buttons (link2 yg available, skip yg di-blocked)

// handleGroupCmd — Settings entry, list all configured commands.
func (h *Handler) handleGroupCmd(c tele.Context) error {
	cmds := h.groupCmds.GetAll()

	var sb strings.Builder
	sb.WriteString("💬 *Group Commands*\n═══════════════════════════\n\n")
	sb.WriteString("Slash commands buat member di group. Member ketik `/cmd` → bot reply link.\n\n")

	if len(cmds) == 0 {
		sb.WriteString("📭 Belum ada command terdaftar.\n\nKlik ➕ buat tambah.")
	} else {
		sb.WriteString("*Terdaftar:*\n")
		for i, gc := range cmds {
			sb.WriteString(fmt.Sprintf("%d. /%s → 📂 *%s* (id %d)\n",
				i+1, escapeMD(gc.Command), escapeMD(gc.ProjectName), gc.ProjectID))
			if gc.Description != "" {
				sb.WriteString(fmt.Sprintf("   📝 %s\n", escapeMD(gc.Description)))
			}
		}
		sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\nTotal: *%d* command", len(cmds)))
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	rows = append(rows, m.Row(m.Data("➕ Tambah Command", cbGroupCmdAdd)))
	for _, gc := range cmds {
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("💬 /%s", gc.Command), cbNoop),
			m.Data("🗑 Hapus", cbGroupCmdDelete, gc.Command),
		))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbSettings)))
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// handleGroupCmdAdd — step 1: prompt user buat ketik nama command
func (h *Handler) handleGroupCmdAdd(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set.*\n\nSet dulu via *🔧 Settings → 🔗 Klikcepat* — karena command pakai project klikcepat.",
			backToSettings(), tele.ModeMarkdown)
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepGroupCmdInputName,
		Data:      map[string]string{},
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"➕ *Tambah Group Command*\n\n"+
			"Ketik *nama command* (tanpa `/`).\n\n"+
			"*Contoh:* `rtp`, `daftar`, `bukti`, `wa`\n\n"+
			"_Rule: huruf kecil semua, max 32 karakter, gak boleh ada spasi atau special char._",
		cancelMenu(), tele.ModeMarkdown)
}

// wizardGroupCmdInputName — user ketik command name, lalu show project picker.
func (h *Handler) wizardGroupCmdInputName(c tele.Context, sess *Session) error {
	h.showTyping(c)
	cmd := strings.ToLower(strings.TrimSpace(c.Text()))
	cmd = strings.TrimPrefix(cmd, "/")

	// Validate: alphanumeric + underscore allowed
	if cmd == "" || len(cmd) > 32 || !isValidCommandName(cmd) {
		return h.reply(c,
			"❌ Nama command gak valid. Pake huruf kecil + angka + underscore (`a-z 0-9 _`), max 32 char.\n\nCoba lagi:",
			cancelMenu(), tele.ModeMarkdown)
	}

	// Cek duplicate
	if _, exists := h.groupCmds.GetByCommand(cmd); exists {
		return h.reply(c,
			fmt.Sprintf("❌ Command `/%s` udah ada. Hapus dulu lewat list, atau pakai nama lain.", cmd),
			cancelMenu(), tele.ModeMarkdown)
	}

	sess.Data["command"] = cmd
	h.sessions.Set(c.Sender().ID, sess)

	// Fetch projects dari klikcepat
	projects, err := h.klikcepat.ListProjects()
	if err != nil {
		return h.reply(c,
			fmt.Sprintf("❌ Gagal fetch projects dari klikcepat:\n```\n%s\n```", escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown)
	}
	if len(projects) == 0 {
		return h.reply(c,
			"📭 *Belum ada project di klikcepat lo.*\n\n"+
				"Bikin project dulu lewat klikcepat web → menu *Projects* → assign link2 ke project.",
			backToSettings(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range projects {
		rows = append(rows, m.Row(m.Data(
			fmt.Sprintf("📂 %s", truncate(p.Name, 40)),
			cbGroupCmdPickProject, strconv.Itoa(int(p.ID)))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return h.reply(c,
		fmt.Sprintf("✅ Command: */%s*\n\n📂 *Step 2/3: Pick Project*\n\nPilih project klikcepat — bot bakal fetch semua link di project ini saat command dipake:", cmd),
		m, tele.ModeMarkdown)
}

// handleGroupCmdPickProject — user picked project, prompt for description.
func (h *Handler) handleGroupCmdPickProject(c tele.Context) error {
	pidStr := extractParam(c)
	pid, _ := strconv.Atoi(pidStr)
	if pid <= 0 {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Project ID invalid", ShowAlert: true})
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepGroupCmdInputName {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Session expired", ShowAlert: true})
	}

	// Fetch project name buat display
	proj, err := h.klikcepat.GetProject(pid)
	if err != nil {
		return c.Edit(fmt.Sprintf("❌ Gagal fetch project:\n```\n%s\n```", escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown)
	}

	sess.Data["project_id"] = pidStr
	sess.Data["project_name"] = proj.Name
	sess.Step = StepGroupCmdInputDesc
	sess.PromptMsg = c.Message()
	h.sessions.Set(c.Sender().ID, sess)

	return c.Edit(
		fmt.Sprintf(
			"✅ Project: *%s* (id %d)\n\n"+
				"📝 *Step 3/3: Description*\n\n"+
				"Ketik deskripsi singkat (max 256 char). Ini muncul di autocomplete Telegram pas user ketik /\n\n"+
				"*Contoh:* `Daftar link RTP MAHASLOT`\n\n"+
				"Kirim `-` buat skip deskripsi.",
			escapeMD(proj.Name), pid),
		cancelMenu(), tele.ModeMarkdown)
}

// wizardGroupCmdInputDesc — user ketik deskripsi, save command, sync ke Telegram.
func (h *Handler) wizardGroupCmdInputDesc(c tele.Context, sess *Session) error {
	h.showTyping(c)
	desc := strings.TrimSpace(c.Text())
	if desc == "-" {
		desc = ""
	}
	if len(desc) > 256 {
		desc = desc[:256]
	}

	cmd := sess.Data["command"]
	pid, _ := strconv.Atoi(sess.Data["project_id"])
	projectName := sess.Data["project_name"]
	h.sessions.Delete(c.Sender().ID)

	gc := store.GroupCommand{
		Command:     cmd,
		ProjectID:   pid,
		ProjectName: projectName,
		Description: desc,
	}
	if err := h.groupCmds.Add(gc); err != nil {
		return h.reply(c, fmt.Sprintf("❌ Gagal save: %s", escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown)
	}

	// Sync ke Telegram autocomplete
	if err := h.syncTelegramCommands(); err != nil {
		return h.reply(c,
			fmt.Sprintf("✅ Command */%s* ditambahin (pointing ke project *%s*).\n\n"+
				"⚠️ Sync autocomplete ke Telegram gagal: %s\nCoba reload Telegram dalam beberapa menit.",
				cmd, escapeMD(projectName), escapeMD(err.Error())),
			backToSettings(), tele.ModeMarkdown)
	}

	descLine := ""
	if desc != "" {
		descLine = fmt.Sprintf("\n📝 %s", escapeMD(desc))
	}
	return h.reply(c,
		fmt.Sprintf("✅ *Group Command dibuat!*\n\n"+
			"💬 Trigger: `/%s`\n"+
			"📂 Project: *%s* (id %d)%s\n\n"+
			"🚀 Sekarang member di group bisa ketik /%s buat dapet link.\n"+
			"(Autocomplete Telegram update dalam 1-2 menit.)",
			cmd, escapeMD(projectName), pid, descLine, cmd),
		backToSettings(), tele.ModeMarkdown)
}

// handleGroupCmdDelete — confirm delete.
func (h *Handler) handleGroupCmdDelete(c tele.Context) error {
	cmd := extractParam(c)
	if cmd == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Command kosong", ShowAlert: true})
	}
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("✅ Yakin Hapus", cbGroupCmdDeleteConfirm, cmd),
			m.Data("❌ Batal", cbGroupCmd),
		),
	)
	return c.Edit(
		fmt.Sprintf("🗑 *Hapus Command /%s ?*\n\nMember di group gak bisa pake command ini lagi setelah dihapus.", escapeMD(cmd)),
		m, tele.ModeMarkdown)
}

// handleGroupCmdDeleteConfirm — do delete + sync.
func (h *Handler) handleGroupCmdDeleteConfirm(c tele.Context) error {
	cmd := extractParam(c)
	if cmd == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Command kosong", ShowAlert: true})
	}
	if !h.groupCmds.Delete(cmd) {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Command gak ketemu", ShowAlert: true})
	}
	_ = h.syncTelegramCommands()
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("🗑 /%s dihapus", cmd)})
	return h.handleGroupCmd(c)
}

// syncTelegramCommands — register semua group commands ke Telegram via setMyCommands.
// Scope = AllGroupChats supaya autocomplete cuma muncul di group.
func (h *Handler) syncTelegramCommands() error {
	cmds := h.groupCmds.GetAll()
	teleCmds := make([]tele.Command, 0, len(cmds))
	for _, gc := range cmds {
		desc := gc.Description
		if desc == "" {
			desc = fmt.Sprintf("Daftar link %s", strings.ToUpper(gc.Command))
		}
		teleCmds = append(teleCmds, tele.Command{
			Text:        gc.Command,
			Description: desc,
		})
	}
	return h.bot.SetCommands(teleCmds, tele.CommandScope{Type: tele.CommandScopeAllGroupChats})
}

// handleGroupSlashCommand — dipanggil dari handleText kalau pesan di group + start dengan "/".
// Match command name → fetch links by project_id → reply dgn inline buttons.
func (h *Handler) handleGroupSlashCommand(c tele.Context, text string) error {
	// Strip "/" prefix + handle "@botname" suffix (e.g. "/rtp@BongBot" → "rtp")
	cmd := strings.TrimPrefix(text, "/")
	if at := strings.IndexByte(cmd, '@'); at != -1 {
		cmd = cmd[:at]
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	gc, ok := h.groupCmds.GetByCommand(cmd)
	if !ok {
		return nil // unknown command, ignore
	}

	// Fetch all links via API (Pixly API gak filter by project_id reliably → client-side)
	allLinks, err := h.klikcepat.ListLinks("")
	if err != nil {
		return c.Reply(fmt.Sprintf("⚠️ Gagal fetch link: %s", err.Error()))
	}

	var matched []klikcepat.Link
	for _, l := range allLinks {
		if int(l.ProjectID) != gc.ProjectID {
			continue
		}
		if l.IsEnabled.Int() != 1 {
			continue
		}
		matched = append(matched, l)
	}

	if len(matched) == 0 {
		return c.Reply(fmt.Sprintf("📭 Belum ada link di project *%s*.", escapeMD(gc.ProjectName)),
			tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()

	// Build inline buttons — 1 link per row, label = host+slug
	// Skip link yg location_url-nya saat ini blocked di sticky cache (best-effort).
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	safeCount := 0
	skippedBlocked := 0
	for _, l := range matched {
		full := klikcepat.BuildShortlinkURL(l, userMap, nil)
		// Health check: kalau destination keblock di sticky, skip
		if l.LocationURL != "" {
			host := extractHostFromURL(l.LocationURL)
			if host != "" {
				if blocked, _ := checker.Default().IsSticky(host); blocked {
					skippedBlocked++
					continue
				}
			}
		}
		label := strings.TrimPrefix(full, "https://")
		label = strings.TrimPrefix(label, "http://")
		rows = append(rows, m.Row(m.URL("🔗 "+truncate(label, 45), full)))
		safeCount++
	}

	if safeCount == 0 {
		return c.Reply(fmt.Sprintf(
			"⚠️ *Semua link di project %s lagi blocked.*\n\nLagi diproses team admin, tunggu sebentar ya.",
			escapeMD(gc.ProjectName)), tele.ModeMarkdown)
	}

	m.Inline(rows...)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎰 *%s — LINK TERSEDIA*\n", escapeMD(strings.ToUpper(gc.ProjectName))))
	sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
	if gc.Description != "" {
		sb.WriteString(fmt.Sprintf("_%s_\n\n", escapeMD(gc.Description)))
	}
	sb.WriteString(fmt.Sprintf("✅ %d link aktif", safeCount))
	if skippedBlocked > 0 {
		sb.WriteString(fmt.Sprintf(" • ⚠️ %d link sementara di-skip (blocked)", skippedBlocked))
	}
	sb.WriteString("\n\n_Klik tombol di bawah buat akses._")

	return c.Reply(sb.String(), m, tele.ModeMarkdown)
}

// isValidCommandName — Telegram bot command rules: lowercase, alphanumeric + underscore, 1-32 chars.
func isValidCommandName(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// _unused but kept for context — ensures imports
var _ = context.Background
var _ = time.Now
