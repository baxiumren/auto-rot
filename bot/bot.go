package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"bongbot/checker"
	"bongbot/cloudflare"
	"bongbot/config"
	"bongbot/klikcepat"
	"bongbot/rotator"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// hint cooldown — biar bot gak nyepam reminder kalau user ngetik banyak teks
// di group chat (misal: diskusi normal yang kebetulan ada dot-nya)
var (
	hintMu       sync.Mutex
	hintLastSent = make(map[int64]time.Time)
)

const hintCooldown = 3 * time.Minute

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

	klikcepat         *klikcepat.Client
	klikcepatRotators *store.KlikcepatRotatorStore
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

// NotifyBlockedAlert: alert blocked + button "🗑 Hapus dari Monitor".
func (n *botNotifier) NotifyBlockedAlert(msg, domain string) {
	mkup := &tele.ReplyMarkup{}
	mkup.Inline(
		mkup.Row(mkup.Data("🗑 Hapus dari Monitor", cbAlertRemove, domain)),
	)
	if _, err := n.b.Send(&tele.Chat{ID: n.chatID}, msg, mkup, tele.ModeMarkdown); err != nil {
		log.Printf("[NOTIFY-ALERT ERROR] %v", err)
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
	klc *klikcepat.Client,
	klcRotators *store.KlikcepatRotatorStore,
) *Handler {
	return &Handler{
		cfg:               cfg,
		bot:               b,
		sessions:          newSessionStore(),
		domains:           domains,
		cfrules:           cfrules,
		rotators:          rotators,
		creds:             creds,
		cf:                cf,
		rotSvc:            rotSvc,
		monScanner:        monScanner,
		history:           history,
		klikcepat:         klc,
		klikcepatRotators: klcRotators,
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

// isAllowed: filter siapa yang boleh interact dengan bot.
//
// Aturan:
//   - DM (chat private) → cuma ADMIN_IDS yang boleh.
//     Non-admin yang DM → di-reject dengan pesan "private bot, contact @owner".
//   - Group → cuma ALLOWED_CHAT_ID yang dilayani.
//     Di group, SEMUA member boleh liat menu read-only (Status, List Domain, List CF).
//     Tapi callback yang butuh admin action (misal Hapus Domain dari alert) di-check
//     ADMIN_IDS terpisah di handler-nya.
//
// Return: (allowed bool, isDM bool). isDM dipake handler buat decide UI (group vs DM).
func (h *Handler) isAllowed(c tele.Context) (allowed, isDM bool) {
	isDM = c.Chat().Type == tele.ChatPrivate

	if isDM {
		// DM cuma untuk admin
		return h.cfg.IsAdmin(c.Sender().ID), true
	}

	// Group: cek chat ID di-allowlist
	if h.cfg.AllowedChatID == 0 || c.Chat().ID != h.cfg.AllowedChatID {
		return false, false
	}
	// Di group, sembarang user boleh view (action button cek admin terpisah)
	return true, false
}

// requireAdmin: gate untuk callback yang butuh privilege admin di group.
// Return true kalau OK, false kalau ditolak (response udah dikirim).
func (h *Handler) requireAdmin(c tele.Context) bool {
	if h.cfg.IsAdmin(c.Sender().ID) {
		return true
	}
	c.Respond(&tele.CallbackResponse{
		Text:      "⛔ Cuma admin yang boleh aksi ini.",
		ShowAlert: true,
	})
	return false
}

func (h *Handler) handleStart(c tele.Context) error {
	allowed, isDM := h.isAllowed(c)

	// Non-admin DM → reject template + contact button
	if isDM && !allowed {
		return c.Send(
			fmt.Sprintf(textNonAdminReject, "@"+h.cfg.ContactUsername),
			nonAdminRejectMenu(h.cfg.ContactUsername),
			tele.ModeMarkdown,
		)
	}
	if !allowed {
		return nil // group asing → diem aja
	}

	// Group → tampilin group welcome + 4 tombol read-only
	if !isDM {
		return c.Send(textGroupWelcome, groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
	}

	// DM admin → tampilin reply keyboard + welcome lengkap
	h.bot.Send(c.Chat(),
		"👋🔥 *Halo bos, selamat datang di markas BONG BOT!* 🎰\n\n"+
			"⚡ 4 tombol shortcut ter-pin di bawah chat (gak perlu ngetik command):\n"+
			"• *🚀 START* — restart ke welcome\n"+
			"• *🏠 MENU* — balik ke menu utama\n"+
			"• *🩺 STATUS* — health dashboard (cek kondisi bot)\n"+
			"• *🔍 CARI* — cari domain di mana aja (Monitor / CF / Rotator / sticky)",
		startReplyKeyboard(), tele.ModeMarkdown)

	if !h.cf.HasCredentials() {
		return c.Send(
			"🎰🚀 *BONG BOT — ALL IN ONE FITUR BOT PALING GACOR!* 🔥\n\n"+
				"🤖 Bot anti-nawala buat pejuang affiliate Indonesia. Auto-swap domain Cloudflare + Klikcepat saat kena blokir, biar duit gak putus! 💰\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"⚠️ *Setup awal — wajib dulu bos:*\n\n"+
				"Sebelum pakai fitur auto-swap, set dulu credentials di menu *🔧 Settings*:\n"+
				"• ⚙️ *Cloudflare* — email + Global API Key\n"+
				"• 🔗 *Klikcepat* — API Key (kalau pake klikcepat)\n\n"+
				"Klik tombol Settings di bawah 👇",
			mainMenu(), tele.ModeMarkdown,
		)
	}

	return c.Send(
		fmt.Sprintf(
			"🎰🚀 *BONG BOT v%s — ALL IN ONE FITUR BOT PALING GACOR!* 🔥\n\n"+
				"🤖 _Bot anti-nawala buat pejuang affiliate Indonesia._ 🇮🇩\n\n"+
				"💡 *Yang bot ini bisa:*\n"+
				"• 👀 Pantau domain 24/7 (Kominfo + TrustPositif + NawalaCheck)\n"+
				"• ⚡ Auto-swap Cloudflare redirect rule pas kena blokir\n"+
				"• 🔗 Auto-swap link klikcepat (biolink + shortlink) juga\n"+
				"• 📦 Bulk setup — handle banyak rule/link sekaligus\n"+
				"• 🛡 Multi-admin private bot dengan group alert mode\n\n"+
				"━━━━━━━━━━━━━━━━━━\n"+
				"*🔰 Buat pemula, urutannya:*\n\n"+
				"1️⃣ *📡 Monitor* — daftarin semua domain kamu (per label)\n\n"+
				"2️⃣ *⚙️ CF Redirect* atau *🔗 KLIKCEPAT* — register rule/link\n\n"+
				"3️⃣ *🔄 Auto Rotator* — hubungin CF/Klikcepat + pool domain → bot kerja otomatis 24/7\n\n"+
				"🎯 _Pilih menu di bawah buat mulai 👇_",
			config.Version),
		mainMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) handleMenu(c tele.Context) error {
	allowed, isDM := h.isAllowed(c)
	if !allowed {
		if isDM {
			return c.Send(
				fmt.Sprintf(textNonAdminReject, "@"+h.cfg.ContactUsername),
				nonAdminRejectMenu(h.cfg.ContactUsername),
				tele.ModeMarkdown,
			)
		}
		return nil
	}
	if !isDM {
		return c.Send(textGroupWelcome, groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
	}
	return c.Send("🏠 *Menu Utama*\n\nPilih section:", mainMenu(), tele.ModeMarkdown)
}

// ─── Callback Router ──────────────────────────────────────────────────────────

func (h *Handler) handleCallback(c tele.Context) error {
	allowed, isDM := h.isAllowed(c)
	if !allowed {
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

	log.Printf("[CB] unique=%s param=%s user=%d isDM=%v", unique, param, c.Sender().ID, isDM)

	// ─── Group whitelist ──────────────────────────────────────────────────────
	// Di group: cuma 4 callback yg allowed. Sisanya redirect ke "Setup di DM".
	if !isDM {
		switch unique {
		case cbGroupStatus:
			return h.handleGroupStatus(c)
		case cbGroupListDomain:
			return h.handleGroupListDomain(c)
		case cbGroupListCF:
			return h.handleGroupListCF(c)
		case cbAlertRemove:
			return h.handleAlertRemove(c, param)
		case cbMain:
			return c.Edit(textGroupWelcome, groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
		default:
			return c.Respond(&tele.CallbackResponse{
				Text:      "⚠️ Action ini cuma bisa di DM bot. Klik 🤖 Setup di DM →",
				ShowAlert: true,
			})
		}
	}

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
			// Label dipilih dari tombol (bukan ketik manual).
			// Defensive: butuh session aktif StepMonitorAddLabel DENGAN domain valid.
			// Kalau gak ada (stale button dari pesan lama, session expired, dll) →
			// kasih notif jelas, jangan diam-diam handleMonitorAdd biar user gak bingung.
			sess, ok := h.sessions.Get(c.Sender().ID)
			if !ok {
				return c.Respond(&tele.CallbackResponse{
					Text:      "⚠️ Wizard udah expired/cancelled. Klik ➕ Add Domain lagi.",
					ShowAlert: true,
				})
			}
			if sess.Step != StepMonitorAddLabel {
				return c.Respond(&tele.CallbackResponse{
					Text:      "⚠️ Tombol ini dari sesi lama. Klik MENU dulu.",
					ShowAlert: true,
				})
			}
			if sess.Data["domain"] == "" {
				return c.Respond(&tele.CallbackResponse{
					Text:      "⚠️ Domain di session kosong. Ulang dari Add Domain.",
					ShowAlert: true,
				})
			}
			return h.doAddDomain(c, sess, param)
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
	case cbMonitorCheckKominfo:
		return h.handleMonitorCheckPickSource(c, checker.SourceKominfo)
	case cbMonitorCheckTP:
		return h.handleMonitorCheckPickSource(c, checker.SourceTrustPositif)
	case cbMonitorCheckNawala:
		return h.handleMonitorCheckPickSource(c, checker.SourceNawalaCheck)

	// List Domain pagination
	case cbMonitorListMenuPage:
		return h.handleMonitorListMenuPage(c)
	case cbMonitorListAll:
		return h.handleMonitorListAll(c)
	case cbMonitorListLabel:
		return h.handleMonitorListLabel(c)
	case cbNoop:
		return c.Respond() // page indicator — gak ngapa-ngapain

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
	case cbRotatorAddTypeCF:
		return h.handleRotatorAddTypeCF(c)
	case cbRotatorAddTypeKlikcepat:
		return h.handleRotatorAddTypeKlikcepat(c)
	case cbKlikcepatRotPickLink:
		return h.handleKlikcepatRotPickLink(c)
	case cbKlikcepatRotPickPool:
		return h.handleKlikcepatRotPickPool(c)
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
	case cbRotatorBulkTypeCF:
		return h.handleRotatorBulkTypeCF(c)
	case cbRotatorBulkTypeKlikcepat:
		return h.handleRotatorBulkTypeKlikcepat(c)
	case cbKlikcepatRotBulkToggle:
		return h.handleKlikcepatRotBulkToggle(c)
	case cbKlikcepatRotBulkSelAll:
		return h.handleKlikcepatRotBulkSelectAll(c, true)
	case cbKlikcepatRotBulkSelNone:
		return h.handleKlikcepatRotBulkSelectAll(c, false)
	case cbKlikcepatRotBulkProceed:
		return h.handleKlikcepatRotBulkProceed(c)
	case cbKlikcepatRotBulkPickPool:
		return h.handleKlikcepatRotBulkPickPool(c)
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

	// Klikcepat
	case cbKlikcepat:
		return h.handleKlikcepat(c)
	case cbKlikcepatAdd:
		return h.handleKlikcepatAdd(c)
	case cbKlikcepatAddType:
		return h.handleKlikcepatAddType(c)
	case cbKlikcepatAddPickProject:
		return h.handleKlikcepatAddPickProject(c)
	case cbKlikcepatList:
		return h.handleKlikcepatList(c)
	case cbKlikcepatEdit:
		return h.handleKlikcepatEdit(c)
	case cbKlikcepatEditPick:
		return h.handleKlikcepatEditPick(c)
	case cbKlikcepatEditField:
		return h.handleKlikcepatEditField(c)
	case cbKlikcepatDelete:
		return h.handleKlikcepatDelete(c)
	case cbKlikcepatDeletePick:
		return h.handleKlikcepatDeletePick(c)
	case cbKlikcepatDeleteConfirm:
		return h.handleKlikcepatDeleteConfirm(c)

	case cbKlikcepatProjects:
		return h.handleKlikcepatProjects(c)
	case cbKlikcepatProjectAdd:
		return h.handleKlikcepatProjectAdd(c)
	case cbKlikcepatProjectList:
		return h.handleKlikcepatProjectList(c)
	case cbKlikcepatProjectEdit:
		return h.handleKlikcepatProjectEdit(c)
	case cbKlikcepatProjectEditPick:
		return h.handleKlikcepatProjectEditPick(c)
	case cbKlikcepatProjectDelete:
		return h.handleKlikcepatProjectDelete(c)
	case cbKlikcepatProjectDeletePick:
		return h.handleKlikcepatProjectDeletePick(c)
	case cbKlikcepatProjectDeleteConfirm:
		return h.handleKlikcepatProjectDeleteConfirm(c)

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
	case cbSettingsCF:
		return h.handleSettingsCF(c)
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

	// Klikcepat Settings
	case cbSettingsKlikcepat:
		return h.handleSettingsKlikcepat(c)
	case cbSettingsKlikcepatSetURL:
		return h.handleSettingsKlikcepatSetURL(c)
	case cbSettingsKlikcepatSetKey:
		return h.handleSettingsKlikcepatSetKey(c)
	case cbSettingsKlikcepatTest:
		return h.handleSettingsKlikcepatTest(c)
	case cbSettingsKlikcepatClear:
		return h.handleSettingsKlikcepatClear(c)
	}

	return nil
}

// ─── Text Router (Wizard Steps) ───────────────────────────────────────────────

func (h *Handler) handleText(c tele.Context) error {
	allowed, isDM := h.isAllowed(c)
	if !allowed {
		log.Printf("[TEXT] DENIED user=%d chat=%d isDM=%v text=%q",
			c.Sender().ID, c.Chat().ID, isDM, c.Text())
		// Non-admin DM → kirim reject + contact button (sekali, biar mereka tau gimana lanjut)
		if isDM {
			return c.Send(
				fmt.Sprintf(textNonAdminReject, "@"+h.cfg.ContactUsername),
				nonAdminRejectMenu(h.cfg.ContactUsername),
				tele.ModeMarkdown,
			)
		}
		return nil
	}
	// Di group: gak ada wizard, jadi semua text di-ignore (cuma slash command handler aktif)
	if !isDM {
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
		// Soft-nudge: kalau text kayak domain (ada dot), kasih hint sekali — rate-limited
		text := strings.TrimSpace(c.Text())
		if looksLikeDomain(text) && shouldSendHint(c.Sender().ID) {
			h.reply(c,
				"❓ _Hmm, aku gak nemu wizard aktif buat kamu._\n\n"+
					"Mau *tambah domain ke Monitor*? Klik *🏠 MENU → 📡 Monitor → ➕ Add Domain* dulu.\n"+
					"Mau *cek status domain*? Klik *🏠 MENU → 📡 Monitor → 🔍 Cek Domain*.\n\n"+
					"_(Setiap admin di group punya session terpisah — wizard harus dimulai dengan klik tombol.)_",
				tele.ModeMarkdown)
		}
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
	case StepMonitorCheckSrc:
		return nil // handled via callback button
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

	// Klikcepat Rotator
	case StepKlikcepatRotatorPickLink, StepKlikcepatRotatorPickPool:
		return nil // callback-only
	case StepKlikcepatRotatorAddLabel:
		return h.wizardKlikcepatRotatorAddLabel(c, sess)

	// Klikcepat Bulk Setup Rotator
	case StepKlikcepatRotBulkPick, StepKlikcepatRotBulkPool:
		return nil // callback-only

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
	case StepSettingsKlikcepatURL:
		return h.wizardSettingsKlikcepatURL(c, sess)
	case StepSettingsKlikcepatKey:
		return h.wizardSettingsKlikcepatKey(c, sess)

	// Klikcepat Add Link wizard
	case StepKlikcepatAddType, StepKlikcepatAddProject:
		return nil // callback-only steps
	case StepKlikcepatAddTitle:
		return h.wizardKlikcepatAddTitle(c, sess)
	case StepKlikcepatAddSlug:
		return h.wizardKlikcepatAddSlug(c, sess)
	case StepKlikcepatAddLocationURL:
		return h.wizardKlikcepatAddLocation(c, sess)

	// Klikcepat Edit Link wizard
	case StepKlikcepatEditPickField:
		return nil // callback-only
	case StepKlikcepatEditValue:
		return h.wizardKlikcepatEditValue(c, sess)

	case StepKlikcepatProjectAddName:
		return h.wizardKlikcepatProjectAddName(c, sess)
	case StepKlikcepatProjectAddColor:
		return h.wizardKlikcepatProjectAddColor(c, sess)
	case StepKlikcepatProjectEditName:
		return h.wizardKlikcepatProjectEditName(c, sess)
	}

	return nil
}

// looksLikeDomain — heuristic sederhana: text yang mirip domain biasanya
// punya dot di tengah, gak ada space, dan minimal 4 karakter. Dipakai untuk
// kasih soft-nudge ke user yang ketik domain tanpa wizard aktif.
func looksLikeDomain(text string) bool {
	if len(text) < 4 || len(text) > 100 {
		return false
	}
	if strings.ContainsAny(text, " \t\n") {
		return false
	}
	// Harus ada dot di tengah (gak di awal/akhir)
	dot := strings.Index(text, ".")
	if dot <= 0 || dot >= len(text)-1 {
		return false
	}
	return true
}

// shouldSendHint rate-limit hint per user — hindari spam reminder di group chat
// kalau ada admin yang nge-chat banyak teks normal yang kebetulan ada dot-nya.
func shouldSendHint(userID int64) bool {
	hintMu.Lock()
	defer hintMu.Unlock()
	now := time.Now()
	if last, ok := hintLastSent[userID]; ok && now.Sub(last) < hintCooldown {
		return false
	}
	hintLastSent[userID] = now
	return true
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
