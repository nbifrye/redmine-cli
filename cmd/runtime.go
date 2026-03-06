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
	"strings"

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

func LoadRuntime(hostFlag, apiKeyFlag string, verbose, debug bool) (*Runtime, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	host := firstNonEmpty(hostFlag, os.Getenv("REDMINE_HOST"), cfg.DefaultHost)
	if host == "" {
		return nil, errors.New("host is not configured; run `redmine auth login` or set REDMINE_HOST")
	}

	apiKey := firstNonEmpty(apiKeyFlag, os.Getenv("REDMINE_API_KEY"), cfg.Hosts[host].APIKey)
	if apiKey == "" {
		return nil, errors.New("API key is not configured; run `redmine auth login` or set REDMINE_API_KEY")
	}

	return &Runtime{Host: strings.TrimRight(host, "/"), APIKey: apiKey, Verbose: verbose, Debug: debug, Client: &http.Client{}}, nil
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
			fmt.Fprintf(os.Stderr, "> %s: %s\n", k, strings.Join(values, ","))
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
		return "authentication failed (401): APIキーを確認し、`redmine auth login` を再実行してください" + suffix
	case 404:
		return "resource not found (404): project/issue ID やパスを確認してください" + suffix
	default:
		return fmt.Sprintf("request failed (%d)%s", code, suffix)
	}
}
