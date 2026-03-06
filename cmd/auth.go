package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	authCmd := &cobra.Command{Use: "auth", Short: "Authentication commands"}
	authCmd.AddCommand(newAuthLoginCommand())
	authCmd.AddCommand(newAuthStatusCommand())
	return authCmd
}

func newAuthLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Login and save host/API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)
			fmt.Fprint(os.Stdout, "Host URL: ")
			host, _ := reader.ReadString('\n')
			host = strings.TrimSpace(host)
			if host == "" {
				return errors.New("host is required")
			}
			fmt.Fprint(os.Stdout, "API key: ")
			apiKey, _ := reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)
			if apiKey == "" {
				return errors.New("API key is required")
			}

			r := newRuntime(host, apiKey, false, false)
			raw, code, err := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/users/current.json"})
			if err != nil {
				if code > 0 {
					exitFunc(code)
				}
				return err
			}

			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			cfg.DefaultHost = host
			cfg.Hosts[host] = HostConfig{APIKey: apiKey}
			if err := SaveConfig(cfg); err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "Login successful")
			return handleRequestResult(raw, 0, nil)
		},
	}
}

func newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/users/current.json"})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}
