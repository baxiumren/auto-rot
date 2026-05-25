package bot

import (
	"fmt"
	"log"
	"strings"

	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── New Domain Registration Flow ─────────────────────────────────────────────
//
// Dipanggil saat wizardCFAddDomain → GetZoneID error (domain belum di CF).
// Flow:
// 1. Bot tawarin "Daftarkan otomatis?" → klik Ya
// 2. AddZone ke Cloudflare → dapet ZoneID + Nameservers
// 3. Bikin DNS A record placeholder (192.0.2.1, proxied) — biar redirect bisa kerja
// 4. Tanya: V1 (Page Rules) atau V2 (Redirect Rules)?
// 5. Tanya: URL tujuan redirect
// 6. Bikin rule → simpan ke bot store
// 7. Tampilkan nameservers buat di-set di registrar

const placeholderIP = "192.0.2.1" // RFC 5737 - reserved untuk dokumentasi/example

// handleCFNewYes — user setuju register domain baru.
func (h *Handler) handleCFNewYes(c tele.Context) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFNewConfirm {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}
	domain := sess.Data["domain"]

	loadingMsg, _ := h.bot.Edit(c.Message(),
		fmt.Sprintf("⏳ *Mendaftarkan `%s` ke Cloudflare...*\n\nMohon tunggu, lagi:\n• AddZone ke akun CF kamu\n• Generate nameservers", domain),
		tele.ModeMarkdown)

	zoneInfo, err := h.cf.AddZone(domain)
	if err != nil {
		h.sessions.Delete(c.Sender().ID)
		errText := fmt.Sprintf(
			"❌ *Gagal mendaftarkan domain*\n\n"+
				"Domain: `%s`\n\n"+
				"_%s_\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"*Penyebab umum:*\n"+
				"• Domain udah pernah ditambahkan ke akun CF lain → hapus dulu dari akun itu\n"+
				"• Domain TLD tidak didukung Free plan\n"+
				"• Credentials kamu cuma Token (bukan Global API Key)",
			domain, escapeMD(err.Error()),
		)
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToCF(), tele.ModeMarkdown)
			return nil
		}
		return c.Send(errText, backToCF(), tele.ModeMarkdown)
	}

	// Save zone info
	sess.Data["zone_id"] = zoneInfo.ZoneID
	sess.Data["nameservers"] = strings.Join(zoneInfo.NameServers, ",")
	log.Printf("[CF_NEW] zone created domain=%s zoneID=%s NS=%v", domain, zoneInfo.ZoneID, zoneInfo.NameServers)

	// Bikin DNS A record placeholder (biar redirect bisa kerja walau IP-nya dummy)
	if err := h.cf.CreateDNSRecord(zoneInfo.ZoneID, "A", domain, placeholderIP, true); err != nil {
		// Non-fatal — user bisa bikin DNS sendiri di CF dashboard
		log.Printf("[CF_NEW] CreateDNSRecord (root) failed: %v", err)
	}
	if err := h.cf.CreateDNSRecord(zoneInfo.ZoneID, "A", "www."+domain, placeholderIP, true); err != nil {
		log.Printf("[CF_NEW] CreateDNSRecord (www) failed: %v", err)
	}

	// Tanya tipe redirect
	sess.Step = StepCFNewPickType
	h.sessions.Set(c.Sender().ID, sess)

	mkup := &tele.ReplyMarkup{}
	mkup.Inline(
		mkup.Row(
			mkup.Data("✅ V2 - Redirect Rules (Recommended)", cbCFNewTypeV2),
		),
		mkup.Row(
			mkup.Data("V1 - Page Rules (Legacy)", cbCFNewTypeV1),
		),
		mkup.Row(mkup.Data("❌ Batal", cbCancel)),
	)

	text := fmt.Sprintf(
		"✅ *Domain `%s` berhasil didaftarkan ke Cloudflare!*\n\n"+
			"🔑 Zone ID: `%s`\n"+
			"📡 DNS A record placeholder otomatis ke-buat (root + www, proxied).\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"*Langkah berikutnya — pilih tipe redirect:*\n\n"+
			"📌 *V2 - Redirect Rules* _(rekomendasi)_\n"+
			"Engine baru CF, lebih powerful. Pakai ini kalau gak ada alasan khusus.\n\n"+
			"📌 *V1 - Page Rules* _(legacy)_\n"+
			"Engine lama, terbatas (max 3 rule di Free plan). Pakai kalau kamu udah biasa.",
		domain, zoneInfo.ZoneID,
	)

	if loadingMsg != nil {
		h.bot.Edit(loadingMsg, text, mkup, tele.ModeMarkdown)
		return nil
	}
	return c.Send(text, mkup, tele.ModeMarkdown)
}

// handleCFNewPickType — user pilih V1 atau V2.
func (h *Handler) handleCFNewPickType(c tele.Context, ruleType string) error {
	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok || sess.Step != StepCFNewPickType {
		return c.Edit(textCF, cfMenu(), tele.ModeMarkdown)
	}
	sess.Data["type"] = ruleType
	sess.Step = StepCFNewTargetURL
	h.sessions.Set(c.Sender().ID, sess)

	typeLabel := "Redirect Rules (v2)"
	if ruleType == "page_rules" {
		typeLabel = "Page Rules (v1)"
	}

	text := fmt.Sprintf(
		"✅ Tipe: *%s*\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"🎯 *URL Tujuan Redirect*\n\n"+
			"Ketik URL lengkap yang mau jadi tujuan redirect.\n\n"+
			"*Contoh:*\n"+
			"• `https://landing-utama.com`\n"+
			"• `https://promo.tokoku.id/special`\n"+
			"• `https://hub.kwai.app/abc123`\n\n"+
			"⚠️ *Harus diawali `https://`* (atau bot auto-tambah).",
		typeLabel,
	)
	return c.Edit(text, cancelMenu(), tele.ModeMarkdown)
}

// wizardCFNewTargetURL — user ketik URL tujuan → bot bikin rule.
func (h *Handler) wizardCFNewTargetURL(c tele.Context, sess *Session) error {
	targetURL := strings.TrimSpace(c.Text())
	if targetURL == "" {
		return c.Send("❌ URL tidak boleh kosong, coba lagi:", cancelMenu(), tele.ModeMarkdown)
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	domain := sess.Data["domain"]
	zoneID := sess.Data["zone_id"]
	ruleType := sess.Data["type"]
	nameservers := strings.Split(sess.Data["nameservers"], ",")

	loadingMsg, _ := h.bot.Send(c.Chat(),
		fmt.Sprintf("⏳ Bikin redirect rule (%s)...", ruleType))

	var rulesetID, ruleID string
	var createErr error

	if ruleType == "redirect_rules" {
		rulesetID, ruleID, createErr = h.cf.CreateRedirectRuleV2(zoneID, targetURL)
	} else {
		// page_rules — pakai pattern *.domain/*
		pattern := domain + "/*"
		ruleID, createErr = h.cf.CreatePageRule(zoneID, pattern, targetURL)
	}

	if createErr != nil {
		h.sessions.Delete(c.Sender().ID)
		errText := fmt.Sprintf(
			"❌ *Gagal membuat redirect rule*\n\n_%s_\n\n"+
				"Zone CF udah ke-buat, tapi rule belum. Kamu bisa:\n"+
				"1. Coba ulangin via *➕ Add Rule*\n"+
				"2. Atau bikin manual di Cloudflare dashboard",
			escapeMD(createErr.Error()),
		)
		if loadingMsg != nil {
			h.bot.Edit(loadingMsg, errText, backToCF(), tele.ModeMarkdown)
			return nil
		}
		return c.Send(errText, backToCF(), tele.ModeMarkdown)
	}

	// Save ke store
	rule := store.CFRule{
		Label:     sess.Data["label"],
		Domain:    domain,
		ZoneID:    zoneID,
		Type:      ruleType,
		RulesetID: rulesetID,
		RuleID:    ruleID,
	}
	h.cfrules.Add(rule)
	h.sessions.Delete(c.Sender().ID)
	log.Printf("[CF_NEW] saved new domain rule label=%s domain=%s type=%s", rule.Label, domain, ruleType)

	// Build success message dengan nameservers info
	typeLabel := "Redirect Rules (v2)"
	if ruleType == "page_rules" {
		typeLabel = "Page Rules (v1)"
	}

	var nsLines strings.Builder
	for _, ns := range nameservers {
		if ns != "" {
			nsLines.WriteString(fmt.Sprintf("• `%s`\n", ns))
		}
	}

	text := fmt.Sprintf(
		"🎉 *SUKSES! Domain baru ter-setup lengkap!*\n"+
			"═══════════════════════════\n\n"+
			"📛 *Label:* %s\n"+
			"🌐 *Domain:* `%s`\n"+
			"🎯 *Tujuan:* `%s`\n"+
			"📌 *Tipe:* %s\n"+
			"🔑 *Zone ID:* `%s`\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"⚠️ *PENTING — Update Nameservers di Registrar!*\n\n"+
			"Domain baru aktif kalau nameservers di registrar (tempat kamu beli domain) udah pointing ke Cloudflare.\n\n"+
			"📡 *Set nameservers berikut di registrar:*\n%s\n"+
			"📍 *Caranya:*\n"+
			"1. Login ke registrar (Niagahoster/Namecheap/Cloudflare Registrar/dll)\n"+
			"2. Cari menu *Domain → DNS / Nameservers*\n"+
			"3. Pilih *Custom Nameservers*\n"+
			"4. Paste 2 NS di atas → save\n\n"+
			"⏳ Propagasi biasanya 5 menit - 24 jam. Setelah aktif, redirect & Auto Rotator langsung jalan.",
		escapeMD(rule.Label), domain, escapeMD(targetURL), typeLabel, zoneID, nsLines.String(),
	)

	if loadingMsg != nil {
		if _, err := h.bot.Edit(loadingMsg, text, backToCF(), tele.ModeMarkdown); err != nil {
			log.Printf("[CF_NEW] Edit success msg failed: %v — fallback Send", err)
			h.bot.Send(c.Chat(), text, backToCF(), tele.ModeMarkdown)
		}
		return nil
	}
	return c.Send(text, backToCF(), tele.ModeMarkdown)
}
