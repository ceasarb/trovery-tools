VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MODULE  := github.com/ceasarb/demigo-tools

# Each binary injects its version into its own package.
DEMI_FORGE_LDFLAGS := -s -w -X $(MODULE)/internal/forge/cli.Version=$(VERSION)
DEMI_VIGIL_LDFLAGS := -s -w -X $(MODULE)/internal/vigil/cli.Version=$(VERSION)
DEMI_LDFLAGS       := -s -w -X main.version=$(VERSION)

BIN := bin

.PHONY: build build-forge build-vigil build-demi install install-forge install-vigil install-demi test vet ci clean

build: build-forge build-vigil build-demi

build-forge:
	go build -ldflags "$(DEMI_FORGE_LDFLAGS)" -o $(BIN)/demi-forge ./cmd/demi-forge

build-vigil:
	go build -ldflags "$(DEMI_VIGIL_LDFLAGS)" -o $(BIN)/demi-vigil ./cmd/demi-vigil

# The thin dispatcher — routes `demi forge …` / `demi vigil …` to the two binaries.
build-demi:
	go build -ldflags "$(DEMI_LDFLAGS)" -o $(BIN)/demi ./cmd/demi

install: install-forge install-vigil install-demi

install-forge:
	go install -ldflags "$(DEMI_FORGE_LDFLAGS)" ./cmd/demi-forge

install-vigil:
	go install -ldflags "$(DEMI_VIGIL_LDFLAGS)" ./cmd/demi-vigil

install-demi:
	go install -ldflags "$(DEMI_LDFLAGS)" ./cmd/demi

test:
	go test ./...

vet:
	go vet ./...

# Same checks CI runs, runnable locally (nothing is pushed).
ci: vet build test

clean:
	rm -rf $(BIN)
