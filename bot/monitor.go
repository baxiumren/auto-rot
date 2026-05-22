package bot

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"bongbot/checker"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleMonitor(c tele.Context) error {
	return c.Edit(textMonitor, monitorMenu(), tele.ModeMarkdown)
}

// ─── Add Domain ───────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorAdd(c tele.Context) error {
	prompt := "📝 *Add Domain*\n\nKetik domain yang mau ditambahkan:\n_(contoh: example\\.com)_"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorAddDomain,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorAddDomain(c tele.Context, sess *Session) error {
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Domain tidak valid, coba lagi:", cancelMenu(), tele.ModeMarkdown)
		return nil
	}
	sess.Data["domain"] = domain
	sess.Step = StepMonitorAddLabel
	h.sessions.Set(c.Sender().ID, sess)

	// Tampilkan existing labels sebagai pilihan + input manual
	labels := h.domains.Labels()
	prompt := fmt.Sprintf("✅ Domain: `%s`\n\n📂 Ketik nama label/kategori:\n_(contoh: KWAI, STOCK-MS, dll)_", domain)
	if len(labels) > 0 {
		prompt += "\n\n*Label yang sudah ada:*\n" + strings.Join(labels, ", ")
	}

	// Buat inline buttons untuk label yang sudah ada
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	// Buat baris per 3 label
	var row tele.Row
	for i, lbl := range labels {
		row = append(row, m.Data(lbl, cbMonitorAdd, lbl))
		if len(row) == 3 || i == len(labels)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	h.bot.Edit(sess.PromptMsg, prompt, m, tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardMonitorAddLabel(c tele.Context, sess *Session) error {
	label := strings.ToUpper(strings.TrimSpace(c.Text()))
	if label == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Label tidak boleh kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
		return nil
	}
	return h.doAddDomain(c, sess, label)
}

func (h *Handler) handleMonitorAddLabelSelect(c tele.Context) error {
	label := c.Data()
	if label == "" {
		return h.handleMonitorAdd(c)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepMonitorAddLabel {
		return c.Edit(textMonitor, monitorMenu(), tele.ModeMarkdown)
	}
	return h.doAddDomain(c, sess, label)
}

func (h *Handler) doAddDomain(c tele.Context, sess *Session, label string) error {
	h.sessions.Delete(c.Sender().ID)
	domain := sess.Data["domain"]

	isMove, oldLabel := h.domains.Add(domain, label)

	var loadingMsg string
	if isMove {
		loadingMsg = fmt.Sprintf("✏️ *Domain dipindahkan!*\n🌐 `%s`\n📂 *%s* → *%s*\n\n⏳ Cek status nawala...", domain, oldLabel, label)
	} else {
		loadingMsg = fmt.Sprintf("✅ *Domain ditambahkan!*\n🌐 `%s`\n📂 Kategori: *%s*\n\n⏳ Cek status nawala...", domain, label)
	}

	h.bot.Edit(sess.PromptMsg, loadingMsg, tele.ModeMarkdown)

	go func() {
		status := checker.CheckDomain(domain)
		var statusLine string
		switch status {
		case "BLOCKED":
			statusLine = "🛑 Status: *DIBLOKIR KOMINFO*"
		case "SAFE":
			statusLine = "🟢 Status: *AMAN*"
		default:
			statusLine = "⚠️ Gagal cek status"
		}
		finalMsg := strings.Replace(loadingMsg, "\n\n⏳ Cek status nawala...", "\n"+statusLine, 1)
		h.bot.Edit(sess.PromptMsg, finalMsg, backToMonitor(), tele.ModeMarkdown)
	}()
	return nil
}

// ─── Remove Domain ────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorRemove(c tele.Context) error {
	prompt := "🗑 *Remove Domain*\n\nKetik domain yang mau dihapus:"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorRemove,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorRemove(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Domain tidak valid", backToMonitor(), tele.ModeMarkdown)
		return nil
	}
	label, found := h.domains.Remove(domain)
	if !found {
		h.bot.Edit(sess.PromptMsg,
			fmt.Sprintf("⚠️ Domain `%s` tidak ditemukan di list", domain),
			backToMonitor(), tele.ModeMarkdown)
		return nil
	}
	h.bot.Edit(sess.PromptMsg,
		fmt.Sprintf("🗑 *Domain dihapus!*\n🌐 `%s`\n📂 Kategori: *%s*", domain, label),
		backToMonitor(), tele.ModeMarkdown)
	return nil
}

// ─── Check Domain ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorCheck(c tele.Context) error {
	prompt := "🔍 *Cek Domain*\n\nKetik domain yang mau dicek:\n_(contoh: example\\.com)_"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorCheck,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorCheck(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	domain := store.CleanDomain(c.Text())
	if domain == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Domain tidak valid", backToMonitor(), tele.ModeMarkdown)
		return nil
	}
	h.bot.Edit(sess.PromptMsg,
		fmt.Sprintf("⏳ Mengecek `%s`...", domain),
		tele.ModeMarkdown)

	go func() {
		status := checker.CheckDomain(domain)
		label := h.domains.FindLabel(domain)
		inList := label != ""

		var msg string
		switch status {
		case "BLOCKED":
			msg = fmt.Sprintf("🛑 *DIBLOKIR KOMINFO*\n🌐 `%s`", domain)
			if inList {
				msg += fmt.Sprintf("\n📂 Kategori: *%s*", label)
			}
		case "SAFE":
			msg = fmt.Sprintf("🟢 *AMAN*\n🌐 `%s`", domain)
			if inList {
				msg += fmt.Sprintf("\n📂 Kategori: *%s*", label)
			}
		default:
			msg = fmt.Sprintf("⚠️ Gagal cek domain: `%s`", domain)
		}
		h.bot.Edit(sess.PromptMsg, msg, backToMonitor(), tele.ModeMarkdown)
	}()
	return nil
}

// ─── List Domain ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorList(c tele.Context) error {
	all := h.domains.GetAll()
	if len(all) == 0 {
		return c.Edit("📭 Belum ada domain terdaftar.", backToMonitor(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Domain List*\n═══════════════\n\n")

	labels := make([]string, 0, len(all))
	for l := range all {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	total := 0
	for _, label := range labels {
		domains := all[label]
		sb.WriteString(fmt.Sprintf("*[%s]* (%d domain)\n", label, len(domains)))
		for _, d := range domains {
			sb.WriteString(fmt.Sprintf("• `%s`\n", d))
		}
		sb.WriteString("\n")
		total += len(domains)
	}
	sb.WriteString(fmt.Sprintf("📊 Total: *%d* domain", total))

	text := sb.String()
	if len(text) > 3800 {
		text = text[:3800] + "\n\n_(dipotong — terlalu panjang)_"
	}
	return c.Edit(text, backToMonitor(), tele.ModeMarkdown)
}

// ─── Status Blocked ───────────────────────────────────────────────────────────

func (h *Handler) handleMonitorStatus(c tele.Context) error {
	blocked := h.rotSvc.GetBlockedDomains()
	if len(blocked) == 0 {
		return c.Edit("✅ *Tidak ada domain yang terblokir saat ini.*", backToMonitor(), tele.ModeMarkdown)
	}
	var sb strings.Builder
	sb.WriteString("🚨 *Domain Terblokir*\n═══════════════\n\n")
	for domain, since := range blocked {
		sb.WriteString(fmt.Sprintf("🔴 `%s`\n📅 Sejak: %s\n\n", domain, since.Format("02/01 15:04")))
	}
	return c.Edit(sb.String(), backToMonitor(), tele.ModeMarkdown)
}

// ─── Set Interval ─────────────────────────────────────────────────────────────

func (h *Handler) handleMonitorInterval(c tele.Context) error {
	current := h.rotSvc.GetInterval()
	prompt := fmt.Sprintf(
		"⏱ *Set Interval Cek*\n\nInterval saat ini: *%v*\n\nKetik interval baru:\n_(contoh: 30s, 1m, 2m30s)_",
		current,
	)
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepMonitorInterval,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardMonitorInterval(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	d, err := time.ParseDuration(strings.TrimSpace(c.Text()))
	if err != nil || d < 10*time.Second {
		h.bot.Edit(sess.PromptMsg,
			"❌ Interval tidak valid. Minimal 10s\n_(contoh: 30s, 1m, 2m30s)_",
			cancelMenu(), tele.ModeMarkdown)
		return nil
	}
	h.rotSvc.SetInterval(d)
	h.bot.Edit(sess.PromptMsg,
		fmt.Sprintf("✅ Interval diubah ke *%v*", d),
		backToMonitor(), tele.ModeMarkdown)
	return nil
}
