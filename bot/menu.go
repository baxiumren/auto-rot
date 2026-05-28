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

	// Pagination list domain
	cbMonitorListMenuPage = "monitor_list_pg"    // page kategori picker (param=pageIdx)
	cbMonitorListAll      = "monitor_list_all"   // semua domain paginated (param=pageIdx)
	cbMonitorListLabel    = "monitor_list_label" // per-label paginated (param=label|pageIdx)
	cbNoop                = "noop"               // non-functional button (page indicator)
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
	cbSettingsCF       = "settings_cf"
	cbSettingsSetEmail = "settings_set_email"
	cbSettingsSetKey   = "settings_set_key"
	cbSettingsSetBoth  = "settings_set_both"
	cbSettingsTest     = "settings_test"
	cbSettingsClear    = "settings_clear"
	cbSettingsClearYes = "settings_clear_yes"

	// Cancel
	cbCancel = "cancel"

	// ─── Group-only callbacks (read-only views) ──────────────────────────────
	cbGroupStatus     = "g_status"      // health check
	cbGroupListDomain = "g_list_domain" // count per label
	cbGroupListCF     = "g_list_cf"     // CF rules summary
	cbAlertRemove     = "alert_remove"  // hapus domain dari alert (param=domain)
)

const (
	// Klikcepat root menu
	cbKlikcepat = "klikcepat"

	// Klikcepat Settings
	cbSettingsKlikcepat       = "settings_klikcepat"
	cbSettingsKlikcepatSetURL    = "settings_klc_url"
	cbSettingsKlikcepatSetKey    = "settings_klc_key"
	cbSettingsKlikcepatSetDomain   = "settings_klc_domain"
	cbSettingsKlikcepatDomMap      = "settings_klc_dommap"        // root domain map manager
	cbSettingsKlikcepatDomMapAdd   = "settings_klc_dommap_add"    // add wizard entry
	cbSettingsKlikcepatDomMapDel   = "settings_klc_dommap_del"    // delete picker entry
	cbSettingsKlikcepatDomMapDelID = "settings_klc_dommap_delid"  // param = id to delete
	cbSettingsKlikcepatTest   = "settings_klc_test"
	cbSettingsKlikcepatClear  = "settings_klc_clear"

	// Klikcepat Link CRUD
	cbKlikcepatAdd            = "klc_add"
	cbKlikcepatAddType        = "klc_add_type"   // param = link type
	cbKlikcepatAddPickProject = "klc_add_proj"   // param = project_id (0 = skip)
	cbKlikcepatList           = "klc_list"       // param = page index
	cbKlikcepatListByProj     = "klc_list_proj"  // param = "projectID|page"
	cbKlikcepatEdit           = "klc_edit"
	cbKlikcepatEditPick       = "klc_edit_pick"  // param = link ID
	cbKlikcepatEditField      = "klc_edit_field" // param = field name
	cbKlikcepatDelete         = "klc_delete"
	cbKlikcepatDeletePick     = "klc_delete_pick"  // param = link ID
	cbKlikcepatDeleteConfirm  = "klc_del_yes"      // param = link ID
	cbKlikcepatOpenDashboard  = "klc_dashboard"

	// Klikcepat Projects
	cbKlikcepatProjects             = "klc_projects"
	cbKlikcepatProjectAdd           = "klc_proj_add"
	cbKlikcepatProjectList          = "klc_proj_list"
	cbKlikcepatProjectEdit          = "klc_proj_edit"
	cbKlikcepatProjectEditPick      = "klc_proj_edit_pick"
	cbKlikcepatProjectDelete        = "klc_proj_del"
	cbKlikcepatProjectDeletePick    = "klc_proj_del_pick"
	cbKlikcepatProjectDeleteConfirm = "klc_proj_del_yes"

	// Auto Rotator unified — pick type
	cbRotatorAddPickType      = "rotator_add_pick"
	cbRotatorAddTypeCF        = "rotator_add_cf"
	cbRotatorAddTypeKlikcepat = "rotator_add_klc"

	// Klikcepat subtype picker (after KLIKCEPAT clicked → BIOLINK vs SHORTLINK)
	cbRotatorAddTypeKlcShortlink = "rotator_add_klc_sl" // param = page index
	cbRotatorAddTypeKlcBiolink   = "rotator_add_klc_bl" // param = page index

	// Klikcepat Rotator (shortlink)
	cbKlikcepatRotPickLink = "klc_rot_picklink" // param = link ID
	cbKlikcepatRotPickPool = "klc_rot_pickpool" // param = pool label
	cbKlikcepatRotToggle   = "klc_rot_toggle"   // param = rotator ID
	cbKlikcepatRotDelete   = "klc_rot_delete"   // param = rotator ID
	cbKlikcepatRotForce    = "klc_rot_force"    // param = rotator ID

	// Klikcepat Block Rotator (biolink button)
	cbKlcBlockRotPickBiolink = "klc_blk_picklink" // param = biolink link_id
	cbKlcBlockRotPickBlock   = "klc_blk_pickblock" // param = block_id
	cbKlcBlockRotPickPool    = "klc_blk_pickpool"  // param = pool label
	cbKlcBlockRotToggle      = "klc_blk_toggle"    // param = rotator ID
	cbKlcBlockRotDelete      = "klc_blk_delete"    // param = rotator ID

	// Bulk Setup Rotator — pick type (CF or Klikcepat) then subtype
	cbRotatorBulkTypeCF           = "rotator_bulk_cf"
	cbRotatorBulkTypeKlikcepat    = "rotator_bulk_klc"
	cbRotatorBulkTypeKlcShortlink = "rotator_bulk_klc_sl" // param = page
	cbRotatorBulkTypeKlcBiolink   = "rotator_bulk_klc_bl" // param = page

	cbKlikcepatRotBulkToggle   = "klc_bulk_tg" // param = link ID
	cbKlikcepatRotBulkSelAll   = "klc_bulk_all"
	cbKlikcepatRotBulkSelNone  = "klc_bulk_none"
	cbKlikcepatRotBulkProceed  = "klc_bulk_proceed"
	cbKlikcepatRotBulkPickPool = "klc_bulk_pool" // param = pool label
	cbKlikcepatRotBulkPage     = "klc_bulk_page" // param = page index

	// Bulk Block Rotator (biolink multi-block)
	cbKlcBlockBulkPickBiolink = "klc_blkb_pick" // param = biolink id
	cbKlcBlockBulkToggle      = "klc_blkb_tg"   // param = block id
	cbKlcBlockBulkSelAll      = "klc_blkb_all"
	cbKlcBlockBulkSelNone     = "klc_blkb_none"
	cbKlcBlockBulkProceed     = "klc_blkb_proceed"
	cbKlcBlockBulkPickPool    = "klc_blkb_pool" // param = pool label
)

// ─── Menu Texts ───────────────────────────────────────────────────────────────

const (
	textMain = "🎰🔥 *BONG BOT — ALL IN ONE FITUR BOT PALING GACOR!* 🚀✨\n\n" +
		"🤖 _Bot anti-nawala buat para pejuang affiliate Indonesia._ 🇮🇩\n\n" +
		"💡 *Kemampuan utama:*\n" +
		"• 👀 Pantau domain 24/7 dari Kominfo nawala\n" +
		"• ⚡ Auto-swap Cloudflare rule + Klikcepat link begitu blocked\n" +
		"• 🔗 Full CRUD biolink + shortlink via Telegram\n" +
		"• 📊 Multi-source check (Kominfo + TrustPositif + NawalaCheck)\n\n" +
		"━━━━━━━━━━━━━━━━━━\n" +
		"📋 *Workflow 3 langkah (buat pemula):*\n" +
		"1️⃣ *📡 Monitor* — daftarin domain yang mau dipantau\n" +
		"2️⃣ *⚙️ CF Redirect* / *🔗 KLIKCEPAT* — register rule/link\n" +
		"3️⃣ *🔄 Auto Rotator* — link rule + pool → bot kerja sendiri\n\n" +
		"🎯 _Pilih menu di bawah buat mulai:_"

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

	textKlikcepat = "🔗 *KLIKCEPAT — Bio Link & Short URL*\n\n" +
		"Manage link & project di klikcepat.com langsung dari bot.\n\n" +
		"*Yang bisa dilakuin:*\n" +
		"• ➕ *Tambah Link* — bikin shortlink/biolink page baru\n" +
		"• 📋 *List Link* — liat semua link kamu (paginated)\n" +
		"• ✏️ *Edit Link* — update title/slug/target URL/project\n" +
		"• 🗑 *Hapus Link* — delete link permanent\n" +
		"• 📁 *Projects* — manage project grouping\n\n" +
		"_💡 Auto-Swap setup ada di menu *🔄 Auto Rotator* (unified untuk CF + Klikcepat)._"

	textKlikcepatProjects = "📁 *KLIKCEPAT — Projects*\n\n" +
		"Group link berdasarkan project (misal: KONTAK, PROMO, RTP).\n\n" +
		"Project bisa di-assign saat create link, dan bisa difilter di List Link."
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
			m.Data("🔗 KLIKCEPAT", cbKlikcepat),
		),
		m.Row(
			m.Data("🔧 Settings", cbSettings),
		),
	)
	return m
}

// ─── Settings Menus ──────────────────────────────────────────────────────────
//
// Hierarchy:
//   🔧 Settings (cbSettings → handleSettings)
//   └── Hub picker: pick CF or Klikcepat
//       ├── ⚙️ Cloudflare → settingsCFMenu (set email/key/test/clear)
//       └── 🔗 Klikcepat → klikcepatSettingsMenu (set URL/key/test/clear)

// settingsMenu is the HUB picker (top-level Settings entry).
func settingsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("⚙️ Cloudflare", cbSettingsCF),
			m.Data("🔗 Klikcepat", cbSettingsKlikcepat),
		),
		m.Row(m.Data("🔙 Kembali", cbMain)),
	)
	return m
}

// settingsCFMenu — CF-specific settings (was the OLD settingsMenu).
func settingsCFMenu() *tele.ReplyMarkup {
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
		m.Row(m.Data("🔙 Kembali", cbSettings)),
	)
	return m
}

// backToSettings goes back to the Settings Hub picker.
func backToSettings() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbSettings)))
	return m
}

// backToSettingsCF goes back to the CF settings sub-menu.
// Use this for CF sub-handler success messages (so user lands back in CF section, not hub).
func backToSettingsCF() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbSettingsCF)))
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

// ─── Klikcepat Menus ──────────────────────────────────────────────────────────

func klikcepatMenu(botUsername string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	rows := []tele.Row{
		m.Row(
			m.Data("➕ Tambah Link", cbKlikcepatAdd),
			m.Data("📋 List Link", cbKlikcepatList),
		),
		m.Row(
			m.Data("✏️ Edit Link", cbKlikcepatEdit),
			m.Data("🗑 Hapus Link", cbKlikcepatDelete),
		),
		m.Row(
			m.Data("📁 Projects", cbKlikcepatProjects),
		),
	}
	if botUsername != "" {
		// Dashboard quick-access (URL button)
		rows = append(rows, m.Row(
			m.URL("🌐 Open Dashboard", "https://klikcepat.com"),
		))
	}
	rows = append(rows, m.Row(m.Data("🔙 Kembali", cbMain)))
	m.Inline(rows...)
	return m
}

func backToKlikcepat() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbKlikcepat)))
	return m
}

func klikcepatProjectsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(
			m.Data("➕ Tambah Project", cbKlikcepatProjectAdd),
			m.Data("📋 List Project", cbKlikcepatProjectList),
		),
		m.Row(
			m.Data("✏️ Edit Project", cbKlikcepatProjectEdit),
			m.Data("🗑 Hapus Project", cbKlikcepatProjectDelete),
		),
		m.Row(m.Data("🔙 Kembali", cbKlikcepat)),
	)
	return m
}

func backToKlikcepatProjects() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("🔙 Kembali", cbKlikcepatProjects)))
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

// ─── Group Menu (Read-only) ───────────────────────────────────────────────────
// Cuma 4 tombol: Status, List Domain, List CF, Setup di DM.
// Wizard/setup gak ada di group — semua redirect ke DM.

const textGroupWelcome = "🎰🚀 *BONG BOT — ALL IN ONE FITUR BOT PALING GACOR!* 🔥\n\n" +
	"_Group ini cuma buat notifikasi alert nawala & auto-swap._\n\n" +
	"Setup & konfigurasi lewat *DM bot* langsung.\n\n" +
	"Tombol di bawah cuma read-only — buat liat status & list."

func groupMenu(botUsername string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	rows := []tele.Row{
		m.Row(
			m.Data("🩺 Status Bot", cbGroupStatus),
			m.Data("📋 List Domain", cbGroupListDomain),
		),
		m.Row(
			m.Data("🔄 List CF", cbGroupListCF),
		),
	}
	if botUsername != "" {
		// URL button deep-link → langsung jump ke DM bot
		rows = append(rows, m.Row(
			m.URL("🤖 Setup di DM →", "https://t.me/"+botUsername+"?start=setup"),
		))
	}
	m.Inline(rows...)
	return m
}

func backToGroup(botUsername string) *tele.ReplyMarkup {
	// Group views balik ke group welcome via "main" callback (yang re-route ke groupMenu di group)
	return groupMenu(botUsername)
}

// ─── Non-Admin Reject Template ───────────────────────────────────────────────

const textNonAdminReject = "🔒 *This is private bot*\n\n" +
	"You need access to use this bot.\n" +
	"Contact %s for access."

func nonAdminRejectMenu(contactUsername string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.URL("💬 Chat @"+contactUsername, "https://t.me/"+contactUsername)),
	)
	return m
}

// ─── Alert Action Buttons (per blocked alert) ────────────────────────────────

func alertActionMenu(domain string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("🗑 Hapus dari Monitor", cbAlertRemove, domain)),
	)
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
