// Command pipelineiq is the agent CLI. Jenkins calls report after a build.
// quarantine is a stub until P3.
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: pipelineiq report | quarantine")
		return 2
	}
	switch args[0] {
	case "report":
		return report(args[1:])
	case "quarantine":
		return quarantine(args[1:])
	case "-h", "--help", "help":
		fmt.Fprintln(os.Stderr, "usage: pipelineiq report | quarantine")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		return 2
	}
}
