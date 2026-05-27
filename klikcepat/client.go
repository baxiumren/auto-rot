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

func (c *Client) HasCredentials() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL != "" && c.apiKey != ""
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
	if base == "" || key == "" {
		return nil, fmt.Errorf("klikcepat credentials belum di-set")
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

// ListLinks fetches all user's links. Optional filter by type ("link", "biolink", etc.)
// Empty linkType returns all types.
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
	data, err := c.do(http.MethodGet, fmt.Sprintf("/api/links/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// CreateLink creates a new link.
// linkType: "link" (shortlink), "biolink" (biolink page), etc.
// slug: custom slug (empty = klikcepat auto-generate)
// projectID: 0 = no project, >0 = assign to project
func (c *Client) CreateLink(linkType, title, slug, locationURL string, projectID int) (*Link, error) {
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
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateLink updates fields of an existing link.
// Pass key-value pairs for fields to update (e.g., "title", "url", "location_url", "is_enabled").
// Use UpdateLinkLocation for swap-only operations (more efficient).
func (c *Client) UpdateLink(id int, fields map[string]string) (*Link, error) {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	data, err := c.do(http.MethodPost, fmt.Sprintf("/api/links/%d", id), form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Link `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateLinkLocation is the primary swap method — updates location_url only.
// Lightweight: doesn't fetch full Link response, just verifies success.
func (c *Client) UpdateLinkLocation(id int, locationURL string) error {
	form := url.Values{}
	form.Set("location_url", locationURL)
	_, err := c.do(http.MethodPost, fmt.Sprintf("/api/links/%d", id), form)
	return err
}

func (c *Client) DeleteLink(id int) error {
	_, err := c.do(http.MethodDelete, fmt.Sprintf("/api/links/%d", id), nil)
	return err
}
