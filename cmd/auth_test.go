package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAuthLoginSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer server.Close()
	newHTTPClient = func() *http.Client { return server.Client() }

	stdinOld, stdoutOld := os.Stdin, os.Stdout
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
}

func TestAuthLoginValidation(t *testing.T) {
	withConfigRuntime(t)
	stdinOld := os.Stdin
	t.Cleanup(func() { os.Stdin = stdinOld })

	// No host entered → error
	rIn, wIn, _ := os.Pipe()
	os.Stdin = rIn
	_, _ = wIn.WriteString("\n")
	_ = wIn.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected host required error")
	}

	// HTTP host → scheme validation error
	rIn2, wIn2, _ := os.Pipe()
	os.Stdin = rIn2
	_, _ = wIn2.WriteString("http://example.com\nkey\n")
	_ = wIn2.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected https-only error for http:// host")
	}

	// No API key entered (valid HTTPS host) → error
	rIn3, wIn3, _ := os.Pipe()
	os.Stdin = rIn3
	_, _ = wIn3.WriteString("https://example.com\n\n")
	_ = wIn3.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected api key required error")
	}
}

func TestAuthLoginStdinEOF(t *testing.T) {
	withConfigRuntime(t)
	stdinOld := os.Stdin
	t.Cleanup(func() { os.Stdin = stdinOld })

	// EOF before host newline → error reading host
	rIn, wIn, _ := os.Pipe()
	os.Stdin = rIn
	_, _ = wIn.WriteString("https://no-newline")
	_ = wIn.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected EOF error reading host")
	}

	// EOF before API key newline → error reading API key
	rIn2, wIn2, _ := os.Pipe()
	os.Stdin = rIn2
	_, _ = wIn2.WriteString("https://example.com\nno-newline")
	_ = wIn2.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected EOF error reading API key")
	}
}

func TestAuthLoginConfigError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{}}`))
	}))
	defer server.Close()

	stdinOld, stdoutOld := os.Stdin, os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	t.Cleanup(func() { os.Stdin = stdinOld; os.Stdout = stdoutOld })

	oldHTTPClient := newHTTPClient
	t.Cleanup(func() { newHTTPClient = oldHTTPClient })

	// userHomeDir error → LoadConfig fails
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("home") }
	newHTTPClient = func() *http.Client { return server.Client() }
	rIn, wIn, _ := os.Pipe()
	os.Stdin = rIn
	_, _ = wIn.WriteString(server.URL + "\nkey\n")
	_ = wIn.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected LoadConfig error")
	}
	userHomeDir = oldHome

	// osWriteFile error → SaveConfig fails
	t.Setenv("HOME", t.TempDir())
	oldWrite := osWriteFile
	osWriteFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	newHTTPClient = func() *http.Client { return server.Client() }
	rIn2, wIn2, _ := os.Pipe()
	os.Stdin = rIn2
	_, _ = wIn2.WriteString(server.URL + "\nkey\n")
	_ = wIn2.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected SaveConfig error")
	}
	osWriteFile = oldWrite

	// Bad host URL (%) → validateHost error (invalid scheme)
	rIn3, wIn3, _ := os.Pipe()
	os.Stdin = rIn3
	_, _ = wIn3.WriteString("%\nkey\n")
	_ = wIn3.Close()
	if err := newAuthLoginCommand().RunE(newAuthLoginCommand(), nil); err == nil {
		t.Fatal("expected validateHost error for invalid URL")
	}

	_ = wOut.Close()
	_, _ = io.Copy(io.Discard, rOut)
}

func TestAuthLoginHTTPError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/current.json" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("bad"))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	hostFlag, apiKeyFlag = server.URL, "k"
	oldHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client { return server.Client() }
	t.Cleanup(func() { newHTTPClient = oldHTTPClient })

	exited := 0
	oldExit := exitFunc
	exitFunc = func(code int) { exited = code }
	t.Cleanup(func() { exitFunc = oldExit })

	stdinOld, stdoutOld := os.Stdin, os.Stdout
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

func TestAuthStatusCommandSuccess(t *testing.T) {
	withConfigRuntime(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer server.Close()
	hostFlag = server.URL
	apiKeyFlag = "k"
	newHTTPClient = func() *http.Client { return server.Client() }

	if err := newAuthStatusCommand().RunE(newAuthStatusCommand(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestAuthCommandsMustRuntimeError(t *testing.T) {
	hostFlag, apiKeyFlag = "", ""
	t.Setenv("HOME", t.TempDir())

	if err := newAuthStatusCommand().RunE(newAuthStatusCommand(), nil); err == nil {
		t.Fatal("expected mustRuntime error")
	}
}
