package main

import (
	"os"

	"github.com/graphaelli/loadbeat/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
