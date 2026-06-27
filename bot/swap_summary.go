package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"bongbot/rotator"

	tele "gopkg.in/telebot.v3"
)

// ─── Swap Summary Notifier ────────────────────────────────────────────────────
//
// Called by monitor scanner after BLOCKED cycle finishes with ≥2 swap results.
// Renders unified summary message + retry button (kalau ada gagal).
//
// Auto-split kalau message > 4000 chars (Telegram limit 4096).

const maxTelegramMsgLen = 4000 // safe headroom below 4096 limit

// retryCache stores recent SwapBatch by ID buat retry button.
// TTL: 30 menit. Cleanup via background goroutine.
type retryCache struct {
	mu    sync.Mutex
	items map[string]*retryEntry
}

type retryEntry struct {
	domain    string
	label     string
	expiresAt time.Time
}

var globalRetryCache = &retryCache{items: make(map[string]*retryEntry)}

func init() {
	go globalRetryCache.cleanupLoop()
}

func (c *retryCache) put(id, domain, label string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[id] = &retryEntry{
		domain:    domain,
		label:     label,
		expiresAt: time.Now().Add(30 * time.Minute),
	}
}

func (c *retryCache) get(id string) (domain, label string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, exists := c.items[id]
	if !exists || time.Now().After(e.expiresAt) {
		return "", "", false
	}
	return e.domain, e.label, true
}

func (c *retryCache) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for id, e := range c.items {
			if now.After(e.expiresAt) {
				delete(c.items, id)
			}
		}
		c.mu.Unlock()
	}
}

// genRetryID makes a short unique ID for retry cache.
func genRetryID(domain string) string {
	return fmt.Sprintf("%s-%d", strings.ReplaceAll(domain, ".", "_"), time.Now().Unix())
}

// NotifySwapSummary implements rotator.SummaryNotifier — called from monitor scanner.
// Handler punya akses ke bot + cfg buat send ke allowed chat.
func (h *Handler) NotifySwapSummary(batch *rotator.SwapBatch) {
	// Build messages (split if needed)
	messages := renderSwapSummary(batch)
	if len(messages) == 0 {
		return
	}

	// Retry button only kalau ada gagal swap
	_, failed := batch.Stats()
	var lastMsgMarkup *tele.ReplyMarkup
	if failed > 0 {
		retryID := genRetryID(batch.BlockedDomain)
		globalRetryCache.put(retryID, batch.BlockedDomain, batch.BlockedLabel)
		m := &tele.ReplyMarkup{}
		m.Inline(m.Row(
			m.Data(fmt.Sprintf("🔄 Retry %d Failed", failed), cbSwapRetry, retryID),
		))
		lastMsgMarkup = m
	}

	// Send all parts. Markup hanya di message terakhir.
	chat := &tele.Chat{ID: h.cfg.AllowedChatID}
	for i, msg := range messages {
		opts := &tele.SendOptions{ParseMode: tele.ModeMarkdown}
		if i == len(messages)-1 && lastMsgMarkup != nil {
			opts.ReplyMarkup = lastMsgMarkup
		}
		if _, err := h.bot.Send(chat, msg, opts); err != nil {
			log.Printf("[SUMMARY] gagal kirim msg %d: %v", i+1, err)
		}
	}
}

// renderSwapSummary builds 1+ message strings. Splits if >maxTelegramMsgLen.
func renderSwapSummary(batch *rotator.SwapBatch) []string {
	results := batch.Results()
	total := len(results)
	success, failed := batch.Stats()
	successRate := 0
	if total > 0 {
		successRate = success * 100 / total
	}

	// Build success + fail sections
	var succLines []string
	var failLines []string
	for _, r := range results {
		if r.Success {
			succLines = append(succLines, fmt.Sprintf("└ %s %s → `%s`",
				r.Platform.Icon(), escapeMD(r.Label), r.NextDomain))
		} else {
			failLines = append(failLines, fmt.Sprintf("└ %s %s — %s",
				r.Platform.Icon(), escapeMD(r.Label), escapeMD(r.ErrorMsg)))
		}
	}

	// Header (always appears in first message)
	header := buildSummaryHeader(batch)

	// Stats footer (last message)
	footer := fmt.Sprintf("\n|\n"+
		"📊 *STATISTIK*\n"+
		"└ Total      : %d rotator\n"+
		"└ Success    : %d (%d%%)\n"+
		"└ Failed     : %d\n"+
		"└ Duration   : %.1fs",
		total, success, successRate, failed, batch.Duration.Seconds())

	// Try single message first
	single := header
	if len(succLines) > 0 {
		single += fmt.Sprintf("\n|\n✅ *BERHASIL SWAP* (%d)\n%s",
			success, strings.Join(succLines, "\n"))
	}
	if len(failLines) > 0 {
		single += fmt.Sprintf("\n|\n❌ *GAGAL SWAP* (%d)\n%s",
			failed, strings.Join(failLines, "\n"))
	}
	single += footer

	if len(single) <= maxTelegramMsgLen {
		return []string{single}
	}

	// Multi-message split — header in part 1, content paginated, footer in last
	return splitSwapSummary(batch, header, footer, succLines, failLines, success, failed)
}

func buildSummaryHeader(batch *rotator.SwapBatch) string {
	pool := batch.Pool
	if pool == "" {
		pool = "(N/A)"
	}
	newDom := batch.NewDomain
	if newDom == "" {
		newDom = "(N/A)"
	}
	return fmt.Sprintf(
		"💎 *S W A P   S U M M A R Y* 💎\n"+
			"🕐 %s\n"+
			"|\n"+
			"🚫 *BLOCKED DOMAIN*\n"+
			"└ Host  : `%s`\n"+
			"└ Label : `%s`\n"+
			"└ Pool  : `%s`\n"+
			"└ New   : `%s`",
		time.Now().Format("02/01 15:04:05"),
		batch.BlockedDomain, batch.BlockedLabel, pool, newDom,
	)
}

func splitSwapSummary(batch *rotator.SwapBatch, header, footer string, succLines, failLines []string, success, failed int) []string {
	var parts []string
	current := header

	// Helper to flush current part when section header + lines added
	flushAndStart := func(newSection string) {
		if current != "" {
			parts = append(parts, current)
		}
		current = newSection
	}

	// Success section
	if len(succLines) > 0 {
		sectionHeader := fmt.Sprintf("\n|\n✅ *BERHASIL SWAP* (%d)", success)
		// Try to append section header to current
		if len(current)+len(sectionHeader) > maxTelegramMsgLen {
			flushAndStart(sectionHeader[3:]) // strip leading "\n|\n" for new part
		} else {
			current += sectionHeader
		}
		for _, line := range succLines {
			withLine := current + "\n" + line
			if len(withLine) > maxTelegramMsgLen {
				flushAndStart(line)
			} else {
				current = withLine
			}
		}
	}

	// Failed section
	if len(failLines) > 0 {
		sectionHeader := fmt.Sprintf("\n|\n❌ *GAGAL SWAP* (%d)", failed)
		if len(current)+len(sectionHeader) > maxTelegramMsgLen {
			flushAndStart(sectionHeader[3:])
		} else {
			current += sectionHeader
		}
		for _, line := range failLines {
			withLine := current + "\n" + line
			if len(withLine) > maxTelegramMsgLen {
				flushAndStart(line)
			} else {
				current = withLine
			}
		}
	}

	// Footer — try to append to current, else new part
	if len(current)+len(footer) > maxTelegramMsgLen {
		flushAndStart(strings.TrimPrefix(footer, "\n|\n"))
	} else {
		current += footer
	}

	// Push final part
	if current != "" {
		parts = append(parts, current)
	}

	// Add "(N/M)" prefix to each part header (untuk multi-message tracking)
	totalParts := len(parts)
	if totalParts > 1 {
		for i := range parts {
			prefix := fmt.Sprintf("💎 *S W A P   S U M M A R Y  (%d/%d)* 💎\n", i+1, totalParts)
			// Replace original diamond header in first part
			if i == 0 {
				parts[i] = strings.Replace(parts[i], "💎 *S W A P   S U M M A R Y* 💎\n", prefix, 1)
			} else {
				parts[i] = prefix + "|\n" + parts[i]
			}
		}
	}

	return parts
}

// ─── Retry Button Handler ────────────────────────────────────────────────────

// handleSwapRetry — user clicked Retry button → re-trigger swap for the blocked domain.
func (h *Handler) handleSwapRetry(c tele.Context) error {
	retryID := extractParam(c)
	if retryID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "⚠️ Invalid retry ID", ShowAlert: true})
	}
	domain, label, ok := globalRetryCache.get(retryID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{
			Text:      "⚠️ Retry session expired (>30 menit)",
			ShowAlert: true,
		})
	}

	c.Respond(&tele.CallbackResponse{Text: "⏳ Retrying..."})

	// Send progress message
	progressMsg, _ := h.bot.Send(c.Chat(),
		fmt.Sprintf("⏳ Retrying failed swap untuk `%s`...", domain),
		tele.ModeMarkdown)

	// Trigger retry via monitor scanner (returns new batch)
	batch := h.monScanner.TriggerSwapForDomain(domain, label)

	// Delete progress message
	if progressMsg != nil {
		_ = h.bot.Delete(progressMsg)
	}

	// Send new summary if anything happened
	success, failed := batch.Stats()
	if success == 0 && failed == 0 {
		// Nothing changed — semua udah swapped sebelumnya
		h.bot.Send(c.Chat(),
			fmt.Sprintf("💎 *R E T R Y   R E S U L T* 💎\n"+
				"|\n"+
				"ℹ️ Gak ada swap yang perlu di-retry\n"+
				"└ Domain `%s` udah ter-swap semua di cycle sebelumnya",
				domain),
			tele.ModeMarkdown)
		return nil
	}

	// Send full summary with retry button if still failed
	h.NotifySwapSummary(batch)
	return nil
}
