package bot

import tele "gopkg.in/telebot.v3"

// ─── Callback Unique IDs ──────────────────────────────────────────────────────

const (
	cbMain = "main"

	// Monitor
	cbMonitor          = "monitor"
	cbMonitorAdd       = "monitor_add"
	cbMonitorRemove    = "monitor_remove"
	cbMonitorList      = "monitor_list"
	cbMonitorCheck     = "monitor_check"
	cbMonitorInterval  = "monitor_interval"
	cbMonitorStatus    = "monitor_status"
	cbMonitorSticky      = "monitor_sticky"      // list sticky-blocked + unblock
	cbMonitorStickyDel   = "monitor_sticky_del"  // remove from sticky (param=domain)
	cbMonitorStickyClean = "monitor_sticky_cln"  // cleanup all orphan sticky
	cbMonitorCheckKominfo  = "monitor_check_kominfo"   // manual check via Kominfo komdigi.go.id
	cbMonitorCheckTP       = "monitor_check_tp"        // manual check via trustpositif.id
	cbMonitorCheckNawala   = "monitor_check_nawala"    // manual check via NawalaCheck
	cbMonitorForce       = "monitor_force"       // open force-block menu
	cbMonitorForceAdd    = "monitor_force_add"   // add force-block (wizard)
	cbMonitorForceDel    = "monitor_force_del"   // remove force-block (param=domain)
	cbMonitorForceClean  = "monitor_force_cln"   // cleanup all orphan force

	// CF Redirect
	cbCF              = "cf"
	cbCFAdd           = "cf_add"
	cbCFAddPickV1     = "cf_add_pick_v1"  // pilih tipe page_rules
	cbCFAddPickV2     = "cf_add_pick_v2"  // pilih tipe redirect_rules
	cbCFAddPickIdx    = "cf_add_pick_idx" // pilih index rule (param = "0", "1", dll)
	cbCFList          = "cf_list"
	cbCFChange        = "cf_change" // Data = ruleID (single change)
	cbCFDelete        = "cf_delete" // Data = ruleID
	cbCFBulk        = "cf_bulk" // open bulk change picker
	cbCFBulkToggle  = "cf_bulk_tg"
	cbCFBulkSelAll  = "cf_bulk_all"
	cbCFBulkSelNone = "cf_bulk_none"
	cbCFBulkApply   = "cf_bulk_apply"

	// New domain registration
	cbCFNewYes    = "cf_new_yes"     // confirm register new domain
	cbCFNewTypeV1 = "cf_new_type_v1" // pilih V1 page rules
	cbCFNewTypeV2 = "cf_new_type_v2" // pilih V2 redirect rules

	// Auto Rotator
	cbRotator       = "rotator"
	cbRotatorAdd    = "rotator_add"
	cbRotatorList   = "rotator_list"
	cbRotatorToggle = "rotator_toggle" // Data = rotatorID
	cbRotatorDelete = "rotator_delete" // Data = rotatorID
	cbRotatorForce  = "rotator_force"  // Data = rotatorID
	cbRotatorCFSel  = "rotator_cfsel"  // Data = cfRuleID (wizard: pilih CF rule)
	cbRotatorPool   = "rotator_pool"   // Data = poolLabel (wizard: pilih pool)

	// Bulk Setup Rotator
	cbRotatorBulk         = "rotator_bulk"
	cbRotatorBulkToggle   = "rotator_bulk_tg"
	cbRotatorBulkSelAll   = "rotator_bulk_all"
	cbRotatorBulkSelNone  = "rotator_bulk_none"
	cbRotatorBulkProceed  = "rotator_bulk_proceed"
	cbRotatorBulkPickPool = "rotator_bulk_pool"

	// Swap History
	cbHistory      = "history"
	cbHistoryClear = "history_clear"
	cbHistoryClearYes = "history_clear_yes"

	// Settings
	cbSettings         = "settings"
	cbSettingsSetEmail = "settings_set_email"
	cbSettingsSetKey   = "settings_set_key"
	cbSettingsSetBoth  = "settings_set_both"
	cbSettingsTest     = "settings_test"
	cbSettingsClear    = "settings_clear"
	cbSettingsClearYes = "settings_clear_yes"

	// Cancel
	cbCancel = "cancel"
)

// ─── Menu Texts ───────────────────────────────────────────────────────────────

const (
	textMain = "🏠 *Menu Utama*\n\n" +
		"Bot ini bantu kamu *otomatis ganti domain* yang kena blokir Kominfo (nawala), supaya iklan/redirect kamu gak mati.\n\n" +
		"*Cara pakainya 3 langkah:*\n" +
		"1️⃣ *Monitor* — daftarin domain yang mau dipantau\n" +
		"2️⃣ *CF Redirect* — daftarin redirect rule Cloudflare-mu\n" +
		"3️⃣ *Auto Rotator* — gabungin keduanya → otomatis swap\n\n" +
		"_Klik tombol di bawah untuk mulai:_"

	textMonitor = "📡 *Monitor — Pantau Domain*\n\n" +
		"Section ini buat *mendaftarkan domain* yang mau dipantau apakah kena nawala Kominfo (TrustPositif).\n\n" +
		"Domain dikelompokkan per *label* (contoh: KWAI, MONEYSITE, STOCK-MS). Nanti label ini dipakai sebagai *pool* (kumpulan domain cadangan) di Auto Rotator.\n\n" +
		"*Tips:* satu label sebaiknya berisi *domain-domain serupa* (misal semua untuk halaman promo Kwai). Kalau ada yang kena blokir, bot ganti ke domain lain di label yang sama."

	textCF = "⚙️ *CF Redirect — Cloudflare Redirect Rules*\n\n" +
		"Section ini buat *menghubungkan bot dengan redirect rule Cloudflare* kamu.\n\n" +
		"*Apa itu redirect rule?*\n" +
		"Rule di Cloudflare yang ngarahin pengunjung domain A → URL tujuan B (misal `iklan.com` → `https://landing-kamu.com`).\n\n" +
		"Daftarin rule kamu di sini, lalu bot bisa *ganti URL tujuannya* (otomatis lewat Auto Rotator, atau manual via Ganti URL).\n\n" +
		"*Tips:* gak perlu hapal Zone ID / Rule ID — cukup ketik nama domain, bot fetch sendiri dari Cloudflare."

	textRotator = "🔄 *Auto Rotator — Konfigurasi Swap*\n\n" +
		"Section paling penting! Di sini kamu *gabungkan*:\n" +
		"• *CF Rule* — yang URL-nya mau di-rotate\n" +
		"• *Pool Label* — kelompok domain cadangan dari Monitor\n\n" +
		"*Cara kerja swap otomatis:*\n" +
		"1️⃣ Monitor Scanner pantau SEMUA domain di list (24/7)\n" +
		"2️⃣ Begitu ada domain detected BLOCKED → cross-check ke CF Rule\n" +
		"3️⃣ Kalau current URL CF Rule = domain yg blocked → cek Rotator config\n" +
		"4️⃣ Ambil domain berikutnya dari *Pool Label* (yang kamu set di sini) → swap\n\n" +
		"⚠️ *Tanpa Rotator config, auto-swap GAK jalan* — bot cuma kirim notif blocked.\n\n" +
		"_Syarat: udah ada CF Rule + minimal 2 domain di pool Monitor._"
)

// ─── Main Menu ────────────────────────────────────────────────────────────────

func mainMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📡 Monitor", cbMonitor),
			m.Data("⚙️ CF Redirect", cbCF),
		),
		m.Row(
			m.Data("🔄 Auto Rotator", cbRotator),
			m.Data("🔧 Settings", cbSettings),
		),
	)
	return m
}

// ─── Settings Menu ────────────────────────────────────────────────────────────

func settingsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("📧 Set CF Email", cbSettingsSetEmail),
			m.Data("🔑 Set CF API Key", cbSettingsSetKey),
		),
		m.Row(
			m.Data("🔄 Set Keduanya", cbSettingsSetBoth),
			m.Data("✅ Test Koneksi", cbSettingsTest),
		),
		m.Row(
			m.Data("🗑 Hapus Credentials", cbSettingsClear),
		),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	)
	return m
}

func backToSettings() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbSettings)))
	return m
}

// ─── Monitor Menu ─────────────────────────────────────────────────────────────

func monitorMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Add Domain", cbMonitorAdd),
			m.Data("🔍 Cek Domain", cbMonitorCheck),
		),
		m.Row(
			m.Data("🗑 Remove Domain", cbMonitorRemove),
			m.Data("📋 List Domain", cbMonitorList),
		),
		m.Row(
			m.Data("⏱ Set Interval", cbMonitorInterval),
			m.Data("📊 Status Blocked", cbMonitorStatus),
		),
		m.Row(
			m.Data("📌 Sticky List", cbMonitorSticky),
			m.Data("🔨 Force Block", cbMonitorForce),
		),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	)
	return m
}

// ─── CF Redirect Menu ─────────────────────────────────────────────────────────

func cfMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Add Rule", cbCFAdd),
			m.Data("📋 List Rules", cbCFList),
		),
		m.Row(
			m.Data("✏️ Ganti URL", cbCFChange, ""),
			m.Data("📦 Bulk Change", cbCFBulk),
		),
		m.Row(
			m.Data("🗑 Hapus Rule", cbCFDelete, ""),
		),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	)
	return m
}

// ─── Rotator Menu ─────────────────────────────────────────────────────────────

func rotatorMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Setup Rotator", cbRotatorAdd),
			m.Data("📦 Bulk Setup", cbRotatorBulk),
		),
		m.Row(
			m.Data("📋 List Rotator", cbRotatorList),
			m.Data("📜 Swap History", cbHistory),
		),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	)
	return m
}

// ─── Shared Helpers ───────────────────────────────────────────────────────────

func backToMain() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Menu Utama", cbMain)))
	return m
}

func backToMonitor() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbMonitor)))
	return m
}

func backToCF() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbCF)))
	return m
}

func backToRotator() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbRotator)))
	return m
}

func cancelMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("❌ Batal", cbCancel)))
	return m
}

// ─── Persistent Reply Keyboard ────────────────────────────────────────────────
// Tombol START yang selalu nempel di bawah chat input, gak perlu ketik /start.

const (
	replyBtnStart  = "🚀 START"
	replyBtnMenu   = "🏠 MENU"
	replyBtnStatus = "🩺 STATUS"
	replyBtnSearch = "🔍 CARI"
)

func startReplyKeyboard() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
	m.Reply(
		m.Row(
			m.Text(replyBtnStart),
			m.Text(replyBtnMenu),
		),
		m.Row(
			m.Text(replyBtnStatus),
			m.Text(replyBtnSearch),
		),
	)
	return m
}
