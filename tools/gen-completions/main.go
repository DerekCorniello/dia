package main

import (
	"os"

	"github.com/DerekCorniello/dia/internal/cli"
)

func main() {
	if err := cli.GenerateCompletions("completions"); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
