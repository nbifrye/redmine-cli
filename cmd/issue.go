package cmd

import (
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newIssueCommand() *cobra.Command {
	issueCmd := &cobra.Command{Use: "issue", Short: "Issue commands"}
	issueCmd.AddCommand(newIssueListCommand())
	issueCmd.AddCommand(newIssueViewCommand())
	issueCmd.AddCommand(newIssueCreateCommand())
	return issueCmd
}

func newIssueListCommand() *cobra.Command {
	var project, status, assignedTo string
	var page, perPage int
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			query := map[string]string{
				"project_id":  project,
				"status_id":   status,
				"assigned_to": assignedTo,
				"page":        strconv.Itoa(page),
				"limit":       strconv.Itoa(perPage),
			}
			if all {
				query["limit"] = "100"
				query["offset"] = "0"
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/issues.json", Query: query})
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Filter by project identifier")
	cmd.Flags().StringVarP(&status, "status", "s", "open", "Filter by status id (default: open)")
	cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "Filter by assignee (e.g. me)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")
	cmd.Flags().BoolVar(&all, "all", false, "Fetch all pages (best effort)")
	return cmd
}

func newIssueViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view <issue-id>",
		Short: "View issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/issues/" + args[0] + ".json"})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}

func newIssueCreateCommand() *cobra.Command {
	var project, subject, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			payload := map[string]any{"issue": map[string]any{"project_id": project, "subject": subject, "description": description}}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/issues.json", Body: payload})
			if reqErr != nil {
				if code > 0 {
					os.Exit(code)
				}
				return reqErr
			}
			return handleRequestResult(raw, code, nil)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project identifier or id")
	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("subject")
	return cmd
}
