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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
