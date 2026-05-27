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
