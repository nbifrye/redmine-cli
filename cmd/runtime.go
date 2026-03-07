package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type HostConfig struct {
	APIKey string `yaml:"api_key"`
}

type Config struct {
	DefaultHost string                `yaml:"default_host"`
	Hosts       map[string]HostConfig `yaml:"hosts"`
}

type Runtime struct {
	Host    string
	APIKey  string
	Verbose bool
	Debug   bool
	Client  *http.Client
}

var (
	userHomeDir   = os.UserHomeDir
	osReadFile    = os.ReadFile
	osMkdirAll    = os.MkdirAll
	osWriteFile   = os.WriteFile
	yamlMarshal   = yaml.Marshal
	yamlUnmarshal = yaml.Unmarshal
)

type RequestOptions struct {
	Method      string
	Path        string
	Query       map[string]string
	Body        any
	RawBodyJSON []byte
}

// newHTTPClient is the factory for creating HTTP clients.
// Replaced in tests to inject TLS-capable clients for httptest.NewTLSServer.
var newHTTPClient = func() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// newRuntime constructs a Runtime with the standard HTTP client timeout.
// Use this instead of constructing Runtime directly to ensure consistent configuration.
func newRuntime(host, apiKey string, verbose, debug bool) *Runtime {
	return &Runtime{
		Host:    strings.TrimRight(host, "/"),
		APIKey:  apiKey,
		Verbose: verbose,
		Debug:   debug,
		Client:  newHTTPClient(),
	}
}

func LoadRuntime(hostFlag, apiKeyFlag string, verbose, debug bool) (*Runtime, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	host := firstNonEmpty(hostFlag, os.Getenv("REDMINE_HOST"), cfg.DefaultHost)
	if host == "" {
		return nil, errors.New("host is not configured; run `redmine auth login` or set REDMINE_HOST")
	}
	if err := validateHost(host); err != nil {
		return nil, err
	}

	apiKey := firstNonEmpty(apiKeyFlag, os.Getenv("REDMINE_API_KEY"), cfg.Hosts[host].APIKey)
	if apiKey == "" {
		return nil, errors.New("API key is not configured; run `redmine auth login` or set REDMINE_API_KEY")
	}

	return newRuntime(host, apiKey, verbose, debug), nil
}

func (r *Runtime) DoJSON(opts RequestOptions) (json.RawMessage, int, error) {
	targetURL, err := url.Parse(r.Host + normalizePath(opts.Path))
	if err != nil {
		return nil, 0, err
	}

	q := targetURL.Query()
	for k, v := range opts.Query {
		if v != "" {
			q.Set(k, v)
		}
	}
	targetURL.RawQuery = q.Encode()

	var bodyReader io.Reader
	if len(opts.RawBodyJSON) > 0 {
		bodyReader = bytes.NewReader(opts.RawBodyJSON)
	} else if opts.Body != nil {
		payload, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(opts.Method, targetURL.String(), bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Redmine-API-Key", r.APIKey)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if r.Verbose || r.Debug {
		fmt.Fprintf(os.Stderr, "> %s %s\n", req.Method, req.URL.String())
	}
	if r.Debug {
		for k, values := range req.Header {
			val := strings.Join(values, ",")
			if strings.EqualFold(k, "X-Redmine-API-Key") {
				val = "[REDACTED]"
			}
			fmt.Fprintf(os.Stderr, "> %s: %s\n", k, val)
		}
	}

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, 2, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 2, err
	}

	if r.Verbose || r.Debug {
		fmt.Fprintf(os.Stderr, "< status: %d\n", resp.StatusCode)
	}
	if r.Debug {
		keys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "< %s: %s\n", k, strings.Join(resp.Header[k], ","))
		}
	}

	if resp.StatusCode >= 400 {
		return nil, mapStatus(resp.StatusCode), fmt.Errorf("%s", buildStatusMessage(resp.StatusCode, strings.TrimSpace(string(respBody))))
	}

	if len(bytes.TrimSpace(respBody)) == 0 {
		return json.RawMessage("{}"), 0, nil
	}

	if !json.Valid(respBody) {
		wrapped, _ := json.Marshal(map[string]string{"raw": string(respBody)})
		return wrapped, 0, nil
	}

	return respBody, 0, nil
}

func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Hosts: map[string]HostConfig{}}

	b, err := osReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yamlUnmarshal(b, cfg); err != nil {
		return nil, err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]HostConfig{}
	}
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]HostConfig{}
	}
	if err := osMkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yamlMarshal(cfg)
	if err != nil {
		return err
	}
	if err := osWriteFile(path, b, 0o600); err != nil {
		return err
	}
	return nil
}

func configPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "redmine-cli", "config.yml"), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func mapStatus(code int) int {
	if code >= 500 {
		return 2
	}
	return 1
}

func buildStatusMessage(code int, body string) string {
	suffix := ""
	if body != "" {
		suffix = ": " + body
	}
	switch code {
	case 401:
		return "authentication failed (401): check your API key and re-run `redmine auth login`" + suffix
	case 404:
		return "resource not found (404): check the project/issue ID or path" + suffix
	default:
		return fmt.Sprintf("request failed (%d)%s", code, suffix)
	}
}

// validateHost returns an error if host is not a valid https:// URL.
func validateHost(host string) error {
	u, err := url.Parse(host)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("invalid host URL %q: must start with https://", host)
	}
	return nil
}

// validateNumericID returns an error if id is not a positive integer.
// Use for Redmine resources that only accept numeric IDs (issues, users).
func validateNumericID(id string) error {
	n, err := strconv.Atoi(id)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid ID %q: must be a positive integer", id)
	}
	return nil
}

// validateProjectIdentifier returns an error if id contains path separators
// or traversal sequences. Project identifiers may be numeric or alphanumeric strings.
func validateProjectIdentifier(id string) error {
	if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid project ID %q: must not contain path separators or traversal sequences", id)
	}
	return nil
}
