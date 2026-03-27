package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type mcpToolDef struct {
	Name        string
	Description string
	Path        []string
	Factory     func() *cobra.Command
	Args        []mcpArgDef
	Flags       []mcpFlagDef
}

type mcpArgDef struct {
	Name     string
	Required bool
}

type mcpFlagDef struct {
	Name        string
	Description string
	Type        string
	Required    bool
	Persistent  bool
}

type mcpToolResult struct {
	StructuredContent any
	Text              string
}

type mcpServer struct {
	tools map[string]mcpToolDef
}

func newMCPCommand() *cobra.Command {
	mcpCmd := &cobra.Command{Use: "mcp", Short: "MCP commands"}
	mcpCmd.AddCommand(newMCPServeCommand())
	return mcpCmd
}

func newMCPServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Serve CLI commands as MCP tools over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := &mcpServer{tools: buildMCPTools()}
			return srv.serve(os.Stdin, os.Stdout)
		},
	}
}

func buildMCPTools() map[string]mcpToolDef {
	persistent := persistentFlagDefs()
	resources := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "issue", cmd: newIssueCommand()},
		{name: "project", cmd: newProjectCommand()},
		{name: "user", cmd: newUserCommand()},
	}

	out := map[string]mcpToolDef{}
	for _, res := range resources {
		for _, child := range res.cmd.Commands() {
			leafName := strings.Fields(child.Use)[0]
			toolName := res.name + "." + leafName
			def := mcpToolDef{
				Name:        toolName,
				Description: child.Short,
				Path:        []string{res.name, leafName},
				Factory:     childFactory(res.name, leafName),
				Args:        parseUseArgs(child.Use),
				Flags:       append(append([]mcpFlagDef{}, persistent...), localFlagDefs(child)...),
			}
			out[toolName] = def
		}
	}
	return out
}

func childFactory(parent, leaf string) func() *cobra.Command {
	return func() *cobra.Command {
		var parentCmd *cobra.Command
		switch parent {
		case "issue":
			parentCmd = newIssueCommand()
		case "project":
			parentCmd = newProjectCommand()
		case "user":
			parentCmd = newUserCommand()
		default:
			return nil
		}
		for _, c := range parentCmd.Commands() {
			if strings.Fields(c.Use)[0] == leaf {
				return c
			}
		}
		return nil
	}
}

func persistentFlagDefs() []mcpFlagDef {
	defs := []mcpFlagDef{}
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		defs = append(defs, toMCPFlag(f, true))
	})
	return defs
}

func localFlagDefs(cmd *cobra.Command) []mcpFlagDef {
	defs := []mcpFlagDef{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		defs = append(defs, toMCPFlag(f, false))
	})
	return defs
}

func toMCPFlag(f *pflag.Flag, persistent bool) mcpFlagDef {
	required := false
	if vals, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && len(vals) > 0 && vals[0] == "true" {
		required = true
	}
	return mcpFlagDef{
		Name:        f.Name,
		Description: f.Usage,
		Type:        f.Value.Type(),
		Required:    required,
		Persistent:  persistent,
	}
}

func parseUseArgs(use string) []mcpArgDef {
	parts := strings.Fields(use)
	out := make([]mcpArgDef, 0, len(parts))
	for _, p := range parts[1:] {
		if len(p) < 3 {
			continue
		}
		switch {
		case strings.HasPrefix(p, "<") && strings.HasSuffix(p, ">"):
			out = append(out, mcpArgDef{Name: p[1 : len(p)-1], Required: true})
		case strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]"):
			out = append(out, mcpArgDef{Name: p[1 : len(p)-1], Required: false})
		}
	}
	return out
}

func (d mcpToolDef) inputSchema() map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, a := range d.Args {
		properties[a.Name] = map[string]any{"type": "string", "description": "Positional argument"}
		if a.Required {
			required = append(required, a.Name)
		}
	}
	for _, f := range d.Flags {
		prop := map[string]any{"description": f.Description}
		switch f.Type {
		case "bool":
			prop["type"] = "boolean"
		case "int", "int64", "int32":
			prop["type"] = "integer"
		default:
			prop["type"] = "string"
		}
		properties[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}

	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func (s *mcpServer) callTool(name string, arguments map[string]any) (mcpToolResult, error) {
	def, ok := s.tools[name]
	if !ok {
		return mcpToolResult{}, fmt.Errorf("unknown tool: %s", name)
	}
	if def.Factory == nil {
		return mcpToolResult{}, errors.New("tool factory is not available")
	}

	allowed := map[string]struct{}{}
	for _, a := range def.Args {
		allowed[a.Name] = struct{}{}
	}
	for _, f := range def.Flags {
		allowed[f.Name] = struct{}{}
	}
	for k := range arguments {
		if _, ok := allowed[k]; !ok {
			return mcpToolResult{}, fmt.Errorf("unknown argument: %s", k)
		}
	}

	resetRuntimeFlags()
	for _, f := range def.Flags {
		if !f.Persistent {
			continue
		}
		v, ok := arguments[f.Name]
		if !ok {
			continue
		}
		s, err := stringifyValue(v)
		if err != nil {
			return mcpToolResult{}, fmt.Errorf("invalid value for %s: %w", f.Name, err)
		}
		switch f.Name {
		case "host":
			hostFlag = s
		case "api-key":
			apiKeyFlag = s
		case "verbose":
			verbose = strings.EqualFold(s, "true")
		case "debug":
			debug = strings.EqualFold(s, "true")
		}
	}

	cmd := def.Factory()
	if cmd == nil {
		return mcpToolResult{}, errors.New("failed to create command")
	}

	pos := make([]string, 0, len(def.Args))
	for _, a := range def.Args {
		v, ok := arguments[a.Name]
		if !ok {
			if a.Required {
				return mcpToolResult{}, fmt.Errorf("missing required argument: %s", a.Name)
			}
			continue
		}
		s, err := stringifyValue(v)
		if err != nil {
			return mcpToolResult{}, fmt.Errorf("invalid value for %s: %w", a.Name, err)
		}
		pos = append(pos, s)
	}

	for _, f := range def.Flags {
		if f.Persistent {
			continue
		}
		v, ok := arguments[f.Name]
		if !ok {
			if f.Required {
				return mcpToolResult{}, fmt.Errorf("missing required argument: %s", f.Name)
			}
			continue
		}
		s, err := stringifyValue(v)
		if err != nil {
			return mcpToolResult{}, fmt.Errorf("invalid value for %s: %w", f.Name, err)
		}
		if err := cmd.Flags().Set(f.Name, s); err != nil {
			return mcpToolResult{}, err
		}
	}

	if cmd.Args != nil {
		if err := cmd.Args(cmd, pos); err != nil {
			return mcpToolResult{}, err
		}
	}

	var runErr error
	out, capErr := captureStdout(func() {
		oldExit := exitFunc
		exitFunc = func(int) {}
		defer func() { exitFunc = oldExit }()
		runErr = cmd.RunE(cmd, pos)
	})
	if capErr != nil {
		return mcpToolResult{}, capErr
	}
	if runErr != nil {
		return mcpToolResult{}, runErr
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		trimmed = "{}"
	}
	var structured any
	if err := json.Unmarshal([]byte(trimmed), &structured); err != nil {
		return mcpToolResult{}, fmt.Errorf("failed to parse command JSON output: %w", err)
	}
	pretty, _ := json.MarshalIndent(structured, "", "  ")
	return mcpToolResult{StructuredContent: structured, Text: string(pretty)}, nil
}

func resetRuntimeFlags() {
	hostFlag = ""
	apiKeyFlag = ""
	verbose = false
	debug = false
}

func stringifyValue(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case float64:
		if x != float64(int64(x)) {
			return "", errors.New("must be an integer value")
		}
		return strconv.FormatInt(int64(x), 10), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case json.Number:
		return x.String(), nil
	default:
		return "", fmt.Errorf("unsupported type %T", v)
	}
}

func captureStdout(fn func()) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	done := make(chan []byte, 1)
	errC := make(chan error, 1)
	go func() {
		b, e := io.ReadAll(r)
		if e != nil {
			errC <- e
			return
		}
		done <- b
	}()

	fn()
	_ = w.Close()
	os.Stdout = old

	select {
	case e := <-errC:
		_ = r.Close()
		return "", e
	case b := <-done:
		_ = r.Close()
		return string(b), nil
	}
}

func (s *mcpServer) serve(in io.Reader, out io.Writer) error {
	br := bufio.NewReader(in)
	for {
		body, err := readMCPFrame(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		resp := s.handleMessage(body)
		if resp == nil {
			continue
		}
		if err := writeMCPFrame(out, resp); err != nil {
			return err
		}
	}
}

func (s *mcpServer) handleMessage(body []byte) map[string]any {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	if len(req.ID) == 0 {
		return nil
	}

	switch req.Method {
	case "initialize":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(req.ID),
			"result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "redmine-cli", "version": rootCmd.Version},
			},
		}
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.tools))
		names := make([]string, 0, len(s.tools))
		for k := range s.tools {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, name := range names {
			d := s.tools[name]
			tools = append(tools, map[string]any{
				"name":        d.Name,
				"description": d.Description,
				"inputSchema": d.inputSchema(),
			})
		}
		return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID), "result": map[string]any{"tools": tools}}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID), "result": map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": err.Error()}}}}
		}
		res, err := s.callTool(p.Name, p.Arguments)
		if err != nil {
			return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID), "result": map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": err.Error()}}}}
		}
		return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID), "result": map[string]any{"content": []map[string]any{{"type": "text", "text": res.Text}}, "structuredContent": res.StructuredContent}}
	default:
		return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID), "error": map[string]any{"code": -32601, "message": "method not found"}}
	}
}

func readMCPFrame(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trim := strings.TrimSpace(line)
		if trim == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(trim), "content-length:") {
			v := strings.TrimSpace(trim[len("content-length:"):])
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return nil, errors.New("invalid content-length")
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, errors.New("missing content-length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeMCPFrame(w io.Writer, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var frame bytes.Buffer
	_, _ = fmt.Fprintf(&frame, "Content-Length: %d\r\n\r\n", len(b))
	_, _ = frame.Write(b)
	_, err = w.Write(frame.Bytes())
	return err
}
