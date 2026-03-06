package cmd

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
)

func newTimeEntryCommand() *cobra.Command {
	timeEntryCmd := &cobra.Command{Use: "time-entry", Short: "Time entry commands"}
	timeEntryCmd.AddCommand(newTimeEntryListCommand())
	timeEntryCmd.AddCommand(newTimeEntryCreateCommand())
	return timeEntryCmd
}

func newTimeEntryListCommand() *cobra.Command {
	var issueID, projectID, userID, from, to string
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List time entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			query := map[string]string{
				"issue_id":   issueID,
				"project_id": projectID,
				"user_id":    userID,
				"from":       from,
				"to":         to,
				"page":       strconv.Itoa(page),
				"limit":      strconv.Itoa(perPage),
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/time_entries.json", Query: query})
			return handleRequestResult(raw, code, reqErr)
		},
	}

	cmd.Flags().StringVar(&issueID, "issue-id", "", "Filter by issue id")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by project id")
	cmd.Flags().StringVar(&userID, "user-id", "", "Filter by user id (e.g. me)")
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")
	return cmd
}

func newTimeEntryCreateCommand() *cobra.Command {
	var issueID, projectID, activityID, spentOn, comments string
	var hours float64

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if issueID == "" && projectID == "" {
				return errors.New("either --issue-id or --project-id is required")
			}
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			entry := map[string]any{
				"hours":       hours,
				"activity_id": activityID,
				"spent_on":    spentOn,
				"comments":    comments,
			}
			if issueID != "" {
				entry["issue_id"] = issueID
			}
			if projectID != "" {
				entry["project_id"] = projectID
			}
			payload := map[string]any{"time_entry": entry}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/time_entries.json", Body: payload})
			return handleRequestResult(raw, code, reqErr)
		},
	}

	cmd.Flags().StringVar(&issueID, "issue-id", "", "Issue id")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project id (required if --issue-id is not set)")
	cmd.Flags().Float64Var(&hours, "hours", 0, "Spent hours")
	cmd.Flags().StringVar(&activityID, "activity-id", "", "Activity id")
	cmd.Flags().StringVar(&spentOn, "spent-on", "", "Spent date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&comments, "comments", "", "Comments")
	_ = cmd.MarkFlagRequired("hours")
	_ = cmd.MarkFlagRequired("activity-id")
	_ = cmd.MarkFlagRequired("spent-on")
	return cmd
}
