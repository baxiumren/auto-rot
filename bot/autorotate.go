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

// handleRotatorAdd — unified entry: pick type (CF / Klikcepat) before existing flow.
func (h *Handler) handleRotatorAdd(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("⚙️ CF Redirect", cbRotatorAddTypeCF),
			m.Data("🔗 KLIKCEPAT", cbRotatorAddTypeKlikcepat),
		),
		m.Row(m.Data("❌ Batal", cbCancel)),
	)
	return c.Edit(
		"🔄 *Setup Rotator — Pilih Tipe*\n\n"+
			"Pilih platform mana yang mau di-setup auto-swap-nya:\n\n"+
			"• *⚙️ CF Redirect* — auto-swap target URL Cloudflare rule\n"+
			"• *🔗 KLIKCEPAT* — auto-swap `location_url` link klikcepat",
		m, tele.ModeMarkdown)
}

// handleRotatorAddTypeCF — original CF rotator flow (was handleRotatorAdd before unification).
func (h *Handler) handleRotatorAddTypeCF(c tele.Context) error {
	allRules := h.cfrules.GetAll()
	if len(allRules) == 0 {
		return c.Edit(
			"⚠️ *Belum ada CF Rule terdaftar*\n\n"+
				"Auto Rotator butuh CF Rule untuk di-rotate. Tambah dulu rule kamu di menu *⚙️ CF Redirect → ➕ Add Rule*.\n\n"+
				"_Kalau bingung, balik ke menu utama dan ikutin urutan 1️⃣ Monitor → 2️⃣ CF Redirect → 3️⃣ Rotator._",
			backToRotator(), tele.ModeMarkdown)
	}

	// Filter: skip rule yang udah punya rotator config.
	// User harus hapus rotator lama dulu kalau mau bikin baru untuk CF rule yang sama.
	hasRotator := make(map[string]bool)
	for _, rot := range h.rotators.GetAll() {
		hasRotator[rot.CFRuleID] = true
	}
	var rules []store.CFRule
	for _, r := range allRules {
		if !hasRotator[r.ID] {
			rules = append(rules, r)
		}
	}

	if len(rules) == 0 {
		return c.Edit(
			fmt.Sprintf(
				"✅ *Semua CF Rule udah punya Rotator*\n\n"+
					"Total %d CF Rule, semuanya udah di-setup auto-rotate.\n\n"+
					"_Mau ganti pool? Hapus rotator lama dulu via *📋 List Rotator*._",
				len(allRules)),
			backToRotator(), tele.ModeMarkdown)
	}

	// Pilih CF rule
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, r := range rules {
		btn := "⚙️ " + r.Label
		if r.Domain != "" {
			btn = fmt.Sprintf("⚙️ %s (%s)", r.Label, r.Domain)
		}
		rows = append(rows, m.Row(m.Data(btn, cbRotatorCFSel, r.ID)))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	configuredCount := len(allRules) - len(rules)
	headerNote := ""
	if configuredCount > 0 {
		headerNote = fmt.Sprintf("\n_(%d rule udah punya rotator — disembunyikan dari list.)_", configuredCount)
	}

	return c.Edit(
		fmt.Sprintf(
			"🔄 *Setup Rotator — Langkah 1 dari 3*\n\n"+
				"Pilih *CF Rule* yang mau di-rotate otomatis:%s\n\n"+
				"_(Ini rule yang URL tujuannya akan otomatis diganti kalau domainnya kena nawala.)_",
			headerNote),
		m, tele.ModeMarkdown)
}

func (h *Handler) handleRotatorCFSelect(c tele.Context) error {
	cfRuleID := extractParam(c)
	if cfRuleID == "" {
		return h.handleRotatorAdd(c)
	}

	cfRule, ok := h.cfrules.GetByID(cfRuleID)
	if !ok {
		return c.Edit("❌ CF Rule tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}

	labels := h.domains.Labels()
	if len(labels) == 0 {
		return c.Edit(
			"⚠️ *Belum ada domain di Monitor*\n\n"+
				"Auto Rotator butuh *pool domain cadangan*. Tambah dulu domain di menu *📡 Monitor → ➕ Add Domain*.\n\n"+
				"_Minimal 2 domain di label yang sama biar bot ada pilihan saat swap._",
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
		fmt.Sprintf(
			"🔄 *Setup Rotator — Langkah 2 dari 3*\n\n"+
				"✅ CF Rule: *%s*\n\n"+
				"Sekarang pilih *pool domain cadangan* (dari label di Monitor):\n\n"+
				"_Kalau domain CF Rule kena nawala, bot bakal pilih domain lain dari pool ini sebagai pengganti._",
			cfRule.Label),
		m, tele.ModeMarkdown,
	)
}

func (h *Handler) handleRotatorPoolSelect(c tele.Context) error {
	// Param format: "cfRuleID|poolLabel"
	param := extractParam(c)
	parts := strings.SplitN(param, "|", 2)
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
		"🔄 *Setup Rotator — Langkah 3 dari 3 (Terakhir!)*\n\n"+
			"✅ CF Rule: *%s*\n"+
			"✅ Pool: *%s* (%d domain cadangan)\n\n"+
			"📛 *Ketik nama/label untuk rotator ini* (bebas — buat kamu identifikasi nanti).\n\n"+
			"_Contoh: `KWAI-MAIN`, `Promo-Toko`, `Backup-Iklan`._",
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
	label := strings.TrimSpace(c.Text())
	if label == "" {
		return h.reply(c, "❌ Label tidak boleh kosong", backToRotator(), tele.ModeMarkdown)
	}
	h.sessions.Delete(c.Sender().ID)

	cfRule, ok := h.cfrules.GetByID(sess.Data["cf_rule_id"])
	if !ok {
		return h.reply(c, "❌ CF Rule tidak ditemukan", backToRotator(), tele.ModeMarkdown)
	}

	rule := store.RotatorRule{
		Label:     label,
		CFRuleID:  sess.Data["cf_rule_id"],
		PoolLabel: sess.Data["pool_label"],
	}
	h.rotators.Add(rule)

	return h.reply(c, 
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
}

// ─── List Rotators ────────────────────────────────────────────────────────────

func (h *Handler) handleRotatorList(c tele.Context) error {
	cfRotators := h.rotators.GetAll()
	klcRotators := h.klikcepatRotators.GetAll()

	if len(cfRotators) == 0 && len(klcRotators) == 0 {
		return c.Edit(
			"📭 *Belum ada Auto Rotator*\n\n"+
				"Buat dulu lewat *➕ Setup Rotator*. Wizard 3 langkah aja, gak ribet.\n\n"+
				"_Syaratnya: udah ada CF Rule di menu CF Redirect + minimal 2 domain di pool Monitor._",
			backToRotator(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("📋 *Auto Rotator List*\n═══════════════════════════\n")

	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	if len(cfRotators) > 0 {
		sb.WriteString("\n═══ ⚙️ CF Redirect ═══\n")
		activeCF := 0
		for i, rot := range cfRotators {
			cfRule, _ := h.cfrules.GetByID(rot.CFRuleID)
			cfLabel := rot.CFRuleID
			if cfRule.Label != "" {
				cfLabel = cfRule.Label
			}
			status := "▶️ AKTIF"
			toggleIcon := "⏸ Pause"
			if !rot.Active {
				status = "⏸ PAUSE"
				toggleIcon = "▶️ Resume"
			} else {
				activeCF++
			}
			sb.WriteString(fmt.Sprintf("%d. *%s* %s\n", i+1, escapeMD(rot.Label), status))
			sb.WriteString(fmt.Sprintf("   ⚙️ CF: *%s*\n", escapeMD(cfLabel)))
			sb.WriteString(fmt.Sprintf("   📂 Pool: *%s*\n", escapeMD(rot.PoolLabel)))
			rows = append(rows, m.Row(
				m.Data(fmt.Sprintf("⚙️ %s", truncate(rot.Label, 18)), cbNoop),
				m.Data(toggleIcon, cbRotatorToggle, rot.ID),
				m.Data("🗑 Hapus", cbRotatorDelete, rot.ID),
			))
		}
		sb.WriteString(fmt.Sprintf("\n_%d CF rotator (%d aktif)_\n", len(cfRotators), activeCF))
	}

	if len(klcRotators) > 0 {
		sb.WriteString("\n═══ 🔗 KLIKCEPAT ═══\n")
		activeKLC := 0
		for i, rot := range klcRotators {
			status := "▶️ AKTIF"
			toggleIcon := "⏸ Pause"
			if !rot.Active {
				status = "⏸ PAUSE"
				toggleIcon = "▶️ Resume"
			} else {
				activeKLC++
			}
			typeIcon := "🔗"
			if rot.LinkType == "biolink" {
				typeIcon = "📄"
			}
			sb.WriteString(fmt.Sprintf("%d. *%s* %s\n", i+1, escapeMD(rot.Label), status))
			sb.WriteString(fmt.Sprintf("   %s Link: `/%s`\n", typeIcon, escapeMD(rot.LinkURL)))
			sb.WriteString(fmt.Sprintf("   📂 Pool: *%s*\n", escapeMD(rot.PoolLabel)))
			rows = append(rows, m.Row(
				m.Data(fmt.Sprintf("%s %s", typeIcon, truncate(rot.Label, 18)), cbNoop),
				m.Data(toggleIcon, cbKlikcepatRotToggle, rot.ID),
				m.Data("🗑 Hapus", cbKlikcepatRotDelete, rot.ID),
			))
		}
		sb.WriteString(fmt.Sprintf("\n_%d Klikcepat rotator (%d aktif)_\n", len(klcRotators), activeKLC))
	}

	totalActive := 0
	for _, r := range cfRotators {
		if r.Active {
			totalActive++
		}
	}
	for _, r := range klcRotators {
		if r.Active {
			totalActive++
		}
	}
	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n*Total:* %d CF + %d Klikcepat = %d rotator (%d aktif)",
		len(cfRotators), len(klcRotators), len(cfRotators)+len(klcRotators), totalActive))

	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotator)))
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// ─── Toggle Pause/Resume ──────────────────────────────────────────────────────

func (h *Handler) handleRotatorToggle(c tele.Context) error {
	rotID := extractParam(c)
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
	rotID := extractParam(c)
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
	rotID := extractParam(c)
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
