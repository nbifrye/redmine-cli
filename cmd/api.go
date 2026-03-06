package cmd

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newAPICommand() *cobra.Command {
	apiCmd := &cobra.Command{Use: "api", Short: "Generic API command"}
	apiCmd.AddCommand(newAPIGetCommand())
	apiCmd.AddCommand(newAPIPostCommand())
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

func readBodyArg(v string) ([]byte, error) {
	if v == "" {
		return nil, errors.New("--body is required")
	}
	if strings.HasPrefix(v, "@") {
		return os.ReadFile(strings.TrimPrefix(v, "@"))
	}
	return []byte(v), nil
}
