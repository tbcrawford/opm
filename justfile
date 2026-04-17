# opm — OpenCode Profile Manager
# Development commands modeled after Gradle's core task lifecycle.
# All commands are single words.

set dotenv-load := true

binary := "opm"
module := `go list -m`

# List available recipes (default)
help:
    @just --list

# ── Lifecycle ─────────────────────────────────────────────────────────────────

# Fetch and tidy dependencies (like Gradle's `dependencies`)
deps:
    go mod tidy
    go mod download

# Compile the binary (like Gradle's `assemble`)
assemble:
    go build -o {{ binary }} .

# Compile with version/commit metadata injected (like Gradle's `build`)
build:
    go build \
        -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
        -o {{ binary }} .

# Run all tests (like Gradle's `test`)
test:
    go test ./...

# Run tests with verbose output
testv:
    go test -v ./...

# Run tests with race detector
race:
    go test -race ./...

# Generate test coverage report (like Gradle's `jacocoTestReport`)
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Lint and vet the codebase (like Gradle's `check`)
check:
    go vet ./...
    golangci-lint run

# Format Go source files
fmt:
    gofmt -w .

# Run check + test (like Gradle's `verify`)
verify: check test

# Build cross-platform release archives via GoReleaser (like Gradle's `publish`)
release:
    goreleaser release --clean

# Dry-run release build without publishing (like Gradle's `publishToMavenLocal`)
snapshot:
    goreleaser release --snapshot --clean

# Remove build artifacts (like Gradle's `clean`)
clean:
    rm -f {{ binary }} coverage.out coverage.html
    rm -rf dist/

# ── Development ───────────────────────────────────────────────────────────────

# Build and run with arguments (e.g. `just run context ls`)
run *args:
    go run . {{ args }}

# Install the binary to $GOPATH/bin (like Gradle's `install`)
install:
    go install .

# Watch for changes and re-run tests (requires watchexec)
watch:
    watchexec --exts go -- go test ./...
