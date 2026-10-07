package main

import (
	"fmt"
	"os"
)

func quarantine(_ []string) int {
	fmt.Fprintln(os.Stderr, "pipelineiq quarantine is not implemented until P3")
	return 0
}
