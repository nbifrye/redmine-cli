package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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

func TestReadMCPFrameErrors(t *testing.T) {
	if _, err := readMCPFrame(bufio.NewReader(strings.NewReader("Header: x\r\n\r\n{}"))); err == nil {
		t.Fatal("expected missing content-length error")
	}
	if _, err := readMCPFrame(bufio.NewReader(strings.NewReader("Content-Length: bad\r\n\r\n{}"))); err == nil {
		t.Fatal("expected invalid content-length error")
	}
}
