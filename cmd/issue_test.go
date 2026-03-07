package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newIssueTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/issues") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"issues":[]}`))
		case strings.HasPrefix(r.URL.Path, "/issues") && (r.Method == http.MethodPost || r.Method == http.MethodPut):
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte(`{"issue":{"id":1}}`))
		case strings.HasPrefix(r.URL.Path, "/issues") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
}

func TestIssueCommandsSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := newIssueTestServer(t)
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"

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
}

func TestIssueCreateHTTPError(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("bad"))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"

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
}

func TestIssueUpdateValidation(t *testing.T) {
	withConfigRuntime(t)
	server := newIssueTestServer(t)
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"

	// No fields set → validation error
	if err := newIssueUpdateCommand().RunE(newIssueUpdateCommand(), []string{"1"}); err == nil {
		t.Fatal("expected update field validation error")
	}
}

func TestIssueListQueryParams(t *testing.T) {
	tests := []struct {
		name   string
		flags  map[string]string
		checks map[string]string
	}{
		{
			name:   "assigned_to_id is sent",
			flags:  map[string]string{"assigned-to": "me"},
			checks: map[string]string{"assigned_to_id": "me"},
		},
		{
			name:   "offset is sent",
			flags:  map[string]string{"offset": "25"},
			checks: map[string]string{"offset": "25"},
		},
		{
			name:   "all sets limit=100 and offset=0",
			flags:  map[string]string{"all": "true"},
			checks: map[string]string{"limit": "100", "offset": "0"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withConfigRuntime(t)
			queryC := make(chan url.Values, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				queryC <- r.URL.Query()
				_, _ = w.Write([]byte(`{"issues":[]}`))
			}))
			defer server.Close()
			hostFlag = server.URL
			apiKeyFlag = "k"

			cmd := newIssueListCommand()
			for k, v := range tc.flags {
				_ = cmd.Flags().Set(k, v)
			}
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			got := <-queryC
			for key, want := range tc.checks {
				if v := got.Get(key); v != want {
					t.Errorf("query param %q: got %q, want %q", key, v, want)
				}
			}
		})
	}
}

func TestIssueCommandsMustRuntimeError(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())

	checks := []func() error{
		func() error { return newIssueListCommand().RunE(newIssueListCommand(), nil) },
		func() error { return newIssueViewCommand().RunE(newIssueViewCommand(), []string{"1"}) },
		func() error {
			c := newIssueCreateCommand()
			_ = c.Flags().Set("project", "p")
			_ = c.Flags().Set("subject", "s")
			return c.RunE(c, nil)
		},
		func() error { return newIssueUpdateCommand().RunE(newIssueUpdateCommand(), []string{"1"}) },
		func() error { return newIssueCloseCommand().RunE(newIssueCloseCommand(), []string{"1"}) },
		func() error {
			c := newIssueNoteAddCommand()
			_ = c.Flags().Set("notes", "x")
			return c.RunE(c, []string{"1"})
		},
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Fatalf("expected mustRuntime error at index %d", i)
		}
	}
}
