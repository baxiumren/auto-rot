package klikcepat

// Link represents a klikcepat link object (biolink, shortlink, vcard, etc).
type Link struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	ProjectID   int    `json:"project_id"`
	DomainID    int    `json:"domain_id"`
	Type        string `json:"type"`         // biolink, link, file, vcard, event, static
	Title       string `json:"title"`
	URL         string `json:"url"`          // slug (klikcepat.com/SLUG)
	LocationURL string `json:"location_url"` // target redirect — what we swap
	IsEnabled   int    `json:"is_enabled"`
	Datetime    string `json:"datetime"`
}

// Project represents a klikcepat project (link grouping).
type Project struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
