package klikcepat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Write([]byte(`{"data": {"id": 1, "email": "test@test.com"}}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	if err := c.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestPingBadAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"unauthorized"}`, 401)
	}))
	defer server.Close()

	c := New(server.URL, "wrong-key")
	if err := c.Ping(); err == nil {
		t.Fatal("expected Ping to fail on 401, got nil")
	}
}

func TestListLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/links") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": 1, "type": "link", "title": "Test", "url": "test", "location_url": "https://example.com"},
				{"id": 2, "type": "biolink", "title": "Bio", "url": "bio", "location_url": ""},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	links, err := c.ListLinks("")
	if err != nil {
		t.Fatalf("ListLinks failed: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].LocationURL != "https://example.com" {
		t.Errorf("link[0].LocationURL = %q", links[0].LocationURL)
	}
}

func TestUpdateLinkLocation(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/links/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		receivedBody = r.PostForm.Encode()
		_, _ = w.Write([]byte(`{"data": {"id": 42, "location_url": "https://new.com"}}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	if err := c.UpdateLinkLocation(42, "https://new.com"); err != nil {
		t.Fatalf("UpdateLinkLocation failed: %v", err)
	}
	if !strings.Contains(receivedBody, "location_url=https") {
		t.Errorf("body missing location_url: %s", receivedBody)
	}
}

func TestUpdateLinkLocationInvalidID(t *testing.T) {
	c := New("https://example.com", "test-key")
	if err := c.UpdateLinkLocation(0, "https://new.com"); err == nil {
		t.Fatal("expected error for id=0")
	}
	if err := c.UpdateLinkLocation(-1, "https://new.com"); err == nil {
		t.Fatal("expected error for id=-1")
	}
}

func TestCreateLinkValidation(t *testing.T) {
	c := New("https://example.com", "test-key")
	if _, err := c.CreateLink("", "title", "slug", "https://x.com", 0); err == nil {
		t.Fatal("expected error for empty linkType")
	}
	if _, err := c.CreateLink("link", "title", "slug", "", 0); err == nil {
		t.Fatal("expected error for empty locationURL")
	}
}

func TestUpdateLinkEmptyFields(t *testing.T) {
	c := New("https://example.com", "test-key")
	if _, err := c.UpdateLink(1, map[string]string{}); err == nil {
		t.Fatal("expected error for empty fields map")
	}
}

func TestHasCredentials(t *testing.T) {
	c := New("", "")
	if c.HasCredentials() {
		t.Error("expected false for empty creds")
	}
	c.SetCredentials("https://klikcepat.com", "key")
	if !c.HasCredentials() {
		t.Error("expected true after SetCredentials")
	}
}

func TestCreateProjectValidation(t *testing.T) {
	c := New("https://example.com", "test-key")
	if _, err := c.CreateProject("", "#FF0000"); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestListProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/projects") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data": [{"id": 1, "name": "Foo", "color": "#FF0000"}]}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	projects, err := c.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Foo" {
		t.Errorf("unexpected projects: %+v", projects)
	}
}
