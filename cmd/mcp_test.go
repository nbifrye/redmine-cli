package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildMCPToolsIncludesOnlyTargetResources(t *testing.T) {
	tools := buildMCPTools()
	if _, ok := tools["auth.login"]; ok {
		t.Fatal("auth.login must be excluded")
	}
	if _, ok := tools["api.get"]; ok {
		t.Fatal("api.get must be excluded")
	}
	for _, want := range []string{"issue.list", "issue.view", "issue.create", "issue.update", "issue.close", "issue.note-add", "project.list", "project.view", "project.create", "user.list", "user.view"} {
		if _, ok := tools[want]; !ok {
			t.Fatalf("missing tool: %s", want)
		}
	}
}

func TestMCPToolInputSchemaReflectsCommandDefinitions(t *testing.T) {
	def := buildMCPTools()["issue.create"]
	schema := def.inputSchema()
	props := schema["properties"].(map[string]any)
	if props["project"].(map[string]any)["type"] != "string" {
		t.Fatal("issue.create project flag should be string")
	}
	if props["verbose"].(map[string]any)["type"] != "boolean" {
		t.Fatal("persistent verbose flag should be boolean")
	}
	req := map[string]bool{}
	for _, r := range schema["required"].([]string) {
		req[r] = true
	}
	if !req["project"] || !req["subject"] {
		t.Fatal("required flags should include project and subject")
	}

	view := buildMCPTools()["issue.view"].inputSchema()
	viewReq := map[string]bool{}
	for _, r := range view["required"].([]string) {
		viewReq[r] = true
	}
	if !viewReq["issue-id"] {
		t.Fatal("issue.view should require positional issue-id")
	}
}

func TestMCPCallToolRunsExistingCommandAndParsesStructuredContent(t *testing.T) {
	withConfigRuntime(t)
	queryC := make(chan url.Values, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryC <- r.URL.Query()
		_, _ = w.Write([]byte(`{"issues":[]}`))
	}))
	defer server.Close()
	newHTTPClient = func() *http.Client { return server.Client() }

	srv := &mcpServer{tools: buildMCPTools()}
	result, err := srv.callTool("issue.list", map[string]any{"host": server.URL, "api-key": "k", "assigned-to": "me", "offset": 25.0, "limit": 10.0})
	if err != nil {
		t.Fatalf("callTool error: %v", err)
	}
	if !strings.Contains(result.Text, "issues") {
		t.Fatalf("unexpected text output: %s", result.Text)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type: %T", result.StructuredContent)
	}
	if _, ok := structured["issues"]; !ok {
		t.Fatalf("missing issues key: %#v", structured)
	}
	got := <-queryC
	if got.Get("assigned_to_id") != "me" {
		t.Fatalf("assigned_to_id mismatch: %q", got.Get("assigned_to_id"))
	}
	if got.Get("offset") != "25" || got.Get("limit") != "10" {
		t.Fatalf("pagination mismatch: offset=%q limit=%q", got.Get("offset"), got.Get("limit"))
	}
}

func TestMCPCallToolValidationErrors(t *testing.T) {
	srv := &mcpServer{tools: buildMCPTools()}
	if _, err := srv.callTool("issue.view", map[string]any{}); err == nil {
		t.Fatal("expected missing required positional arg error")
	}
	if _, err := srv.callTool("issue.view", map[string]any{"issue-id": "1", "unknown": "x"}); err == nil {
		t.Fatal("expected unknown argument error")
	}
	if _, err := srv.callTool("unknown.tool", map[string]any{}); err == nil {
		t.Fatal("expected unknown tool error")
	}
	srv.tools["broken.factory"] = mcpToolDef{Name: "broken.factory"}
	if _, err := srv.callTool("broken.factory", map[string]any{}); err == nil {
		t.Fatal("expected nil factory error")
	}
	srv.tools["nil.command"] = mcpToolDef{Name: "nil.command", Factory: func() *cobra.Command { return nil }}
	if _, err := srv.callTool("nil.command", map[string]any{}); err == nil {
		t.Fatal("expected nil command error")
	}
	if _, err := srv.callTool("issue.list", map[string]any{"host": map[string]any{"bad": true}}); err == nil {
		t.Fatal("expected invalid persistent flag value error")
	}
	if _, err := srv.callTool("issue.list", map[string]any{"limit": map[string]any{"bad": true}}); err == nil {
		t.Fatal("expected invalid local flag value error")
	}
	if _, err := srv.callTool("issue.view", map[string]any{"issue-id": map[string]any{"bad": true}}); err == nil {
		t.Fatal("expected invalid positional value error")
	}
	if _, err := srv.callTool("issue.create", map[string]any{}); err == nil {
		t.Fatal("expected missing required flag error")
	}
	if _, err := srv.callTool("issue.view", map[string]any{"issue-id": "1", "notes": "not-bool"}); err == nil {
		t.Fatal("expected bool flag parse error")
	}

	srv.tools["args.error"] = mcpToolDef{
		Name: "args.error",
		Factory: func() *cobra.Command {
			return &cobra.Command{
				Use:  "x",
				Args: func(*cobra.Command, []string) error { return errors.New("args error") },
				RunE: func(*cobra.Command, []string) error { return nil },
			}
		},
	}
	if _, err := srv.callTool("args.error", map[string]any{}); err == nil {
		t.Fatal("expected args validation error")
	}
	srv.tools["optional.arg"] = mcpToolDef{
		Name: "optional.arg",
		Factory: func() *cobra.Command {
			return &cobra.Command{
				Use: "x",
				RunE: func(*cobra.Command, []string) error {
					_, _ = os.Stdout.WriteString(`{"ok":true}`)
					return nil
				},
			}
		},
		Args: []mcpArgDef{{Name: "optional", Required: false}},
	}
	if _, err := srv.callTool("optional.arg", map[string]any{}); err != nil {
		t.Fatalf("optional arg should be allowed: %v", err)
	}

	srv.tools["nonjson.output"] = mcpToolDef{
		Name: "nonjson.output",
		Factory: func() *cobra.Command {
			return &cobra.Command{
				Use: "x",
				RunE: func(*cobra.Command, []string) error {
					_, _ = os.Stdout.WriteString("not-json")
					return nil
				},
			}
		},
	}
	if _, err := srv.callTool("nonjson.output", map[string]any{}); err == nil {
		t.Fatal("expected non-json output parse error")
	}
	srv.tools["empty.output"] = mcpToolDef{
		Name: "empty.output",
		Factory: func() *cobra.Command {
			return &cobra.Command{
				Use:  "x",
				RunE: func(*cobra.Command, []string) error { return nil },
			}
		},
	}
	if _, err := srv.callTool("empty.output", map[string]any{}); err != nil {
		t.Fatalf("empty output should be converted to empty object: %v", err)
	}

	if _, err := srv.callTool("issue.list", map[string]any{}); err == nil {
		t.Fatal("expected command runtime error")
	}
	withConfigRuntime(t)
	newHTTPClient = func() *http.Client { return &http.Client{} }
	if _, err := srv.callTool("issue.list", map[string]any{"host": "https://example.invalid", "api-key": "k", "verbose": true, "debug": true}); err == nil {
		t.Fatal("expected runtime http error for verbose/debug path")
	}
}

func TestMCPFramingAndMethodHandling(t *testing.T) {
	srv := &mcpServer{tools: buildMCPTools()}

	initReq := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	listReq := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	input := bytes.NewBuffer(nil)
	if err := writeMCPFrame(input, json.RawMessage(initReq)); err != nil {
		t.Fatalf("write init frame: %v", err)
	}
	if err := writeMCPFrame(input, json.RawMessage(listReq)); err != nil {
		t.Fatalf("write list frame: %v", err)
	}

	output := bytes.NewBuffer(nil)
	if err := srv.serve(input, output); err != nil {
		t.Fatalf("serve: %v", err)
	}

	br := bufio.NewReader(output)
	for i := 0; i < 2; i++ {
		body, err := readMCPFrame(br)
		if err != nil {
			t.Fatalf("readMCPFrame #%d: %v", i, err)
		}
		var msg map[string]any
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Fatalf("unmarshal response #%d: %v", i, err)
		}
		if msg["jsonrpc"] != "2.0" {
			t.Fatalf("unexpected jsonrpc value: %#v", msg)
		}
	}
}

func TestMCPHandleMessageBranches(t *testing.T) {
	srv := &mcpServer{tools: buildMCPTools()}
	if got := srv.handleMessage([]byte("{")); got != nil {
		t.Fatal("invalid json should be ignored")
	}
	if got := srv.handleMessage([]byte(`{"jsonrpc":"2.0","method":"tools/list"}`)); got != nil {
		t.Fatal("notification without id should be ignored")
	}

	unknown := srv.handleMessage([]byte(`{"jsonrpc":"2.0","id":1,"method":"unknown"}`))
	if unknown["error"] == nil {
		t.Fatal("unknown method should return method-not-found error")
	}

	badParams := srv.handleMessage([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"x"}`))
	result := badParams["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatal("invalid tools/call params should return isError")
	}

	callErr := srv.handleMessage([]byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"unknown.tool","arguments":{}}}`))
	result = callErr["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatal("unknown tool call should return isError")
	}
}

func TestReadMCPFrameErrors(t *testing.T) {
	if _, err := readMCPFrame(bufio.NewReader(strings.NewReader("Header: x\r\n\r\n{}"))); err == nil {
		t.Fatal("expected missing content-length error")
	}
	if _, err := readMCPFrame(bufio.NewReader(strings.NewReader("Content-Length: bad\r\n\r\n{}"))); err == nil {
		t.Fatal("expected invalid content-length error")
	}
	if _, err := readMCPFrame(bufio.NewReader(strings.NewReader("Content-Length: 3\r\n\r\n{}"))); err == nil {
		t.Fatal("expected short body read error")
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write error") }

func TestServeAndWriteFrameErrorBranches(t *testing.T) {
	srv := &mcpServer{tools: buildMCPTools()}
	if err := srv.serve(strings.NewReader("Content-Length: bad\r\n\r\n{}"), io.Discard); err == nil {
		t.Fatal("expected serve read frame error")
	}

	input := bytes.NewBuffer(nil)
	if err := writeMCPFrame(input, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	if err := srv.serve(input, errWriter{}); err == nil {
		t.Fatal("expected serve write frame error")
	}
	if err := writeMCPFrame(io.Discard, map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestNewMCPCommandAndServeCommand(t *testing.T) {
	mcpCmd := newMCPCommand()
	if mcpCmd.Use != "mcp" {
		t.Fatalf("unexpected use: %s", mcpCmd.Use)
	}
	serveCmd := newMCPServeCommand()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		_ = r.Close()
		os.Stdin = oldStdin
	})
	if err := serveCmd.RunE(serveCmd, nil); err != nil {
		t.Fatalf("serve command should return nil on EOF: %v", err)
	}
}

func TestHelperCoverage(t *testing.T) {
	for _, tc := range []struct {
		parent string
		leaf   string
	}{
		{parent: "issue", leaf: "list"},
		{parent: "project", leaf: "view"},
		{parent: "user", leaf: "view"},
	} {
		f := childFactory(tc.parent, tc.leaf)
		if got := f(); got == nil {
			t.Fatalf("expected factory command for %s.%s", tc.parent, tc.leaf)
		}
	}
	f := childFactory("unknown", "x")
	if got := f(); got != nil {
		t.Fatal("unknown parent factory should return nil")
	}
	if got := childFactory("issue", "missing")(); got != nil {
		t.Fatal("unknown child should return nil")
	}
	if got := parseUseArgs("list"); len(got) != 0 {
		t.Fatalf("unexpected parseUseArgs result: %#v", got)
	}
	if got := parseUseArgs("list a"); len(got) != 0 {
		t.Fatalf("unexpected parseUseArgs short token result: %#v", got)
	}
	if got := parseUseArgs("list [opt] <id> ???"); len(got) != 2 {
		t.Fatalf("unexpected parseUseArgs result: %#v", got)
	}
	if _, err := stringifyValue("x"); err != nil {
		t.Fatal(err)
	}
	if _, err := stringifyValue(true); err != nil {
		t.Fatal(err)
	}
	if _, err := stringifyValue(false); err != nil {
		t.Fatal(err)
	}
	if _, err := stringifyValue(1); err != nil {
		t.Fatal(err)
	}
	if _, err := stringifyValue(int64(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := stringifyValue(json.Number("3")); err != nil {
		t.Fatal(err)
	}
	if _, err := stringifyValue(1.5); err == nil {
		t.Fatal("expected float non-integer error")
	}
	if _, err := stringifyValue(struct{}{}); err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestCaptureStdoutErrorBranches(t *testing.T) {
	oldPipe := osPipe
	osPipe = func() (*os.File, *os.File, error) { return nil, nil, errors.New("pipe err") }
	if _, err := captureStdout(func() {}); err == nil {
		t.Fatal("expected osPipe error")
	}
	osPipe = oldPipe

	oldReadAll := ioReadAll
	ioReadAll = func(io.Reader) ([]byte, error) { return nil, errors.New("read err") }
	if _, err := captureStdout(func() { _, _ = os.Stdout.WriteString("x") }); err == nil {
		t.Fatal("expected readAll error")
	}
	ioReadAll = oldReadAll

	oldPipe = osPipe
	osPipe = func() (*os.File, *os.File, error) {
		r, w, err := os.Pipe()
		if err != nil {
			return nil, nil, err
		}
		ioReadAll = func(io.Reader) ([]byte, error) { return nil, errors.New("capture error") }
		return r, w, nil
	}
	defer func() {
		osPipe = oldPipe
		ioReadAll = oldReadAll
	}()
	srv := &mcpServer{
		tools: map[string]mcpToolDef{
			"cap.err": {
				Name: "cap.err",
				Factory: func() *cobra.Command {
					return &cobra.Command{
						Use:  "x",
						RunE: func(*cobra.Command, []string) error { return nil },
					}
				},
			},
		},
	}
	if _, err := srv.callTool("cap.err", map[string]any{}); err == nil {
		t.Fatal("expected callTool capture error")
	}
}

func TestServeIgnoresNotificationFrame(t *testing.T) {
	srv := &mcpServer{tools: buildMCPTools()}
	input := bytes.NewBuffer(nil)
	if err := writeMCPFrame(input, map[string]any{"jsonrpc": "2.0", "method": "tools/list"}); err != nil {
		t.Fatal(err)
	}
	out := bytes.NewBuffer(nil)
	if err := srv.serve(input, out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no response for notification, got %d bytes", out.Len())
	}
}

func TestHandleMessageToolCallSuccess(t *testing.T) {
	srv := &mcpServer{
		tools: map[string]mcpToolDef{
			"ok.tool": {
				Name: "ok.tool",
				Factory: func() *cobra.Command {
					return &cobra.Command{
						Use: "ok",
						RunE: func(*cobra.Command, []string) error {
							_, _ = os.Stdout.WriteString(`{"ok":true}`)
							return nil
						},
					}
				},
			},
		},
	}
	resp := srv.handleMessage([]byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"ok.tool","arguments":{}}}`))
	result := resp["result"].(map[string]any)
	if result["structuredContent"] == nil {
		t.Fatalf("expected structuredContent in response: %#v", resp)
	}
}
