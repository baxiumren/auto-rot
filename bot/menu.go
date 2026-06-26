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
	cbKlikcepatEditShortlink  = "klc_edit_sl"     // param = page
	cbKlikcepatEditBiolink    = "klc_edit_bl"     // param = page
	cbKlikcepatEditSearchSL   = "klc_edit_srch_sl" // search prompt — shortlink
	cbKlikcepatEditSearchBL   = "klc_edit_srch_bl" // search prompt — biolink
	cbKlikcepatEditSearchPage = "klc_edit_srch_pg" // pagination dalam search result. param = "type|page"
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

	// List Rotator picker (split by type)
	cbRotatorListCF        = "rot_list_cf"
	cbRotatorListKlc       = "rot_list_klc"
	cbRotatorListKlcSL     = "rot_list_klc_sl"
	cbRotatorListKlcBL     = "rot_list_klc_bl"

	// Group Commands (slash commands buat member di group)
	cbGroupCmd              = "gcmd"
	cbGroupCmdAdd           = "gcmd_add"
	cbGroupCmdPickProject   = "gcmd_pick" // param = project_id
	cbGroupCmdDelete        = "gcmd_del"  // param = command name
	cbGroupCmdDeleteConfirm = "gcmd_del_y"

	// LinkFB (separate Pixly instance — shortlink + project only, no biolink edit)
	cbLinkfb              = "linkfb"
	cbLinkfbAdd           = "linkfb_add"
	cbLinkfbAddType       = "linkfb_addt"   // param = type
	cbLinkfbAddPickProj   = "linkfb_addpr"  // param = project_id
	cbLinkfbList          = "linkfb_list"   // param = "page"
	cbLinkfbEdit          = "linkfb_edit"   // param = page
	cbLinkfbEditPick      = "linkfb_editp"  // param = link_id
	cbLinkfbEditField     = "linkfb_editf"  // param = field
	cbLinkfbDelete        = "linkfb_del"    // param = page
	cbLinkfbDeletePick    = "linkfb_delp"   // param = link_id
	cbLinkfbDeleteConfirm = "linkfb_delc"   // param = link_id

	// LinkFB Projects (CRUD parity dengan klikcepat)
	cbLinkfbProjects             = "linkfb_proj"
	cbLinkfbProjectAdd           = "linkfb_proj_add"
	cbLinkfbProjectList          = "linkfb_proj_list"
	cbLinkfbProjectEdit          = "linkfb_proj_edit"
	cbLinkfbProjectEditPick      = "linkfb_proj_editp" // param = project_id
	cbLinkfbProjectDelete        = "linkfb_proj_del"
	cbLinkfbProjectDeletePick    = "linkfb_proj_delp" // param = project_id
	cbLinkfbProjectDeleteConfirm = "linkfb_proj_delc" // param = project_id

	// LinkFB Settings
	cbSettingsLinkfb       = "set_lfb"
	cbSettingsLinkfbSetURL = "set_lfb_url"
	cbSettingsLinkfbSetKey = "set_lfb_key"
	cbSettingsLinkfbTest   = "set_lfb_test"
	cbSettingsLinkfbClear  = "set_lfb_clear"

	// Info Tools (Telegram utility)
	cbTools          = "tools"
	cbToolsUserID    = "t_uid"   // /id @username
	cbToolsChatID    = "t_cid"   // /cekid <t.me link>
	cbToolsUserInfo  = "t_uinfo" // /info @username
	cbToolsChatInfo  = "t_cinfo" // /cinfo @username
	cbToolsHelp      = "t_help"

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

	textMonitor = "💎 *M O N I T O R   D O M A I N* 💎\n" +
		"|\n" +
		"📡 *FUNGSI*\n" +
		"└ Daftarin domain yg mau dipantau\n" +
		"└ Cek nawala Kominfo + TrustPositif + Nawala\n" +
		"└ Group per label = pool untuk Auto Rotator\n" +
		"|\n" +
		"📂 *CONTOH LABEL*\n" +
		"└ KWAI — semua domain promo Kwai\n" +
		"└ MONEYSITE — money page utama\n" +
		"└ STOCK-MS — cadangan MoneySite\n" +
		"└ RTP — page RTP\n" +
		"|\n" +
		"💡 *TIPS*\n" +
		"└ 1 label = domain serupa (fungsi sama)\n" +
		"└ Kalau 1 keblock → bot swap ke label yg sama\n" +
		"└ Min 2 domain per label biar swap ada cadangan\n" +
		"|\n" +
		"🎯 Pilih action di bawah 👇"

	textCF = "💎 *C F   R E D I R E C T* 💎\n" +
		"|\n" +
		"⚙️ *FUNGSI*\n" +
		"└ Connect bot ke redirect rule Cloudflare\n" +
		"└ Auto-swap URL tujuan saat domain blocked\n" +
		"|\n" +
		"📐 *APA ITU REDIRECT RULE?*\n" +
		"└ Rule CF yg ngarahin domain A → URL B\n" +
		"└ Contoh: `iklan.com` → `landing.com`\n" +
		"|\n" +
		"🎯 *YG BISA DILAKUIN*\n" +
		"└ ➕ Daftarin rule via nama domain\n" +
		"└ ✏️ Ganti URL tujuan (manual / Auto Rotator)\n" +
		"└ 📋 List + bulk operations\n" +
		"|\n" +
		"💡 *TIPS*\n" +
		"└ Gak perlu Zone ID / Rule ID\n" +
		"└ Cukup nama domain — bot fetch sendiri"

	textRotator = "💎 *A U T O   R O T A T O R* 💎\n" +
		"|\n" +
		"🔄 *FUNGSI*\n" +
		"└ Section paling penting!\n" +
		"└ Gabungkan: CF Rule / Klikcepat link + Pool\n" +
		"└ Bot auto-swap saat domain keblock\n" +
		"|\n" +
		"⚙️ *CARA KERJA SWAP OTOMATIS*\n" +
		"└ 1️⃣ Monitor pantau semua domain 24/7\n" +
		"└ 2️⃣ Domain BLOCKED → cross-check rule/link\n" +
		"└ 3️⃣ Match → cek Rotator config\n" +
		"└ 4️⃣ Ambil domain next dari Pool → swap\n" +
		"|\n" +
		"📦 *YG BISA DI-SETUP*\n" +
		"└ ⚙️ CF Redirect rotator\n" +
		"└ 🔗 Klikcepat shortlink rotator\n" +
		"└ 📄 Klikcepat biolink block rotator\n" +
		"└ 📋 List + Bulk Setup multi-rule\n" +
		"|\n" +
		"⚠️ *SYARAT*\n" +
		"└ Tanpa Rotator config → auto-swap MATI\n" +
		"└ Min 2 domain di pool Monitor\n" +
		"└ Udah ada CF Rule / Klikcepat link"

	textKlikcepat = "💎 *K L I K C E P A T* 💎\n" +
		"|\n" +
		"🔗 *FUNGSI*\n" +
		"└ Manage link & project di klikcepat.com\n" +
		"└ Langsung dari bot — gak perlu buka web\n" +
		"|\n" +
		"🎯 *YG BISA DILAKUIN*\n" +
		"└ ➕ Tambah Link — bikin shortlink/biolink baru\n" +
		"└ 📋 List Link — liat semua link (paginated)\n" +
		"└ ✏️ Edit Link — update title/slug/target/project\n" +
		"└ 🗑 Hapus Link — delete permanent\n" +
		"└ 📁 Projects — manage project grouping\n" +
		"|\n" +
		"💡 *AUTO-SWAP*\n" +
		"└ Setup di menu 🔄 Auto Rotator\n" +
		"└ Unified untuk CF + Klikcepat"

	textKlikcepatProjects = "💎 *K L I K C E P A T   P R O J E C T S* 💎\n" +
		"|\n" +
		"📁 *FUNGSI*\n" +
		"└ Group link by project\n" +
		"└ Contoh: KONTAK, PROMO, RTP, DAFTAR\n" +
		"|\n" +
		"🎯 *PEMAKAIAN*\n" +
		"└ Assign saat create link\n" +
		"└ Filter di List Link\n" +
		"└ Source utama Group Commands (`/rtp`, dll)"
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
			m.Data("🔗 LINKFB", cbLinkfb),
			m.Data("🛠 Info Tools", cbTools),
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
		m.Row(m.Data("🔗 LinkFB", cbSettingsLinkfb)),
		m.Row(m.Data("💬 Group Commands", cbGroupCmd)),
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

const textGroupWelcome = "💎 *B O N G   B O T* 💎\n" +
	"|\n" +
	"🎰 *ALL IN ONE FITUR BOT PALING GACOR* 🔥\n" +
	"|\n" +
	"🔔 *BUAT MEMBER*\n" +
	"└ Ketik `/` di chat → autocomplete muncul\n" +
	"└ Contoh: /rtp, /daftar, /login, /bukti, /apk, /wa\n" +
	"└ Bot kasih link aktif — auto-skip yg keblock 🚀\n" +
	"|\n" +
	"⚙️ *BUAT ADMIN*\n" +
	"└ Group ini buat notifikasi alert nawala\n" +
	"└ Setup & konfigurasi lewat DM bot\n" +
	"└ Tombol di bawah read-only (status & list)"

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
