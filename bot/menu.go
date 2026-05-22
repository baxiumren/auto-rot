package bot

import tele "gopkg.in/telebot.v3"

// ─── Callback Unique IDs ──────────────────────────────────────────────────────

const (
	cbMain = "main"

	// Monitor
	cbMonitor         = "monitor"
	cbMonitorAdd      = "monitor_add"
	cbMonitorRemove   = "monitor_remove"
	cbMonitorList     = "monitor_list"
	cbMonitorCheck    = "monitor_check"
	cbMonitorInterval = "monitor_interval"
	cbMonitorStatus   = "monitor_status"

	// CF Redirect
	cbCF       = "cf"
	cbCFAdd    = "cf_add"
	cbCFList   = "cf_list"
	cbCFChange = "cf_change" // Data = ruleID
	cbCFDelete = "cf_delete" // Data = ruleID

	// Auto Rotator
	cbRotator       = "rotator"
	cbRotatorAdd    = "rotator_add"
	cbRotatorList   = "rotator_list"
	cbRotatorToggle = "rotator_toggle" // Data = rotatorID
	cbRotatorDelete = "rotator_delete" // Data = rotatorID
	cbRotatorForce  = "rotator_force"  // Data = rotatorID
	cbRotatorCFSel  = "rotator_cfsel"  // Data = cfRuleID (wizard: pilih CF rule)
	cbRotatorPool   = "rotator_pool"   // Data = poolLabel (wizard: pilih pool)

	// Cancel
	cbCancel = "cancel"
)

// ─── Menu Texts ───────────────────────────────────────────────────────────────

const (
	textMain    = "🏠 *Menu Utama*\n\nPilih section:"
	textMonitor = "📡 *Monitor*\n\nKelola domain yang ingin dipantau nawala-nya:"
	textCF      = "⚙️ *CF Redirect*\n\nKelola Cloudflare redirect rules:"
	textRotator = "🔄 *Auto Rotator*\n\nOtomatis ganti domain CF ketika kena nawala:"
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
		),
	)
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
			m.Data("📋 List Rotator", cbRotatorList),
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
