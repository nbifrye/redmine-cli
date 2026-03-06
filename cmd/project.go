package cmd

import (
	"net/http"

	"github.com/spf13/cobra"
)

func newProjectCommand() *cobra.Command {
	projectCmd := &cobra.Command{Use: "project", Short: "Project commands"}
	projectCmd.AddCommand(newProjectListCommand())
	projectCmd.AddCommand(newProjectViewCommand())
	projectCmd.AddCommand(newProjectCreateCommand())
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

func newProjectCreateCommand() *cobra.Command {
	var identifier, name, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create project",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := mustRuntime()
			if err != nil {
				return err
			}

			payload := map[string]any{"project": map[string]any{"identifier": identifier, "name": name, "description": description}}
			raw, code, reqErr := r.DoJSON(RequestOptions{Method: http.MethodPost, Path: "/projects.json", Body: payload})
			return handleRequestResult(raw, code, reqErr)
		},
	}
	cmd.Flags().StringVar(&identifier, "identifier", "", "Project identifier")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	_ = cmd.MarkFlagRequired("identifier")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
