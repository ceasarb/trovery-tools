package main

import (
	"os"

	"github.com/ceasarb/demigo-tools/internal/forge/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
