package main

import (
	"os"

	"github.com/ceasarb/trovery-tools/internal/forge/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
