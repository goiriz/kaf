BINARY_NAME=kaf
BUILD_FLAGS=-ldflags="-s -w -X main.Version=$(shell git describe --tags --always || echo dev)"
STATIC_FLAGS=-tags "netgo,osusergo" -ldflags="-s -w -extldflags '-static' -X main.Version=$(shell git describe --tags --always || echo dev)"

.PHONY: all build test clean static lint help check

all: test build

build:
	go build $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd/kaf

static:
	CGO_ENABLED=0 go build $(STATIC_FLAGS) -o $(BINARY_NAME) ./cmd/kaf
	@$(MAKE) check

check:
	@file $(BINARY_NAME)
	@ldd $(BINARY_NAME) 2>&1 | grep "not a dynamic executable" || (echo "Warning: Binary is still dynamic!" && exit 1)

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
