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
	cfg      *config.Config
	bot      *tele.Bot
	sessions *sessionStore
	domains  *store.DomainStore
	cfrules  *store.CFRuleStore
	rotators *store.RotatorStore
	cf       *cloudflare.Client
	rotSvc   *rotator.Service
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
	cf *cloudflare.Client,
	rotSvc *rotator.Service,
) *Handler {
	return &Handler{
		cfg:      cfg,
		bot:      b,
		sessions: newSessionStore(),
		domains:  domains,
		cfrules:  cfrules,
		rotators: rotators,
		cf:       cf,
		rotSvc:   rotSvc,
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
	return c.Send(
		"🤖 *BongBot*\n\nBot gabungan: Monitor nawala + CF Redirect + Auto Rotator\n\nPilih menu:",
		mainMenu(), tele.ModeMarkdown,
	)
}

func (h *Handler) handleMenu(c tele.Context) error {
	return c.Send("🏠 *Menu Utama*\n\nPilih section:", mainMenu(), tele.ModeMarkdown)
}

// ─── Callback Router ──────────────────────────────────────────────────────────

func (h *Handler) handleCallback(c tele.Context) error {
	if !h.isAllowed(c) {
		return c.Respond(&tele.CallbackResponse{Text: "⛔ Akses ditolak"})
	}

	c.Respond()
	data := c.Data()

	// Strip "\f" prefix yang ditambah telebot untuk InlineButton dengan Unique
	data = strings.TrimPrefix(data, "\f")

	// Split data: format adalah "unique data" atau "unique" saja
	parts := strings.SplitN(data, " ", 2)
	unique := parts[0]
	param := ""
	if len(parts) > 1 {
		param = parts[1]
	}
	_ = param // beberapa handler pakai c.Data() langsung

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

	// CF Redirect
	case cbCF:
		return h.handleCF(c)
	case cbCFAdd:
		if param != "" {
			return h.handleCFAddTypeSelect(c)
		}
		return h.handleCFAdd(c)
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
	}

	return nil
}

// ─── Text Router (Wizard Steps) ───────────────────────────────────────────────

func (h *Handler) handleText(c tele.Context) error {
	if !h.isAllowed(c) {
		return nil
	}

	sess, ok := h.sessions.Get(c.Sender().ID)
	if !ok {
		return nil
	}

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

	// CF Redirect
	case StepCFAddLabel:
		return h.wizardCFAddLabel(c, sess)
	case StepCFAddZone:
		return h.wizardCFAddZone(c, sess)
	case StepCFAddType:
		return nil // ini handled via callback button
	case StepCFAddRuleset:
		return h.wizardCFAddRuleset(c, sess)
	case StepCFAddRuleID:
		return h.wizardCFAddRuleID(c, sess)
	case StepCFChangeURL:
		return h.wizardCFChangeURL(c, sess)

	// Auto Rotator
	case StepRotatorAddLabel:
		return h.wizardRotatorAddLabel(c, sess)
	}

	return nil
}

// ─── c.Data() override untuk param extraction ─────────────────────────────────
// telebot.v3 menyimpan callback data sebagai "\f{unique} {param}"
// c.Data() sudah strip prefix secara otomatis dan mengembalikan {param}
// Tapi karena kita handle manual via OnCallback, kita butuh helper ini.
func extractParam(c tele.Context) string {
	data := c.Data()
	data = strings.TrimPrefix(data, "\f")
	parts := strings.SplitN(data, " ", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
