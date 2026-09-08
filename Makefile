SERVER_BINARY := tfws
CLIENT_BINARY := tfw
SERVER_PKG    := ./server
CLIENT_PKG    := ./client
PROTO_DIR     := grpc
GEN_DIR       := gen/grpc
PROTO_FILE    := $(PROTO_DIR)/filewatcher/filewatcher.proto
GOPATH        := $(shell go env GOPATH)
INSTALL_DIR   := $(GOPATH)/bin

# Ensure GOPATH/bin is on PATH so protoc plugins and golangci-lint are found.
export PATH := $(INSTALL_DIR):$(PATH)

GOLANGCI_LINT      := $(INSTALL_DIR)/golangci-lint

LAUNCH_AGENTS_DIR := $(HOME)/Library/LaunchAgents
PLIST_LABEL       := louissantucci.tfws
PLIST_TEMPLATE    := launchd/$(PLIST_LABEL).plist
PLIST_DEST        := $(LAUNCH_AGENTS_DIR)/$(PLIST_LABEL).plist

SYSTEMD_USER_DIR  := $(HOME)/.config/systemd/user
SERVICE_NAME      := tfws.service
SERVICE_TEMPLATE  := systemd/$(SERVICE_NAME)
SERVICE_DEST      := $(SYSTEMD_USER_DIR)/$(SERVICE_NAME)

IOS_GEN_DIR := tiny-file-watcher-app/tiny-file-watcher-app/Generated

.PHONY: all help install-tools generate build build-client build-all install test lint clean tag tag-major tag-minor tag-patch

## help: list all available make rules with descriptions
help:
	@echo "Usage: make <rule>"
	@echo ""
	@awk '/^## [a-zA-Z]/ { \
		split($$0, a, ": "); \
		rule = substr(a[1], 4); \
		desc = a[2]; \
		for (i = 3; i <= length(a); i++) desc = desc ": " a[i]; \
		printf "  %-20s %s\n", rule, desc \
	}' $(MAKEFILE_LIST)

all: generate build

## install-tools: install protoc plugins and golangci-lint
install-tools:
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## generate: regenerate Go code from .proto file
generate: $(PROTO_FILE)
	@buf generate

## build: compile the server binary (tfws)
build: generate build-client build-server

## build-client: compile the CLI client binary (tfw)
build-client:
	@go build -o $(CLIENT_BINARY) $(CLIENT_PKG)

## build-all: compile both server and client binaries
build-server: generate
	@go build -o $(SERVER_BINARY) $(SERVER_PKG)

## install: build and copy binary to GOPATH/bin
install: build
	@install -m 0755 $(SERVER_BINARY) $(INSTALL_DIR)/$(SERVER_BINARY)
	@install -m 0755 $(CLIENT_BINARY) $(INSTALL_DIR)/$(CLIENT_BINARY)

## test: run all tests
test: generate
	@go test -tags integration -race -v -timeout 30s ./...

## lint: run golangci-lint
lint: generate | $(GOLANGCI_LINT)
	@golangci-lint run ./...

## clean: remove built binaries and generated proto files
clean:
	@rm -f $(SERVER_BINARY) $(CLIENT_BINARY)
	@rm -f $(GEN_DIR)/*.pb.go $(GEN_DIR)/*_grpc.pb.go

$(GOLANGCI_LINT):
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## tag: tag existing patch + current branch if not master
tag:
	@td . --docker -t

## tag-patch: tag patch on git + docker
tag-patch:
	@td . patch --docker -t

## tag-minor: tag minor on git + docker
tag-minor:
	@td . minor --docker -t

## tag-major: tag major on git + docker
tag-major:
	@td . major --docker -t
