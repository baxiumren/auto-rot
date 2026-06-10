package bot

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"bongbot/checker"
	"bongbot/store"
	tele "gopkg.in/telebot.v3"
)

// ─── Group Read-only Handlers ────────────────────────────────────────────────
//
// Di group, bot cuma kasih view singkat — gak ada wizard, gak ada action button
// kecuali "🗑 Hapus" yang nempel di alert blocked.
//
// Setup, add/remove, settings, dll → DM bot (deep-link "🤖 Setup di DM →").

var botStartTime = time.Now()

func (h *Handler) handleGroupStatus(c tele.Context) error {
	totalDomains := h.domains.TotalCount()
	cfRules := h.cfrules.GetAll()
	rotators := h.rotators.GetAll()
	blocked := h.monScanner.GetBlockedSnapshot()
	chunkNum, chunkOf, _, chunkSize := h.monScanner.GetChunkInfo()
	interval := h.monScanner.GetInterval()
	uptime := time.Since(botStartTime)

	activeRotators := 0
	for _, r := range rotators {
		if r.Active {
			activeRotators++
		}
	}

	mode := "🟢 Full Scan"
	cycleInfo := ""
	if chunkOf > 1 {
		mode = "🔄 Rotating Batch"
		fullCycle := time.Duration(chunkOf) * interval
		cycleInfo = fmt.Sprintf("\n└ Chunk            : %d/%d (max %d/chunk)\n└ Siklus penuh     : %.1f menit",
			chunkNum, chunkOf, chunkSize, fullCycle.Minutes())
	}

	stickyCount := len(checker.Default().GetStickyList())

	text := fmt.Sprintf(
		"💎 *S T A T U S   B O T* 💎\n"+
			"|\n"+
			"📡 *MONITOR*\n"+
			"└ Domain terdaftar : %d\n"+
			"└ Sedang blocked   : %d\n"+
			"└ Sticky cache     : %d\n"+
			"└ Interval tick    : %v\n"+
			"└ Mode scan        : %s%s\n"+
			"|\n"+
			"⚙️ *CLOUDFLARE*\n"+
			"└ CF Rule : %d\n"+
			"|\n"+
			"🔄 *ROTATOR*\n"+
			"└ Total config : %d\n"+
			"└ ▶️ Aktif      : %d\n"+
			"|\n"+
			"⏱ *BOT UPTIME*\n"+
			"└ %s\n"+
			"|\n"+
			"🕐 Update : %s",
		totalDomains, len(blocked), stickyCount, interval, mode, cycleInfo,
		len(cfRules),
		len(rotators), activeRotators,
		formatUptime(uptime),
		time.Now().Format("02/01/2006 15:04:05"),
	)

	return c.Edit(text, groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
}

func (h *Handler) handleGroupListDomain(c tele.Context) error {
	all := h.domains.GetAll()
	if len(all) == 0 {
		return c.Edit(
			"📋 *List Domain*\n\n_Belum ada domain di Monitor._\n\nSetup via DM bot 👇",
			groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
	}

	// Sort labels for consistent order
	labels := make([]string, 0, len(all))
	for lbl := range all {
		labels = append(labels, lbl)
	}
	sort.Strings(labels)

	totalDom := 0
	for _, lbl := range labels {
		totalDom += len(all[lbl])
	}

	// Build full list dulu (detail per domain — `backtick` biar copy-friendly)
	var full strings.Builder
	full.WriteString("💎 *L I S T   D O M A I N* 💎\n")
	full.WriteString("|\n")
	for _, lbl := range labels {
		domains := append([]string{}, all[lbl]...)
		sort.Strings(domains)
		full.WriteString(fmt.Sprintf("📂 *%s* — %d domain\n", escapeMD(lbl), len(domains)))
		for _, d := range domains {
			full.WriteString(fmt.Sprintf("└ `%s`\n", d))
		}
		full.WriteString("|\n")
	}
	full.WriteString(fmt.Sprintf("📊 *TOTAL*\n└ %d domain dalam %d label",
		totalDom, len(labels)))

	text := full.String()

	// Telegram limit 4096 chars per message. Kalau over → fallback ke ringkasan
	const tgMaxLen = 3900
	if len(text) > tgMaxLen {
		var summary strings.Builder
		summary.WriteString("💎 *L I S T   D O M A I N* 💎  `(ringkasan)`\n")
		summary.WriteString("|\n")
		summary.WriteString(fmt.Sprintf("⚠️ Total %d domain — terlalu panjang.\n   Detail lengkap → buka DM bot.\n", totalDom))
		summary.WriteString("|\n")
		summary.WriteString("📊 *PER KATEGORI*\n")
		for _, lbl := range labels {
			summary.WriteString(fmt.Sprintf("└ 📂 %s : %d domain\n", escapeMD(lbl), len(all[lbl])))
		}
		summary.WriteString("|\n")
		summary.WriteString(fmt.Sprintf("📊 *TOTAL*\n└ %d domain dalam %d label",
			totalDom, len(labels)))
		text = summary.String()
	}

	return c.Edit(text, groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
}

func (h *Handler) handleGroupListCF(c tele.Context) error {
	rules := h.cfrules.GetAll()
	if len(rules) == 0 {
		return c.Edit(
			"🔄 *List CF Redirect*\n\n_Belum ada CF Rule terdaftar._\n\nSetup via DM bot 👇",
			groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].Label < rules[j].Label })

	// Cross-reference dengan rotator config
	rotByRule := make(map[string]string)
	for _, rot := range h.rotators.GetAll() {
		if rot.Active {
			rotByRule[rot.CFRuleID] = rot.PoolLabel
		}
	}

	// Build full list dulu — include current URL kalau credentials ada
	hasCreds := h.cf.HasCredentials()
	currentURLs := make(map[string]string)
	if hasCreds {
		// Fetch paralel biar gak lama
		type result struct {
			id  string
			url string
		}
		results := make(chan result, len(rules))
		for _, r := range rules {
			go func(r store.CFRule) {
				url, err := h.cf.GetCurrentURL(r)
				if err == nil {
					results <- result{r.ID, url}
				} else {
					results <- result{r.ID, ""}
				}
			}(r)
		}
		for range rules {
			res := <-results
			currentURLs[res.id] = res.url
		}
	}

	var full strings.Builder
	full.WriteString("💎 *L I S T   C F   R U L E S* 💎\n")
	full.WriteString("|\n")
	for _, r := range rules {
		dom := r.Domain
		if dom == "" {
			dom = "(no domain)"
		}
		typeShort := "v2"
		if r.Type == "page_rules" {
			typeShort = "v1"
		}
		full.WriteString(fmt.Sprintf("⚙️ *%s* (%s)\n", escapeMD(r.Label), typeShort))
		full.WriteString(fmt.Sprintf("└ 🌐 Domain : `%s`\n", dom))
		if curURL := currentURLs[r.ID]; curURL != "" {
			full.WriteString(fmt.Sprintf("└ 🎯 Target : `%s`\n", curURL))
		}
		if pool, ok := rotByRule[r.ID]; ok {
			full.WriteString(fmt.Sprintf("└ 🔄 Pool   : `%s`\n", escapeMD(pool)))
		} else {
			full.WriteString("└ ⚪ Pool   : (no auto-swap)\n")
		}
		full.WriteString("|\n")
	}
	full.WriteString(fmt.Sprintf("📊 *TOTAL*\n└ %d CF Rule", len(rules)))

	text := full.String()
	const tgMaxLen = 3900
	if len(text) > tgMaxLen {
		var summary strings.Builder
		summary.WriteString("💎 *L I S T   C F   R U L E S* 💎  `(ringkasan)`\n")
		summary.WriteString("|\n")
		summary.WriteString("⚠️ Detail target URL kepanjangan.\n   Buka DM bot untuk full info.\n")
		summary.WriteString("|\n")
		summary.WriteString("⚙️ *RULES*\n")
		for _, r := range rules {
			dom := r.Domain
			if dom == "" {
				dom = "(no domain)"
			}
			typeShort := "v2"
			if r.Type == "page_rules" {
				typeShort = "v1"
			}
			rotInfo := ""
			if pool, ok := rotByRule[r.ID]; ok {
				rotInfo = fmt.Sprintf(" → 🔄 %s", escapeMD(pool))
			}
			summary.WriteString(fmt.Sprintf("└ *%s* (%s) — `%s`%s\n",
				escapeMD(r.Label), typeShort, dom, rotInfo))
		}
		summary.WriteString("|\n")
		summary.WriteString(fmt.Sprintf("📊 *TOTAL*\n└ %d CF Rule", len(rules)))
		text = summary.String()
	}

	return c.Edit(text, groupMenu(h.cfg.BotUsername), tele.ModeMarkdown)
}

// handleAlertRemove — admin klik tombol 🗑 Hapus dari Monitor di alert blocked
func (h *Handler) handleAlertRemove(c tele.Context, domain string) error {
	if !h.requireAdmin(c) {
		return nil
	}
	if domain == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Domain kosong"})
	}

	label, found := h.domains.Remove(domain)
	if !found {
		// Mungkin udah dihapus orang lain
		c.Respond(&tele.CallbackResponse{Text: "ℹ️ Domain udah gak ada di Monitor"})
		// Tetep bersihin sticky biar konsisten
		checker.Default().RemoveSticky(domain)
		checker.Default().RemoveForceBlock(domain)
		return nil
	}

	// Cleanup sticky + force-block
	checker.Default().RemoveSticky(domain)
	checker.Default().RemoveForceBlock(domain)

	c.Respond(&tele.CallbackResponse{
		Text:      fmt.Sprintf("✅ %s dihapus dari Monitor", domain),
		ShowAlert: false,
	})

	// Edit alert message: tambah footer "✅ HANDLED"
	currentText := c.Message().Text
	if currentText == "" {
		// Cuma caption? gak update
		return nil
	}
	newText := currentText + fmt.Sprintf(
		"\n\n━━━━━━━━━━━━━━━━━━\n✅ *DIHAPUS* oleh %s pada %s\n📂 Kategori: `%s`",
		userTag(c.Sender()), time.Now().Format("02/01 15:04:05"), escapeMD(label))
	h.bot.Edit(c.Message(), newText, tele.ModeMarkdown)
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d detik", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d menit", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%d jam %d menit", h, m)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%d hari %d jam", days, hours)
}
