package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"bongbot/store"
)

// DiscoveredRule = hasil auto-fetch redirect rule (v2) dari Cloudflare.
type DiscoveredRule struct {
	RulesetID string
	RuleID    string
	TargetURL string
}

// DiscoveredPageRule = hasil auto-fetch page rule (v1).
type DiscoveredPageRule struct {
	RuleID    string
	Pattern   string
	TargetURL string
}

// ZoneInfo = hasil registrasi domain baru ke Cloudflare.
type ZoneInfo struct {
	ZoneID      string
	NameServers []string
	Status      string // "pending" sampai NS aktif
}

const baseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
	mu     sync.RWMutex
	email  string
	apiKey string
	http   *http.Client
}

func New(email, apiKey string) *Client {
	return &Client{
		email:  email,
		apiKey: apiKey,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// SetCredentials updates the CF email & API key at runtime (thread-safe).
func (c *Client) SetCredentials(email, apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.email = email
	c.apiKey = apiKey
}

// Credentials returns the currently active CF email & API key.
func (c *Client) Credentials() (email, apiKey string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.email, c.apiKey
}

// HasCredentials returns true kalau email & API key sudah di-set.
func (c *Client) HasCredentials() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.email != "" && c.apiKey != "" &&
		c.email != "email@example.com" && c.apiKey != "global_api_key_cloudflare"
}

// Ping memverifikasi credential valid dengan hit endpoint /user.
func (c *Client) Ping() error {
	if !c.HasCredentials() {
		return fmt.Errorf("CF credentials belum di-set")
	}
	_, err := c.do(http.MethodGet, baseURL+"/user", nil)
	return err
}

func (c *Client) do(method, url string, body any) ([]byte, error) {
	c.mu.RLock()
	email := c.email
	apiKey := c.apiKey
	c.mu.RUnlock()

	if email == "" || apiKey == "" {
		return nil, fmt.Errorf("CF credentials belum di-set — pakai menu ⚙️ Settings di bot")
	}

	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Email", email)
	req.Header.Set("X-Auth-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var cfErr struct {
			Errors []struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"errors"`
		}
		if json.Unmarshal(data, &cfErr) == nil && len(cfErr.Errors) > 0 {
			return nil, fmt.Errorf("CF error %d: %s", cfErr.Errors[0].Code, cfErr.Errors[0].Message)
		}
		return nil, fmt.Errorf("CF API error: status %d", resp.StatusCode)
	}
	return data, nil
}

// ─── V2: Redirect Rules (Rulesets API, recommended by Cloudflare) ────────────

func (c *Client) GetRedirectRuleURL(rule store.CFRule) (string, error) {
	url := fmt.Sprintf("%s/zones/%s/rulesets/%s", baseURL, rule.ZoneID, rule.RulesetID)
	data, err := c.do(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Result struct {
			Rules []struct {
				ID               string `json:"id"`
				ActionParameters struct {
					FromValue struct {
						TargetURL struct {
							Value string `json:"value"`
						} `json:"target_url"`
					} `json:"from_value"`
				} `json:"action_parameters"`
			} `json:"rules"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	for _, r := range result.Result.Rules {
		if r.ID == rule.RuleID {
			return r.ActionParameters.FromValue.TargetURL.Value, nil
		}
	}
	return "", fmt.Errorf("rule %s tidak ditemukan", rule.RuleID)
}

func (c *Client) UpdateRedirectRuleURL(rule store.CFRule, newURL string) error {
	body := map[string]interface{}{
		"action": "redirect",
		"action_parameters": map[string]interface{}{
			"from_value": map[string]interface{}{
				"target_url":            map[string]interface{}{"value": newURL},
				"status_code":           301,
				"preserve_query_string": false,
			},
		},
		"expression": "true",
		"enabled":    true,
	}
	url := fmt.Sprintf("%s/zones/%s/rulesets/%s/rules/%s", baseURL, rule.ZoneID, rule.RulesetID, rule.RuleID)
	_, err := c.do(http.MethodPatch, url, body)
	return err
}

// ─── V1: Page Rules (legacy, masih support) ──────────────────────────────────

func (c *Client) GetPageRuleURL(rule store.CFRule) (string, error) {
	url := fmt.Sprintf("%s/zones/%s/pagerules/%s", baseURL, rule.ZoneID, rule.RuleID)
	data, err := c.do(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Result struct {
			Actions []struct {
				ID    string `json:"id"`
				Value struct {
					URL string `json:"url"`
				} `json:"value"`
			} `json:"actions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	for _, a := range result.Result.Actions {
		if a.ID == "forwarding_url" {
			return a.Value.URL, nil
		}
	}
	return "", fmt.Errorf("forwarding_url tidak ditemukan di page rule %s", rule.RuleID)
}

func (c *Client) UpdatePageRuleURL(rule store.CFRule, newURL string) error {
	body := map[string]interface{}{
		"actions": []map[string]interface{}{
			{
				"id": "forwarding_url",
				"value": map[string]interface{}{
					"url":         newURL,
					"status_code": 301,
				},
			},
		},
	}
	url := fmt.Sprintf("%s/zones/%s/pagerules/%s", baseURL, rule.ZoneID, rule.RuleID)
	_, err := c.do(http.MethodPatch, url, body)
	return err
}

// ─── Discovery: auto-fetch Zone & Rules ───────────────────────────────────────

// GetZoneID cari Zone ID berdasarkan nama domain di akun CF.
// Otomatis strip subdomain (sub.domain.com → domain.com) karena CF zone selalu di root.
func (c *Client) GetZoneID(domainName string) (string, error) {
	root := strings.ToLower(strings.TrimSpace(domainName))
	parts := strings.Split(root, ".")
	if len(parts) > 2 {
		root = strings.Join(parts[len(parts)-2:], ".")
	}

	url := fmt.Sprintf("%s/zones?name=%s", baseURL, root)
	data, err := c.do(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("gagal cari zone: %w", err)
	}
	var resp struct {
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Result) == 0 {
		return "", fmt.Errorf("zone tidak ditemukan untuk domain: %s", domainName)
	}
	return resp.Result[0].ID, nil
}

// ListRedirectRules fetch semua redirect rules (v2) di zone.
func (c *Client) ListRedirectRules(zoneID string) ([]DiscoveredRule, error) {
	// 1. List rulesets, filter phase "http_request_dynamic_redirect"
	url := fmt.Sprintf("%s/zones/%s/rulesets", baseURL, zoneID)
	data, err := c.do(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal ambil rulesets: %w", err)
	}
	var listResp struct {
		Result []struct {
			ID    string `json:"id"`
			Phase string `json:"phase"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &listResp); err != nil {
		return nil, err
	}

	var results []DiscoveredRule
	for _, rs := range listResp.Result {
		if rs.Phase != "http_request_dynamic_redirect" {
			continue
		}
		// 2. Fetch rules dalam ruleset
		rsURL := fmt.Sprintf("%s/zones/%s/rulesets/%s", baseURL, zoneID, rs.ID)
		rsData, err := c.do(http.MethodGet, rsURL, nil)
		if err != nil {
			continue
		}
		var rsResp struct {
			Result struct {
				Rules []struct {
					ID               string `json:"id"`
					ActionParameters struct {
						FromValue struct {
							TargetURL struct {
								Value string `json:"value"`
							} `json:"target_url"`
						} `json:"from_value"`
					} `json:"action_parameters"`
				} `json:"rules"`
			} `json:"result"`
		}
		if err := json.Unmarshal(rsData, &rsResp); err != nil {
			continue
		}
		for _, rule := range rsResp.Result.Rules {
			results = append(results, DiscoveredRule{
				RulesetID: rs.ID,
				RuleID:    rule.ID,
				TargetURL: rule.ActionParameters.FromValue.TargetURL.Value,
			})
		}
	}
	return results, nil
}

// ListPageRules fetch semua page rules (v1) yang aktif di zone.
func (c *Client) ListPageRules(zoneID string) ([]DiscoveredPageRule, error) {
	url := fmt.Sprintf("%s/zones/%s/pagerules?status=active", baseURL, zoneID)
	data, err := c.do(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal ambil page rules: %w", err)
	}
	var resp struct {
		Result []struct {
			ID      string `json:"id"`
			Targets []struct {
				Constraint struct {
					Value string `json:"value"`
				} `json:"constraint"`
			} `json:"targets"`
			Actions []struct {
				ID    string `json:"id"`
				Value struct {
					URL string `json:"url"`
				} `json:"value"`
			} `json:"actions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var results []DiscoveredPageRule
	for _, pr := range resp.Result {
		pattern := ""
		if len(pr.Targets) > 0 {
			pattern = pr.Targets[0].Constraint.Value
		}
		targetURL := ""
		for _, a := range pr.Actions {
			if a.ID == "forwarding_url" {
				targetURL = a.Value.URL
				break
			}
		}
		results = append(results, DiscoveredPageRule{
			RuleID:    pr.ID,
			Pattern:   pattern,
			TargetURL: targetURL,
		})
	}
	return results, nil
}

// ─── New Domain Registration (AddZone + DNS + Redirect Rule) ─────────────────

// AddZone mendaftarkan domain baru ke akun Cloudflare.
// Return: ZoneID + Nameservers yang harus di-set user di registrar.
func (c *Client) AddZone(domainName string) (ZoneInfo, error) {
	body := map[string]interface{}{
		"name":       strings.ToLower(strings.TrimSpace(domainName)),
		"jump_start": false,
	}
	data, err := c.do(http.MethodPost, baseURL+"/zones", body)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("gagal daftarkan domain: %w", err)
	}
	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			ID          string   `json:"id"`
			NameServers []string `json:"name_servers"`
			Status      string   `json:"status"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return ZoneInfo{}, err
	}
	return ZoneInfo{
		ZoneID:      resp.Result.ID,
		NameServers: resp.Result.NameServers,
		Status:      resp.Result.Status,
	}, nil
}

// DeleteZone hapus zone dari Cloudflare (untuk cleanup kalau cancel).
func (c *Client) DeleteZone(zoneID string) error {
	_, err := c.do(http.MethodDelete, fmt.Sprintf("%s/zones/%s", baseURL, zoneID), nil)
	return err
}

// CreateDNSRecord bikin DNS record di zone.
// recordType: "A", "AAAA", "CNAME". proxied: orange cloud on/off.
func (c *Client) CreateDNSRecord(zoneID, recordType, name, content string, proxied bool) error {
	ttl := 1 // auto (wajib untuk proxied)
	if !proxied {
		ttl = 3600
	}
	body := map[string]interface{}{
		"type":    recordType,
		"name":    name,
		"content": content,
		"proxied": proxied,
		"ttl":     ttl,
	}
	url := fmt.Sprintf("%s/zones/%s/dns_records", baseURL, zoneID)
	_, err := c.do(http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("gagal buat DNS record: %w", err)
	}
	return nil
}

// CreatePageRule bikin Page Rule (V1) baru di zone.
// Return: Rule ID.
func (c *Client) CreatePageRule(zoneID, pattern, targetURL string) (string, error) {
	body := map[string]interface{}{
		"targets": []map[string]interface{}{
			{
				"target": "url",
				"constraint": map[string]interface{}{
					"operator": "matches",
					"value":    pattern,
				},
			},
		},
		"actions": []map[string]interface{}{
			{
				"id": "forwarding_url",
				"value": map[string]interface{}{
					"url":         targetURL,
					"status_code": 301,
				},
			},
		},
		"status":   "active",
		"priority": 1,
	}
	url := fmt.Sprintf("%s/zones/%s/pagerules", baseURL, zoneID)
	data, err := c.do(http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("gagal buat page rule: %w", err)
	}
	var resp struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if resp.Result.ID == "" {
		return "", fmt.Errorf("page rule terbuat tapi ID kosong")
	}
	return resp.Result.ID, nil
}

// CreateRedirectRuleV2 bikin Redirect Rule (V2 - Rulesets) baru.
// Kalau ruleset http_request_dynamic_redirect sudah ada → tambah rule ke ruleset itu.
// Kalau belum ada → bikin ruleset baru dengan rule di dalamnya.
// Return: RulesetID + RuleID.
func (c *Client) CreateRedirectRuleV2(zoneID, targetURL string) (string, string, error) {
	rule := map[string]interface{}{
		"action": "redirect",
		"action_parameters": map[string]interface{}{
			"from_value": map[string]interface{}{
				"target_url":            map[string]interface{}{"value": targetURL},
				"status_code":           301,
				"preserve_query_string": false,
			},
		},
		"expression": "true",
		"enabled":    true,
	}

	// Cek apakah sudah ada ruleset http_request_dynamic_redirect
	listURL := fmt.Sprintf("%s/zones/%s/rulesets", baseURL, zoneID)
	listData, err := c.do(http.MethodGet, listURL, nil)
	if err == nil {
		var listResp struct {
			Result []struct {
				ID    string `json:"id"`
				Phase string `json:"phase"`
			} `json:"result"`
		}
		if json.Unmarshal(listData, &listResp) == nil {
			for _, rs := range listResp.Result {
				if rs.Phase == "http_request_dynamic_redirect" {
					// Append rule ke ruleset existing
					addURL := fmt.Sprintf("%s/zones/%s/rulesets/%s/rules", baseURL, zoneID, rs.ID)
					rData, rErr := c.do(http.MethodPost, addURL, rule)
					if rErr == nil {
						var rResp struct {
							Result struct {
								Rules []struct {
									ID string `json:"id"`
								} `json:"rules"`
							} `json:"result"`
						}
						if json.Unmarshal(rData, &rResp) == nil && len(rResp.Result.Rules) > 0 {
							last := rResp.Result.Rules[len(rResp.Result.Rules)-1]
							return rs.ID, last.ID, nil
						}
					}
				}
			}
		}
	}

	// Belum ada ruleset → bikin baru
	body := map[string]interface{}{
		"name":  "Redirect Rules",
		"kind":  "zone",
		"phase": "http_request_dynamic_redirect",
		"rules": []interface{}{rule},
	}
	data, err := c.do(http.MethodPost, listURL, body)
	if err != nil {
		return "", "", fmt.Errorf("gagal buat redirect ruleset: %w", err)
	}
	var resp struct {
		Result struct {
			ID    string `json:"id"`
			Rules []struct {
				ID string `json:"id"`
			} `json:"rules"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", err
	}
	if len(resp.Result.Rules) == 0 {
		return "", "", fmt.Errorf("ruleset terbuat tapi rule kosong")
	}
	return resp.Result.ID, resp.Result.Rules[0].ID, nil
}

// ─── Public dispatch ─────────────────────────────────────────────────────────

func (c *Client) GetCurrentURL(rule store.CFRule) (string, error) {
	switch rule.Type {
	case "redirect_rules":
		return c.GetRedirectRuleURL(rule)
	case "page_rules":
		return c.GetPageRuleURL(rule)
	default:
		return "", fmt.Errorf("tipe tidak dikenal: %s", rule.Type)
	}
}

func (c *Client) UpdateURL(rule store.CFRule, newURL string) error {
	switch rule.Type {
	case "redirect_rules":
		return c.UpdateRedirectRuleURL(rule, newURL)
	case "page_rules":
		return c.UpdatePageRuleURL(rule, newURL)
	default:
		return fmt.Errorf("tipe tidak dikenal: %s", rule.Type)
	}
}
