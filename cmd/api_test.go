package cmd

import (
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
}
