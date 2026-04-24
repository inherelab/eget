## Eget — Makefile

APP     := eget
MAIN_DIR := ./cmd/eget
GOEXE = $(shell go env GOEXE)
BINARY  := $(APP)$(GOEXE)
VERSION ?= $(shell echo "$$(git for-each-ref refs/tags/ --count=1 --sort=-version:refname --format='%(refname:short)' | echo 'dev' 2>/dev/null)-$(REV)" | sed 's/^v//')

# Build metadata
GIT_HASH  := $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.GitHash=$(GIT_HASH) \
	-X 'main.BuildTime=$(BUILD_TIME)'

.PHONY: all build backend clean help

## all: build (default)
all: build

## build: build Go binary (current platform)
build:
	@echo "🐹 Building Go binary ($(VERSION) @ $(GIT_HASH))..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN_DIR)
	@echo "✅ Binary: $(BINARY) ($$(du -sh $(BINARY) | cut -f1))"

## install: install Go binary to $GOPATH/bin
install:
	@go install -ldflags "$(LDFLAGS)" $(MAIN_DIR)
	@echo "✅ Installed to GOPATH/bin"

## run: build and run with current directory
run: build
	./$(BINARY)

# ─── Cross Compilation ────────────────────────────────────────────────────────

DIST_DIR := dist

## build-all: cross-compile for all platforms
build-all: build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows

## build-linux: compile for Linux amd64
build-linux:
	@echo "🐧 linux/amd64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-amd64 $(MAIN_DIR)
	@echo "   → $(DIST_DIR)/$(APP)-linux-amd64"

## build-linux-arm64: compile for Linux arm64
build-linux-arm64:
	@echo "🐧 linux/arm64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-arm64 $(MAIN_DIR)
	@echo "   → $(DIST_DIR)/$(APP)-linux-arm64"

## build-darwin: compile for macOS amd64
build-darwin:
	@echo "🍎 darwin/amd64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-darwin-amd64 $(MAIN_DIR)
	@echo "   → $(DIST_DIR)/$(APP)-darwin-amd64"

## build-darwin-arm64: compile for macOS Apple Silicon
build-darwin-arm64:
	@echo "🍎 darwin/arm64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-darwin-arm64 $(MAIN_DIR)
	@echo "   → $(DIST_DIR)/$(APP)-darwin-arm64"

## build-windows: compile for Windows amd64
build-windows:
	@echo "🪟 windows/amd64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-windows-amd64.exe $(MAIN_DIR)
	@echo "   → $(DIST_DIR)/$(APP)-windows-amd64.exe"

## clean: remove build artifacts
clean:
	@rm -f $(BINARY)
	@rm -rf $(DIST_DIR)
	@echo "🧹 Cleaned"

## help: show this help
help:
	@echo "Skillc Build System"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
