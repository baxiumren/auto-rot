package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"bongbot/store"
)

const baseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
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

func (c *Client) do(method, url string, body any) ([]byte, error) {
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
	req.Header.Set("X-Auth-Email", c.email)
	req.Header.Set("X-Auth-Key", c.apiKey)
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

// ─── Redirect Rules (v1) ─────────────────────────────────────────────────────

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

// ─── Page Rules (v2) ─────────────────────────────────────────────────────────

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
