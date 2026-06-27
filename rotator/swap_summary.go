package rotator

import (
	"sync"
	"time"
)

// ─── Swap Summary Tracking ───────────────────────────────────────────────────
//
// SwapBatch collects all swap results during 1 BLOCKED domain cycle.
// Setelah semua trigger function done, summary message dikirim ke chat
// kalau ≥2 swap terjadi (skip notif kalau cuma 1 — udah ada notif individual).

// SwapPlatform identifies which rotator type produced the result.
type SwapPlatform string

const (
	PlatformCF             SwapPlatform = "cf"
	PlatformKlikcepatLink  SwapPlatform = "klc-shortlink"
	PlatformKlikcepatBlock SwapPlatform = "klc-block"
	PlatformLinkFB         SwapPlatform = "linkfb"
)

// Icon returns emoji for the platform.
func (p SwapPlatform) Icon() string {
	switch p {
	case PlatformCF:
		return "⚙️"
	case PlatformKlikcepatLink:
		return "🔗"
	case PlatformKlikcepatBlock:
		return "📄"
	case PlatformLinkFB:
		return "🔗"
	}
	return "•"
}

// SwapResult is single rotator swap attempt outcome.
type SwapResult struct {
	Platform   SwapPlatform
	Label      string // rotator label (display)
	NextDomain string // domain di pool yg dipilih (kalau success, sama dengan target swap)
	Success    bool
	ErrorMsg   string // populated if Success=false
}

// SwapBatch collects all results from one BLOCKED domain cycle.
type SwapBatch struct {
	mu sync.Mutex

	BlockedDomain string
	BlockedLabel  string
	Pool          string // first pool used (from first successful swap)
	NewDomain     string // first next domain (most common — biar header summary konsisten)
	StartedAt     time.Time
	Duration      time.Duration

	results []SwapResult
}

// NewSwapBatch returns a fresh batch for a blocked domain cycle.
func NewSwapBatch(blockedDomain, blockedLabel string) *SwapBatch {
	return &SwapBatch{
		BlockedDomain: blockedDomain,
		BlockedLabel:  blockedLabel,
		StartedAt:     time.Now(),
	}
}

// Add records a swap result (thread-safe).
func (b *SwapBatch) Add(r SwapResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.results = append(b.results, r)
	// Track first success metadata for header display
	if r.Success && b.NewDomain == "" {
		b.NewDomain = r.NextDomain
	}
}

// SetPool sets the pool name (from first matching rotator).
func (b *SwapBatch) SetPool(pool string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Pool == "" {
		b.Pool = pool
	}
}

// Finish marks batch complete + duration calculated.
func (b *SwapBatch) Finish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Duration == 0 {
		b.Duration = time.Since(b.StartedAt)
	}
}

// Results returns copy of all results (thread-safe).
func (b *SwapBatch) Results() []SwapResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]SwapResult, len(b.results))
	copy(out, b.results)
	return out
}

// Stats returns success/failed counts.
func (b *SwapBatch) Stats() (success, failed int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.results {
		if r.Success {
			success++
		} else {
			failed++
		}
	}
	return
}

// Total returns total number of swap attempts in batch.
func (b *SwapBatch) Total() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.results)
}

// SuccessRate returns percentage (0-100) of successful swaps.
func (b *SwapBatch) SuccessRate() int {
	success, failed := b.Stats()
	total := success + failed
	if total == 0 {
		return 0
	}
	return success * 100 / total
}

// ─── Summary Notifier Interface ──────────────────────────────────────────────

// SummaryNotifier is implemented by bot to send formatted summary message.
type SummaryNotifier interface {
	NotifySwapSummary(batch *SwapBatch)
}

// truncateErrMsg trims long error strings for inline display.
func truncateErrMsg(s string, max int) string {
	s = trimSpaceAndNewlines(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func trimSpaceAndNewlines(s string) string {
	// Collapse multi-line into single line
	out := ""
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			out += " "
		} else {
			out += string(r)
		}
	}
	// Trim leading/trailing
	for len(out) > 0 && (out[0] == ' ') {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1] == ' ') {
		out = out[:len(out)-1]
	}
	return out
}
