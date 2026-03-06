package main

import (
	"errors"
	"testing"
)

func TestRunAndMain(t *testing.T) {
	old := execute
	oldExit := osExit
	t.Cleanup(func() { execute = old; osExit = oldExit })

	execute = func() error { return nil }
	if got := run(); got != 0 {
		t.Fatalf("run success=%d", got)
	}

	execute = func() error { return errors.New("boom") }
	if got := run(); got != 1 {
		t.Fatalf("run fail=%d", got)
	}

	exited := -1
	osExit = func(code int) { exited = code }
	execute = func() error { return nil }
	main()
	if exited != 0 {
		t.Fatalf("main exit=%d", exited)
	}
	execute = func() error { return errors.New("x") }
	main()
	if exited != 1 {
		t.Fatalf("main exit fail=%d", exited)
	}
}
