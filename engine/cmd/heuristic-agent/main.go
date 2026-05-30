package main

import (
	"fmt"
	"os"

	"github.com/RobertGumeny/agent-poker/internal/heuristicagent"
)

func main() {
	if err := heuristicagent.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
