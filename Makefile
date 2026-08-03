VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MODULE  := github.com/ceasarb/trovery-tools

# Each binary injects its version into its own package.
TROVE_FORGE_LDFLAGS := -s -w -X $(MODULE)/internal/forge/cli.Version=$(VERSION)
TROVE_VIGIL_LDFLAGS := -s -w -X $(MODULE)/internal/vigil/cli.Version=$(VERSION)
TROVE_LDFLAGS       := -s -w -X main.version=$(VERSION)

BIN := bin

.PHONY: build build-forge build-vigil build-trove install install-forge install-vigil install-trove test vet ci clean

build: build-forge build-vigil build-trove

build-forge:
	go build -ldflags "$(TROVE_FORGE_LDFLAGS)" -o $(BIN)/trove-forge ./cmd/trove-forge

build-vigil:
	go build -ldflags "$(TROVE_VIGIL_LDFLAGS)" -o $(BIN)/trove-vigil ./cmd/trove-vigil

# The thin dispatcher — routes `trove forge …` / `trove vigil …` to the two binaries.
build-trove:
	go build -ldflags "$(TROVE_LDFLAGS)" -o $(BIN)/trove ./cmd/trove

install: install-forge install-vigil install-trove

install-forge:
	go install -ldflags "$(TROVE_FORGE_LDFLAGS)" ./cmd/trove-forge

install-vigil:
	go install -ldflags "$(TROVE_VIGIL_LDFLAGS)" ./cmd/trove-vigil

install-trove:
	go install -ldflags "$(TROVE_LDFLAGS)" ./cmd/trove

test:
	go test ./...

vet:
	go vet ./...

# Same checks CI runs, runnable locally (nothing is pushed).
ci: vet build test

clean:
	rm -rf $(BIN)
