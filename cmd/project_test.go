package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectCommandsSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/projects") && r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte(`{"project":{"id":1}}`))
		default:
			_, _ = w.Write([]byte(`{"projects":[]}`))
		}
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

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
}

func TestProjectViewInvalidIdentifier(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"projects":[]}`))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

	invalidIDs := []string{"../admin", "a/b", `a\b`, "a..b"}
	for _, id := range invalidIDs {
		if err := newProjectViewCommand().RunE(newProjectViewCommand(), []string{id}); err == nil {
			t.Errorf("project view %q: expected invalid identifier error, got nil", id)
		}
	}

	// Valid identifiers should pass validation
	validIDs := []string{"my-project", "123", "proj_name"}
	for _, id := range validIDs {
		// Just check that validation passes (server call may succeed or fail due to TLS, etc.)
		if err := validateProjectIdentifier(id); err != nil {
			t.Errorf("validateProjectIdentifier(%q) unexpected error: %v", id, err)
		}
	}
}

func TestProjectCommandsMustRuntimeError(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())

	checks := []func() error{
		func() error { return newProjectListCommand().RunE(newProjectListCommand(), nil) },
		func() error { return newProjectViewCommand().RunE(newProjectViewCommand(), []string{"p"}) },
		func() error { return newProjectCreateCommand().RunE(newProjectCreateCommand(), nil) },
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Fatalf("expected mustRuntime error at index %d", i)
		}
	}
}

func TestProjectListHTTPError(t *testing.T) {
	cases := []struct {
		status   int
		wantExit int
	}{
		{http.StatusUnauthorized, 1},
		{http.StatusInternalServerError, 2},
	}
	for _, tc := range cases {
		withConfigRuntime(t)
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		hostFlag = srv.URL
		apiKeyFlag = "k"
		newHTTPClient = func() *http.Client { return srv.Client() }

		exited := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exited = code }

		err := newProjectListCommand().RunE(newProjectListCommand(), nil)
		exitFunc = oldExit
		srv.Close()

		if err == nil || exited != tc.wantExit {
			t.Errorf("status %d: want err+exit=%d, got err=%v exited=%d", tc.status, tc.wantExit, err, exited)
		}
	}
}

func TestProjectViewHTTPError(t *testing.T) {
	cases := []struct {
		status   int
		wantExit int
	}{
		{http.StatusUnauthorized, 1},
		{http.StatusNotFound, 1},
		{http.StatusInternalServerError, 2},
	}
	for _, tc := range cases {
		withConfigRuntime(t)
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		hostFlag = srv.URL
		apiKeyFlag = "k"
		newHTTPClient = func() *http.Client { return srv.Client() }

		exited := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exited = code }

		err := newProjectViewCommand().RunE(newProjectViewCommand(), []string{"abc"})
		exitFunc = oldExit
		srv.Close()

		if err == nil || exited != tc.wantExit {
			t.Errorf("status %d: want err+exit=%d, got err=%v exited=%d", tc.status, tc.wantExit, err, exited)
		}
	}
}

func TestProjectCreateHTTPError(t *testing.T) {
	cases := []struct {
		status   int
		wantExit int
	}{
		{http.StatusUnauthorized, 1},
		{http.StatusInternalServerError, 2},
	}
	for _, tc := range cases {
		withConfigRuntime(t)
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		hostFlag = srv.URL
		apiKeyFlag = "k"
		newHTTPClient = func() *http.Client { return srv.Client() }

		exited := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exited = code }

		cmd := newProjectCreateCommand()
		_ = cmd.Flags().Set("identifier", "proj")
		_ = cmd.Flags().Set("name", "Project")
		err := cmd.RunE(cmd, nil)
		exitFunc = oldExit
		srv.Close()

		if err == nil || exited != tc.wantExit {
			t.Errorf("status %d: want err+exit=%d, got err=%v exited=%d", tc.status, tc.wantExit, err, exited)
		}
	}
}
