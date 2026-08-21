GO ?= go
PNPM ?= pnpm
PROTOC ?= protoc
GOLANGCI_LINT ?= golangci-lint

BINARY ?= farmbot

.PHONY: build build-web sync-web-assets gen-proto test lint docker release release-windows release-linux release-macos release-target clean

VERSION ?= $(shell node -p "require('./package.json').version")
RELEASE_DIR ?= dist
RELEASE_LDFLAGS ?= -s -w -X main.version=$(VERSION)

ifeq ($(OS),Windows_NT)
MKDIR_RELEASE = if not exist "$(RELEASE_DIR)" mkdir "$(RELEASE_DIR)"
define GO_RELEASE
	set "CGO_ENABLED=0"&& set "GOOS=$(GOOS)"&& set "GOARCH=$(GOARCH)"&& $(GO) build -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o "$(RELEASE_DIR)/farmbot-$(VERSION)-$(GOOS)-$(GOARCH)$(EXT)" ./cmd/farmbot
endef
else
MKDIR_RELEASE = mkdir -p "$(RELEASE_DIR)"
define GO_RELEASE
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o "$(RELEASE_DIR)/farmbot-$(VERSION)-$(GOOS)-$(GOARCH)$(EXT)" ./cmd/farmbot
endef
endif

build:
	$(GO) build -o $(BINARY) ./cmd/farmbot

build-web:
	$(PNPM) -C web build
	$(GO) run ./internal/tools/syncassets

gen-proto:
	$(GO) run ./internal/tools/protogen

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run --config .golangci.yml
	$(PNPM) -C web exec eslint "src/**/*.{ts,vue}"

docker:
	docker compose build

# Release uses only pure-Go dependencies so every target is statically linked
# and can be built from one checkout without a platform-specific toolchain.
release: build-web release-windows release-linux release-macos

release-windows:
	$(MAKE) release-target GOOS=windows GOARCH=amd64 EXT=.exe

release-linux:
	$(MAKE) release-target GOOS=linux GOARCH=amd64 EXT=

release-macos:
	$(MAKE) release-target GOOS=darwin GOARCH=amd64 EXT=
	$(MAKE) release-target GOOS=darwin GOARCH=arm64 EXT=

release-target:
	@$(MKDIR_RELEASE)
	$(GO_RELEASE)

clean:
	$(GO) clean
