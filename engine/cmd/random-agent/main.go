package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/RobertGumeny/agent-poker/internal/randomagent"
)

func main() {
	seed := time.Now().UnixNano()
	if err := randomagent.Run(os.Stdin, os.Stdout, rand.New(rand.NewSource(seed))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
