package klikcepat

import (
	"encoding/json"
	"strconv"
)

// FlexInt is an int that accepts both JSON number ("123" → 123) and JSON int (123).
// Klikcepat PHP backend sometimes returns int fields as strings due to
// PHP MySQL fetch_object() default behavior.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	raw := string(b)
	// Handle boolean (klikcepat API balikin is_enabled sebagai true/false)
	if raw == "true" {
		*f = 1
		return nil
	}
	if raw == "false" || raw == "null" {
		*f = 0
		return nil
	}
	// Try as string (e.g., "123")
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*f = FlexInt(n)
		return nil
	}
	// Otherwise try as raw number
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

// MarshalJSON ensures FlexInt serializes as a plain JSON number.
func (f FlexInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(f))
}

// Int returns the underlying int value (convenience for callers).
func (f FlexInt) Int() int { return int(f) }

// Link represents a klikcepat link object (biolink, shortlink, vcard, etc).
// All numeric fields use FlexInt to handle PHP backend that may return strings.
type Link struct {
	ID          FlexInt `json:"id"`
	UserID      FlexInt `json:"user_id"`
	ProjectID   FlexInt `json:"project_id"`
	DomainID    FlexInt `json:"domain_id"`
	Type        string  `json:"type"`         // biolink, link, file, vcard, event, static
	Title       string  `json:"title"`
	URL         string  `json:"url"`          // slug (klikcepat.com/SLUG)
	LocationURL string  `json:"location_url"` // target redirect — what we swap
	IsEnabled   FlexInt `json:"is_enabled"`
	Datetime    string  `json:"datetime"`
}

// Project represents a klikcepat project (link grouping).
type Project struct {
	ID    FlexInt `json:"id"`
	Name  string  `json:"name"`
	Color string  `json:"color"`
}

// Domain represents a klikcepat custom domain (e.g., klikcepat.vip, links.maha.com).
// Each link can be assigned a domain_id; if 0, uses platform default.
type Domain struct {
	ID        FlexInt `json:"id"`
	UserID    FlexInt `json:"user_id"`
	Scheme    string  `json:"scheme"` // "http" or "https"
	Host      string  `json:"host"`   // "klikcepat.vip"
	IsEnabled FlexInt `json:"is_enabled"`
}

// BiolinkBlock represents one block (button/heading/avatar/dll) di dalam biolink.
// Cuma block type "link" yg punya location_url buat di-swap.
// Settings di-decode best-effort (struktur bervariasi tergantung type).
type BiolinkBlock struct {
	ID           FlexInt         `json:"id"`
	UserID       FlexInt         `json:"user_id"`
	LinkID       FlexInt         `json:"link_id"`        // biolink parent ID
	Type         string          `json:"type"`           // link, heading, paragraph, avatar, dll
	LocationURL  string          `json:"location_url"`
	Settings     json.RawMessage `json:"settings"`       // raw — extract Name kalau perlu
	Order        FlexInt         `json:"order"`
	Clicks       FlexInt         `json:"clicks"`
	IsEnabled    FlexInt         `json:"is_enabled"`
	Datetime     string          `json:"datetime"`
	LastDatetime string          `json:"last_datetime"`
}

// BlockName extract field "name" dari settings (untuk display label tombol).
// Return empty string kalau gak ada / parse error.
func (b *BiolinkBlock) BlockName() string {
	if len(b.Settings) == 0 {
		return ""
	}
	var s struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b.Settings, &s); err != nil {
		return ""
	}
	return s.Name
}
