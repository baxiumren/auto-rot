package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"bongbot/cloudflare"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleCF(c tele.Context) error {
	return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
}

// ─── Add CF Rule (AUTO-DISCOVERY) ─────────────────────────────────────────────
//
// Flow baru: user cukup kasih label + nama domain, bot fetch sendiri Zone ID,
// list rules, dan auto-pick kalau cuma 1 atau kasih picker kalau banyak.

func (h *Handler) handleCFAdd(c tele.Context) error {
	h.cancelPriorPrompt(c, StepCFAddLabel)
	if !h.cf.HasCredentials() {
		return c.Edit(
			"⚠️ *CF Credentials belum di-set*\n\nSet email & API key dulu lewat menu *🔧 Settings* sebelum menambah rule.",
			backToCF(), tele.ModeMarkdown,
		)
	}

	prompt := "⚙️ *Tambah CF Rule — Langkah 1 dari 2*\n\n" +
		"📛 *Ketik label/nama* untuk rule ini.\n" +
		"_Label bebas, buat kamu identifikasi rule nanti di list._\n\n" +
		"*Contoh:*\n" +
		"• `MAIN-REDIRECT`\n" +
		"• `PROMO-KWAI`\n" +
		"• `TOKO-LANDING`"
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
	h.showTyping(c)
	label := strings.TrimSpace(c.Text())
	log.Printf("[CF_ADD] label step user=%d label=%q", c.Sender().ID, label)
	if label == "" {
		return h.reply(c, "❌ Label tidak boleh kosong:", cancelMenu(), tele.ModeMarkdown)
	}
	sess.Data["label"] = label
	sess.Step = StepCFAddDomain
	h.sessions.Set(c.Sender().ID, sess)

	newMsg, sendErr := h.bot.Send(c.Chat(),
		userTag(c.Sender())+" ⚙️ *Tambah CF Rule — Langkah 2 dari 2*\n\n"+
			"🌐 *Ketik nama domain* yang ada di akun Cloudflare kamu:\n\n"+
			"*Contoh:*\n"+
			"• `example.com`\n"+
			"• `mysite.net`\n"+
			"• `tokoku.id`\n\n"+
			"💡 _Gak perlu Zone ID atau Rule ID — bot bakal *auto-fetch* dari Cloudflare API. Pastikan domain udah didaftarkan di Cloudflare dan punya redirect rule aktif._",
		&tele.SendOptions{
			ReplyTo:     c.Message(),
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: cancelMenu(),
		})
	if sendErr != nil {
		log.Printf("[CF_ADD] gagal kirim Step 2 prompt: %v — fallback plain text", sendErr)
		// Fallback: kirim plain text tanpa markdown
		newMsg, _ = h.bot.Send(c.Chat(),
			"Add CF Rule - Step 2/2\n\nKetik nama domain di Cloudflare:\n(contoh: example.com)",
			cancelMenu())
	}
	if newMsg != nil {
		sess.PromptMsg = newMsg
		h.sessions.Set(c.Sender().ID, sess)
	}
	return nil
}

// wizardCFAddDomain — auto-discover zone & rules berdasarkan nama domain.
func (h *Handler) wizardCFAddDomain(c tele.Context, sess *Session) error {
	h.showTyping(c) // CF API discovery butuh waktu, kasih feedback langsung
	domain := store.CleanDomain(c.Text())
	log.Printf("[CF_ADD] user=%d raw=%q cleaned=%q", c.Sender().ID, c.Text(), domain)

	if domain == "" {
		log.Printf("[CF_ADD] domain kosong setelah CleanDomain")
		return h.reply(c, "❌ Nama domain tidak valid, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}

	// Defensive: re-cek credential di sini juga (kalau dihapus pas wizard jalan)
	if !h.cf.HasCredentials() {
		log.Printf("[CF_ADD] credentials missing")
		h.sessions.Delete(c.Sender().ID)
		return h.reply(c, 
			"⚠️ *CF Credentials belum di-set*\n\nSet email & API key dulu via menu *🔧 Settings*.",
			backToCF(), tele.ModeMarkdown,
		)
	}

	sess.Data["domain"] = domain

	// Pakai plain text dulu (tanpa markdown) biar gak ada chance silent-fail parsing
	loadingMsg, sendErr := h.bot.Send(c.Chat(),
		fmt.Sprintf("🔍 Mencari domain %s di Cloudflare...", domain))
	if sendErr != nil {
		log.Printf("[CF_ADD] Send loadingMsg error: %v", sendErr)
	}

	// 1. Auto-fetch Zone ID
	log.Printf("[CF_ADD] calling GetZoneID(%s)", domain)
	zoneID, err := h.cf.GetZoneID(domain)
	log.Printf("[CF_ADD] GetZoneID result zoneID=%q err=%v", zoneID, err)

	if err != nil {
		// Domain tidak ditemukan di CF → tawarkan registrasi otomatis
		sess.Step = StepCFNewConfirm
		h.sessions.Set(c.Sender().ID, sess)

		mkup := &tele.ReplyMarkup{}
		mkup.Inline(
			mkup.Row(
				mkup.Data("✅ Ya, Daftarkan Sekarang", cbCFNewYes),
				mkup.Data("❌ Batal", cbCancel),
			),
		)
		offerText := fmt.Sprintf(
			"⚠️ *Domain belum terdaftar di Cloudflare*\n\n"+
				"🌐 Domain: `%s`\n\n"+
				"Tapi tenang — bot bisa *daftarin otomatis ke Cloudflare* sekaligus bikin redirect rule-nya.\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"*Yang akan dilakukan bot:*\n"+
				"1. Daftarkan `%s` ke akun CF kamu\n"+
				"2. Bikin DNS record placeholder (proxied)\n"+
				"3. Bikin redirect rule (V1/V2 pilihan kamu)\n"+
				"4. Kasih nameservers buat di-set ke registrar\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"⚠️ *Syarat:* kamu harus punya akses ke domain registrar (Niagahoster/Namecheap/dll) buat update nameservers nanti.\n\n"+
				"_Lanjutkan?_",
			domain, domain,
		)
		if loadingMsg != nil {
			if _, editErr := h.bot.Edit(loadingMsg, offerText, mkup, tele.ModeMarkdown); editErr != nil {
				log.Printf("[CF_ADD] Edit offerText failed: %v", editErr)
				h.bot.Send(c.Chat(), offerText, mkup, tele.ModeMarkdown)
			}
			return nil
		}
		return h.reply(c, offerText, mkup, tele.ModeMarkdown)
	}
	sess.Data["zone_id"] = zoneID

	// 2. Fetch kedua tipe rule
	redirectRules, rrErr := h.cf.ListRedirectRules(zoneID)
	pageRules, prErr := h.cf.ListPageRules(zoneID)
	log.Printf("[CF_ADD] ListRedirectRules count=%d err=%v | ListPageRules count=%d err=%v",
		len(redirectRules), rrErr, len(pageRules), prErr)

	hasV2 := len(redirectRules) > 0
	hasV1 := len(pageRules) > 0

	// 3. Routing berdasarkan kondisi
	switch {
	case hasV2 && !hasV1:
		// Hanya redirect rules → langsung pakai
		sess.Data["type"] = "redirect_rules"
		return h.applyDiscoveredRedirect(c, sess, zoneID, redirectRules, loadingMsg)

	case hasV1 && !hasV2:
		// Hanya page rules → langsung pakai
		sess.Data["type"] = "page_rules"
		return h.applyDiscoveredPage(c, sess, zoneID, pageRules, loadingMsg)

	case hasV2 && hasV1:
		// Dua-duanya ada → tanya user pilih tipe
		// Simpan kedua list ke session (encode index ke Data)
		sess.Data["rrules_count"] = strconv.Itoa(len(redirectRules))
		sess.Data["prules_count"] = strconv.Itoa(len(pageRules))
		for i, r := range redirectRules {
			sess.Data[fmt.Sprintf("rr_%d_rs", i)] = r.RulesetID
			sess.Data[fmt.Sprintf("rr_%d_id", i)] = r.RuleID
			sess.Data[fmt.Sprintf("rr_%d_url", i)] = r.TargetURL
		}
		for i, r := range pageRules {
			sess.Data[fmt.Sprintf("pr_%d_id", i)] = r.RuleID
			sess.Data[fmt.Sprintf("pr_%d_url", i)] = r.TargetURL
			sess.Data[fmt.Sprintf("pr_%d_pat", i)] = r.Pattern
		}
		sess.Step = StepCFAddPickType
		h.sessions.Set(c.Sender().ID, sess)

		m := &tele.ReplyMarkup{}
		m.Inline(
			m.Row(
				m.Data(fmt.Sprintf("Redirect Rules v2 (%d)", len(redirectRules)), cbCFAddPickV2),
				m.Data(fmt.Sprintf("Page Rules v1 (%d)", len(pageRules)), cbCFAddPickV1),
			),
			m.Row(m.Data("❌ Batal", cbCancel)),
		)
		text := fmt.Sprintf(
			"✅ *Domain ditemukan!*\n🌐 `%s`\n🔑 Zone ID: `%s`\n\n"+
				"⚠️ Domain ini punya *dua tipe rule* aktif. Pilih yang mau di-rotate:",
			domain, zoneID,
		)
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, text, m, tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, text, m, tele.ModeMarkdown)

	default:
		// Tidak ada rule sama sekali
		h.sessions.Delete(c.Sender().ID)
		errText := fmt.Sprintf(
			"⚠️ *Belum ada redirect rule*\n\n"+
				"Domain `%s` udah ada di Cloudflare (Zone ID: `%s`), tapi belum punya redirect/page rule aktif.\n\n"+
				"📍 *Solusi:* Buat dulu redirect rule di Cloudflare:\n"+
				"   Cloudflare → pilih domain → *Rules → Redirect Rules*\n\n"+
				"Setelah dibuat, balik ke sini & tambah ulang.",
			domain, zoneID,
		)
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToCF(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToCF(), tele.ModeMarkdown)
	}
}

// applyDiscoveredRedirect — handle setelah dapet list redirect rules.
func (h *Handler) applyDiscoveredRedirect(c tele.Context, sess *Session, zoneID string, rules []cloudflare.DiscoveredRule, loadingMsg *tele.Message) error {
	// 1 rule → auto-save, gak perlu picker
	if len(rules) == 1 {
		r := rules[0]
		sess.Data["ruleset_id"] = r.RulesetID
		sess.Data["rule_id"] = r.RuleID
		return h.saveCFRule(c, sess, loadingMsg, r.TargetURL)
	}

	// Multiple → picker
	for i, r := range rules {
		sess.Data[fmt.Sprintf("rr_%d_rs", i)] = r.RulesetID
		sess.Data[fmt.Sprintf("rr_%d_id", i)] = r.RuleID
		sess.Data[fmt.Sprintf("rr_%d_url", i)] = r.TargetURL
	}
	sess.Data["rrules_count"] = strconv.Itoa(len(rules))
	sess.Step = StepCFAddPickRule
	h.sessions.Set(c.Sender().ID, sess)

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, r := range rules {
		url := r.TargetURL
		if len(url) > 35 {
			url = url[:32] + "..."
		}
		if url == "" {
			url = "(no target)"
		}
		rows = append(rows, m.Row(m.Data(fmt.Sprintf("%d. %s", i+1, url), cbCFAddPickIdx, strconv.Itoa(i))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	text := fmt.Sprintf("✅ *Ditemukan %d Redirect Rule:*\n\nPilih rule yang mau di-rotate 👇", len(rules))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, text, m, tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, text, m, tele.ModeMarkdown)
}

// applyDiscoveredPage — handle setelah dapet list page rules.
func (h *Handler) applyDiscoveredPage(c tele.Context, sess *Session, zoneID string, rules []cloudflare.DiscoveredPageRule, loadingMsg *tele.Message) error {
	if len(rules) == 1 {
		r := rules[0]
		sess.Data["rule_id"] = r.RuleID
		return h.saveCFRule(c, sess, loadingMsg, r.TargetURL)
	}

	for i, r := range rules {
		sess.Data[fmt.Sprintf("pr_%d_id", i)] = r.RuleID
		sess.Data[fmt.Sprintf("pr_%d_url", i)] = r.TargetURL
		sess.Data[fmt.Sprintf("pr_%d_pat", i)] = r.Pattern
	}
	sess.Data["prules_count"] = strconv.Itoa(len(rules))
	sess.Step = StepCFAddPickRule
	h.sessions.Set(c.Sender().ID, sess)

	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, r := range rules {
		url := r.TargetURL
		if len(url) > 30 {
			url = url[:27] + "..."
		}
		rows = append(rows, m.Row(m.Data(fmt.Sprintf("%d. %s → %s", i+1, r.Pattern, url), cbCFAddPickIdx, strconv.Itoa(i))))
	}
	rows = append(rows, m.Row(m.Data("❌ Batal", cbCancel)))
	m.Inline(rows...)

	text := fmt.Sprintf("✅ *Ditemukan %d Page Rule:*\n\nPilih rule yang mau di-rotate 👇", len(rules))
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, text, m, tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, text, m, tele.ModeMarkdown)
}

// handleCFAddPickTypeV1/V2 — user pilih tipe saat ada v1+v2 dua-duanya.
func (h *Handler) handleCFAddPickType(c tele.Context, ruleType string) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFAddPickType {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}
	sess.Data["type"] = ruleType
	zoneID := sess.Data["zone_id"]

	// Rebuild rule list dari session Data
	if ruleType == "redirect_rules" {
		count, _ := strconv.Atoi(sess.Data["rrules_count"])
		var rules []cloudflare.DiscoveredRule
		for i := 0; i < count; i++ {
			rules = append(rules, cloudflare.DiscoveredRule{
				RulesetID: sess.Data[fmt.Sprintf("rr_%d_rs", i)],
				RuleID:    sess.Data[fmt.Sprintf("rr_%d_id", i)],
				TargetURL: sess.Data[fmt.Sprintf("rr_%d_url", i)],
			})
		}
		return h.applyDiscoveredRedirect(c, sess, zoneID, rules, c.Message())
	}
	// page_rules
	count, _ := strconv.Atoi(sess.Data["prules_count"])
	var rules []cloudflare.DiscoveredPageRule
	for i := 0; i < count; i++ {
		rules = append(rules, cloudflare.DiscoveredPageRule{
			RuleID:    sess.Data[fmt.Sprintf("pr_%d_id", i)],
			TargetURL: sess.Data[fmt.Sprintf("pr_%d_url", i)],
			Pattern:   sess.Data[fmt.Sprintf("pr_%d_pat", i)],
		})
	}
	return h.applyDiscoveredPage(c, sess, zoneID, rules, c.Message())
}

// handleCFAddPickRule — user pilih index rule dari picker.
func (h *Handler) handleCFAddPickRule(c tele.Context) error {
	idxStr := extractParam(c)
	if idxStr == "" {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFAddPickRule {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}

	var targetURL string
	if sess.Data["type"] == "redirect_rules" {
		sess.Data["ruleset_id"] = sess.Data[fmt.Sprintf("rr_%d_rs", idx)]
		sess.Data["rule_id"] = sess.Data[fmt.Sprintf("rr_%d_id", idx)]
		targetURL = sess.Data[fmt.Sprintf("rr_%d_url", idx)]
	} else {
		sess.Data["rule_id"] = sess.Data[fmt.Sprintf("pr_%d_id", idx)]
		targetURL = sess.Data[fmt.Sprintf("pr_%d_url", idx)]
	}
	return h.saveCFRule(c, sess, c.Message(), targetURL)
}

// saveCFRule — final step: simpan CF rule ke store.
func (h *Handler) saveCFRule(c tele.Context, sess *Session, targetMsg *tele.Message, targetURL string) error {
	h.sessions.Delete(c.Sender().ID)

	rule := store.CFRule{
		Label:     sess.Data["label"],
		Domain:    sess.Data["domain"],
		ZoneID:    sess.Data["zone_id"],
		Type:      sess.Data["type"],
		RulesetID: sess.Data["ruleset_id"],
		RuleID:    sess.Data["rule_id"],
	}
	h.cfrules.Add(rule)
	log.Printf("[CF_ADD] saved rule label=%s domain=%s zone=%s type=%s rule_id=%s",
		rule.Label, rule.Domain, rule.ZoneID, rule.Type, rule.RuleID)

	typeLabel := "Redirect Rules (v2)"
	if rule.Type == "page_rules" {
		typeLabel = "Page Rules (v1)"
	}

	urlLine := ""
	if targetURL != "" {
		urlLine = fmt.Sprintf("\n🔗 Current target: `%s`", escapeMD(targetURL))
	}

	text := fmt.Sprintf(
		"✅ *CF Rule ditambahkan!*\n\n"+
			"📛 Label: *%s*\n"+
			"🌐 Domain: `%s`\n"+
			"🔑 Zone ID: `%s`\n"+
			"📌 Tipe: *%s*\n"+
			"🆔 Rule ID: `%s`%s\n\n"+
			"_Sekarang bisa setup Auto Rotator buat rule ini._",
		escapeMD(rule.Label), escapeMD(sess.Data["domain"]), rule.ZoneID, typeLabel, rule.RuleID, urlLine,
	)

	if targetMsg != nil {
		if _, err := h.bot.Edit(targetMsg, text, backToCF(), tele.ModeMarkdown); err != nil {
			log.Printf("[CF_ADD] Edit targetMsg failed: %v — fallback to Send", err)
			h.bot.Send(c.Chat(), text, backToCF(), tele.ModeMarkdown)
		}
		return nil
	}
	return h.reply(c, text, backToCF(), tele.ModeMarkdown)
}

// ─── List CF Rules ────────────────────────────────────────────────────────────

func (h *Handler) handleCFList(c tele.Context) error {
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit("📭 Belum ada CF rule terdaftar.\n\nGunakan *Add Rule* untuk menambahkan.", backToCF(), tele.ModeMarkdown)
	}

	// Tampilkan loading dulu kalau credentials ada (karena bakal fetch live URL dari CF)
	loadingShown := false
	if h.cf.HasCredentials() {
		c.Edit(fmt.Sprintf("⏳ Mengambil data %d rule dari Cloudflare...", len(rules)), tele.ModeMarkdown)
		loadingShown = true
	}

	// Fetch URL terkini per rule secara paralel
	type fetchResult struct {
		idx int
		url string
		err error
	}
	results := make([]fetchResult, len(rules))
	var wg sync.WaitGroup
	for i, r := range rules {
		results[i] = fetchResult{idx: i}
		if !h.cf.HasCredentials() {
			results[i].err = fmt.Errorf("creds")
			continue
		}
		wg.Add(1)
		go func(idx int, rule store.CFRule) {
			defer wg.Done()
			url, err := h.cf.GetCurrentURL(rule)
			results[idx].url = url
			results[idx].err = err
		}(i, r)
	}
	wg.Wait()

	// Build pesan
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *CF Rules — %d rule*\n", len(rules)))
	sb.WriteString("═══════════════════════════\n\n")

	for i, r := range rules {
		typeLabel := "Redirect Rules (v2)"
		if r.Type == "page_rules" {
			typeLabel = "Page Rules (v1)"
		}

		domainDisplay := r.Domain
		if domainDisplay == "" {
			domainDisplay = "_(belum ada — tambah ulang via Add Rule)_"
		}

		// Tujuan URL
		var targetDisplay string
		res := results[i]
		if res.err != nil {
			if !h.cf.HasCredentials() {
				targetDisplay = "_(set CF Credentials dulu di Settings)_"
			} else {
				targetDisplay = fmt.Sprintf("⚠️ _gagal fetch: %s_", escapeMD(truncate(res.err.Error(), 60)))
			}
		} else if res.url == "" {
			targetDisplay = "_(kosong)_"
		} else {
			targetDisplay = fmt.Sprintf("`%s`", escapeMD(res.url))
		}

		sb.WriteString(fmt.Sprintf("*%d. %s*\n", i+1, escapeMD(r.Label)))
		sb.WriteString(fmt.Sprintf("├ 🌐 *Domain:* `%s`\n", escapeMD(domainDisplay)))
		sb.WriteString(fmt.Sprintf("├ 🎯 *Tujuan:* %s\n", targetDisplay))
		sb.WriteString(fmt.Sprintf("├ 📌 *Tipe:* %s\n", typeLabel))
		sb.WriteString(fmt.Sprintf("├ 🔑 *Zone ID:* `%s`\n", r.ZoneID))
		if r.RulesetID != "" {
			sb.WriteString(fmt.Sprintf("├ 🆔 *Ruleset:* `%s`\n", r.RulesetID))
		}
		sb.WriteString(fmt.Sprintf("└ 🆔 *Rule ID:* `%s`\n\n", r.RuleID))
	}

	text := sb.String()
	if len(text) > 3800 {
		text = text[:3800] + "\n\n_(dipotong — terlalu panjang)_"
	}

	if loadingShown {
		if _, err := h.bot.Edit(c.Message(), text, backToCF(), tele.ModeMarkdown); err != nil {
			log.Printf("[CF_LIST] Edit failed: %v — fallback Send", err)
			h.bot.Send(c.Chat(), text, backToCF(), tele.ModeMarkdown)
		}
		return nil
	}
	return c.Edit(text, backToCF(), tele.ModeMarkdown)
}

// truncate memendekkan string ke max length dengan "..." di akhir.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
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

	return c.Edit(
		"✏️ *Ganti URL Tujuan (Manual)*\n\n"+
			"Pilih rule yang URL tujuannya mau diganti:\n\n"+
			"_(Untuk ganti banyak rule sekaligus → balik & pilih *📦 Bulk Change*.)_",
		m, tele.ModeMarkdown)
}

func (h *Handler) handleCFChangeSelect(c tele.Context) error {
	ruleID := extractParam(c)
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

	domainInfo := ""
	if rule.Domain != "" {
		domainInfo = fmt.Sprintf("🌐 *Domain:* `%s`\n", rule.Domain)
	}
	prompt := fmt.Sprintf(
		"✏️ *Ganti URL Tujuan*\n\n"+
			"📛 *Rule:* %s\n"+
			"%s"+
			"🎯 *URL saat ini:*\n`%s`\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"Ketik *URL baru* (lengkap dengan `https://`):\n\n"+
			"*Contoh:*\n"+
			"• `https://landing-baru.com`\n"+
			"• `https://promo.tokoku.id/special`\n\n"+
			"_Bot bakal langsung apply ke Cloudflare._",
		rule.Label, domainInfo, currentURL,
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
	newURL := strings.TrimSpace(c.Text())
	if newURL == "" {
		return h.reply(c, "❌ URL tidak boleh kosong", backToCF(), tele.ModeMarkdown)
	}
	h.sessions.Delete(c.Sender().ID)

	rule, ok := h.cfrules.GetByID(sess.Data["rule_id"])
	if !ok {
		return h.reply(c, "❌ Rule tidak ditemukan", backToCF(), tele.ModeMarkdown)
	}

	loadingMsg, _ := h.bot.Send(c.Chat(), "⏳ Mengupdate CF rule...", tele.ModeMarkdown)

	// Ambil current URL dulu untuk history
	prevURL, _ := h.cf.GetCurrentURL(rule)

	if err := h.cf.UpdateURL(rule, newURL); err != nil {
		h.history.LogSwap("manual", rule.Label, rule.Domain, prevURL, newURL, false, err.Error())
		errText := fmt.Sprintf("❌ *Gagal update!*\n\nError: %v", err)
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToCF(), tele.ModeMarkdown)
			return nil
		}
		return h.reply(c, errText, backToCF(), tele.ModeMarkdown)
	}
	h.history.LogSwap("manual", rule.Label, rule.Domain, prevURL, newURL, true, "")

	successText := fmt.Sprintf("✅ *URL berhasil diubah!*\n\n📛 Rule: *%s*\n🔗 URL Baru: `%s`",
		rule.Label, newURL)
	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, successText, backToCF(), tele.ModeMarkdown)
		return nil
	}
	return h.reply(c, successText, backToCF(), tele.ModeMarkdown)
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

	return c.Edit(
		"🗑 *Hapus CF Rule dari Bot*\n\n"+
			"Pilih rule yang mau dihapus:\n\n"+
			"⚠️ _Cuma hapus dari list bot — rule asli di dashboard Cloudflare tidak terpengaruh._",
		m, tele.ModeMarkdown)
}

func (h *Handler) handleCFDeleteConfirm(c tele.Context) error {
	ruleID := extractParam(c)
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
