package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReadBodyArg(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, err := readBodyArg(""); err == nil {
			t.Fatalf("expected error for empty body")
		}
	})

	t.Run("inline json", func(t *testing.T) {
		got, err := readBodyArg(`{"a":1}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != `{"a":1}` {
			t.Fatalf("unexpected body: %s", string(got))
		}
	})

	t.Run("from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "body.json")
		if err := os.WriteFile(path, []byte(`{"ok":true}`), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
		got, err := readBodyArg("@" + path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != `{"ok":true}` {
			t.Fatalf("unexpected file body: %s", string(got))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		if _, err := readBodyArg("@/nonexistent/file.json"); err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("file exceeds size limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "big.json")
		// Write a file just over the 10 MB limit
		big := make([]byte, maxBodyFileSize+1)
		if err := os.WriteFile(path, big, 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
		if _, err := readBodyArg("@" + path); err == nil {
			t.Fatal("expected error for file exceeding 10 MB limit")
		}
	})
}

func TestAPICommandsSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

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
}

func TestAPICommandsBodyError(t *testing.T) {
	hostFlag, apiKeyFlag = "https://example.com", "k"

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
}

func TestAPICommandsMustRuntimeError(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())

	if err := newAPIGetCommand().RunE(newAPIGetCommand(), []string{"/x"}); err == nil {
		t.Fatal("expected mustRuntime error")
	}
	c := newAPIPostCommand()
	_ = c.Flags().Set("body", `{"a":1}`)
	if err := c.RunE(c, []string{"/x"}); err == nil {
		t.Fatal("expected mustRuntime error")
	}
	c2 := newAPIPutCommand()
	_ = c2.Flags().Set("body", `{"a":1}`)
	if err := c2.RunE(c2, []string{"/x"}); err == nil {
		t.Fatal("expected mustRuntime error")
	}
	if err := newAPIDeleteCommand().RunE(newAPIDeleteCommand(), []string{"/x"}); err == nil {
		t.Fatal("expected mustRuntime error")
	}
}
