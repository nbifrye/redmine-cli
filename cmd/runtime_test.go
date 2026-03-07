package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- shared test helpers ---

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// withConfigRuntime sets up a temporary HOME with a valid config file and resets global flags.
// Tests that make HTTP calls must also switch to httptest.NewTLSServer and set newHTTPClient.
func withConfigRuntime(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &Config{DefaultHost: "https://example.invalid", Hosts: map[string]HostConfig{"https://example.invalid": {APIKey: "k"}}}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	hostFlag = ""
	apiKeyFlag = ""
	verbose = false
	debug = false
	oldNewHTTPClient := newHTTPClient
	t.Cleanup(func() { newHTTPClient = oldNewHTTPClient })
	return home
}

// --- LoadRuntime / config precedence ---

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

func TestLoadRuntimeVerboseDebugAndTimeout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("REDMINE_HOST", "https://h")
	t.Setenv("REDMINE_API_KEY", "k")
	r, err := LoadRuntime("", "", true, true)
	if err != nil || r.Host != "https://h" || r.APIKey != "k" || !r.Verbose || !r.Debug {
		t.Fatalf("LoadRuntime unexpected: r=%+v err=%v", r, err)
	}
	if r.Client.Timeout == 0 {
		t.Fatal("expected non-zero HTTP client timeout")
	}
}

func TestLoadRuntimeEdgeCases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REDMINE_HOST", "")
	t.Setenv("REDMINE_API_KEY", "")

	// No host anywhere → error
	if _, err := LoadRuntime("", "", false, false); err == nil {
		t.Fatal("expected missing host error")
	}

	// HTTP host → invalid scheme error (HTTPS only)
	t.Setenv("REDMINE_HOST", "http://h")
	if _, err := LoadRuntime("", "", false, false); err == nil {
		t.Fatal("expected invalid scheme error for http://")
	}

	// Host set via env (HTTPS) but no API key → error
	t.Setenv("REDMINE_HOST", "https://h")
	if _, err := LoadRuntime("", "", false, false); err == nil {
		t.Fatal("expected missing API key")
	}

	// Bad config file (YAML parse error)
	cfgPath := filepath.Join(home, ".config", "redmine-cli", "config.yml")
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0o700)
	_ = os.WriteFile(cfgPath, []byte("bad: ["), 0o600)
	t.Setenv("REDMINE_HOST", "")
	if _, err := LoadRuntime("", "", false, false); err == nil {
		t.Fatal("expected config load error")
	}
}

// --- runtime helper functions ---

func TestRuntimeHelperFuncs(t *testing.T) {
	if got := firstNonEmpty(" ", " a "); got != "a" {
		t.Fatalf("firstNonEmpty=%q", got)
	}
	if got := firstNonEmpty(" ", " "); got != "" {
		t.Fatalf("firstNonEmpty empty=%q", got)
	}
	if normalizePath("x") != "/x" || normalizePath("/x") != "/x" {
		t.Fatal("normalizePath failed")
	}
	if mapStatus(500) != 2 || mapStatus(400) != 1 {
		t.Fatal("mapStatus failed")
	}
	if !strings.Contains(buildStatusMessage(401, "b"), "authentication failed") {
		t.Fatal("401 message")
	}
	if !strings.Contains(buildStatusMessage(404, "b"), "resource not found") {
		t.Fatal("404 message")
	}
	if !strings.Contains(buildStatusMessage(418, "b"), "request failed (418)") {
		t.Fatal("default message")
	}
}

// --- SaveConfig ---

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

func TestConfigLoadAndSaveBranches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".config", "redmine-cli", "config.yml")

	// Missing file → returns empty config without error
	if cfg, err := LoadConfig(); err != nil || cfg.Hosts == nil {
		t.Fatalf("LoadConfig missing file failed: %+v %v", cfg, err)
	}

	// Bad YAML → error
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("bad: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected yaml error")
	}

	// Valid YAML with no hosts → Hosts initialized
	if err := os.WriteFile(cfgPath, []byte("default_host: h\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil || cfg.Hosts == nil {
		t.Fatalf("expected hosts init: %+v %v", cfg, err)
	}

	// configPath / LoadConfig / SaveConfig all fail when userHomeDir errors
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("home err") }
	if _, err := configPath(); err == nil {
		t.Fatal("expected configPath error")
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected LoadConfig path error")
	}
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected SaveConfig path error")
	}
	userHomeDir = oldHome

	// SaveConfig fails when yamlMarshal errors
	oldMarshal := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errors.New("marshal err") }
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected marshal error")
	}
	yamlMarshal = oldMarshal

	// LoadConfig fails when osReadFile errors
	oldRead := osReadFile
	osReadFile = func(string) ([]byte, error) { return nil, errors.New("read err") }
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected read error")
	}
	osReadFile = oldRead

	// SaveConfig fails when osMkdirAll errors
	oldMk := osMkdirAll
	osMkdirAll = func(string, os.FileMode) error { return errors.New("mkdir err") }
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected mkdir error")
	}
	osMkdirAll = oldMk

	// SaveConfig fails when osWriteFile errors
	oldWrite := osWriteFile
	osWriteFile = func(string, []byte, os.FileMode) error { return errors.New("write err") }
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected write error")
	}
	osWriteFile = oldWrite

	// LoadConfig fails when yamlUnmarshal errors
	oldUnmarshal := yamlUnmarshal
	yamlUnmarshal = func([]byte, any) error { return errors.New("unmarshal err") }
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected unmarshal override err")
	}
	// yamlUnmarshal returning nil with Hosts=nil → Hosts re-initialized
	yamlUnmarshal = func(_ []byte, out any) error {
		cfg := out.(*Config)
		cfg.Hosts = nil
		return nil
	}
	cfg2, err := LoadConfig()
	if err != nil || cfg2.Hosts == nil {
		t.Fatalf("expected hosts re-init, cfg=%+v err=%v", cfg2, err)
	}
	yamlUnmarshal = oldUnmarshal
}

// --- DoJSON ---

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

func TestDoJSONAdditionalBranches(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/empty":
			w.WriteHeader(http.StatusOK)
		case "/text":
			_, _ = w.Write([]byte("hello"))
		case "/ok":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/e401":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("bad"))
		case "/e500":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("bad"))
		}
	}))
	defer ts.Close()

	r := &Runtime{Host: ts.URL, APIKey: "k", Client: ts.Client(), Verbose: true, Debug: true}

	// Query params and successful JSON response
	if _, _, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/ok", Query: map[string]string{"a": "1", "b": ""}}); err != nil {
		t.Fatal(err)
	}
	// Empty body → returns "{}"
	if raw, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/empty"}); err != nil || code != 0 || string(raw) != "{}" {
		t.Fatalf("empty: %s %d %v", raw, code, err)
	}
	// Non-JSON text → wrapped in response object
	if raw, _, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/text"}); err != nil || !strings.Contains(string(raw), "hello") {
		t.Fatalf("text wrap: %s %v", raw, err)
	}
	// 401 → exit code 1
	if _, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/e401"}); err == nil || code != 1 {
		t.Fatalf("401 expected code1 err, got %d %v", code, err)
	}
	// 500 → exit code 2
	if _, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/e500"}); err == nil || code != 2 {
		t.Fatalf("500 expected code2 err, got %d %v", code, err)
	}
	// Unmarshalable body → marshal error
	if _, _, err := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/ok", Body: map[string]any{"x": func() {}}}); err == nil {
		t.Fatal("expected marshal error")
	}
	// Invalid HTTP method → new request error
	if _, _, err := r.DoJSON(RequestOptions{Method: "GET\n", Path: "/ok"}); err == nil {
		t.Fatal("expected new request error")
	}
	// Bad host URL → parse error
	rBad := &Runtime{Host: "%", APIKey: "k", Client: ts.Client()}
	if _, _, err := rBad.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err == nil {
		t.Fatal("expected parse error")
	}
	// Transport error → exit code 2
	rErr := &Runtime{Host: ts.URL, APIKey: "k", Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("rt")
	})}}
	if _, code, err := rErr.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err == nil || code != 2 {
		t.Fatalf("expected client err code2: %d %v", code, err)
	}
	// Body read error → exit code 2
	rReadErr := &Runtime{Host: ts.URL, APIKey: "k", Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(errReader{}), Header: make(http.Header)}, nil
	})}}
	if _, code, err := rReadErr.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err == nil || code != 2 {
		t.Fatalf("expected read err code2: %d %v", code, err)
	}
	// RawBodyJSON
	if _, _, err := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/ok", RawBodyJSON: []byte(`{"k":1}`)}); err != nil {
		t.Fatal(err)
	}
}

// --- validateHost ---

func TestValidateHost(t *testing.T) {
	ok := []string{
		"https://redmine.example.com",
		"https://redmine.example.com:8443",
		"https://redmine.example.com/path",
	}
	for _, h := range ok {
		if err := validateHost(h); err != nil {
			t.Errorf("validateHost(%q) want nil, got %v", h, err)
		}
	}

	bad := []string{
		"http://redmine.example.com",
		"ftp://redmine.example.com",
		"//redmine.example.com",
		"redmine.example.com",
		"https://",
		"",
		"javascript:alert(1)",
	}
	for _, h := range bad {
		if err := validateHost(h); err == nil {
			t.Errorf("validateHost(%q) want error, got nil", h)
		}
	}
}

// --- validateNumericID ---

func TestValidateNumericID(t *testing.T) {
	ok := []string{"1", "42", "999999"}
	for _, id := range ok {
		if err := validateNumericID(id); err != nil {
			t.Errorf("validateNumericID(%q) want nil, got %v", id, err)
		}
	}

	bad := []string{"0", "-1", "abc", "1.5", "", "1a", " 1"}
	for _, id := range bad {
		if err := validateNumericID(id); err == nil {
			t.Errorf("validateNumericID(%q) want error, got nil", id)
		}
	}
}

// --- validateProjectIdentifier ---

func TestValidateProjectIdentifier(t *testing.T) {
	ok := []string{"my-project", "123", "proj_name", "abc123"}
	for _, id := range ok {
		if err := validateProjectIdentifier(id); err != nil {
			t.Errorf("validateProjectIdentifier(%q) want nil, got %v", id, err)
		}
	}

	bad := []string{"../admin", "a/b", `a\b`, "a..b", ".."}
	for _, id := range bad {
		if err := validateProjectIdentifier(id); err == nil {
			t.Errorf("validateProjectIdentifier(%q) want error, got nil", id)
		}
	}
}

// --- debug output redacts API key ---

func TestDebugHeaderRedactsAPIKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	// Capture stderr
	oldErr := os.Stderr
	pr, pw, _ := os.Pipe()
	os.Stderr = pw
	t.Cleanup(func() { os.Stderr = oldErr })

	rt := &Runtime{Host: ts.URL, APIKey: "super-secret-key", Client: ts.Client(), Debug: true}
	if _, _, err := rt.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err != nil {
		t.Fatal(err)
	}

	_ = pw.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, pr)

	output := buf.String()
	if strings.Contains(output, "super-secret-key") {
		t.Error("API key must not appear in debug output")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected [REDACTED] in debug output for X-Redmine-API-Key header")
	}
}

// --- newHTTPClient is injectable ---

func TestNewHTTPClientInjectable(t *testing.T) {
	called := false
	old := newHTTPClient
	newHTTPClient = func() *http.Client {
		called = true
		return &http.Client{}
	}
	t.Cleanup(func() { newHTTPClient = old })

	_ = newRuntime("https://h", "k", false, false)
	if !called {
		t.Error("expected newHTTPClient to be called by newRuntime")
	}
}
