BINARY_NAME=kaf
VERSION=1.0.0
BUILD_FLAGS=-ldflags="-s -w"

.PHONY: all build test clean static lint help

all: test build

build:
	go build $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd/kaf

static:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd/kaf

test:
	go test -v ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		go fmt ./...; \
		go vet ./...; \
	fi

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out

help:
	@echo "Available targets:"
	@echo "  build   - Build the binary (dynamic)"
	@echo "  static  - Build a fully static binary (portable)"
	@echo "  test    - Run all unit tests"
	@echo "  cover   - Run tests and show coverage report"
	@echo "  lint    - Run linter (golangci-lint or go fmt/vet)"
	@echo "  clean   - Remove binary and coverage files"
