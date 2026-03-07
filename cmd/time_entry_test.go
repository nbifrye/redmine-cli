package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTimeEntryCommandsSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/time_entries") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"time_entries":[]}`))
		case strings.HasPrefix(r.URL.Path, "/time_entries") && r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte(`{"time_entry":{"id":1}}`))
		}
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"

	timeList := newTimeEntryListCommand()
	_ = timeList.Flags().Set("user-id", "me")
	if err := timeList.RunE(timeList, nil); err != nil {
		t.Fatal(err)
	}

	// Create with issue-id
	timeCreate := newTimeEntryCreateCommand()
	_ = timeCreate.Flags().Set("issue-id", "1")
	_ = timeCreate.Flags().Set("hours", "1.5")
	_ = timeCreate.Flags().Set("activity-id", "9")
	_ = timeCreate.Flags().Set("spent-on", "2025-01-01")
	_ = timeCreate.Flags().Set("comments", "work")
	if err := timeCreate.RunE(timeCreate, nil); err != nil {
		t.Fatal(err)
	}

	// Create with project-id
	timeCreateWithProject := newTimeEntryCreateCommand()
	_ = timeCreateWithProject.Flags().Set("project-id", "2")
	_ = timeCreateWithProject.Flags().Set("hours", "2")
	_ = timeCreateWithProject.Flags().Set("activity-id", "9")
	_ = timeCreateWithProject.Flags().Set("spent-on", "2025-01-02")
	if err := timeCreateWithProject.RunE(timeCreateWithProject, nil); err != nil {
		t.Fatal(err)
	}
}

func TestTimeEntryCreateValidation(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"time_entry":{"id":1}}`))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"

	// Neither issue-id nor project-id → validation error
	c := newTimeEntryCreateCommand()
	_ = c.Flags().Set("hours", "1")
	_ = c.Flags().Set("activity-id", "9")
	_ = c.Flags().Set("spent-on", "2025-01-01")
	if err := c.RunE(c, nil); err == nil {
		t.Fatal("expected validation error (no issue-id or project-id)")
	}
}

func TestTimeEntryCommandsMustRuntimeError(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())

	checks := []func() error{
		func() error { return newTimeEntryListCommand().RunE(newTimeEntryListCommand(), nil) },
		func() error {
			c := newTimeEntryCreateCommand()
			_ = c.Flags().Set("issue-id", "1")
			_ = c.Flags().Set("hours", "1")
			_ = c.Flags().Set("activity-id", "9")
			_ = c.Flags().Set("spent-on", "2025-01-01")
			return c.RunE(c, nil)
		},
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Fatalf("expected mustRuntime error at index %d", i)
		}
	}
}
