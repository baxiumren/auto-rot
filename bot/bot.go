package bot

import (
	"log"
	"strings"

	"bongbot/cloudflare"
	"bongbot/config"
	"bongbot/rotator"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

type Handler struct {
	cfg        *config.Config
	bot        *tele.Bot
	sessions   *sessionStore
	domains    *store.DomainStore
	cfrules    *store.CFRuleStore
	rotators   *store.RotatorStore
	creds      *store.CredentialStore
	cf         *cloudflare.Client
	rotSvc     *rotator.Service
	monScanner *rotator.MonitorScanner
	history    *store.HistoryStore
}

type botNotifier struct {
	b      *tele.Bot
	chatID int64
}

func (n *botNotifier) Notify(msg string) {
	if _, err := n.b.Send(&tele.Chat{ID: n.chatID}, msg, tele.ModeMarkdown); err != nil {
		log.Printf("[NOTIFY ERROR] %v", err)
	}
}

func New(
	b *tele.Bot,
	cfg *config.Config,
	domains *store.DomainStore,
	cfrules *store.CFRuleStore,
	rotators *store.RotatorStore,
	creds *store.CredentialStore,
	cf *cloudflare.Client,
	rotSvc *rotator.Service,
	monScanner *rotator.MonitorScanner,
	history *store.HistoryStore,
) *Handler {
	return &Handler{
		cfg:        cfg,
		bot:        b,
		sessions:   newSessionStore(),
		domains:    domains,
		cfrules:    cfrules,
		rotators:   rotators,
		creds:      creds,
		cf:         cf,
		rotSvc:     rotSvc,
		monScanner: monScanner,
		history:    history,
	}
}

func (h *Handler) Register() {
	h.bot.Handle("/start", h.handleStart)
	h.bot.Handle("/menu", h.handleMenu)

	// Callbacks routing
	h.bot.Handle(tele.OnCallback, h.handleCallback)

	// Text input (wizard steps)
	h.bot.Handle(tele.OnText, h.handleText)
}

// isAllowed: boleh pakai bot kalau:
// 1. Pesan dari grup yang di-allowlist (ALLOWED_CHAT_ID), ATAU
// 2. DM dari admin (ADMIN_IDS)
func (h *Handler) isAllowed(c tele.Context) bool {
	if h.cfg.AllowedChatID != 0 && c.Chat().ID == h.cfg.AllowedChatID {
		return true
	}
	return h.cfg.IsAdmin(c.Sender().ID)
}

func (h *Handler) handleStart(c tele.Context) error {
	if !h.isAllowed(c) {
		return nil
	}
	// Kirim reply keyboard persistent (4 tombol shortcut di bawah chat input)
	h.bot.Send(c.Chat(),
		"👋 *Selamat datang!*\n\n"+
			"4 tombol shortcut udah ter-pin di bawah chat — gak perlu ngetik command:\n"+
			"• *🚀 START* — restart ke welcome\n"+
			"• *🏠 MENU* — balik ke menu utama\n"+
			"• *🩺 STATUS* — health dashboard (cek kondisi bot)\n"+
			"• *🔍 CARI* — cari domain di mana aja (Monitor / CF / Rotator / sticky)",
		startReplyKeyboard(), tele.ModeMarkdown)

	// Cek apakah CF credential udah di-set. Kalau belum → arahin ke Settings dulu.
	if !h.cf.HasCredentials() {
		return c.Send(
			"🤖 *BongBot — Auto Domain Rotator*\n\n"+
				"Bot ini bantu kamu *otomatis ganti domain redirect* di Cloudflare kalau domain kena blokir Kominfo (nawala).\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"⚠️ *Setup awal — wajib dulu:*\n\n"+
				"Sebelum pakai fitur lain, set dulu *email & API key Cloudflare* kamu di menu *🔧 Settings*. "+
				"Klik tombol Settings di bawah 👇",
			mainMenu(), tele.ModeMarkdown,
		)
	}

	return c.Send(
		"🤖 *BongBot — Auto Domain Rotator*\n\n"+
			"Bot ini bantu kamu *otomatis ganti domain redirect* di Cloudflare kalau domain kena blokir Kominfo.\n\n"+
			"━━━━━━━━━━━━━━━━━━\n"+
			"*🔰 Buat pemula, urutannya gini:*\n\n"+
			"1️⃣ *📡 Monitor* — daftarin semua domain kamu (pengelompokan pake label)\n\n"+
			"2️⃣ *⚙️ CF Redirect* — daftarin redirect rule yang ada di Cloudflare (cukup ketik nama domainnya, bot fetch sendiri)\n\n"+
			"3️⃣ *🔄 Auto Rotator* — hubungin CF Rule + pool domain → bot kerja otomatis 24/7\n\n"+
			"_Pilih menu untuk mulai 👇_",
		mainMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) handleMenu(c tele.Context) error {
	if !h.isAllowed(c) {
		return nil
	}
	return c.Send("🏠 *Menu Utama*\n\nPilih section:", mainMenu(), tele.ModeMarkdown)
}

// ─── Callback Router ──────────────────────────────────────────────────────────

func (h *Handler) handleCallback(c tele.Context) error {
	if !h.isAllowed(c) {
		return c.Respond(&tele.CallbackResponse{Text: "⛔ Akses ditolak"})
	}

	c.Respond()
	data := c.Data()

	// Telebot v3 encode callback data sebagai "\f{unique}|{param1}|{param2}|..."
	// Strip "\f" prefix, lalu split pakai "|" (BUKAN spasi!)
	data = strings.TrimPrefix(data, "\f")
	parts := strings.SplitN(data, "|", 2)
	unique := parts[0]
	param := ""
	if len(parts) > 1 {
		param = parts[1]
	}
	_ = param // beberapa handler pakai extractParam(c) langsung

	log.Printf("[CB] unique=%s param=%s user=%d", unique, param, c.Sender().ID)

	switch unique {
	case cbMain:
		return c.Edit(textMain, mainMenu(), tele.ModeMarkdown)
	case cbCancel:
		h.sessions.Delete(c.Sender().ID)
		return c.Edit(textMain, mainMenu(), tele.ModeMarkdown)

	// Monitor
	case cbMonitor:
		return h.handleMonitor(c)
	case cbMonitorAdd:
		if param != "" {
			// Label dipilih dari tombol (bukan ketik manual)
			sess, ok := h.sessions.Get(c.Sender().ID)
			if ok && sess.Step == StepMonitorAddLabel {
				return h.doAddDomain(c, sess, param)
			}
		}
		return h.handleMonitorAdd(c)
	case cbMonitorRemove:
		return h.handleMonitorRemove(c)
	case cbMonitorList:
		return h.handleMonitorList(c)
	case cbMonitorCheck:
		return h.handleMonitorCheck(c)
	case cbMonitorInterval:
		return h.handleMonitorInterval(c)
	case cbMonitorStatus:
		return h.handleMonitorStatus(c)
	case cbMonitorSticky:
		return h.handleMonitorSticky(c)
	case cbMonitorStickyDel:
		return h.handleMonitorStickyDel(c)
	case cbMonitorForce:
		return h.handleMonitorForce(c)
	case cbMonitorForceAdd:
		return h.handleMonitorForceAdd(c)
	case cbMonitorForceDel:
		return h.handleMonitorForceDel(c)
	case cbMonitorStickyClean:
		return h.handleMonitorStickyClean(c)
	case cbMonitorForceClean:
		return h.handleMonitorForceClean(c)

	// CF Redirect
	case cbCF:
		return h.handleCF(c)
	case cbCFAdd:
		return h.handleCFAdd(c)
	case cbCFAddPickV1:
		return h.handleCFAddPickType(c, "page_rules")
	case cbCFAddPickV2:
		return h.handleCFAddPickType(c, "redirect_rules")
	case cbCFAddPickIdx:
		return h.handleCFAddPickRule(c)
	case cbCFList:
		return h.handleCFList(c)
	case cbCFChange:
		if param != "" {
			return h.handleCFChangeSelect(c)
		}
		return h.handleCFChangeMenu(c)
	case cbCFDelete:
		if param != "" {
			return h.handleCFDeleteConfirm(c)
		}
		return h.handleCFDeleteMenu(c)

	// CF Bulk Change
	case cbCFBulk:
		return h.handleCFBulkStart(c)
	case cbCFBulkToggle:
		return h.handleCFBulkToggle(c)
	case cbCFBulkSelAll:
		return h.handleCFBulkSelectAll(c, true)
	case cbCFBulkSelNone:
		return h.handleCFBulkSelectAll(c, false)
	case cbCFBulkApply:
		return h.handleCFBulkApply(c)

	// CF New Domain Registration
	case cbCFNewYes:
		return h.handleCFNewYes(c)
	case cbCFNewTypeV2:
		return h.handleCFNewPickType(c, "redirect_rules")
	case cbCFNewTypeV1:
		return h.handleCFNewPickType(c, "page_rules")

	// Auto Rotator
	case cbRotator:
		return h.handleRotator(c)
	case cbRotatorAdd:
		return h.handleRotatorAdd(c)
	case cbRotatorCFSel:
		return h.handleRotatorCFSelect(c)
	case cbRotatorPool:
		return h.handleRotatorPoolSelect(c)
	case cbRotatorList:
		return h.handleRotatorList(c)
	case cbRotatorToggle:
		return h.handleRotatorToggle(c)
	case cbRotatorDelete:
		return h.handleRotatorDelete(c)
	case cbRotatorForce:
		return h.handleRotatorForce(c)

	// Bulk Setup Rotator
	case cbRotatorBulk:
		return h.handleRotatorBulk(c)
	case cbRotatorBulkToggle:
		return h.handleRotatorBulkToggle(c)
	case cbRotatorBulkSelAll:
		return h.handleRotatorBulkSelectAll(c, true)
	case cbRotatorBulkSelNone:
		return h.handleRotatorBulkSelectAll(c, false)
	case cbRotatorBulkProceed:
		return h.handleRotatorBulkProceed(c)
	case cbRotatorBulkPickPool:
		return h.handleRotatorBulkPickPool(c)

	// Health Dashboard & History
	case cbHistory:
		return h.handleHistory(c)
	case cbHistoryClear:
		return h.handleHistoryClearConfirm(c)
	case cbHistoryClearYes:
		return h.handleHistoryClearDo(c)

	// Settings
	case cbSettings:
		return h.handleSettings(c)
	case cbSettingsSetEmail:
		return h.handleSettingsSetEmail(c)
	case cbSettingsSetKey:
		return h.handleSettingsSetKey(c)
	case cbSettingsSetBoth:
		return h.handleSettingsSetBoth(c)
	case cbSettingsTest:
		return h.handleSettingsTest(c)
	case cbSettingsClear:
		return h.handleSettingsClearConfirm(c)
	case cbSettingsClearYes:
		return h.handleSettingsClearDo(c)
	}

	return nil
}

// ─── Text Router (Wizard Steps) ───────────────────────────────────────────────

func (h *Handler) handleText(c tele.Context) error {
	if !h.isAllowed(c) {
		log.Printf("[TEXT] DENIED user=%d chat=%d text=%q", c.Sender().ID, c.Chat().ID, c.Text())
		return nil
	}

	// Intercept reply keyboard buttons — override session apapun.
	switch strings.TrimSpace(c.Text()) {
	case replyBtnStart:
		h.sessions.Delete(c.Sender().ID)
		return h.handleStart(c)
	case replyBtnMenu:
		h.sessions.Delete(c.Sender().ID)
		return h.handleMenu(c)
	case replyBtnStatus:
		h.sessions.Delete(c.Sender().ID)
		return h.handleHealth(c)
	case replyBtnSearch:
		h.sessions.Delete(c.Sender().ID)
		return h.handleSearchPrompt(c)
	}

	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok {
		log.Printf("[TEXT] NO_SESSION user=%d text=%q (ignored)", c.Sender().ID, c.Text())
		return nil
	}

	log.Printf("[TEXT] user=%d step=%s text=%q", c.Sender().ID, sess.Step, c.Text())

	switch sess.Step {
	// Monitor
	case StepMonitorAddDomain:
		return h.wizardMonitorAddDomain(c, sess)
	case StepMonitorAddLabel:
		return h.wizardMonitorAddLabel(c, sess)
	case StepMonitorRemove:
		return h.wizardMonitorRemove(c, sess)
	case StepMonitorCheck:
		return h.wizardMonitorCheck(c, sess)
	case StepMonitorInterval:
		return h.wizardMonitorInterval(c, sess)
	case StepMonitorForceAdd:
		return h.wizardMonitorForceAdd(c, sess)

	// CF Redirect
	case StepCFAddLabel:
		return h.wizardCFAddLabel(c, sess)
	case StepCFAddDomain:
		return h.wizardCFAddDomain(c, sess)
	case StepCFAddPickType, StepCFAddPickRule:
		return nil // handled via callback button
	case StepCFChangeURL:
		return h.wizardCFChangeURL(c, sess)
	case StepCFBulkURL:
		return h.wizardCFBulkURL(c, sess)
	case StepCFBulkPick:
		return nil // handled via callback button

	// CF New Domain
	case StepCFNewConfirm, StepCFNewPickType:
		return nil // handled via callback
	case StepCFNewTargetURL:
		return h.wizardCFNewTargetURL(c, sess)

	// Auto Rotator
	case StepRotatorAddLabel:
		return h.wizardRotatorAddLabel(c, sess)
	case StepRotatorBulkPick, StepRotatorBulkPool:
		return nil // handled via callback button

	// Global search
	case StepGlobalSearch:
		return h.wizardGlobalSearch(c, sess)

	// Settings
	case StepSettingsEmail:
		return h.wizardSettingsEmail(c, sess)
	case StepSettingsKey:
		return h.wizardSettingsKey(c, sess)
	case StepSettingsBothEmail:
		return h.wizardSettingsBothEmail(c, sess)
	case StepSettingsBothKey:
		return h.wizardSettingsBothKey(c, sess)
	}

	return nil
}

// extractParam mengambil param dari callback data.
// Telebot v3 encode data sebagai "\f{unique}|{param1}|{param2}|..."
// extractParam() return semua setelah unique pertama (bisa "param1|param2|..." kalau multi).
func extractParam(c tele.Context) string {
	data := strings.TrimPrefix(c.Data(), "\f")
	if i := strings.Index(data, "|"); i >= 0 {
		return data[i+1:]
	}
	return ""
}
