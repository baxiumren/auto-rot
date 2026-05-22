package checker

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var httpClient = &http.Client{
	Timeout:       20 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
}

func CheckDomain(domain string) string {
	domain = Clean(domain)
	for attempt := 1; attempt <= 2; attempt++ {
		status, err := checkTrustPositif(domain)
		if err != nil {
			log.Printf("[NAWALA] attempt %d domain %s error: %v", attempt, domain, err)
			if attempt < 2 {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return "ERROR"
		}
		result := toStatus(status)
		log.Printf("[NAWALA] %s → %s", domain, result)
		return result
	}
	return "ERROR"
}

func Clean(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "www.")
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	return strings.TrimSuffix(domain, "/")
}

func checkTrustPositif(domain string) (string, error) {
	const baseURL = "https://trustpositif.komdigi.go.id/"

	req, _ := http.NewRequest("GET", baseURL, nil)
	setHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	csrf := extractCSRF(string(body))
	if csrf == "" {
		return "", fmt.Errorf("csrf_token tidak ketemu")
	}

	checkURL := fmt.Sprintf(
		"https://trustpositif.komdigi.go.id/welcome?csrf_token=%s&recaptcha_token=&domains=%s",
		url.QueryEscape(csrf), url.QueryEscape(domain),
	)
	req2, _ := http.NewRequest("GET", checkURL, nil)
	setHeaders(req2)
	req2.Header.Set("Referer", baseURL)

	resp2, err := httpClient.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)

	return parseHTML(string(body2), domain)
}

func setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
}

func extractCSRF(html string) string {
	if m := regexp.MustCompile(`csrf_token=([a-fA-F0-9]+)`).FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := regexp.MustCompile(`csrf_token["'\s:=]+([a-fA-F0-9]+)`).FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseHTML(html, domain string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	var found string
	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() >= 2 {
			if normalize(tds.Eq(0).Text()) == normalize(domain) {
				found = strings.TrimSpace(tds.Eq(1).Text())
			}
		}
	})
	if found == "" {
		text := strings.ToLower(doc.Text())
		if strings.Contains(text, "tidak ada") {
			return "Tidak Ada", nil
		}
		if strings.Contains(text, "ada") {
			return "Ada", nil
		}
		return "", fmt.Errorf("status tidak ketemu")
	}
	return found, nil
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "www.")
	return strings.TrimSuffix(s, "/")
}

func toStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	if strings.Contains(s, "tidak ada") {
		return "SAFE"
	}
	if strings.Contains(s, "ada") {
		return "BLOCKED"
	}
	return "ERROR"
}
