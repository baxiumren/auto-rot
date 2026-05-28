package bot

import (
	"fmt"
	"strings"

	"bongbot/klikcepat"
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

// handleRotatorList — TYPE PICKER (CF | KLIKCEPAT)
func (h *Handler) handleRotatorList(c tele.Context) error {
	cfCount := len(h.rotators.GetAll())
	klcSL := len(h.klikcepatRotators.GetAll())
	klcBL := len(h.klikcepatBlockRotators.GetAll())
	klcTotal := klcSL + klcBL
	total := cfCount + klcTotal

	if total == 0 {
		return c.Edit(
			"📭 *Belum ada Auto Rotator*\n\n"+
				"Buat dulu lewat *➕ Setup Rotator*.",
			backToRotator(), tele.ModeMarkdown)
	}

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data(fmt.Sprintf("⚙️ CF Redirect (%d)", cfCount), cbRotatorListCF),
			m.Data(fmt.Sprintf("🔗 KLIKCEPAT (%d)", klcTotal), cbRotatorListKlc),
		),
		m.Row(m.Data("🔙 Kembali", cbRotator)),
	)
	return c.Edit(
		fmt.Sprintf(
			"📋 *Auto Rotator List — Pilih Tipe*\n\n"+
				"Total: *%d rotator* (⚙️ %d CF + 🔗 %d Klikcepat)\n\n"+
				"Pilih tipe yang mau ditampilin:",
			total, cfCount, klcTotal),
		m, tele.ModeMarkdown)
}

// handleRotatorListKlc — KLIKCEPAT sub-picker (BIOLINK | SHORTLINK)
func (h *Handler) handleRotatorListKlc(c tele.Context) error {
	klcSL := len(h.klikcepatRotators.GetAll())
	klcBL := len(h.klikcepatBlockRotators.GetAll())

	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data(fmt.Sprintf("📄 BIOLINK BLOCK (%d)", klcBL), cbRotatorListKlcBL),
			m.Data(fmt.Sprintf("🔗 SHORTLINK (%d)", klcSL), cbRotatorListKlcSL),
		),
		m.Row(m.Data("🔙 Kembali", cbRotatorList)),
	)
	return c.Edit(
		fmt.Sprintf(
			"🔗 *Klikcepat Rotator — Pilih Tipe*\n\n"+
				"• 📄 *BIOLINK BLOCK* — %d rotator\n"+
				"• 🔗 *SHORTLINK* — %d rotator",
			klcBL, klcSL),
		m, tele.ModeMarkdown)
}

// handleRotatorListCF — list CF rotators only.
func (h *Handler) handleRotatorListCF(c tele.Context) error {
	cfRotators := h.rotators.GetAll()
	if len(cfRotators) == 0 {
		return c.Edit(
			"📭 *Belum ada CF Rotator*\n\nBuat dulu lewat *➕ Setup Rotator → ⚙️ CF Redirect*.",
			h.backToRotatorList(), tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("⚙️ *CF Redirect Rotators*\n═══════════════════════════\n")

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
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
		sb.WriteString(fmt.Sprintf("\n%d. *%s* %s\n", i+1, escapeMD(rot.Label), status))
		sb.WriteString(fmt.Sprintf("   ⚙️ CF: *%s*\n", escapeMD(cfLabel)))
		sb.WriteString(fmt.Sprintf("   📂 Pool: *%s*\n", escapeMD(rot.PoolLabel)))
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("⚙️ %s", truncate(rot.Label, 18)), cbNoop),
			m.Data(toggleIcon, cbRotatorToggle, rot.ID),
			m.Data("🗑 Hapus", cbRotatorDelete, rot.ID),
		))
	}
	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n*Total:* %d rotator (%d aktif)", len(cfRotators), activeCF))

	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotatorList)))
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// handleRotatorListKlcShortlink — list klikcepat SHORTLINK rotators only.
func (h *Handler) handleRotatorListKlcShortlink(c tele.Context) error {
	klcRotators := h.klikcepatRotators.GetAll()
	if len(klcRotators) == 0 {
		return c.Edit(
			"📭 *Belum ada Shortlink Rotator*\n\nBuat dulu lewat *➕ Setup Rotator → 🔗 KLIKCEPAT → 🔗 SHORTLINK*.",
			h.backToKlcList(), tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()
	var sb strings.Builder
	sb.WriteString("🔗 *Klikcepat Shortlink Rotators*\n═══════════════════════════\n")

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
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
		displayURL := "/" + rot.LinkURL
		if h.klikcepat != nil && h.klikcepat.HasCredentials() {
			if link, err := h.klikcepat.GetLink(rot.LinkID); err == nil {
				displayURL = klikcepat.BuildShortlinkURL(*link, userMap, nil)
			}
		}
		sb.WriteString(fmt.Sprintf("\n%d. *%s* %s\n", i+1, escapeMD(rot.Label), status))
		sb.WriteString(fmt.Sprintf("   🔗 Link: `%s`\n", escapeMD(displayURL)))
		sb.WriteString(fmt.Sprintf("   📂 Pool: *%s*\n", escapeMD(rot.PoolLabel)))
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("🔗 %s", truncate(rot.Label, 18)), cbNoop),
			m.Data(toggleIcon, cbKlikcepatRotToggle, rot.ID),
			m.Data("🗑 Hapus", cbKlikcepatRotDelete, rot.ID),
		))
	}
	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n*Total:* %d rotator (%d aktif)", len(klcRotators), activeKLC))

	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotatorListKlc)))
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

// handleRotatorListKlcBiolink — list klikcepat BIOLINK BLOCK rotators only.
func (h *Handler) handleRotatorListKlcBiolink(c tele.Context) error {
	klcBlockRotators := h.klikcepatBlockRotators.GetAll()
	if len(klcBlockRotators) == 0 {
		return c.Edit(
			"📭 *Belum ada Biolink Block Rotator*\n\nBuat dulu lewat *➕ Setup Rotator → 🔗 KLIKCEPAT → 📄 BIOLINK*.",
			h.backToKlcList(), tele.ModeMarkdown)
	}

	userMap := h.creds.GetKlikcepatDomainMap()
	buildKlcURL := func(domainID int, slug string) string {
		if host, ok := userMap[domainID]; ok && host != "" {
			return fmt.Sprintf("https://%s/%s", host, slug)
		}
		return fmt.Sprintf("https://klikcepat.com/%s", slug)
	}

	var sb strings.Builder
	sb.WriteString("📄 *Klikcepat Biolink Block Rotators*\n═══════════════════════════\n")

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	activeBlk := 0
	for i, rot := range klcBlockRotators {
		status := "▶️ AKTIF"
		toggleIcon := "⏸ Pause"
		if !rot.Active {
			status = "⏸ PAUSE"
			toggleIcon = "▶️ Resume"
		} else {
			activeBlk++
		}
		blockName := rot.BlockName
		if blockName == "" {
			blockName = "(no name)"
		}
		biolinkURL := buildKlcURL(rot.BiolinkDomain, rot.BiolinkSlug)
		sb.WriteString(fmt.Sprintf("\n%d. *%s* %s\n", i+1, escapeMD(rot.Label), status))
		sb.WriteString(fmt.Sprintf("   📄 Biolink: `%s`\n", escapeMD(biolinkURL)))
		sb.WriteString(fmt.Sprintf("   🔘 Block: *%s*\n", escapeMD(blockName)))
		sb.WriteString(fmt.Sprintf("   📂 Pool: *%s*\n", escapeMD(rot.PoolLabel)))
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("📄 %s", truncate(rot.Label, 18)), cbNoop),
			m.Data(toggleIcon, cbKlcBlockRotToggle, rot.ID),
			m.Data("🗑 Hapus", cbKlcBlockRotDelete, rot.ID),
		))
	}
	sb.WriteString(fmt.Sprintf("\n━━━━━━━━━━━━━━━━━━\n*Total:* %d rotator (%d aktif)", len(klcBlockRotators), activeBlk))

	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbRotatorListKlc)))
	m.Inline(rows...)
	return c.Edit(sb.String(), m, tele.ModeMarkdown)
}

func (h *Handler) backToRotatorList() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbRotatorList)))
	return m
}

func (h *Handler) backToKlcList() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbRotatorListKlc)))
	return m
}


// ─── Toggle Pause/Resume ──────────────────────────────────────────────────────

func (h *Handler) handleRotatorToggle(c tele.Context) error {
	rotID := extractParam(c)
	active, found := h.rotators.Toggle(rotID)
	if !found {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}
	state := "▶️ AKTIF"
	if !active {
		state = "⏸ PAUSE"
	}
	rot, _ := h.rotators.GetByID(rotID)
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("%s → %s", rot.Label, state)})
	return h.handleRotatorListCF(c)
}

// ─── Delete Rotator ───────────────────────────────────────────────────────────

func (h *Handler) handleRotatorDelete(c tele.Context) error {
	rotID := extractParam(c)
	rot, ok := h.rotators.GetByID(rotID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}
	h.rotators.Delete(rotID)
	c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("🗑 %s dihapus", rot.Label)})
	return h.handleRotatorListCF(c)
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
