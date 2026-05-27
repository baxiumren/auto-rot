package klikcepat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	mu      sync.RWMutex
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) SetCredentials(baseURL, apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.apiKey = apiKey
}

// defaultBaseURL — fallback kalau user cuma set API key tanpa Base URL.
// Most users pakai klikcepat.com, jadi auto-default ini hemat 1 setup step.
const defaultBaseURL = "https://klikcepat.com"

// HasCredentials true kalau apiKey ada. baseURL bisa default ke defaultBaseURL
// kalau user gak set manual.
func (c *Client) HasCredentials() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey != ""
}

// Ping verifies credentials by calling /api/user endpoint.
func (c *Client) Ping() error {
	_, err := c.do(http.MethodGet, "/api/user", nil)
	return err
}

// do is the low-level HTTP helper.
// body can be nil (GET/DELETE), url.Values (form POST), or []byte JSON, or struct (marshaled to JSON).
func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	c.mu.RLock()
	base, key := c.baseURL, c.apiKey
	c.mu.RUnlock()
	if key == "" {
		return nil, fmt.Errorf("klikcepat API key belum di-set")
	}
	// Auto-fallback ke default base URL kalau user gak set
	if base == "" {
		base = defaultBaseURL
	}
	var bodyReader io.Reader
	contentType := ""
	switch v := body.(type) {
	case nil:
		// no body
	case url.Values:
		bodyReader = strings.NewReader(v.Encode())
		contentType = "application/x-www-form-urlencoded"
	case []byte:
		bodyReader = bytes.NewReader(v)
		contentType = "application/json"
	default:
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(raw)
		contentType = "application/json"
	}

	req, err := http.NewRequest(method, base+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("klikcepat request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(data, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = errResp.Error
		}
		if msg == "" {
			msg = string(data)
		}
		return nil, fmt.Errorf("klikcepat HTTP %d: %s", resp.StatusCode, msg)
	}
	return data, nil
}

// ─── Links API ────────────────────────────────────────────────────────────────

// unmarshalLink parses a single-link {"data": {...}} response.
func unmarshalLink(data []byte) (*Link, error) {
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse link: %w", err)
	}
	return &resp.Data, nil
}

// ListLinks fetches user's links (up to a 1000 hard cap). Optional filter by type
// ("link", "biolink", etc.) Empty linkType returns all types.
// Note: hardcoded results_per_page=1000 — links beyond that count are silently
// truncated. Future enhancement: paginate.
func (c *Client) ListLinks(linkType string) ([]Link, error) {
	path := "/api/links?results_per_page=1000"
	if linkType != "" {
		path += "&type=" + url.QueryEscape(linkType)
	}
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse links: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) GetLink(id int) (*Link, error) {
	if id <= 0 {
		return nil, fmt.Errorf("klikcepat: invalid link id %d", id)
	}
	data, err := c.do(http.MethodGet, fmt.Sprintf("/api/links/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return unmarshalLink(data)
}

// CreateLink creates a new link.
// linkType: "link" (shortlink), "biolink" (biolink page), etc.
// slug: custom slug (empty = klikcepat auto-generate)
// projectID: 0 = no project, >0 = assign to project
func (c *Client) CreateLink(linkType, title, slug, locationURL string, projectID int) (*Link, error) {
	if linkType == "" {
		return nil, fmt.Errorf("klikcepat: linkType required")
	}
	if locationURL == "" {
		return nil, fmt.Errorf("klikcepat: locationURL required")
	}
	form := url.Values{}
	form.Set("type", linkType)
	form.Set("title", title)
	if slug != "" {
		form.Set("url", slug)
	}
	form.Set("location_url", locationURL)
	if projectID > 0 {
		form.Set("project_id", fmt.Sprintf("%d", projectID))
	}
	data, err := c.do(http.MethodPost, "/api/links", form)
	if err != nil {
		return nil, err
	}
	return unmarshalLink(data)
}

// UpdateLink updates fields of an existing link.
// Pass key-value pairs for fields to update (e.g., "title", "url", "location_url", "is_enabled").
// Use UpdateLinkLocation for swap-only operations (more efficient).
func (c *Client) UpdateLink(id int, fields map[string]string) (*Link, error) {
	if id <= 0 {
		return nil, fmt.Errorf("klikcepat: invalid link id %d", id)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("klikcepat: no fields to update")
	}
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	data, err := c.do(http.MethodPost, fmt.Sprintf("/api/links/%d", id), form)
	if err != nil {
		return nil, err
	}
	return unmarshalLink(data)
}

// UpdateLinkLocation is the primary swap method — updates location_url only.
// Lightweight: doesn't fetch full Link response, just verifies success.
func (c *Client) UpdateLinkLocation(id int, locationURL string) error {
	if id <= 0 {
		return fmt.Errorf("klikcepat: invalid link id %d", id)
	}
	form := url.Values{}
	form.Set("location_url", locationURL)
	_, err := c.do(http.MethodPost, fmt.Sprintf("/api/links/%d", id), form)
	return err
}

func (c *Client) DeleteLink(id int) error {
	if id <= 0 {
		return fmt.Errorf("klikcepat: invalid link id %d", id)
	}
	_, err := c.do(http.MethodDelete, fmt.Sprintf("/api/links/%d", id), nil)
	return err
}

// ─── Projects API ─────────────────────────────────────────────────────────────

// unmarshalProject parses a single-project {"data": {...}} response.
func unmarshalProject(data []byte) (*Project, error) {
	var resp struct {
		Data Project `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}
	return &resp.Data, nil
}

// ListProjects returns up to 1000 projects (hardcoded cap, silently truncated beyond).
func (c *Client) ListProjects() ([]Project, error) {
	data, err := c.do(http.MethodGet, "/api/projects?results_per_page=1000", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Project `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse projects: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) GetProject(id int) (*Project, error) {
	if id <= 0 {
		return nil, fmt.Errorf("klikcepat: invalid project id %d", id)
	}
	data, err := c.do(http.MethodGet, fmt.Sprintf("/api/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return unmarshalProject(data)
}

// CreateProject creates a new project. color defaults to "#000000" if empty.
func (c *Client) CreateProject(name, color string) (*Project, error) {
	if name == "" {
		return nil, fmt.Errorf("klikcepat: project name required")
	}
	if color == "" {
		color = "#000000"
	}
	form := url.Values{}
	form.Set("name", name)
	form.Set("color", color)
	data, err := c.do(http.MethodPost, "/api/projects", form)
	if err != nil {
		return nil, err
	}
	return unmarshalProject(data)
}

// UpdateProject updates name and/or color. Empty strings = field not updated.
func (c *Client) UpdateProject(id int, name, color string) (*Project, error) {
	if id <= 0 {
		return nil, fmt.Errorf("klikcepat: invalid project id %d", id)
	}
	if name == "" && color == "" {
		return nil, fmt.Errorf("klikcepat: no fields to update")
	}
	form := url.Values{}
	if name != "" {
		form.Set("name", name)
	}
	if color != "" {
		form.Set("color", color)
	}
	data, err := c.do(http.MethodPost, fmt.Sprintf("/api/projects/%d", id), form)
	if err != nil {
		return nil, err
	}
	return unmarshalProject(data)
}

func (c *Client) DeleteProject(id int) error {
	if id <= 0 {
		return fmt.Errorf("klikcepat: invalid project id %d", id)
	}
	_, err := c.do(http.MethodDelete, fmt.Sprintf("/api/projects/%d", id), nil)
	return err
}

// ─── Domains API ──────────────────────────────────────────────────────────────

// ListDomains returns user's custom domains. Used for full-URL display in lists.
// Hard cap 1000 (same pagination limit as other endpoints).
func (c *Client) ListDomains() ([]Domain, error) {
	data, err := c.do(http.MethodGet, "/api/domains?results_per_page=1000", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Domain `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse domains: %w", err)
	}
	return resp.Data, nil
}
