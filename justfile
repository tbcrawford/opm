# opm — OpenCode Profile Manager
# All commands are single words.

set dotenv-load := true
set quiet := true

binary := "opm"
module := `go list -m`

# List available recipes (default)
help:
    @just --list

# ── Lifecycle ─────────────────────────────────────────────────────────────────

# Fetch and tidy dependencies
deps:
    go mod tidy
    go mod download

# Compile the binary
assemble:
    go build -o {{ binary }} .

# Compile with version/commit metadata injected
build:
    go build \
        -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
        -o {{ binary }} .

# Run all tests
test:
    go test ./...

# Run tests with verbose output
testv:
    go test -v ./...

# Run tests with race detector
race:
    go test -race ./...

# Generate test coverage report
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Lint and vet the codebase
check:
    go vet ./...
    golangci-lint run

# Format Go source files
fmt:
    gofmt -w .

# Run check + test
verify: check test

# Build cross-platform release archives via GoReleaser
release:
    goreleaser release --clean

# Dry-run release build without publishing
snapshot:
    goreleaser release --snapshot --clean

# Remove build artifacts
clean:
    rm -f {{ binary }} coverage.out coverage.html
    rm -rf dist/

# ── Development ───────────────────────────────────────────────────────────────

# Build and run with arguments (e.g. `just run context ls`)
run *args:
    go run . {{ args }}

# Install the binary to $GOPATH/bin
install:
    go install .

# Remove the installed binary from $GOPATH/bin
uninstall:
    rm -f `go env GOPATH`/bin/{{ binary }}

# Watch for changes and re-run tests (requires watchexec)
watch:
    watchexec --exts go -- go test ./...

# Regenerate the README quick-start demo GIF from its VHS tape
demo-readme:
    mkdir -p assets/demo
    vhs demo/readme-quick-start.tape
