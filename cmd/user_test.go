package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestUserCommandsSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/users/current.json"):
			_, _ = w.Write([]byte(`{"user":{"id":1}}`))
		default:
			_, _ = w.Write([]byte(`{"users":[]}`))
		}
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

	if err := newUserListCommand().RunE(newUserListCommand(), nil); err != nil {
		t.Fatal(err)
	}

	if err := newUserViewCommand().RunE(newUserViewCommand(), []string{"1"}); err != nil {
		t.Fatal(err)
	}
}

func TestUserViewInvalidID(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"users":[]}`))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

	invalidIDs := []string{"abc", "0", "-1", "1.5"}
	for _, id := range invalidIDs {
		if err := newUserViewCommand().RunE(newUserViewCommand(), []string{id}); err == nil {
			t.Errorf("user view %q: expected invalid ID error, got nil", id)
		}
	}
}

func TestUserListQueryParams(t *testing.T) {
	withConfigRuntime(t)
	queryC := make(chan url.Values, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryC <- r.URL.Query()
		_, _ = w.Write([]byte(`{"users":[]}`))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

	cmd := newUserListCommand()
	_ = cmd.Flags().Set("offset", "50")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	got := <-queryC
	if v := got.Get("offset"); v != "50" {
		t.Errorf("query param %q: got %q, want %q", "offset", v, "50")
	}
}

func TestUserCommandsMustRuntimeError(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())

	checks := []func() error{
		func() error { return newUserListCommand().RunE(newUserListCommand(), nil) },
		func() error { return newUserViewCommand().RunE(newUserViewCommand(), []string{"1"}) },
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Fatalf("expected mustRuntime error at index %d", i)
		}
	}
}

func TestUserListHTTPError(t *testing.T) {
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

		err := newUserListCommand().RunE(newUserListCommand(), nil)
		exitFunc = oldExit
		srv.Close()

		if err == nil || exited != tc.wantExit {
			t.Errorf("status %d: want err+exit=%d, got err=%v exited=%d", tc.status, tc.wantExit, err, exited)
		}
	}
}

func TestUserViewHTTPError(t *testing.T) {
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

		err := newUserViewCommand().RunE(newUserViewCommand(), []string{"1"})
		exitFunc = oldExit
		srv.Close()

		if err == nil || exited != tc.wantExit {
			t.Errorf("status %d: want err+exit=%d, got err=%v exited=%d", tc.status, tc.wantExit, err, exited)
		}
	}
}
