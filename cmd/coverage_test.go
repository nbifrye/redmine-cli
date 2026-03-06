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

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func withConfigRuntime(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &Config{DefaultHost: "http://example.invalid", Hosts: map[string]HostConfig{"http://example.invalid": {APIKey: "k"}}}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	hostFlag = ""
	apiKeyFlag = ""
	verbose = false
	debug = false
	return home
}

func TestRootHelpersAndExecute(t *testing.T) {
	if err := printJSON(map[string]string{"a": "b"}); err != nil {
		t.Fatalf("printJSON: %v", err)
	}
	if err := printJSON(map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("expected marshal error")
	}
	if err := handleRequestResult(json.RawMessage(`{"x":1}`), 0, nil); err != nil {
		t.Fatalf("handleRequestResult: %v", err)
	}
	if err := handleRequestResult(json.RawMessage(`{`), 0, nil); err == nil {
		t.Fatal("expected unmarshal error")
	}

	exited := 0
	oldExit := exitFunc
	exitFunc = func(code int) { exited = code }
	t.Cleanup(func() { exitFunc = oldExit })
	err := handleRequestResult(nil, 2, errors.New("boom"))
	if err == nil || exited != 2 {
		t.Fatalf("expected error and exit capture, err=%v exited=%d", err, exited)
	}

	oldArgs := os.Args
	os.Args = []string{"redmine", "--help"}
	t.Cleanup(func() { os.Args = oldArgs })
	rootCmd.SetArgs([]string{"--help"})
	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestLoadRuntimeAndHelpers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, err := LoadRuntime("", "", false, false)
	if err == nil {
		t.Fatal("expected missing host")
	}

	t.Setenv("REDMINE_HOST", "http://h")
	t.Setenv("REDMINE_API_KEY", "k")
	r, err := LoadRuntime("", "", true, true)
	if err != nil || r.Host != "http://h" || r.APIKey != "k" || !r.Verbose || !r.Debug {
		t.Fatalf("LoadRuntime unexpected: r=%+v err=%v", r, err)
	}

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

func TestDoJSONBranches(t *testing.T) {
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
	if _, _, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/ok", Query: map[string]string{"a": "1", "b": ""}}); err != nil {
		t.Fatal(err)
	}
	if raw, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/empty"}); err != nil || code != 0 || string(raw) != "{}" {
		t.Fatalf("empty: %s %d %v", raw, code, err)
	}
	if raw, _, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/text"}); err != nil || !strings.Contains(string(raw), "hello") {
		t.Fatalf("text wrap: %s %v", raw, err)
	}
	if _, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/e401"}); err == nil || code != 1 {
		t.Fatalf("401 expected code1 err, got %d %v", code, err)
	}
	if _, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/e500"}); err == nil || code != 2 {
		t.Fatalf("500 expected code2 err, got %d %v", code, err)
	}

	if _, _, err := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/ok", Body: map[string]any{"x": func() {}}}); err == nil {
		t.Fatal("expected marshal error")
	}
	if _, _, err := r.DoJSON(RequestOptions{Method: "GET\n", Path: "/ok"}); err == nil {
		t.Fatal("expected new request error")
	}

	rBad := &Runtime{Host: "%", APIKey: "k", Client: ts.Client()}
	if _, _, err := rBad.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err == nil {
		t.Fatal("expected parse error")
	}

	rErr := &Runtime{Host: ts.URL, APIKey: "k", Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("rt")
	})}}
	if _, code, err := rErr.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err == nil || code != 2 {
		t.Fatalf("expected client err code2: %d %v", code, err)
	}

	rReadErr := &Runtime{Host: ts.URL, APIKey: "k", Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(errReader{}), Header: make(http.Header)}, nil
	})}}
	if _, code, err := rReadErr.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/"}); err == nil || code != 2 {
		t.Fatalf("expected read err code2: %d %v", code, err)
	}

	if _, _, err := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/ok", RawBodyJSON: []byte(`{"k":1}`)}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigBranches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".config", "redmine-cli", "config.yml")
	if cfg, err := LoadConfig(); err != nil || cfg.Hosts == nil {
		t.Fatalf("LoadConfig missing file failed: %+v %v", cfg, err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("bad: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected yaml error")
	}
	if err := os.WriteFile(cfgPath, []byte("default_host: h\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil || cfg.Hosts == nil {
		t.Fatalf("expected hosts init: %+v %v", cfg, err)
	}

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

	oldMarshal := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errors.New("marshal err") }
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected marshal error")
	}
	yamlMarshal = oldMarshal

	oldRead := osReadFile
	osReadFile = func(string) ([]byte, error) { return nil, errors.New("read err") }
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected read error")
	}
	osReadFile = oldRead

	oldMk := osMkdirAll
	osMkdirAll = func(string, os.FileMode) error { return errors.New("mkdir err") }
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected mkdir error")
	}
	osMkdirAll = oldMk

	oldWrite := osWriteFile
	osWriteFile = func(string, []byte, os.FileMode) error { return errors.New("write err") }
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("expected write error")
	}
	osWriteFile = oldWrite

	oldUnmarshal := yamlUnmarshal
	yamlUnmarshal = func([]byte, any) error { return errors.New("unmarshal err") }
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected unmarshal override err")
	}
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

func TestCommandRunEPaths(t *testing.T) {
	home := withConfigRuntime(t)
	_ = home
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/projects"):
			if r.Method == http.MethodPost {
				_, _ = io.Copy(io.Discard, r.Body)
				_, _ = w.Write([]byte(`{"project":{"id":1}}`))
				return
			}
			_, _ = w.Write([]byte(`{"projects":[]}`))
		case strings.HasPrefix(r.URL.Path, "/issues") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"issues":[]}`))
		case strings.HasPrefix(r.URL.Path, "/issues") && (r.Method == http.MethodPost || r.Method == http.MethodPut):
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte(`{"issue":{"id":1}}`))
		case strings.HasPrefix(r.URL.Path, "/issues") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/users/current.json"):
			_, _ = w.Write([]byte(`{"user":{"id":1}}`))
		case strings.HasPrefix(r.URL.Path, "/users"):
			_, _ = w.Write([]byte(`{"users":[]}`))
		case strings.HasPrefix(r.URL.Path, "/time_entries") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"time_entries":[]}`))
		case strings.HasPrefix(r.URL.Path, "/time_entries") && r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte(`{"time_entry":{"id":1}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"

	if err := newAPIGetCommand().RunE(newAPIGetCommand(), []string{"/ok"}); err != nil {
		t.Fatal(err)
	}
	post := newAPIPostCommand()
	if err := post.Flags().Set("body", `{"x":1}`); err != nil {
		t.Fatal(err)
	}
	if err := post.RunE(post, []string{"/ok"}); err != nil {
		t.Fatal(err)
	}
	put := newAPIPutCommand()
	if err := put.Flags().Set("body", `{"x":2}`); err != nil {
		t.Fatal(err)
	}
	if err := put.RunE(put, []string{"/ok"}); err != nil {
		t.Fatal(err)
	}
	if err := newAPIDeleteCommand().RunE(newAPIDeleteCommand(), []string{"/issues/1.json"}); err != nil {
		t.Fatal(err)
	}

	if err := newProjectListCommand().RunE(newProjectListCommand(), nil); err != nil {
		t.Fatal(err)
	}
	if err := newProjectViewCommand().RunE(newProjectViewCommand(), []string{"abc"}); err != nil {
		t.Fatal(err)
	}
	projectCreate := newProjectCreateCommand()
	_ = projectCreate.Flags().Set("identifier", "proj")
	_ = projectCreate.Flags().Set("name", "Project")
	if err := projectCreate.RunE(projectCreate, nil); err != nil {
		t.Fatal(err)
	}

	list := newIssueListCommand()
	if err := list.Flags().Set("all", "true"); err != nil {
		t.Fatal(err)
	}
	if err := list.RunE(list, nil); err != nil {
		t.Fatal(err)
	}
	if err := newIssueViewCommand().RunE(newIssueViewCommand(), []string{"1"}); err != nil {
		t.Fatal(err)
	}
	update := newIssueUpdateCommand()
	_ = update.Flags().Set("subject", "updated")
	_ = update.Flags().Set("description", "updated description")
	_ = update.Flags().Set("status-id", "2")
	_ = update.Flags().Set("assigned-to-id", "10")
	if err := update.RunE(update, []string{"1"}); err != nil {
		t.Fatal(err)
	}
	if err := newIssueCloseCommand().RunE(newIssueCloseCommand(), []string{"1"}); err != nil {
		t.Fatal(err)
	}
	noteAdd := newIssueNoteAddCommand()
	_ = noteAdd.Flags().Set("notes", "memo")
	if err := noteAdd.RunE(noteAdd, []string{"1"}); err != nil {
		t.Fatal(err)
	}
	create := newIssueCreateCommand()
	_ = create.Flags().Set("project", "p")
	_ = create.Flags().Set("subject", "s")
	_ = create.Flags().Set("description", "d")
	if err := create.RunE(create, nil); err != nil {
		t.Fatal(err)
	}
	if err := newAuthStatusCommand().RunE(newAuthStatusCommand(), nil); err != nil {
		t.Fatal(err)
	}
	if err := newUserListCommand().RunE(newUserListCommand(), nil); err != nil {
		t.Fatal(err)
	}
	if err := newUserViewCommand().RunE(newUserViewCommand(), []string{"1"}); err != nil {
		t.Fatal(err)
	}
	timeList := newTimeEntryListCommand()
	_ = timeList.Flags().Set("user-id", "me")
	if err := timeList.RunE(timeList, nil); err != nil {
		t.Fatal(err)
	}
	timeCreate := newTimeEntryCreateCommand()
	_ = timeCreate.Flags().Set("issue-id", "1")
	_ = timeCreate.Flags().Set("hours", "1.5")
	_ = timeCreate.Flags().Set("activity-id", "9")
	_ = timeCreate.Flags().Set("spent-on", "2025-01-01")
	_ = timeCreate.Flags().Set("comments", "work")
	if err := timeCreate.RunE(timeCreate, nil); err != nil {
		t.Fatal(err)
	}
	timeCreateWithProject := newTimeEntryCreateCommand()
	_ = timeCreateWithProject.Flags().Set("project-id", "2")
	_ = timeCreateWithProject.Flags().Set("hours", "2")
	_ = timeCreateWithProject.Flags().Set("activity-id", "9")
	_ = timeCreateWithProject.Flags().Set("spent-on", "2025-01-02")
	if err := timeCreateWithProject.RunE(timeCreateWithProject, nil); err != nil {
		t.Fatal(err)
	}

	stdinOld := os.Stdin
	stdoutOld := os.Stdout
	rIn, wIn, _ := os.Pipe()
	rOut, wOut, _ := os.Pipe()
	os.Stdin = rIn
	os.Stdout = wOut
	t.Cleanup(func() { os.Stdin = stdinOld; os.Stdout = stdoutOld })
	_, _ = wIn.WriteString(server.URL + "\nkey\n")
	_ = wIn.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err != nil {
		t.Fatal(err)
	}
	_ = wOut.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	if !strings.Contains(buf.String(), "Login successful") {
		t.Fatalf("unexpected output: %s", buf.String())
	}

	rIn2, wIn2, _ := os.Pipe()
	os.Stdin = rIn2
	_, _ = wIn2.WriteString("\n")
	_ = wIn2.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected host required")
	}

	rIn3, wIn3, _ := os.Pipe()
	os.Stdin = rIn3
	_, _ = wIn3.WriteString(server.URL + "\n\n")
	_ = wIn3.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected api key required")
	}
}

func TestAdditionalCommandErrorPaths(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	if err := newAPIGetCommand().RunE(newAPIGetCommand(), []string{"/x"}); err == nil {
		t.Fatal("expected mustRuntime error")
	}
	hostFlag, apiKeyFlag = "http://example.com", "k"
	post := newAPIPostCommand()
	if err := post.Flags().Set("body", "@"); err != nil {
		t.Fatal(err)
	}
	if err := post.RunE(post, []string{"/x"}); err == nil {
		t.Fatal("expected file read error")
	}
	put := newAPIPutCommand()
	if err := put.Flags().Set("body", "@"); err != nil {
		t.Fatal(err)
	}
	if err := put.RunE(put, []string{"/x"}); err == nil {
		t.Fatal("expected file read error")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/issues.json" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("bad"))
			return
		}
		if r.URL.Path == "/users/current.json" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("bad"))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	hostFlag, apiKeyFlag = server.URL, "k"

	exited := 0
	oldExit := exitFunc
	exitFunc = func(code int) { exited = code }
	t.Cleanup(func() { exitFunc = oldExit })

	create := newIssueCreateCommand()
	_ = create.Flags().Set("project", "p")
	_ = create.Flags().Set("subject", "s")
	if err := create.RunE(create, nil); err == nil || exited != 1 {
		t.Fatalf("expected create error + exit capture: err=%v exited=%d", err, exited)
	}
	update := newIssueUpdateCommand()
	if err := update.RunE(update, []string{"1"}); err == nil {
		t.Fatal("expected update field validation error")
	}

	stdinOld := os.Stdin
	stdoutOld := os.Stdout
	rIn, wIn, _ := os.Pipe()
	rOut, wOut, _ := os.Pipe()
	os.Stdin = rIn
	os.Stdout = wOut
	t.Cleanup(func() { os.Stdin = stdinOld; os.Stdout = stdoutOld })
	_, _ = wIn.WriteString(server.URL + "\nkey\n")
	_ = wIn.Close()
	err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil)
	_ = wOut.Close()
	_, _ = io.Copy(io.Discard, rOut)
	if err == nil || exited != 1 {
		t.Fatalf("expected auth login error + exit capture: err=%v exited=%d", err, exited)
	}
}

func TestMustRuntimeErrorBranchesForCommands(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())
	checks := []func() error{
		func() error { return newAuthStatusCommand().RunE(newAuthStatusCommand(), nil) },
		func() error { return newProjectListCommand().RunE(newProjectListCommand(), nil) },
		func() error { return newProjectViewCommand().RunE(newProjectViewCommand(), []string{"p"}) },
		func() error { return newIssueListCommand().RunE(newIssueListCommand(), nil) },
		func() error { return newIssueViewCommand().RunE(newIssueViewCommand(), []string{"1"}) },
		func() error {
			c := newIssueCreateCommand()
			_ = c.Flags().Set("project", "p")
			_ = c.Flags().Set("subject", "s")
			return c.RunE(c, nil)
		},
		func() error {
			c := newAPIPostCommand()
			_ = c.Flags().Set("body", `{"a":1}`)
			return c.RunE(c, []string{"/x"})
		},
		func() error {
			c := newAPIPutCommand()
			_ = c.Flags().Set("body", `{"a":1}`)
			return c.RunE(c, []string{"/x"})
		},
		func() error { return newAPIDeleteCommand().RunE(newAPIDeleteCommand(), []string{"/x"}) },
		func() error { return newProjectCreateCommand().RunE(newProjectCreateCommand(), nil) },
		func() error { return newIssueUpdateCommand().RunE(newIssueUpdateCommand(), []string{"1"}) },
		func() error { return newIssueCloseCommand().RunE(newIssueCloseCommand(), []string{"1"}) },
		func() error {
			c := newIssueNoteAddCommand()
			_ = c.Flags().Set("notes", "x")
			return c.RunE(c, []string{"1"})
		},
		func() error { return newUserListCommand().RunE(newUserListCommand(), nil) },
		func() error { return newUserViewCommand().RunE(newUserViewCommand(), []string{"1"}) },
		func() error { return newTimeEntryListCommand().RunE(newTimeEntryListCommand(), nil) },
		func() error {
			c := newTimeEntryCreateCommand()
			_ = c.Flags().Set("hours", "1")
			_ = c.Flags().Set("activity-id", "9")
			_ = c.Flags().Set("spent-on", "2025-01-01")
			return c.RunE(c, nil)
		},
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Fatalf("expected mustRuntime error at %d", i)
		}
	}
}

func TestLoadRuntimeAdditionalBranches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REDMINE_HOST", "http://h")
	t.Setenv("REDMINE_API_KEY", "")
	if _, err := LoadRuntime("", "", false, false); err == nil {
		t.Fatal("expected missing API key")
	}
	cfgPath := filepath.Join(home, ".config", "redmine-cli", "config.yml")
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0o700)
	_ = os.WriteFile(cfgPath, []byte("bad: ["), 0o600)
	t.Setenv("REDMINE_HOST", "")
	if _, err := LoadRuntime("", "", false, false); err == nil {
		t.Fatal("expected config load error")
	}
}

func TestAuthLoginConfigErrorBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{}}`))
	}))
	defer server.Close()

	stdinOld, stdoutOld := os.Stdin, os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	t.Cleanup(func() { os.Stdin = stdinOld; os.Stdout = stdoutOld })

	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("home") }
	rIn, wIn, _ := os.Pipe()
	os.Stdin = rIn
	_, _ = wIn.WriteString(server.URL + "\nkey\n")
	_ = wIn.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected LoadConfig error")
	}
	userHomeDir = oldHome

	oldWrite := osWriteFile
	osWriteFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	rIn2, wIn2, _ := os.Pipe()
	os.Stdin = rIn2
	_, _ = wIn2.WriteString(server.URL + "\nkey\n")
	_ = wIn2.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected SaveConfig error")
	}
	osWriteFile = oldWrite

	rIn3, wIn3, _ := os.Pipe()
	os.Stdin = rIn3
	_, _ = wIn3.WriteString("%\nkey\n")
	_ = wIn3.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected DoJSON parse error")
	}
	_ = wOut.Close()
	_, _ = io.Copy(io.Discard, rOut)
}
