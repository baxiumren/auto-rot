package bot

import (
	"fmt"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleRotator(c tele.Context) error {
	return c.Edit(textRotator, rotatorMenu(), tele.ModeMarkdown)
}

// ─── Setup Rotator ────────────────────────────────────────────────────────────

func (h *Handler) handleRotatorAdd(c tele.Context) error {
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit("⚠️ Belum ada CF Rule.\n\nTambah CF Rule dulu di menu *⚙️ CF Redirect*.",
			backToRotator(), tele.ModeMarkdown)
	}

	// Pilih CF rule
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, r := range rules {
		rows = append(rows, m.Row(m.Data("⚙️ "+r.Label, cbRotatorCFSel, r.ID)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit("🔄 *Setup Rotator — Step 1/3*\n\nPilih CF Rule yang mau di-rotate:", m, tele.ModeMarkdown)
}

func (h *Handler) handleRotatorCFSelect(c tele.Context) error {
	cfRuleID := c.Data()
	if cfRuleID == "" {
		return h.handleRotatorAdd(c)
	}

	cfRule, ok := h.cfrules.GetByID(cfRuleID)
	if !ok {
		return c.Edit("❌ CF Rule tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}

	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit("⚠️ Belum ada domain di Monitor.\n\nTambah domain dulu di menu *📡 Monitor*.",
			backToRotator(), tele.ModeMarkdown)
	}

	// Pilih pool label dari Monitor
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, lbl := range labels {
		domains := h.domains.GetByLabel(lbl)
		btnText := fmt.Sprintf("📂 %s (%d domain)", lbl, len(domains))
		rows = append(rows, m.Row(m.Data(btnText, cbRotatorPool, cfRuleID+"|"+lbl)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	return c.Edit(
		fmt.Sprintf("🔄 *Setup Rotator — Step 2/3*\n\nCF Rule: *%s*\n\nPilih label pool domain dari Monitor:", cfRule.Label),
		m, tele.ModeMarkdown,
	)
}

func (h *Handler) handleRotatorPoolSelect(c tele.Context) error {
	data := c.Data()
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 {
		return h.handleRotatorAdd(c)
	}
	cfRuleID, poolLabel := parts[0], parts[1]

	cfRule, ok := h.cfrules.GetByID(cfRuleID)
	if !ok {
		return c.Edit("❌ CF Rule tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}

	domains := h.domains.GetByLabel(poolLabel)

	// Step 3: konfirmasi + input label rotator
	prompt := fmt.Sprintf(
		"🔄 *Setup Rotator — Step 3/3*\n\n"+
			"CF Rule: *%s*\n"+
			"Pool: *%s* (%d domain)\n\n"+
			"Ketik nama/label untuk rotator ini:",
		cfRule.Label, poolLabel, len(domains),
	)
	_ = domains

	msg, _ := h.bot.Edit(c.Message(), prompt, cancelMenu(), tele.ModeMarkdown)
	if msg == nil {
		msg = c.Message()
	}
	h.sessions.Set(c.Sender().ID, &Session{
		Step: StepRotatorAddLabel,
		Data: map[string]string{
			"cf_rule_id": cfRuleID,
			"pool_label": poolLabel,
		},
		PromptMsg: msg,
	})
	return nil
}

func (h *Handler) wizardRotatorAddLabel(c tele.Context, sess *Session) error {
	h.sessions.Delete(c.Sender().ID)
	label := strings.TrimSpace(c.Text())
	if label == "" {
		h.bot.Edit(sess.PromptMsg, "❌ Label tidak boleh kosong", backToRotator(), tele.ModeMarkdown)
		return nil
	}

	cfRule, ok := h.cfrules.GetByID(sess.Data["cf_rule_id"])
	if !ok {
		h.bot.Edit(sess.PromptMsg, "❌ CF Rule tidak ditemukan", backToRotator(), tele.ModeMarkdown)
		return nil
	}

	rule := store.RotatorRule{
		Label:      label,
		CFRuleID:   sess.Data["cf_rule_id"],
		PoolLabel:  sess.Data["pool_label"],
	}
	h.rotators.Add(rule)

	h.bot.Edit(sess.PromptMsg,
		fmt.Sprintf(
			"✅ *Rotator aktif!*\n\n"+
				"📛 Label: *%s*\n"+
				"⚙️ CF Rule: *%s*\n"+
				"📂 Pool: *%s*\n"+
				"▶️ Status: *AKTIF*",
			label, cfRule.Label, sess.Data["pool_label"],
		),
		backToRotator(), tele.ModeMarkdown,
	)
	return nil
}

// ─── List Rotators ────────────────────────────────────────────────────────────

func (h *Handler) handleRotatorList(c tele.Context) error {
	rotators := h.rotators.GetAll()
	if len(rotators) == 0 {
		return c.Edit("📭 Belum ada rotator.\n\nGunakan *Setup Rotator* untuk membuat.", backToRotator(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Auto Rotator List*\n═══════════════\n\n")

	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i, rot := range rotators {
		cfRule, _ := h.cfrules.GetByID(rot.CFRuleID)
		cfLabel := rot.CFRuleID
		if cfRule.Label != "" {
			cfLabel = cfRule.Label
		}

		status := "▶️ AKTIF"
		if !rot.Active {
			status = "⏸ PAUSE"
		}

		sb.WriteString(fmt.Sprintf("%d. *%s* %s\n", i+1, rot.Label, status))
		sb.WriteString(fmt.Sprintf("   ⚙️ CF: *%s*\n", cfLabel))
		sb.WriteString(fmt.Sprintf("   📂 Pool: *%s*\n\n", rot.PoolLabel))

		toggleText := "⏸ Pause"
		if !rot.Active {
			toggleText = "▶️ Resume"
		}
		rows = append(rows, m.Row(
			m.Data(toggleText+" "+rot.Label, cbRotatorToggle, rot.ID),
			m.Data("🗑 Hapus", cbRotatorDelete, rot.ID),
		))
	}

	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotator)))
	m.Inline(rows...)

	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// ─── Toggle Pause/Resume ──────────────────────────────────────────────────────

func (h *Handler) handleRotatorToggle(c tele.Context) error {
	rotID := c.Data()
	active, found := h.rotators.Toggle(rotID)
	if !found {
		return c.Edit("❌ Rotator tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}

	rot, _ := h.rotators.GetByID(rotID)
	status := "▶️ *AKTIF*"
	if !active {
		status = "⏸ *PAUSE*"
	}

	return c.Edit(
		fmt.Sprintf("✅ Rotator *%s* sekarang: %s", rot.Label, status),
		backToRotator(), tele.ModeMarkdown,
	)
}

// ─── Delete Rotator ───────────────────────────────────────────────────────────

func (h *Handler) handleRotatorDelete(c tele.Context) error {
	rotID := c.Data()
	rot, ok := h.rotators.GetByID(rotID)
	if !ok {
		return c.Edit("❌ Rotator tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}
	h.rotators.Delete(rotID)
	return c.Edit(
		fmt.Sprintf("🗑 *Rotator dihapus!*\n\n📛 Label: *%s*", rot.Label),
		backToRotator(), tele.ModeMarkdown,
	)
}

// ─── Force Rotate ─────────────────────────────────────────────────────────────

func (h *Handler) handleRotatorForce(c tele.Context) error {
	rotID := c.Data()
	rot, ok := h.rotators.GetByID(rotID)
	if !ok {
		return c.Edit("❌ Rotator tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}

	c.Edit(fmt.Sprintf("⏳ Memaksa rotasi *%s*...", rot.Label), tele.ModeMarkdown)

	go func() {
		err := h.rotSvc.ForceRotate(rot)
		if err != nil {
			h.bot.Send(&tele.Chat{ID: h.cfg.AllowedChatID},
				fmt.Sprintf("❌ Force rotate *%s* gagal: %v", rot.Label, err),
				tele.ModeMarkdown)
		}
	}()
	return nil
}
