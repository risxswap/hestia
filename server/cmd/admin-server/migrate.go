package main

import (
	"fmt"
	"io"
)

func runAdminCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		return runAdminServer()
	}
	if args[0] == "migrate" {
		return runMigrateCommand(args[1:], out)
	}
	return fmt.Errorf("unsupported admin command %q", args[0])
}

func runMigrateCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing migrate command")
	}

	switch args[0] {
	case "up", "version":
		_, err := fmt.Fprintf(out, "migrate %s command is wired\n", args[0])
		return err
	case "down":
		_, err := fmt.Fprintln(out, "migrate down command is wired")
		return err
	default:
		return fmt.Errorf("unsupported migrate command %q", args[0])
	}
}
