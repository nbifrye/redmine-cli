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
	var page, perPage int

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
				"page":     strconv.Itoa(page),
				"limit":    strconv.Itoa(perPage),
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/users.json", Query: query})
			return handleRequestResult(raw, code, reqErr)
		},
	}

	cmd.Flags().StringVar(&status, "status", "1", "User status (1: active, 2: registered, 3: locked)")
	cmd.Flags().StringVar(&name, "name", "", "Filter by user name")
	cmd.Flags().StringVar(&groupID, "group-id", "", "Filter by group id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")
	return cmd
}

func newUserViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view <user-id>",
		Short: "View user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/users/" + args[0] + ".json"})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}
