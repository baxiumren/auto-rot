package bot

import (
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ─── Swap History ────────────────────────────────────────────────────────────
//
// Menu di Auto Rotator → 📜 Swap History.
// Tampilkan log semua swap (auto + manual + force + bulk), newest first.

const historyDisplayLimit = 20 // tampilkan max 20 entry terbaru di Telegram

func (h *Handler) handleHistory(c tele.Context) error {
	total := h.history.Count()
	if total == 0 {
		return c.Edit(
			"📜 *Swap History Kosong*\n\n"+
				"_Belum ada swap yang tercatat._\n\n"+
				"Bot otomatis log SEMUA swap (auto, manual, force, bulk) — nanti muncul di sini.",
			backToRotator(), tele.ModeMarkdown)
	}

	entries := h.history.GetRecent(historyDisplayLimit)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📜 *Swap History — Terbaru %d dari %d*\n", len(entries), total))
	sb.WriteString("═══════════════════════════\n\n")

	for i, e := range entries {
		icon := sourceIcon(e.Source)
		statusIcon := "✅"
		if !e.Success {
			statusIcon = "❌"
		}

		ts := e.Timestamp.Format("02/01 15:04:05")
		ago := timeAgoShort(e.Timestamp)

		dom := e.RuleDomain
		if dom == "" {
			dom = "?"
		}

		sb.WriteString(fmt.Sprintf("*%d. %s %s* — *%s* (`%s`)\n",
			i+1, icon, statusIcon, escapeMD(e.RuleLabel), escapeMD(dom)))
		sb.WriteString(fmt.Sprintf("   🕐 %s _(%s)_\n", ts, ago))

		fromHost := extractHostFromURL(e.FromURL)
		toHost := extractHostFromURL(e.ToURL)
		if fromHost != "" && toHost != "" && fromHost != toHost {
			sb.WriteString(fmt.Sprintf("   🔀 `%s` → `%s`\n", fromHost, toHost))
		} else if e.ToURL != "" {
			sb.WriteString(fmt.Sprintf("   🔗 `%s`\n", escapeMD(truncate(e.ToURL, 50))))
		}

		if !e.Success && e.ErrorMsg != "" {
			sb.WriteString(fmt.Sprintf("   ⚠️ _%s_\n", escapeMD(truncate(e.ErrorMsg, 80))))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("*Sumber swap:*\n")
	sb.WriteString("⚡ monitor-scan • 🔄 rotator • ✋ manual • 📦 bulk • 🔀 force\n\n")
	sb.WriteString(fmt.Sprintf("_Max 200 entry disimpan (`data/swap_history.json`). Sekarang: %d._", total))

	text := sb.String()
	if len(text) > 3800 {
		text = text[:3800] + "\n\n_(dipotong)_"
	}

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("🗑 Clear History", cbHistoryClear)),
		m.Row(m.Data("🔙 Kembali", cbRotator)),
	)
	return c.Edit(text, m, tele.ModeMarkdown)
}

func (h *Handler) handleHistoryClearConfirm(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("✅ Ya, Hapus Semua", cbHistoryClearYes),
			m.Data("❌ Batal", cbHistory),
		),
	)
	return c.Edit(
		"🗑 *Hapus Swap History?*\n\n"+
			"Ini hapus semua log swap dari `data/swap_history.json`.\n\n"+
			"_Action ini gak bisa di-undo. Tapi gak ngaruh ke CF setting atau rotator config — cuma hapus log._\n\n"+
			"Yakin?",
		m, tele.ModeMarkdown)
}

func (h *Handler) handleHistoryClearDo(c tele.Context) error {
	h.history.Clear()
	return c.Edit(
		"✅ *Swap history dibersihkan.*\n\n"+
			"_File `data/swap_history.json` di-reset. Swap baru bakal mulai tercatat lagi._",
		backToRotator(), tele.ModeMarkdown)
}

// sourceIcon: emoji per source.
func sourceIcon(source string) string {
	switch source {
	case "monitor-scan":
		return "⚡"
	case "rotator":
		return "🔄"
	case "manual":
		return "✋"
	case "bulk":
		return "📦"
	case "force":
		return "🔀"
	default:
		return "•"
	}
}

// timeAgoShort: format "5m ago", "2h ago", "3d ago"
func timeAgoShort(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "barusan"
	case d < time.Hour:
		return fmt.Sprintf("%dm lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dj lalu", int(d.Hours()))
	default:
		return fmt.Sprintf("%dh lalu", int(d.Hours()/24))
	}
}
