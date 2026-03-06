package main

import (
	"fmt"
	"os"

	"redmine-cli/cmd"
)

var (
	execute = cmd.Execute
	osExit  = os.Exit
)

func run() int {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func main() {
	osExit(run())
}
