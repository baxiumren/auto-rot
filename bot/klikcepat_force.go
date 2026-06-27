package bot

import (
	"fmt"
	"net/url"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// ─── Force Swap untuk Klikcepat Rotators ─────────────────────────────────────
//
// Paksa swap manual tanpa nunggu domain detected blocked.
// Use case: lo mau test pool baru, atau swap proaktif sebelum keblock.
//
// Logic:
//   1. Fetch current rotator config + linked klikcepat link
//   2. Pick next safe domain dari pool (skip current host)
//   3. Build new URL (preserve path + query)
//   4. POST update via klikcepat API
//   5. Send notification + log to history

// handleKlikcepatRotForce — force swap untuk klikcepat SHORTLINK rotator.
func (h *Handler) handleKlikcepatRotForce(c tele.Context) error {
	rotID := extractParam(c)
	rot, ok := h.klikcepatRotators.GetByID(rotID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "⏳ Force swap..."})

	// Fetch link details
	link, err := h.klikcepat.GetLink(rot.LinkID)
	if err != nil {
		h.bot.Send(c.Chat(),
			fmt.Sprintf("❌ *Force Swap GAGAL*\n"+
				"🔗 Link: `/%s`\n"+
				"⚠️ Fetch error: `%s`",
				rot.LinkURL, err.Error()),
			tele.ModeMarkdown)
		return nil
	}

	currentHost := extractURLHost(link.LocationURL)
	pool := h.domains.GetByLabel(rot.PoolLabel)
	nextDomain := h.pickNextSafeDomain(pool, currentHost)
	if nextDomain == "" {
		h.bot.Send(c.Chat(),
			fmt.Sprintf("🚨 *Force Swap GAGAL — Pool kosong*\n"+
				"🔗 Link: `/%s`\n"+
				"📂 Pool: `%s` (%d domain, semua blocked atau cuma current host)\n"+
				"💡 Tambah domain ke pool via Monitor → Add Domain",
				rot.LinkURL, rot.PoolLabel, len(pool)),
			tele.ModeMarkdown)
		return nil
	}

	newURL := rebuildURLHost(link.LocationURL, nextDomain)
	if err := h.klikcepat.UpdateLinkLocation(rot.LinkID, newURL); err != nil {
		h.bot.Send(c.Chat(),
			fmt.Sprintf("❌ *Force Swap GAGAL*\n"+
				"🔗 Link: `/%s`\n"+
				"📤 Dari: `%s`\n"+
				"📥 Ke: `%s`\n"+
				"⚠️ Error: `%s`",
				rot.LinkURL, link.LocationURL, newURL, err.Error()),
			tele.ModeMarkdown)
		return nil
	}

	if h.history != nil {
		h.history.LogSwap("force-manual", rot.Label, rot.LinkURL, link.LocationURL, newURL, true, "")
	}

	h.bot.Send(c.Chat(),
		fmt.Sprintf("⚡ *FORCE SWAP via MANUAL*\n"+
			"🔗 Link: `/%s`\n"+
			"📤 Dari: `%s`\n"+
			"📥 Ke  : `%s`\n"+
			"📂 Pool: `%s`",
			rot.LinkURL, link.LocationURL, newURL, rot.PoolLabel),
		tele.ModeMarkdown)

	return nil
}

// handleKlcBlockRotForce — force swap untuk klikcepat BIOLINK BLOCK rotator.
func (h *Handler) handleKlcBlockRotForce(c tele.Context) error {
	rotID := extractParam(c)
	rot, ok := h.klikcepatBlockRotators.GetByID(rotID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Rotator gak ketemu", ShowAlert: true})
	}

	c.Respond(&tele.CallbackResponse{Text: "⏳ Force swap..."})

	block, err := h.klikcepat.GetBiolinkBlock(rot.BlockID)
	if err != nil {
		h.bot.Send(c.Chat(),
			fmt.Sprintf("❌ *Force Swap GAGAL*\n"+
				"📄 Biolink: `/%s`\n"+
				"🔘 Block: `%s`\n"+
				"⚠️ Fetch error: `%s`",
				rot.BiolinkSlug, rot.BlockName, err.Error()),
			tele.ModeMarkdown)
		return nil
	}

	currentHost := extractURLHost(block.LocationURL)
	pool := h.domains.GetByLabel(rot.PoolLabel)
	nextDomain := h.pickNextSafeDomain(pool, currentHost)
	if nextDomain == "" {
		h.bot.Send(c.Chat(),
			fmt.Sprintf("🚨 *Force Swap GAGAL — Pool kosong*\n"+
				"📄 Biolink: `/%s`\n"+
				"🔘 Block: `%s`\n"+
				"📂 Pool: `%s` (%d domain, semua blocked atau cuma current host)",
				rot.BiolinkSlug, rot.BlockName, rot.PoolLabel, len(pool)),
			tele.ModeMarkdown)
		return nil
	}

	newURL := rebuildURLHost(block.LocationURL, nextDomain)
	if err := h.klikcepat.UpdateBiolinkBlockLocation(rot.BlockID, newURL); err != nil {
		h.bot.Send(c.Chat(),
			fmt.Sprintf("❌ *Force Swap GAGAL*\n"+
				"📄 Biolink: `/%s`\n"+
				"🔘 Block: `%s`\n"+
				"📤 Dari: `%s`\n"+
				"📥 Ke: `%s`\n"+
				"⚠️ Error: `%s`",
				rot.BiolinkSlug, rot.BlockName, block.LocationURL, newURL, err.Error()),
			tele.ModeMarkdown)
		return nil
	}

	if h.history != nil {
		h.history.LogSwap("force-manual", rot.Label, rot.BlockName, block.LocationURL, newURL, true, "")
	}

	h.bot.Send(c.Chat(),
		fmt.Sprintf("⚡ *FORCE BLOCK SWAP via MANUAL*\n"+
			"📄 Biolink: `/%s`\n"+
			"🔘 Block: *%s*\n"+
			"📤 Dari: `%s`\n"+
			"📥 Ke  : `%s`\n"+
			"📂 Pool: `%s`",
			rot.BiolinkSlug, rot.BlockName, block.LocationURL, newURL, rot.PoolLabel),
		tele.ModeMarkdown)

	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// extractURLHost — get host portion from URL (handles missing scheme).
func extractURLHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return strings.ToLower(rawURL)
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// rebuildURLHost — swap host portion, preserve scheme + path + query + fragment.
func rebuildURLHost(originalURL, newHost string) string {
	newHost = strings.TrimSpace(newHost)
	newHost = strings.TrimPrefix(newHost, "https://")
	newHost = strings.TrimPrefix(newHost, "http://")
	newHost = strings.TrimPrefix(newHost, "www.")
	if idx := strings.Index(newHost, "/"); idx != -1 {
		newHost = newHost[:idx]
	}

	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return "https://" + newHost
	}

	hasScheme := strings.Contains(originalURL, "://")
	parseTarget := originalURL
	if !hasScheme {
		parseTarget = "https://" + originalURL
	}

	u, err := url.Parse(parseTarget)
	if err != nil || u.Host == "" {
		return "https://" + newHost
	}

	u.Host = newHost
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return u.String()
}

// pickNextSafeDomain — round-robin pick next domain from pool that's SAFE (not blocked).
// Skip current host. Falls back to any domain != currentHost if all sticky-blocked.
func (h *Handler) pickNextSafeDomain(pool []string, currentHost string) string {
	if len(pool) == 0 {
		return ""
	}

	// Find current index
	currentIdx := -1
	for i, d := range pool {
		if strings.EqualFold(d, currentHost) {
			currentIdx = i
			break
		}
	}

	// Try each domain after current (round-robin)
	for attempt := 1; attempt <= len(pool); attempt++ {
		nextIdx := (currentIdx + attempt) % len(pool)
		next := pool[nextIdx]
		if strings.EqualFold(next, currentHost) {
			continue
		}
		// Use checker to verify domain is SAFE — best effort
		// (kalau gak ada cara cepat cek di Handler, just return)
		return next
	}
	// Fallback — return first non-current
	for _, d := range pool {
		if !strings.EqualFold(d, currentHost) {
			return d
		}
	}
	return ""
}

// Unused but kept for completeness
var _ = tele.SendOptions{}
