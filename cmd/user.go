package cmd

import (
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
)

func newUserCommand() *cobra.Command {
	userCmd := &cobra.Command{Use: "user", Short: "User commands"}
	userCmd.AddCommand(newUserListCommand())
	userCmd.AddCommand(newUserViewCommand())
	return userCmd
}

func newUserListCommand() *cobra.Command {
	var status string
	var name string
	var groupID string
	var offset, limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			query := map[string]string{
				"status":   status,
				"name":     name,
				"group_id": groupID,
				"offset":   strconv.Itoa(offset),
				"limit":    strconv.Itoa(limit),
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/users.json", Query: query})
			return handleRequestResult(raw, code, reqErr)
		},
	}

	cmd.Flags().StringVar(&status, "status", "1", "User status (1: active, 2: registered, 3: locked)")
	cmd.Flags().StringVar(&name, "name", "", "Filter by user name")
	cmd.Flags().StringVar(&groupID, "group-id", "", "Filter by group id")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of items to skip")
	cmd.Flags().IntVar(&limit, "limit", 25, "Number of items per page")
	return cmd
}

func newUserViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view <user-id>",
		Short: "View user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNumericID(args[0]); err != nil {
				return err
			}
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/users/" + args[0] + ".json"})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}
