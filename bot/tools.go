package bot

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// ─── Info Tools — Telegram utility commands ──────────────────────────────────
//
// Buttons-based UI buat command:
//   /id @username        → user ID
//   /cekid <t.me link>   → chat/channel ID
//   /info @username      → user info lengkap
//   /cinfo @username     → chat/channel info lengkap
//   /help                → list semua tool

// handleTools — entry: tampilin picker tombol semua tool.
func (h *Handler) handleTools(c tele.Context) error {
	text := "💎 *I N F O   T O O L S* 💎\n" +
		"|\n" +
		"🛠 *TOOLS TERSEDIA*\n" +
		"└ 🆔 Get User ID — dari `@username`\n" +
		"└ 📨 Get Chat ID — dari `t.me` link\n" +
		"└ 👤 User Info — info lengkap user\n" +
		"└ 💬 Chat Info — info lengkap channel/group\n" +
		"└ 📚 Help — list semua tool\n" +
		"|\n" +
		"💡 *NOTE*\n" +
		"└ Bot bisa resolve user yg pernah interact\n" +
		"└ Channel/group public selalu bisa di-resolve\n" +
		"|\n" +
		"🎯 Klik tool di bawah 👇"

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("🆔 User ID", cbToolsUserID),
			m.Data("📨 Chat ID", cbToolsChatID),
		),
		m.Row(
			m.Data("👤 User Info", cbToolsUserInfo),
			m.Data("💬 Chat Info", cbToolsChatInfo),
		),
		m.Row(m.Data("📚 Help", cbToolsHelp)),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	)
	return c.Edit(text, m, tele.ModeMarkdown)
}

// ─── 🆔 User ID ──────────────────────────────────────────────────────────────

func (h *Handler) handleToolsUserID(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepToolsUserIDInput,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *G E T   U S E R   I D* 💎\n"+
			"|\n"+
			"🆔 *INPUT*\n"+
			"└ Ketik `@username` user\n"+
			"└ Contoh: `@durov`, `@lupis_keju`\n"+
			"|\n"+
			"💡 *NOTE*\n"+
			"└ User harus pernah interact dengan bot\n"+
			"   atau sama-sama di group dengan bot\n"+
			"└ Username harus public",
		h.backToTools(), tele.ModeMarkdown)
}

func (h *Handler) wizardToolsUserID(c tele.Context, sess *Session) error {
	h.showTyping(c)
	username := normalizeUsername(c.Text())
	h.sessions.Delete(c.Sender().ID)

	if username == "" {
		return h.reply(c, "❌ Format invalid. Coba lagi via 🆔 User ID", h.backToTools(), tele.ModeMarkdown)
	}

	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return h.reply(c,
			fmt.Sprintf(
				"💎 *G E T   U S E R   I D* 💎\n"+
					"|\n"+
					"❌ *GAGAL RESOLVE*\n"+
					"└ Username : `@%s`\n"+
					"└ Error    : `%s`\n"+
					"|\n"+
					"💡 *FIX*\n"+
					"└ Cek username bener\n"+
					"└ User belum pernah interact dengan bot",
				username, escapeMD(err.Error())),
			h.backToTools(), tele.ModeMarkdown)
	}

	return h.reply(c,
		fmt.Sprintf(
			"💎 *U S E R   I D* 💎\n"+
				"|\n"+
				"✅ *RESOLVED*\n"+
				"└ Username : @%s\n"+
				"└ 🆔 ID    : `%d`",
			username, chat.ID),
		h.backToTools(), tele.ModeMarkdown)
}

// ─── 📨 Chat ID ──────────────────────────────────────────────────────────────

func (h *Handler) handleToolsChatID(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepToolsChatIDInput,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *G E T   C H A T   I D* 💎\n"+
			"|\n"+
			"📨 *INPUT*\n"+
			"└ Paste `t.me` link channel/group\n"+
			"|\n"+
			"📝 *CONTOH*\n"+
			"└ `https://t.me/durov`\n"+
			"└ `https://t.me/c/123456789/1` (private)\n"+
			"|\n"+
			"💡 *NOTE*\n"+
			"└ Bisa juga ketik `@username` langsung",
		h.backToTools(), tele.ModeMarkdown)
}

func (h *Handler) wizardToolsChatID(c tele.Context, sess *Session) error {
	h.showTyping(c)
	input := strings.TrimSpace(c.Text())
	h.sessions.Delete(c.Sender().ID)

	username, chatID := parseTmeLink(input)
	if username == "" && chatID == 0 {
		return h.reply(c,
			"❌ Format gak valid. Pakai `https://t.me/username` atau `@username`",
			h.backToTools(), tele.ModeMarkdown)
	}

	// Private chat link (already has ID)
	if chatID != 0 {
		return h.reply(c,
			fmt.Sprintf(
				"💎 *C H A T   I D* 💎\n"+
					"|\n"+
					"✅ *PARSED FROM LINK*\n"+
					"└ Format  : private (`/c/...`)\n"+
					"└ Raw ID  : `%d`\n"+
					"└ 🆔 Full : `-100%d`\n"+
					"|\n"+
					"💡 Pakai `-100%d` untuk reference",
				chatID, chatID, chatID),
			h.backToTools(), tele.ModeMarkdown)
	}

	// Public link → resolve via getChat
	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return h.reply(c,
			fmt.Sprintf(
				"💎 *G E T   C H A T   I D* 💎\n"+
					"|\n"+
					"❌ *GAGAL RESOLVE*\n"+
					"└ Username : `@%s`\n"+
					"└ Error    : `%s`",
				username, escapeMD(err.Error())),
			h.backToTools(), tele.ModeMarkdown)
	}

	chatType := "Private"
	switch chat.Type {
	case tele.ChatGroup:
		chatType = "Group"
	case tele.ChatSuperGroup:
		chatType = "Supergroup"
	case tele.ChatChannel:
		chatType = "Channel"
	case tele.ChatPrivate:
		chatType = "Private (user)"
	}

	return h.reply(c,
		fmt.Sprintf(
			"💎 *C H A T   I D* 💎\n"+
				"|\n"+
				"✅ *RESOLVED*\n"+
				"└ Username : @%s\n"+
				"└ Type     : %s\n"+
				"└ 🆔 ID    : `%d`\n"+
				"└ Title    : %s",
			username, chatType, chat.ID, escapeMD(emptyOrValue(chat.Title))),
		h.backToTools(), tele.ModeMarkdown)
}

// ─── 👤 User Info ────────────────────────────────────────────────────────────

func (h *Handler) handleToolsUserInfo(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepToolsUserInfoInput,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *U S E R   I N F O* 💎\n"+
			"|\n"+
			"👤 *INPUT*\n"+
			"└ Ketik `@username` user\n"+
			"└ Contoh: `@durov`\n"+
			"|\n"+
			"📋 *YG BAKAL DIBALAS*\n"+
			"└ User ID, name, username, bio, premium",
		h.backToTools(), tele.ModeMarkdown)
}

func (h *Handler) wizardToolsUserInfo(c tele.Context, sess *Session) error {
	h.showTyping(c)
	username := normalizeUsername(c.Text())
	h.sessions.Delete(c.Sender().ID)

	if username == "" {
		return h.reply(c, "❌ Format invalid", h.backToTools(), tele.ModeMarkdown)
	}

	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return h.reply(c,
			fmt.Sprintf(
				"💎 *U S E R   I N F O* 💎\n"+
					"|\n"+
					"❌ *GAGAL RESOLVE*\n"+
					"└ @%s\n"+
					"└ Error: `%s`",
				username, escapeMD(err.Error())),
			h.backToTools(), tele.ModeMarkdown)
	}

	firstName := emptyOrValue(chat.FirstName)
	lastName := emptyOrValue(chat.LastName)
	bio := emptyOrValue(chat.Bio)
	fullName := strings.TrimSpace(firstName + " " + lastName)
	if fullName == "(empty)" {
		fullName = "(no name)"
	}

	return h.reply(c,
		fmt.Sprintf(
			"💎 *U S E R   I N F O* 💎\n"+
				"|\n"+
				"👤 *PROFILE*\n"+
				"└ 🆔 ID         : `%d`\n"+
				"└ Username     : @%s\n"+
				"└ Name         : %s\n"+
				"└ First Name   : %s\n"+
				"└ Last Name    : %s\n"+
				"|\n"+
				"📝 *BIO*\n"+
				"└ %s",
			chat.ID, username, escapeMD(fullName),
			escapeMD(firstName), escapeMD(lastName), escapeMD(bio)),
		h.backToTools(), tele.ModeMarkdown)
}

// ─── 💬 Chat Info ────────────────────────────────────────────────────────────

func (h *Handler) handleToolsChatInfo(c tele.Context) error {
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepToolsChatInfoInput,
		Data:      make(map[string]string),
		PromptMsg: c.Message(),
	})
	return c.Edit(
		"💎 *C H A T   I N F O* 💎\n"+
			"|\n"+
			"💬 *INPUT*\n"+
			"└ Ketik `@username` channel/group\n"+
			"└ Atau paste `t.me` link\n"+
			"|\n"+
			"📋 *YG BAKAL DIBALAS*\n"+
			"└ ID, type, title, description, member count",
		h.backToTools(), tele.ModeMarkdown)
}

func (h *Handler) wizardToolsChatInfo(c tele.Context, sess *Session) error {
	h.showTyping(c)
	input := strings.TrimSpace(c.Text())
	h.sessions.Delete(c.Sender().ID)

	username, _ := parseTmeLink(input)
	if username == "" {
		// Try direct @username
		username = normalizeUsername(input)
	}
	if username == "" {
		return h.reply(c, "❌ Format invalid", h.backToTools(), tele.ModeMarkdown)
	}

	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return h.reply(c,
			fmt.Sprintf(
				"💎 *C H A T   I N F O* 💎\n"+
					"|\n"+
					"❌ *GAGAL RESOLVE*\n"+
					"└ @%s\n"+
					"└ Error: `%s`",
				username, escapeMD(err.Error())),
			h.backToTools(), tele.ModeMarkdown)
	}

	chatType := "Private (user)"
	switch chat.Type {
	case tele.ChatGroup:
		chatType = "Group"
	case tele.ChatSuperGroup:
		chatType = "Supergroup"
	case tele.ChatChannel:
		chatType = "Channel"
	}

	memberLine := ""
	if count, err := h.bot.Len(chat); err == nil && count > 0 {
		memberLine = fmt.Sprintf("└ Members      : %d\n", count)
	}

	desc := emptyOrValue(chat.Description)

	return h.reply(c,
		fmt.Sprintf(
			"💎 *C H A T   I N F O* 💎\n"+
				"|\n"+
				"💬 *PROFILE*\n"+
				"└ 🆔 ID         : `%d`\n"+
				"└ Username     : @%s\n"+
				"└ Type         : %s\n"+
				"└ Title        : %s\n"+
				"%s"+
				"|\n"+
				"📝 *DESCRIPTION*\n"+
				"└ %s",
			chat.ID, username, chatType,
			escapeMD(emptyOrValue(chat.Title)),
			memberLine,
			escapeMD(desc)),
		h.backToTools(), tele.ModeMarkdown)
}

// ─── 📚 Help ─────────────────────────────────────────────────────────────────

func (h *Handler) handleToolsHelp(c tele.Context) error {
	return c.Edit(
		"💎 *T O O L S   H E L P* 💎\n"+
			"|\n"+
			"📚 *DAFTAR COMMAND*\n"+
			"|\n"+
			"🆔 */id @username*\n"+
			"└ Get user ID dari username\n"+
			"└ Contoh: `/id @durov`\n"+
			"|\n"+
			"📨 */cekid <link>*\n"+
			"└ Get chat/channel ID\n"+
			"└ Contoh: `/cekid https://t.me/durov`\n"+
			"└ Contoh: `/cekid https://t.me/c/123456789/1`\n"+
			"|\n"+
			"👤 */info @username*\n"+
			"└ Info lengkap user (name, bio, dll)\n"+
			"└ Contoh: `/info @durov`\n"+
			"|\n"+
			"💬 */cinfo @username*\n"+
			"└ Info lengkap chat/channel\n"+
			"└ Contoh: `/cinfo @telegram`\n"+
			"|\n"+
			"📚 */help*\n"+
			"└ Tampilin help ini",
		h.backToTools(), tele.ModeMarkdown)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (h *Handler) backToTools() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbTools)))
	return m
}

// normalizeUsername: "@durov", "durov", "https://t.me/durov" → "durov"
func normalizeUsername(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "t.me/")
	s = strings.TrimPrefix(s, "@")
	// Remove anything after / or ? (e.g., t.me/durov/123)
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	// Validate: alphanumeric + underscore only
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return ""
		}
	}
	if len(s) < 4 || len(s) > 32 {
		return ""
	}
	return s
}

// parseTmeLink:
//   "https://t.me/durov" → username="durov", chatID=0
//   "https://t.me/c/123456789/1" → username="", chatID=123456789
//   "@durov" → username="durov", chatID=0
func parseTmeLink(input string) (username string, chatID int64) {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")

	// Direct @username
	if strings.HasPrefix(s, "@") {
		return normalizeUsername(s), 0
	}

	if !strings.HasPrefix(s, "t.me/") {
		// Maybe just "durov"
		return normalizeUsername(s), 0
	}

	s = strings.TrimPrefix(s, "t.me/")

	// Private link: c/123456789/1
	if strings.HasPrefix(s, "c/") {
		s = strings.TrimPrefix(s, "c/")
		// Get first numeric part
		if i := strings.IndexAny(s, "/?"); i >= 0 {
			s = s[:i]
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			return "", id
		}
		return "", 0
	}

	// Public link: durov or durov/123
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	return normalizeUsername(s), 0
}

func emptyOrValue(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

// ─── Slash Commands (direct, no wizard) ──────────────────────────────────────
// Bisa dipake di DM admin maupun di group (tanpa session).

// handleSlashID: /id @username → reply user ID
func (h *Handler) handleSlashID(c tele.Context) error {
	arg := strings.TrimSpace(c.Message().Payload)
	if arg == "" {
		return c.Reply(
			"💡 *USAGE*\n└ `/id @username`\n└ Contoh: `/id @durov`",
			tele.ModeMarkdown)
	}
	username := normalizeUsername(arg)
	if username == "" {
		return c.Reply("❌ Format invalid. Pakai `@username`", tele.ModeMarkdown)
	}
	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return c.Reply(
			fmt.Sprintf("❌ Gagal resolve `@%s`\n└ `%s`",
				username, escapeMD(err.Error())),
			tele.ModeMarkdown)
	}
	return c.Reply(
		fmt.Sprintf(
			"🆔 *USER ID*\n"+
				"└ @%s\n"+
				"└ ID : `%d`",
			username, chat.ID),
		tele.ModeMarkdown)
}

// handleSlashCekID: /cekid <link> → reply chat/channel ID
func (h *Handler) handleSlashCekID(c tele.Context) error {
	arg := strings.TrimSpace(c.Message().Payload)
	if arg == "" {
		return c.Reply(
			"💡 *USAGE*\n└ `/cekid <link>`\n"+
				"└ Contoh: `/cekid https://t.me/durov`\n"+
				"└ Contoh: `/cekid https://t.me/c/123456789/1`",
			tele.ModeMarkdown)
	}
	username, chatID := parseTmeLink(arg)
	if chatID != 0 {
		return c.Reply(
			fmt.Sprintf(
				"📨 *CHAT ID*\n"+
					"└ Format : private link\n"+
					"└ Raw    : `%d`\n"+
					"└ Full   : `-100%d`",
				chatID, chatID),
			tele.ModeMarkdown)
	}
	if username == "" {
		return c.Reply("❌ Format invalid", tele.ModeMarkdown)
	}
	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return c.Reply(
			fmt.Sprintf("❌ Gagal resolve `@%s`\n└ `%s`",
				username, escapeMD(err.Error())),
			tele.ModeMarkdown)
	}
	return c.Reply(
		fmt.Sprintf(
			"📨 *CHAT ID*\n"+
				"└ @%s\n"+
				"└ Type  : %s\n"+
				"└ ID    : `%d`",
			username, chat.Type, chat.ID),
		tele.ModeMarkdown)
}

// handleSlashInfo: /info @username → reply user info
func (h *Handler) handleSlashInfo(c tele.Context) error {
	arg := strings.TrimSpace(c.Message().Payload)
	if arg == "" {
		return c.Reply(
			"💡 *USAGE*\n└ `/info @username`",
			tele.ModeMarkdown)
	}
	username := normalizeUsername(arg)
	if username == "" {
		return c.Reply("❌ Format invalid", tele.ModeMarkdown)
	}
	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return c.Reply(
			fmt.Sprintf("❌ Gagal resolve `@%s`", username),
			tele.ModeMarkdown)
	}
	fullName := strings.TrimSpace(chat.FirstName + " " + chat.LastName)
	if fullName == "" {
		fullName = "(no name)"
	}
	return c.Reply(
		fmt.Sprintf(
			"👤 *USER INFO*\n"+
				"└ ID       : `%d`\n"+
				"└ Username : @%s\n"+
				"└ Name     : %s\n"+
				"└ Bio      : %s",
			chat.ID, username, escapeMD(fullName), escapeMD(emptyOrValue(chat.Bio))),
		tele.ModeMarkdown)
}

// handleSlashCInfo: /cinfo @username → reply channel/group info
func (h *Handler) handleSlashCInfo(c tele.Context) error {
	arg := strings.TrimSpace(c.Message().Payload)
	if arg == "" {
		return c.Reply(
			"💡 *USAGE*\n└ `/cinfo @username`",
			tele.ModeMarkdown)
	}
	username, _ := parseTmeLink(arg)
	if username == "" {
		username = normalizeUsername(arg)
	}
	if username == "" {
		return c.Reply("❌ Format invalid", tele.ModeMarkdown)
	}
	chat, err := h.bot.ChatByUsername("@" + username)
	if err != nil {
		return c.Reply(
			fmt.Sprintf("❌ Gagal resolve `@%s`", username),
			tele.ModeMarkdown)
	}
	memberLine := ""
	if count, err := h.bot.Len(chat); err == nil && count > 0 {
		memberLine = fmt.Sprintf("\n└ Members  : %d", count)
	}
	return c.Reply(
		fmt.Sprintf(
			"💬 *CHAT INFO*\n"+
				"└ ID       : `%d`\n"+
				"└ Username : @%s\n"+
				"└ Type     : %s\n"+
				"└ Title    : %s%s\n"+
				"└ Desc     : %s",
			chat.ID, username, chat.Type,
			escapeMD(emptyOrValue(chat.Title)), memberLine,
			escapeMD(emptyOrValue(chat.Description))),
		tele.ModeMarkdown)
}

// handleSlashHelp: /help → list semua command
func (h *Handler) handleSlashHelp(c tele.Context) error {
	return c.Reply(
		"📚 *HELP — INFO TOOLS*\n"+
			"|\n"+
			"🆔 `/id @username`\n"+
			"└ Get user ID dari username\n"+
			"|\n"+
			"📨 `/cekid <link>`\n"+
			"└ Get chat/channel ID\n"+
			"└ Support t.me public & /c/ private link\n"+
			"|\n"+
			"👤 `/info @username`\n"+
			"└ Info lengkap user (name, bio)\n"+
			"|\n"+
			"💬 `/cinfo @username`\n"+
			"└ Info lengkap channel/group\n"+
			"|\n"+
			"📚 `/help`\n"+
			"└ Tampilin help ini\n"+
			"|\n"+
			"💡 *TIPS*\n"+
			"└ Pake tombol via 🏠 MENU → 🛠 Info Tools",
		tele.ModeMarkdown)
}
