GO ?= go
PNPM ?= pnpm
PROTOC ?= protoc
GOLANGCI_LINT ?= golangci-lint

BINARY ?= farmbot

.PHONY: build build-web gen-proto test lint docker clean

build:
	$(GO) build -o $(BINARY) ./cmd/farmbot

build-web:
	$(PNPM) -C web build

gen-proto:
	$(PROTOC) --proto_path=proto --go_out=internal/game/pb proto/*.proto

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run --config .golangci.yml
	$(PNPM) -C web exec eslint "src/**/*.{ts,vue}"

docker:
	docker compose build

clean:
	$(GO) clean
