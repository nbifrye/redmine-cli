package cmd

import (
	"net/http"

	"github.com/spf13/cobra"
)

func newProjectCommand() *cobra.Command {
	projectCmd := &cobra.Command{Use: "project", Short: "Project commands"}
	projectCmd.AddCommand(newProjectListCommand())
	projectCmd.AddCommand(newProjectViewCommand())
	return projectCmd
}

func newProjectListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/projects.json"})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}

func newProjectViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view <project-id-or-identifier>",
		Short: "View project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodGet, Path: "/projects/" + args[0] + ".json"})
			return handleRequestResult(raw, code, reqErr)
		},
	}
}
