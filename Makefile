# Makefile for respec

# Metadata
BINARY      := respec
VERSION     ?= $(shell git describe --tags --always --dirty)
COMMIT      := $(shell git rev-parse --short HEAD)
DATE        := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Platforms to build for (OS-ARCH)
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

# Output directory
DIST := dist

# Installation directory
INSTALL_PREFIX ?= $(HOME)/.local
INSTALL_DIR := $(INSTALL_PREFIX)/bin

# Go module name from go.mod
MODULE := $(shell go list -m)

# ldflags to inject version info into the binary
LDFLAGS := -s -w -buildid= -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(DATE)'

.PHONY: all build clean release version install uninstall debug test lint fmt tidy deps dev-setup help

# Default target builds for the current OS/architecture
build: $(DIST)/$(BINARY)

# Local build target
$(DIST)/$(BINARY):
	@mkdir -p $(DIST)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY) ./cmd/respec

# Debug build - builds and installs to local system
debug: build install
	@echo "✅ Debug build installed to $(INSTALL_DIR)/$(BINARY)"
	@echo "💡 Run with: $(BINARY) --help"

# Install the binary to local system
install:
	@echo "📦 Installing $(BINARY) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(DIST)/$(BINARY) $(INSTALL_DIR)/
	@chmod +x $(INSTALL_DIR)/$(BINARY)
	@echo "✅ Installed $(BINARY) to $(INSTALL_DIR)"
	@echo "💡 Make sure $(INSTALL_DIR) is in your PATH"
	@echo "   export PATH=\"$(INSTALL_DIR):\$$PATH\""

# Uninstall the binary from local system
uninstall:
	@echo "🗑️ Removing $(BINARY) from $(INSTALL_DIR)..."
	@rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "✅ Uninstalled $(BINARY)"

# Cross-platform builds for all defined platforms
all: $(PLATFORMS:%=$(DIST)/$(BINARY)-%)

$(DIST)/$(BINARY)-%:
	@platform="$*"; \
	os=$${platform%-*}; arch=$${platform#*-}; \
	outfile="$(DIST)/$(BINARY)-$$os-$$arch"; \
	if [ "$$os" = "windows" ]; then outfile="$$outfile.exe"; fi; \
	mkdir -p $(DIST); \
	echo "--> Building for $$os/$$arch..."; \
	GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
	go build -trimpath -ldflags="$(LDFLAGS)" -o "$$outfile" ./cmd/respec

# Clean the dist directory
clean:
	@rm -rf $(DIST)

# The release target builds all platforms, zips the artifacts, and creates a checksum file.
release: clean all
	@echo "--> Zipping release artifacts...";
	@for platform in $(PLATFORMS); do \
		os=$${platform%-*}; arch=$${platform#*-}; \
		base="$(DIST)/$(BINARY)-$$os-$$arch"; \
		out="$$base"; \
		if [ "$$os" = "windows" ]; then out="$$base.exe"; fi; \
		zipfile="$$base.zip"; \
		zip -j "$$zipfile" "$$out"; \
	done
	@echo "--> Generating checksums...";
	@cd $(DIST) && (command -v sha256sum >/dev/null && sha256sum *.zip > SHA256SUMS || shasum -a 256 *.zip > SHA256SUMS)

# Show version info
version:
	@echo "Version:   $(VERSION)"
	@echo "Commit:    $(COMMIT)"
	@echo "BuildDate: $(DATE)"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Run linter
lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️ golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
	fi

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Tidy dependencies
tidy:
	@echo "🧹 Tidying dependencies..."
	go mod tidy

# Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	go mod download

# Development setup
dev-setup: deps
	@echo "🛠️ Setting up development environment..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "📦 Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "✅ Development environment ready!"
	@echo "💡 Run 'make debug' to build and install locally"

# Show help
help:
	@echo "respec - OpenAPI Generator for Go"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build      Build the binary for the current platform"
	@echo "  debug      Build and install to local system ($(INSTALL_DIR))"
	@echo "  install    Install the built binary to local system"
	@echo "  uninstall  Remove the binary from local system"
	@echo "  all        Cross-compile for all platforms"
	@echo "  release    Cross-compile for all platforms and zip outputs"
	@echo "  clean      Remove build artifacts"
	@echo "  test       Run tests"
	@echo "  lint       Run linter"
	@echo "  fmt        Format code"
	@echo "  tidy       Tidy dependencies"
	@echo "  deps       Download dependencies"
	@echo "  dev-setup  Setup development environment"
	@echo "  version    Show version metadata"
	@echo "  help       Show this help message"
	@echo ""