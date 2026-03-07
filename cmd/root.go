package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	hostFlag   string
	apiKeyFlag string
	verbose    bool
	debug      bool
	exitFunc   = os.Exit
)

var rootCmd = &cobra.Command{
	Use:   "redmine",
	Short: "Redmine CLI",
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hostFlag, "host", "", "Redmine host URL")
	rootCmd.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "Redmine API key")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show request/response summary")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Show HTTP details")

	rootCmd.AddCommand(newAuthCommand())
	rootCmd.AddCommand(newIssueCommand())
	rootCmd.AddCommand(newProjectCommand())
	rootCmd.AddCommand(newUserCommand())
	rootCmd.AddCommand(newAPICommand())
}

func mustRuntime() (*Runtime, error) {
	return LoadRuntime(hostFlag, apiKeyFlag, verbose, debug)
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(b))
	return nil
}

func handleRequestResult(raw json.RawMessage, exitCode int, err error) error {
	if err != nil {
		if exitCode > 0 {
			// exitFunc is os.Exit in production (process terminates here) and
			// overridden in tests to capture the exit code without terminating.
			exitFunc(exitCode)
		}
		return err
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	return printJSON(parsed)
}
