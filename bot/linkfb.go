package bot

import (
	tele "gopkg.in/telebot.v3"
)

// ─── LinkFB Root Handler ──────────────────────────────────────────────────
//
// LinkFB pakai engine Pixly yg sama dengan Klikcepat — instance terpisah.
// Scope: shortlink CRUD + project CRUD (no biolink edit).

func (h *Handler) handleLinkfb(c tele.Context) error {
	if !h.linkfb.HasCredentials() {
		return c.Edit(
			"💎 *L I N K F B* 💎\n"+
				"|\n"+
				"⚠️ *CREDENTIALS BELUM DI-SET*\n"+
				"└ Set Base URL + API Key dulu\n"+
				"|\n"+
				"🎯 *ACTION*\n"+
				"└ Menu: 🔧 Settings → 🔗 LinkFB",
			backToMain(), tele.ModeMarkdown,
		)
	}
	return c.Edit(textLinkfb, linkfbMenu(), tele.ModeMarkdown)
}

const textLinkfb = "💎 *L I N K F B* 💎\n" +
	"|\n" +
	"🔗 *FUNGSI*\n" +
	"└ Manage link & project di linkfb.io\n" +
	"└ Engine Pixly (sama kayak klikcepat)\n" +
	"|\n" +
	"🎯 *YG BISA DILAKUIN*\n" +
	"└ ➕ Tambah Link — bikin shortlink baru\n" +
	"└ 📋 List Link — liat semua link (paginated)\n" +
	"└ ✏️ Edit Link — update title/slug/target/project\n" +
	"└ 🗑 Hapus Link — delete permanent\n" +
	"└ 📁 Projects — manage project grouping\n" +
	"|\n" +
	"💡 *NOTE*\n" +
	"└ Scope shortlink edit aja (no biolink)\n" +
	"└ Biolink view-only di klikcepat web"

func linkfbMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	rows := []tele.Row{
		m.Row(
			m.Data("➕ Tambah Link", cbLinkfbAdd),
			m.Data("📋 List Link", cbLinkfbList),
		),
		m.Row(
			m.Data("✏️ Edit Link", cbLinkfbEdit),
			m.Data("🗑 Hapus Link", cbLinkfbDelete),
		),
		m.Row(
			m.Data("📁 Projects", cbLinkfbProjects),
		),
		m.Row(
			m.URL("🌐 Open Dashboard", "https://linkfb.io"),
		),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	}
	m.Inline(rows...)
	return m
}

func backToLinkfb() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbLinkfb)))
	return m
}

func linkfbProjectsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Tambah Project", cbLinkfbProjectAdd),
			m.Data("📋 List Projects", cbLinkfbProjectList),
		),
		m.Row(
			m.Data("✏️ Edit Project", cbLinkfbProjectEdit),
			m.Data("🗑 Hapus Project", cbLinkfbProjectDelete),
		),
		m.Row(m.Data("🔙 Kembali", cbLinkfb)),
	)
	return m
}

func backToLinkfbProjects() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbLinkfbProjects)))
	return m
}
