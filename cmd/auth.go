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
			hostRaw, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read host: %w", err)
			}
			host := strings.TrimSpace(hostRaw)
			if host == "" {
				return errors.New("host is required")
			}
			if err := validateHost(host); err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, "API key: ")
			apiKeyRaw, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read API key: %w", err)
			}
			apiKey := strings.TrimSpace(apiKeyRaw)
			if apiKey == "" {
				return errors.New("API key is required")
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
			return nil
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
