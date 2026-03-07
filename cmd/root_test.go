package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	if err := printJSON(map[string]string{"a": "b"}); err != nil {
		t.Fatalf("printJSON: %v", err)
	}
	if err := printJSON(map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestHandleRequestResult(t *testing.T) {
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
}

func TestSetVersion(t *testing.T) {
	SetVersion("1.2.3")
	if rootCmd.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %s", rootCmd.Version)
	}
}

func TestExecute(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"redmine", "--help"}
	t.Cleanup(func() { os.Args = oldArgs })
	rootCmd.SetArgs([]string{"--help"})
	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}
