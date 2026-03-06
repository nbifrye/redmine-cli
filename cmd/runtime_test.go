package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuntimePrecedence(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := &Config{
		DefaultHost: "https://from-config.example.com",
		Hosts: map[string]HostConfig{
			"https://from-config.example.com": {APIKey: "config-key"},
			"https://from-env.example.com":    {APIKey: "env-host-config-key"},
			"https://from-flag.example.com":   {APIKey: "flag-host-config-key"},
		},
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	t.Setenv("REDMINE_HOST", "https://from-env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")

	r, err := LoadRuntime("https://from-flag.example.com", "flag-key", true, true)
	if err != nil {
		t.Fatalf("LoadRuntime failed: %v", err)
	}

	if r.Host != "https://from-flag.example.com" {
		t.Fatalf("unexpected host: %s", r.Host)
	}
	if r.APIKey != "flag-key" {
		t.Fatalf("unexpected api key: %s", r.APIKey)
	}
	if !r.Verbose || !r.Debug {
		t.Fatalf("verbose/debug flags were not set")
	}

	r2, err := LoadRuntime("", "", false, false)
	if err != nil {
		t.Fatalf("LoadRuntime without flags failed: %v", err)
	}
	if r2.Host != "https://from-env.example.com" {
		t.Fatalf("expected env host, got: %s", r2.Host)
	}
	if r2.APIKey != "env-key" {
		t.Fatalf("expected env api key, got: %s", r2.APIKey)
	}

	t.Setenv("REDMINE_HOST", "")
	t.Setenv("REDMINE_API_KEY", "")
	r3, err := LoadRuntime("", "", false, false)
	if err != nil {
		t.Fatalf("LoadRuntime from config failed: %v", err)
	}
	if r3.Host != "https://from-config.example.com" {
		t.Fatalf("expected config host, got: %s", r3.Host)
	}
	if r3.APIKey != "config-key" {
		t.Fatalf("expected config api key, got: %s", r3.APIKey)
	}
}

func TestSaveConfigCreatesSecureFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := &Config{DefaultHost: "https://example.com", Hosts: map[string]HostConfig{"https://example.com": {APIKey: "k"}}}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	path := filepath.Join(tmpHome, ".config", "redmine-cli", "config.yml")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file stat failed: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected 0600, got %o", got)
	}
}

func TestDoJSONSuccessAndErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.json":
			if got := r.Header.Get("X-Redmine-API-Key"); got != "token" {
				t.Fatalf("missing API key header: %s", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/notfound.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	r := &Runtime{Host: ts.URL, APIKey: "token", Client: ts.Client()}

	raw, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/ok.json"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected code 0, got: %d", code)
	}
	var parsed map[string]bool
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !parsed["ok"] {
		t.Fatalf("unexpected payload: %s", string(raw))
	}

	_, code, err = r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/notfound.json"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if code != 1 {
		t.Fatalf("expected exit code 1 for 404, got %d", code)
	}
}
