package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserCommandsSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if err := newUserListCommand().RunE(newUserListCommand(), nil); err != nil {
		t.Fatal(err)
	}

	if err := newUserViewCommand().RunE(newUserViewCommand(), []string{"1"}); err != nil {
		t.Fatal(err)
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
