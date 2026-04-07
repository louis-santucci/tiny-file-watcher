SERVER_BINARY := tfws
CLIENT_BINARY := tfw
SERVER_PKG    := ./server
CLIENT_PKG    := ./client
PROTO_DIR     := grpc
GEN_DIR       := gen/grpc
PROTO_FILE    := $(PROTO_DIR)/filewatcher.proto
GOPATH        := $(shell go env GOPATH)
INSTALL_DIR   := $(GOPATH)/bin

# Ensure GOPATH/bin is on PATH so protoc plugins and golangci-lint are found.
export PATH := $(INSTALL_DIR):$(PATH)

PROTOC_GEN_GO      := $(INSTALL_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(INSTALL_DIR)/protoc-gen-go-grpc
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

.PHONY: all help install-tools generate build build-client build-all install test lint clean \
        install-service uninstall-service enable-service disable-service \
        install-service-linux uninstall-service-linux enable-service-linux disable-service-linux

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
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## generate: regenerate Go code from .proto file
generate: $(PROTO_FILE) | $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	@mkdir -p $(GEN_DIR)
	@protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

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

$(PROTOC_GEN_GO):
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

$(PROTOC_GEN_GO_GRPC):
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

$(GOLANGCI_LINT):
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## install-service: install tfws binary and register it as a macOS LaunchAgent (starts at login)
install-service: install
	@mkdir -p $(LAUNCH_AGENTS_DIR)
	@sed -e 's|@@BINARY_PATH@@|$(INSTALL_DIR)/$(SERVER_BINARY)|g' \
	     -e 's|@@HOME@@|$(HOME)|g' \
	     $(PLIST_TEMPLATE) > $(PLIST_DEST)
	@launchctl load -w $(PLIST_DEST)
	@echo "tfws LaunchAgent installed and started."

## uninstall-service: stop and remove the tfws LaunchAgent
uninstall-service:
	@launchctl unload -w $(PLIST_DEST) 2>/dev/null || true
	@rm -f $(PLIST_DEST)
	@echo "tfws LaunchAgent removed."

## enable-service: enable (load) the tfws LaunchAgent
enable-service:
	@launchctl load -w $(PLIST_DEST)
	@echo "tfws LaunchAgent enabled."

## disable-service: disable (unload) the tfws LaunchAgent
disable-service:
	@launchctl unload -w $(PLIST_DEST)
	@echo "tfws LaunchAgent disabled."

## install-service-linux: install tfws and register as a systemd user service (starts at login)
install-service-linux: install
	@mkdir -p $(SYSTEMD_USER_DIR)
	@mkdir -p $(HOME)/.local/share/tfws
	@sed -e 's|@@BINARY_PATH@@|$(INSTALL_DIR)/$(SERVER_BINARY)|g' \
	     -e 's|@@HOME@@|$(HOME)|g' \
	     $(SERVICE_TEMPLATE) > $(SERVICE_DEST)
	@systemctl --user daemon-reload
	@systemctl --user enable --now $(SERVICE_NAME)
	@echo "tfws systemd user service installed and started."

## uninstall-service-linux: stop and remove the tfws systemd user service
uninstall-service-linux:
	@systemctl --user disable --now $(SERVICE_NAME) 2>/dev/null || true
	@rm -f $(SERVICE_DEST)
	@systemctl --user daemon-reload
	@echo "tfws systemd user service removed."

## enable-service-linux: enable (start) the tfws systemd user service
enable-service-linux:
	@systemctl --user enable --now $(SERVICE_NAME)
	@echo "tfws systemd user service enabled."

## disable-service-linux: disable (stop) the tfws systemd user service
disable-service-linux:
	@systemctl --user disable --now $(SERVICE_NAME)
	@echo "tfws systemd user service disabled."
