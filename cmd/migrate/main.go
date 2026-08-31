package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `Usage: migrate status

Reports the repository migration state. Executable migrations begin with DB-001;
ordinary ThinkPixelMP service startup never changes the database schema.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	switch args[0] {
	case "status":
		fmt.Fprintln(stdout, "database migrations: no executable migrations (DB-001 pending)")
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
	default:
		fmt.Fprintf(stderr, "unknown migration command %q\n\n%s\n", args[0], usage)
		return 2
	}
	return 0
}
