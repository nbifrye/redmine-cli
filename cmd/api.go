package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const maxBodyFileSize = 10 * 1024 * 1024 // 10 MB

func newAPICommand() *cobra.Command {
	apiCmd := &cobra.Command{Use: "api", Short: "Generic API command"}
	apiCmd.AddCommand(newAPIGetCommand())
	apiCmd.AddCommand(newAPIPostCommand())
	apiCmd.AddCommand(newAPIPutCommand())
	apiCmd.AddCommand(newAPIDeleteCommand())
	return apiCmd
}

func newAPIGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <path>",
		Short: "GET endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: args[0]})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}

func newAPIPostCommand() *cobra.Command {
	var bodyArg string
	cmd := &cobra.Command{
		Use:   "post <path>",
		Short: "POST endpoint with JSON body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			body, err := readBodyArg(bodyArg)
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: args[0], RawBodyJSON: body})
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVar(&bodyArg, "body", "", "JSON string or @file.json")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newAPIPutCommand() *cobra.Command {
	var bodyArg string
	cmd := &cobra.Command{
		Use:   "put <path>",
		Short: "PUT endpoint with JSON body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			body, err := readBodyArg(bodyArg)
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPut, Path: args[0], RawBodyJSON: body})
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVar(&bodyArg, "body", "", "JSON string or @file.json")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newAPIDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <path>",
		Short: "DELETE endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodDelete, Path: args[0]})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}

func readBodyArg(v string) ([]byte, error) {
	if v == "" {
		return nil, errors.New("--body is required")
	}
	if strings.HasPrefix(v, "@") {
		path := strings.TrimPrefix(v, "@")
		fi, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if fi.Size() > maxBodyFileSize {
			return nil, fmt.Errorf("body file %q exceeds maximum allowed size of 10MB", path)
		}
		return os.ReadFile(path)
	}
	return []byte(v), nil
}
