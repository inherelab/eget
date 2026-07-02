## Eget — Makefile

APP     := eget
MAIN_DIR := ./cmd/eget
GOEXE = $(shell go env GOEXE)
BINARY  := $(APP)$(GOEXE)

# Build metadata
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S)
GIT_HASH  := $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev-$(GIT_HASH)")

LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.GitHash=$(GIT_HASH) \
	-X 'main.BuildTime=$(BUILD_TIME)'

.PHONY: all build backend clean clean-dist help latest

## all: build (default)
all: build

## build: build Go binary (current platform)
build:
	@echo "🐹 Building Go binary ($(VERSION) @ $(GIT_HASH))..."
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN_DIR)
	@echo "📦 Compressing binary..."
	@upx -6 --no-progress $(BINARY)
	@echo "✅ Binary: $(BINARY) ($$(du -sh $(BINARY) | cut -f1))"

## install: install Go binary to $GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(MAIN_DIR)
	upx -6 --no-progress $(GOPATH)/bin/$(BINARY)
	@echo "✅ Installed to GOPATH/bin"

## run: build and run with current directory
run: build
	./$(BINARY)

# ─── Cross Compilation ────────────────────────────────────────────────────────

DIST_DIR := dist
DESCRIPTION := "Easy install and download tools from GitHub, SourceForge and more"
WINDOWS_RESOURCE := $(MAIN_DIR)/resource_windows_amd64.syso
WINDOWS_VERSIONINFO := $(DIST_DIR)/versioninfo.json
GOVERSIONINFO := go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0

## build-all: cross-compile for all platforms
build-all: clean-dist dump-info build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows latest-yaml
	ls -lh $(DIST_DIR)

## dump-info: dump build info
dump-info:
	@echo "Build Info:"
	@echo "  VERSION: $(VERSION)"
	@echo "  GIT_HASH: $(GIT_HASH)"
	@echo "  BUILD_TIME: $(BUILD_TIME)"

## latest-yaml: generate latest.yaml release metadata
latest-yaml:
	@mkdir -p $(DIST_DIR)
	@{ \
		echo "name: $(APP)"; \
		echo "version: $(VERSION)"; \
		echo "released_at: $(BUILD_TIME)"; \
		echo "description: $(DESCRIPTION)"; \
	} > $(DIST_DIR)/latest.yaml
	@echo "   → $(DIST_DIR)/latest.yaml"

## build-linux: compile for Linux amd64
build-linux:
	@echo "🐧 linux/amd64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-amd64 $(MAIN_DIR)
	upx -6 --no-progress $(DIST_DIR)/$(APP)-linux-amd64
	chmod +x $(DIST_DIR)/$(APP)-linux-amd64
	@echo "   → $(DIST_DIR)/$(APP)-linux-amd64"

## build-linux-arm64: compile for Linux arm64
build-linux-arm64:
	@echo "🐧 linux/arm64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-linux-arm64 $(MAIN_DIR)
	upx -6 --no-progress $(DIST_DIR)/$(APP)-linux-arm64
	chmod +x $(DIST_DIR)/$(APP)-linux-arm64
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
	# upx -6 --no-progress $(DIST_DIR)/$(APP)-darwin-arm64 # 压缩有问题在 macos 12+
	@echo "   → $(DIST_DIR)/$(APP)-darwin-arm64"

## build-windows: compile for Windows amd64
build-windows:
	@echo "🪟 windows/amd64..."
	@mkdir -p $(DIST_DIR)
	@set -e; trap 'rm -f $(WINDOWS_RESOURCE) $(WINDOWS_VERSIONINFO)' EXIT; \
	printf '{}\n' > $(WINDOWS_VERSIONINFO); \
	version_nums=$$(printf '%s\n' "$(VERSION)" | sed -E 's/^v?([0-9]+)\.([0-9]+)\.([0-9]+).*/\1 \2 \3/; t; s/.*/0 0 0/'); \
	set -- $$version_nums; \
	$(GOVERSIONINFO) -64 -o $(WINDOWS_RESOURCE) \
		-ver-major $$1 -ver-minor $$2 -ver-patch $$3 -ver-build 0 \
		-product-ver-major $$1 -product-ver-minor $$2 -product-ver-patch $$3 -product-ver-build 0 \
		-file-version "$(VERSION)" -product-version "$(VERSION)" \
		-product-name "$(APP)" -internal-name "$(APP)" -original-name "$(APP).exe" \
		-description $(DESCRIPTION) $(WINDOWS_VERSIONINFO); \
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP)-windows-amd64.exe $(MAIN_DIR)
	upx -6 --no-progress $(DIST_DIR)/$(APP)-windows-amd64.exe
	@echo "   → $(DIST_DIR)/$(APP)-windows-amd64.exe"

.PHONY: release
release: build-all ## Create release archives for all platforms TODO 还未启用的
	@echo "Creating release archives..."
	@mkdir -p release
	@cd $(DIST_DIR) && \
	tar -czf ../release/$(APP)-linux-amd64.tar.gz $(APP)-linux-amd64; \
	tar -czf ../release/$(APP)-linux-arm64.tar.gz $(APP)-linux-arm64; \
	tar -czf ../release/$(APP)-darwin-amd64.tar.gz $(APP)-darwin-amd64; \
	tar -czf ../release/$(APP)-darwin-arm64.tar.gz $(APP)-darwin-arm64; \
	zip ../release/$(APP)-windows-amd64.zip $(APP)-windows-amd64.exe;
	@echo "Release archives created in release/"

## clean: remove build artifacts
clean:
	@rm -f $(BINARY)
	@rm -f $(WINDOWS_RESOURCE) $(WINDOWS_VERSIONINFO)
	@rm -rf $(DIST_DIR)
	@echo "🧹 Cleaned"

## clean-dist: remove old dist files
clean-dist:
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@echo "🧹 Cleaned $(DIST_DIR)"

## help: show this help
help:
	@echo "Skillc Build System"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
