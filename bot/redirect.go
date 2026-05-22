package bot

import (
	"fmt"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleCF(c tele.Context) error {
	return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
}

// ─── Add CF Rule ──────────────────────────────────────────────────────────────

func (h *Handler) handleCFAdd(c tele.Context) error {
	prompt := "⚙️ *Add CF Rule — Step 1/5*\n\nKetik *label* untuk rule ini:\n_(contoh: MAIN REDIRECT, PROMO)_"
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepCFAddLabel,
		Data:      make(map[string]string),
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardCFAddLabel(c tele.Context, sess *Session) error {
	label := strings.TrimSpace(c.Text())
	if label == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Label tidak boleh kosong:", cancelMenu(), tele.ModeMarkdown)
		return nil
	}
	sess.Data["label"] = label
	sess.Step = StepCFAddZone
	h.sessions.Set(c.Sender().ID, sess)
	h.bot.Edit(sess.PromptMsg,
		"⚙️ *Add CF Rule — Step 2/5*\n\nKetik *Zone ID* domain di Cloudflare:",
		cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardCFAddZone(c tele.Context, sess *Session) error {
	zoneID := strings.TrimSpace(c.Text())
	if zoneID == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Zone ID tidak boleh kosong:", cancelMenu(), tele.ModeMarkdown)
		return nil
	}
	sess.Data["zone_id"] = zoneID
	sess.Step = StepCFAddType
	h.sessions.Set(c.Sender().ID, sess)

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("Redirect Rules (v1)", cbCFAdd, "redirect_rules"),
			m.Data("Page Rules (v2)", cbCFAdd, "page_rules"),
		),
		m.Row(m.Data("❌ Batal", cbCancel)),
	)
	h.bot.Edit(sess.PromptMsg,
		"⚙️ *Add CF Rule — Step 3/5*\n\nPilih tipe rule:",
		m, tele.ModeMarkdown)
	return nil
}

func (h *Handler) handleCFAddTypeSelect(c tele.Context) error {
	ruleType := c.Data()
	if ruleType == "" {
		return h.handleCF(c)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFAddType {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}
	sess.Data["type"] = ruleType
	if ruleType == "redirect_rules" {
		sess.Step = StepCFAddRuleset
		h.sessions.Set(c.Sender().ID, sess)
		h.bot.Edit(sess.PromptMsg,
			"⚙️ *Add CF Rule — Step 4/5*\n\nKetik *Ruleset ID*:",
			cancelMenu(), tele.ModeMarkdown)
	} else {
		// page_rules: skip ruleset, langsung ke rule_id
		sess.Step = StepCFAddRuleID
		h.sessions.Set(c.Sender().ID, sess)
		h.bot.Edit(sess.PromptMsg,
			"⚙️ *Add CF Rule — Step 4/5*\n\nKetik *Page Rule ID*:",
			cancelMenu(), tele.ModeMarkdown)
	}
	return nil
}

func (h *Handler) wizardCFAddRuleset(c tele.Context, sess *Session) error {
	rulesetID := strings.TrimSpace(c.Text())
	if rulesetID == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Ruleset ID tidak boleh kosong:", cancelMenu(), tele.ModeMarkdown)
		return nil
	}
	sess.Data["ruleset_id"] = rulesetID
	sess.Step = StepCFAddRuleID
	h.sessions.Set(c.Sender().ID, sess)
	h.bot.Edit(sess.PromptMsg,
		"⚙️ *Add CF Rule — Step 5/5*\n\nKetik *Rule ID*:",
		cancelMenu(), tele.ModeMarkdown)
	return nil
}

func (h *Handler) wizardCFAddRuleID(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	ruleID := strings.TrimSpace(c.Text())
	if ruleID == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Rule ID tidak boleh kosong", backToCF(), tele.ModeMarkdown)
		return nil
	}
	sess.Data["rule_id"] = ruleID

	rule := store.CFRule{
		Label:     sess.Data["label"],
		ZoneID:    sess.Data["zone_id"],
		Type:      sess.Data["type"],
		RulesetID: sess.Data["ruleset_id"],
		RuleID:    ruleID,
	}
	h.cfrules.Add(rule)

	h.bot.Edit(sess.PromptMsg,
		fmt.Sprintf("✅ *CF Rule ditambahkan!*\n\n📛 Label: *%s*\n🌐 Zone: `%s`\n📌 Tipe: `%s`\n🔑 Rule ID: `%s`",
			rule.Label, rule.ZoneID, rule.Type, rule.RuleID),
		backToCF(), tele.ModeMarkdown)
	return nil
}

// ─── List CF Rules ────────────────────────────────────────────────────────────

func (h *Handler) handleCFList(c tele.Context) error {
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit("📭 Belum ada CF rule terdaftar.\n\nGunakan *Add Rule* untuk menambahkan.", backToCF(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📋 *CF Rules*\n═══════════════\n\n")
	for i, r := range rules {
		sb.WriteString(fmt.Sprintf("%d. *%s*\n", i+1, r.Label))
		sb.WriteString(fmt.Sprintf("   🌐 Zone: `%s`\n", r.ZoneID))
		sb.WriteString(fmt.Sprintf("   📌 Tipe: `%s`\n", r.Type))
		sb.WriteString(fmt.Sprintf("   🔑 Rule ID: `%s`\n\n", r.RuleID))
	}
	return c.Edit(sb.String(), backToCF(), tele.ModeMarkdown)
}

// ─── Change URL ───────────────────────────────────────────────────────────────

func (h *Handler) handleCFChangeMenu(c tele.Context) error {
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit("📭 Belum ada CF rule. Tambah rule dulu.", backToCF(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, r := range rules {
		rows = append(rows, m.Row(m.Data("✏️ "+r.Label, cbCFChange, r.ID)))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbCF)))
	m.Inline(rows...)

	return c.Edit("✏️ *Ganti URL*\n\nPilih rule yang mau diganti URL-nya:", m, tele.ModeMarkdown)
}

func (h *Handler) handleCFChangeSelect(c tele.Context) error {
	ruleID := c.Data()
	if ruleID == "" {
		return h.handleCFChangeMenu(c)
	}

	rule, ok := h.cfrules.GetByID(ruleID)
	if !ok {
		return c.Edit("❌ Rule tidak ditemukan", backToCF(), tele.ModeMarkdown)
	}

	// Fetch current URL
	currentURL := "*(gagal fetch)*"
	if url, err := h.cf.GetCurrentURL(rule); err == nil {
		currentURL = url
	}

	prompt := fmt.Sprintf(
		"✏️ *Ganti URL — %s*\n\nURL saat ini:\n`%s`\n\nKetik URL baru:",
		rule.Label, currentURL,
	)
	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step:      StepCFChangeURL,
		Data:      map[string]string{"rule_id": ruleID},
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardCFChangeURL(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	newURL := strings.TrimSpace(c.Text())
	if newURL == "" {
		h.bot.Edit(sess.PromptMsg, "❌ URL tidak boleh kosong", backToCF(), tele.ModeMarkdown)
		return nil
	}

	rule, ok := h.cfrules.GetByID(sess.Data["rule_id"])
	if !ok {
		h.bot.Edit(sess.PromptMsg, "❌ Rule tidak ditemukan", backToCF(), tele.ModeMarkdown)
		return nil
	}

	h.bot.Edit(sess.PromptMsg, "⏳ Mengupdate CF rule...", tele.ModeMarkdown)

	if err := h.cf.UpdateURL(rule, newURL); err != nil {
		h.bot.Edit(sess.PromptMsg,
			fmt.Sprintf("❌ *Gagal update!*\n\nError: %v", err),
			backToCF(), tele.ModeMarkdown)
		return nil
	}

	h.bot.Edit(sess.PromptMsg,
		fmt.Sprintf("✅ *URL berhasil diubah!*\n\n📛 Rule: *%s*\n🔗 URL Baru: `%s`",
			rule.Label, newURL),
		backToCF(), tele.ModeMarkdown)
	return nil
}

// ─── Delete CF Rule ───────────────────────────────────────────────────────────

func (h *Handler) handleCFDeleteMenu(c tele.Context) error {
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit("📭 Belum ada CF rule.", backToCF(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, r := range rules {
		rows = append(rows, m.Row(m.Data("🗑 "+r.Label, cbCFDelete, r.ID)))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbCF)))
	m.Inline(rows...)

	return c.Edit("🗑 *Hapus CF Rule*\n\nPilih rule yang mau dihapus:", m, tele.ModeMarkdown)
}

func (h *Handler) handleCFDeleteConfirm(c tele.Context) error {
	ruleID := c.Data()
	if ruleID == "" {
		return h.handleCFDeleteMenu(c)
	}

	rule, ok := h.cfrules.GetByID(ruleID)
	if !ok {
		return c.Edit("❌ Rule tidak ditemukan", backToCF(), tele.ModeMarkdown)
	}

	h.cfrules.Delete(ruleID)

	return c.Edit(
		fmt.Sprintf("🗑 *CF Rule dihapus!*\n\n📛 Label: *%s*", rule.Label),
		backToCF(), tele.ModeMarkdown,
	)
}
