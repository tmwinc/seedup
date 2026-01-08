.PHONY: build test lint clean install

# Build the binary
build:
	go build -o bin/seedup ./cmd/seedup

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	go vet ./...
	@if command -v staticcheck > /dev/null; then staticcheck ./...; fi

# Clean build artifacts
clean:
	rm -rf bin/

# Install the binary
install:
	go install ./cmd/seedup

# Format code
fmt:
	go fmt ./...
	@if command -v gofumpt > /dev/null; then gofumpt -w .; fi

# Run all checks
check: lint test

# Build for all platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/seedup-linux-amd64 ./cmd/seedup
	GOOS=darwin GOARCH=amd64 go build -o bin/seedup-darwin-amd64 ./cmd/seedup
	GOOS=darwin GOARCH=arm64 go build -o bin/seedup-darwin-arm64 ./cmd/seedup
