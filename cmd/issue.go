package cmd

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
)

func newIssueCommand() *cobra.Command {
	issueCmd := &cobra.Command{Use: "issue", Short: "Issue commands"}
	issueCmd.AddCommand(newIssueListCommand())
	issueCmd.AddCommand(newIssueViewCommand())
	issueCmd.AddCommand(newIssueCreateCommand())
	issueCmd.AddCommand(newIssueUpdateCommand())
	issueCmd.AddCommand(newIssueCloseCommand())
	issueCmd.AddCommand(newIssueNoteAddCommand())
	return issueCmd
}

func newIssueListCommand() *cobra.Command {
	var project, status, assignedTo string
	var offset, limit int
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
				"project_id":     project,
				"status_id":      status,
				"assigned_to_id": assignedTo,
				"offset":         strconv.Itoa(offset),
				"limit":          strconv.Itoa(limit),
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
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of items to skip")
	cmd.Flags().IntVar(&limit, "limit", 25, "Number of items per page")
	cmd.Flags().BoolVar(&all, "all", false, "Fetch up to 100 issues, ignoring --offset and --limit")
	return cmd
}

func newIssueViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view <issue-id>",
		Short: "View issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNumericID(args[0]); err != nil {
				return err
			}
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
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project identifier or id")
	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("subject")
	return cmd
}

func newIssueUpdateCommand() *cobra.Command {
	var subject, description, statusID, assignedToID string

	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "Update issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNumericID(args[0]); err != nil {
				return err
			}
			r, err := mustRuntime()
			if err != nil {
				return err
			}

			issue := map[string]any{}
			if subject != "" {
				issue["subject"] = subject
			}
			if description != "" {
				issue["description"] = description
			}
			if statusID != "" {
				issue["status_id"] = statusID
			}
			if assignedToID != "" {
				issue["assigned_to_id"] = assignedToID
			}

			if len(issue) == 0 {
				return errors.New("at least one field must be provided (--subject, --description, --status-id, --assigned-to-id)")
			}

			payload := map[string]any{"issue": issue}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPut, Path: "/issues/" + args[0] + ".json", Body: payload})
			return handleRequestResult(raw, code, reqErr)
		},
	}

	cmd.Flags().StringVar(&subject, "subject", "", "New issue subject")
	cmd.Flags().StringVar(&description, "description", "", "New issue description")
	cmd.Flags().StringVar(&statusID, "status-id", "", "New status id")
	cmd.Flags().StringVar(&assignedToID, "assigned-to-id", "", "New assignee user id")
	return cmd
}

func newIssueCloseCommand() *cobra.Command {
	var closedStatusID string
	cmd := &cobra.Command{
		Use:   "close <issue-id>",
		Short: "Close issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNumericID(args[0]); err != nil {
				return err
			}
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			payload := map[string]any{"issue": map[string]any{"status_id": closedStatusID}}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPut, Path: "/issues/" + args[0] + ".json", Body: payload})
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVar(&closedStatusID, "status-id", "5", "Status id used for closed issues")
	return cmd
}

func newIssueNoteAddCommand() *cobra.Command {
	var notes string
	cmd := &cobra.Command{
		Use:   "note-add <issue-id>",
		Short: "Add note to issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNumericID(args[0]); err != nil {
				return err
			}
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			payload := map[string]any{"issue": map[string]any{"notes": notes}}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPut, Path: "/issues/" + args[0] + ".json", Body: payload})
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Note body")
	_ = cmd.MarkFlagRequired("notes")
	return cmd
}
