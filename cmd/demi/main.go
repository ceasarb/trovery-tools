// Command demi is the Demigo dispatcher. `demi forge …` and `demi vigil …`
// exec the standalone demi-forge / demi-vigil binaries, keeping the two
// products independently releasable (ADR-004) while presenting one brand verb.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

// version is injected at build time via -ldflags "-X main.version=…".
var version = "dev"

const usage = `Demigo — in tandem with AI. 🦉

Usage:
  demi forge <args…>   Build your AI — agents, MCP servers, skills, evals, deploy   (demi-forge)
  demi vigil <args…>   Govern AI-assisted work — sessions, policy, audit           (demi-vigil)

Run 'demi forge --help' or 'demi vigil --help' for each layer's commands.
`

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		return 0
	}

	var bin string
	switch os.Args[1] {
	case "forge":
		bin = "demi-forge"
	case "vigil":
		bin = "demi-vigil"
	case "-h", "--help", "help":
		fmt.Print(usage)
		return 0
	case "-v", "--version", "version":
		fmt.Println("demi (Demigo dispatcher) " + version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "demi: unknown command %q\n\n%s", os.Args[1], usage)
		return 2
	}

	path, err := exec.LookPath(bin)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"demi: %s not found on PATH — install it with `make install` or "+
				"`go install github.com/ceasarb/demigo-tools/cmd/%s@latest`\n", bin, bin)
		return 1
	}

	cmd := exec.Command(path, os.Args[2:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "demi: %v\n", err)
		return 1
	}
	return 0
}
