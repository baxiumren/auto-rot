package bot

import (
	tele "gopkg.in/telebot.v3"
)

// ─── Klikcepat Root Handler ──────────────────────────────────────────────────
//
// Entry point for the 🔗 KLIKCEPAT menu. Sub-handlers (link CRUD, project CRUD,
// rotator wizard, settings) live in separate files:
//   - klikcepat_link.go      — link CRUD wizards
//   - klikcepat_project.go   — project CRUD
//   - klikcepat_rotator.go   — auto-swap rotator wizard
//   - settings_klikcepat.go  — credentials management
//
// This file keeps the root navigation handler only — multi-file structure
// makes debugging individual flows easier.

func (h *Handler) handleKlikcepat(c tele.Context) error {
	if !h.klikcepat.HasCredentials() {
		return c.Edit(
			"⚠️ *Klikcepat credentials belum di-set*\n\n"+
				"Set Base URL & API Key dulu lewat menu *🔧 Settings → 🔗 Klikcepat* sebelum pakai fitur ini.",
			backToMain(), tele.ModeMarkdown,
		)
	}
	return c.Edit(textKlikcepat, klikcepatMenu(h.cfg.BotUsername), tele.ModeMarkdown)
}
